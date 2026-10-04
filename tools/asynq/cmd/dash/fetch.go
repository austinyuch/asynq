// Copyright 2022 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package dash

import (
	"sort"
	"sync"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
)

type fetcher interface {
	// Fetch retries data required by the given state of the dashboard.
	Fetch(state *State)
}

// fetchContext is captured by the event-loop owner, never by a worker.
// Polls share an epoch; leaving and returning to a view creates a new epoch.
type fetchContext struct {
	view                 viewType
	queue, group, taskID string
	taskState            asynq.TaskState
	page, pageSize       int
	epoch                uint64
}
type fetchResult[T any] struct {
	request fetchContext
	value   T
}

func captureFetchContext(state *State, screen tcell.Screen) fetchContext {
	next := fetchContext{view: state.view}
	if state.view == viewTypeQueueDetails {
		if state.selectedQueue != nil {
			next.queue = state.selectedQueue.Queue
		}
		if state.selectedGroup != nil {
			next.group = state.selectedGroup.Group
		}
		next.taskState, next.taskID = state.taskState, state.taskID
		next.page, next.pageSize = state.pageNum, taskPageSize(screen)
	}
	previous := state.request
	next.epoch = previous.epoch
	if next != previous {
		next.epoch++
	}
	state.request = next
	return next
}

type contextualFetcher struct {
	next   fetcher
	screen tcell.Screen
}

func (f contextualFetcher) Fetch(state *State) {
	captureFetchContext(state, f.screen)
	f.next.Fetch(state)
}

// Storage helpers publish one payload or one error into private buffered
// channels. This adapter preserves their storage contract and tags both paths
// with the immutable admission identity before public result publication.
func launchFetch[T any](f *dataFetcher, request fetchContext, result chan<- fetchResult[T], work func(chan<- T, chan<- error)) {
	f.launch(func() {
		values, failures := make(chan T, 1), make(chan error, 1)
		work(values, failures)
		select {
		case value := <-values:
			publish(result, fetchResult[T]{request, value}, f.done)
		case err := <-failures:
			publish(f.errorCh, fetchResult[error]{request, err}, f.done)
		default: // Shutdown prevented the storage helper from publishing.
		}
	})
}

type dataFetcher struct {
	inspector *asynq.Inspector
	opts      Options
	s         tcell.Screen

	errorCh  chan<- fetchResult[error]
	queueCh  chan<- fetchResult[*asynq.QueueInfo]
	taskCh   chan<- fetchResult[*asynq.TaskInfo]
	queuesCh chan<- fetchResult[[]*asynq.QueueInfo]
	groupsCh chan<- fetchResult[[]*asynq.GroupInfo]
	tasksCh  chan<- fetchResult[[]*asynq.TaskInfo]

	done    <-chan struct{}
	slots   chan struct{}
	workers sync.WaitGroup
}

// Fetch admission is confined to the event loop. Saturated refreshes are
// best-effort: skip this operation and retry on a later refresh. Never block
// the result consumer waiting for a worker slot.
func (f *dataFetcher) launch(work func()) {
	select {
	case <-f.done:
		return
	default:
	}
	if f.slots != nil {
		select {
		case f.slots <- struct{}{}:
		default:
			return
		}
	}
	f.workers.Add(1)
	go func() {
		defer f.workers.Done()
		if f.slots != nil {
			defer func() { <-f.slots }()
		}
		work()
	}()
}

// Called only after the event loop stops admitting work.
func (f *dataFetcher) wait() { f.workers.Wait() }

func fetchDone(done []<-chan struct{}) <-chan struct{} {
	if len(done) == 0 {
		return nil
	}
	return done[0]
}

// Never close result channels: senders can finish after shutdown begins.
func publish[T any](ch chan<- T, value T, done <-chan struct{}) {
	select {
	case <-done:
		return
	default:
	}
	select {
	case ch <- value:
	case <-done:
	}
}

