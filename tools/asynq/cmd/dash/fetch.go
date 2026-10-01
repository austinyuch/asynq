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

type dataFetcher struct {
	inspector *asynq.Inspector
	opts      Options
	s         tcell.Screen

	errorCh  chan<- error
	queueCh  chan<- *asynq.QueueInfo
	taskCh   chan<- *asynq.TaskInfo
	queuesCh chan<- []*asynq.QueueInfo
	groupsCh chan<- []*asynq.GroupInfo
	tasksCh  chan<- []*asynq.TaskInfo

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
	switch state.view {
	case viewTypeQueues:
		f.fetchQueues()
	case viewTypeQueueDetails:
		if shouldShowGroupTable(state) {
			f.fetchGroups(state.selectedQueue.Queue)
		} else if state.taskState == asynq.TaskStateAggregating {
			f.fetchAggregatingTasks(state.selectedQueue.Queue, state.selectedGroup.Group, taskPageSize(f.s), state.pageNum)
		} else {
			f.fetchTasks(state.selectedQueue.Queue, state.taskState, taskPageSize(f.s), state.pageNum)
		}
		// if the task modal is open, additionally fetch the selected task's info
		if state.taskID != "" {
			f.fetchTaskInfo(state.selectedQueue.Queue, state.taskID)
		}
	}
}

func (f *dataFetcher) fetchQueues() {
	var (
		inspector = f.inspector
		queuesCh  = f.queuesCh
		errorCh   = f.errorCh
		opts      = f.opts
	)
	f.launch(func() { fetchQueues(inspector, queuesCh, errorCh, opts, f.done) })
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

func (f *dataFetcher) fetchGroups(qname string) {
	var (
		i        = f.inspector
		groupsCh = f.groupsCh
		errorCh  = f.errorCh
		queueCh  = f.queueCh
	)
	f.launch(func() { fetchGroups(i, qname, groupsCh, errorCh, f.done) })
	f.launch(func() { fetchQueueInfo(i, qname, queueCh, errorCh, f.done) })
}

func fetchGroups(i *asynq.Inspector, qname string, groupsCh chan<- []*asynq.GroupInfo, errorCh chan<- error, done ...<-chan struct{}) {
	groups, err := i.Groups(qname)
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(groupsCh, groups, fetchDone(done))
}

func (f *dataFetcher) fetchAggregatingTasks(qname, group string, pageSize, pageNum int) {
	var (
		i       = f.inspector
		tasksCh = f.tasksCh
		errorCh = f.errorCh
		queueCh = f.queueCh
	)
	f.launch(func() { fetchAggregatingTasks(i, qname, group, pageSize, pageNum, tasksCh, errorCh, f.done) })
	f.launch(func() { fetchQueueInfo(i, qname, queueCh, errorCh, f.done) })
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

func (f *dataFetcher) fetchTasks(qname string, taskState asynq.TaskState, pageSize, pageNum int) {
	var (
		i       = f.inspector
		tasksCh = f.tasksCh
		errorCh = f.errorCh
		queueCh = f.queueCh
	)
	f.launch(func() { fetchTasks(i, qname, taskState, pageSize, pageNum, tasksCh, errorCh, f.done) })
	f.launch(func() { fetchQueueInfo(i, qname, queueCh, errorCh, f.done) })
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

func (f *dataFetcher) fetchTaskInfo(qname, taskID string) {
	var (
		i       = f.inspector
		taskCh  = f.taskCh
		errorCh = f.errorCh
	)
	f.launch(func() { fetchTaskInfo(i, qname, taskID, taskCh, errorCh, f.done) })
}

func fetchTaskInfo(i *asynq.Inspector, qname, taskID string, taskCh chan<- *asynq.TaskInfo, errorCh chan<- error, done ...<-chan struct{}) {
	info, err := i.GetTaskInfo(qname, taskID)
	if err != nil {
		publish(errorCh, err, fetchDone(done))
		return
	}
	publish(taskCh, info, fetchDone(done))
}