func (f *dataFetcher) Fetch(state *State) {
	request := captureFetchContext(state, f.s)
	switch request.view {
	case viewTypeQueues:
		launchFetch(f, request, f.queuesCh, func(values chan<- []*asynq.QueueInfo, failures chan<- error) {
			fetchQueues(f.inspector, values, failures, f.opts, f.done)
		})
	case viewTypeQueueDetails:
		if shouldShowGroupTable(state) {
			launchFetch(f, request, f.groupsCh, func(values chan<- []*asynq.GroupInfo, failures chan<- error) {
				fetchGroups(f.inspector, request.queue, values, failures, f.done)
			})
		} else if request.taskState == asynq.TaskStateAggregating {
			launchFetch(f, request, f.tasksCh, func(values chan<- []*asynq.TaskInfo, failures chan<- error) {
				fetchAggregatingTasks(f.inspector, request.queue, request.group, request.pageSize, request.page, values, failures, f.done)
			})
		} else {
			launchFetch(f, request, f.tasksCh, func(values chan<- []*asynq.TaskInfo, failures chan<- error) {
				fetchTasks(f.inspector, request.queue, request.taskState, request.pageSize, request.page, values, failures, f.done)
			})
		}
		launchFetch(f, request, f.queueCh, func(values chan<- *asynq.QueueInfo, failures chan<- error) {
			fetchQueueInfo(f.inspector, request.queue, values, failures, f.done)
		})
		if request.taskID != "" {
			launchFetch(f, request, f.taskCh, func(values chan<- *asynq.TaskInfo, failures chan<- error) {
				fetchTaskInfo(f.inspector, request.queue, request.taskID, values, failures, f.done)
			})
		}
	}
}

func fetchQueues(i *asynq.Inspector, queuesCh chan<- []*asynq.QueueInfo, errorCh chan<- error, opts Options, done ...<-chan struct{}) {
	queues, err := i.Queues()
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	sort.Strings(queues)
	var res []*asynq.QueueInfo
	for _, q := range queues {
		info, err := i.GetQueueInfo(q)
		if err != nil {
			publish(errorCh, err, fetchDone(done))
			return
		}
		res = append(res, info)
	}
	publish(queuesCh, res, fetchDone(done))
}

func fetchQueueInfo(i *asynq.Inspector, qname string, queueCh chan<- *asynq.QueueInfo, errorCh chan<- error, done ...<-chan struct{}) {
	q, err := i.GetQueueInfo(qname)
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(queueCh, q, fetchDone(done))
}

func fetchGroups(i *asynq.Inspector, qname string, groupsCh chan<- []*asynq.GroupInfo, errorCh chan<- error, done ...<-chan struct{}) {
	groups, err := i.Groups(qname)
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(groupsCh, groups, fetchDone(done))
}

func fetchAggregatingTasks(i *asynq.Inspector, qname, group string, pageSize, pageNum int,
	tasksCh chan<- []*asynq.TaskInfo, errorCh chan<- error, done ...<-chan struct{}) {
	tasks, err := i.ListAggregatingTasks(qname, group, asynq.PageSize(pageSize), asynq.Page(pageNum))
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(tasksCh, tasks, fetchDone(done))
}

func fetchTasks(i *asynq.Inspector, qname string, taskState asynq.TaskState, pageSize, pageNum int,
	tasksCh chan<- []*asynq.TaskInfo, errorCh chan<- error, done ...<-chan struct{}) {
	var (
		tasks []*asynq.TaskInfo
		err   error
	)
	opts := []asynq.ListOption{asynq.PageSize(pageSize), asynq.Page(pageNum)}
	switch taskState {
	case asynq.TaskStateActive:
		tasks, err = i.ListActiveTasks(qname, opts...)
	case asynq.TaskStatePending:
		tasks, err = i.ListPendingTasks(qname, opts...)
	case asynq.TaskStateScheduled:
		tasks, err = i.ListScheduledTasks(qname, opts...)
	case asynq.TaskStateRetry:
		tasks, err = i.ListRetryTasks(qname, opts...)
	case asynq.TaskStateArchived:
		tasks, err = i.ListArchivedTasks(qname, opts...)
	case asynq.TaskStateCompleted:
		tasks, err = i.ListCompletedTasks(qname, opts...)
	}
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(tasksCh, tasks, fetchDone(done))
}

func fetchTaskInfo(i *asynq.Inspector, qname, taskID string, taskCh chan<- *asynq.TaskInfo, errorCh chan<- error, done ...<-chan struct{}) {
	info, err := i.GetTaskInfo(qname, taskID)
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(taskCh, info, fetchDone(done))
}
