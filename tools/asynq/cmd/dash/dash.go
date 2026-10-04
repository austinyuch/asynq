// Copyright 2022 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package dash

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
)

// viewType is an enum for dashboard views.
type viewType int

const (
	viewTypeQueues viewType = iota
	viewTypeQueueDetails
	viewTypeHelp
)

// State holds dashboard state.
type State struct {
	request fetchContext // event-loop-owned admission identity
	queues  []*asynq.QueueInfo
	tasks   []*asynq.TaskInfo
	groups  []*asynq.GroupInfo
	err     error

	// Note: index zero corresponds to the table header; index=1 correctponds to the first element
	queueTableRowIdx int             // highlighted row in queue table
	taskTableRowIdx  int             // highlighted row in task table
	groupTableRowIdx int             // highlighted row in group table
	taskState        asynq.TaskState // highlighted task state in queue details view
	taskID           string          // selected task ID

	selectedQueue *asynq.QueueInfo // queue shown on queue details view
	selectedGroup *asynq.GroupInfo
	selectedTask  *asynq.TaskInfo

	pageNum int // pagination page number

	view     viewType // current view type
	prevView viewType // to support "go back"
}

func (s *State) DebugString() string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "len(queues)=%d ", len(s.queues))
	_, _ = fmt.Fprintf(&b, "len(tasks)=%d ", len(s.tasks))
	_, _ = fmt.Fprintf(&b, "len(groups)=%d ", len(s.groups))
	_, _ = fmt.Fprintf(&b, "err=%v ", s.err)

	if s.taskState != 0 {
		_, _ = fmt.Fprintf(&b, "taskState=%s ", s.taskState.String())
	} else {
		b.WriteString("taskState=0")
	}
	_, _ = fmt.Fprintf(&b, "taskID=%s ", s.taskID)

	_, _ = fmt.Fprintf(&b, "queueTableRowIdx=%d ", s.queueTableRowIdx)
	_, _ = fmt.Fprintf(&b, "taskTableRowIdx=%d ", s.taskTableRowIdx)
	_, _ = fmt.Fprintf(&b, "groupTableRowIdx=%d ", s.groupTableRowIdx)

	if s.selectedQueue != nil {
		_, _ = fmt.Fprintf(&b, "selectedQueue={Queue:%s} ", s.selectedQueue.Queue)
	} else {
		b.WriteString("selectedQueue=nil ")
	}

	if s.selectedGroup != nil {
		_, _ = fmt.Fprintf(&b, "selectedGroup={Group:%s} ", s.selectedGroup.Group)
	} else {
		b.WriteString("selectedGroup=nil ")
	}

	if s.selectedTask != nil {
		_, _ = fmt.Fprintf(&b, "selectedTask={ID:%s} ", s.selectedTask.ID)
	} else {
		b.WriteString("selectedTask=nil ")
	}

	_, _ = fmt.Fprintf(&b, "pageNum=%d", s.pageNum)
	return b.String()
}

type Options struct {
	DebugMode    bool
	PollInterval time.Duration
	RedisConnOpt asynq.RedisConnOpt
}

func Run(opts Options) {
	s, err := tcell.NewScreen()
	if err != nil {
		fmt.Printf("failed to create a screen: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Printf("failed to initialize screen: %v\n", err)
		os.Exit(1)
	}
	runWithScreen(s, opts)
}

// runWithScreen owns an initialized screen and its newly created Inspector.
// Keeping terminal initialization at the process boundary also allows the
// same production wiring to run with a real SimulationScreen.
func runWithScreen(s tcell.Screen, opts Options) {
	// Transfer the initialized screen and owned Inspector to the runner.
	inspector := asynq.NewInspector(opts.RedisConnOpt)
	d := newDashboard(s, opts)
	f := &dataFetcher{
		inspector: inspector, opts: opts, s: s,
		errorCh: d.errorCh, queueCh: d.queueCh, taskCh: d.taskCh,
		queuesCh: d.queuesCh, groupsCh: d.groupsCh, tasksCh: d.tasksCh,
		done: d.done, slots: make(chan struct{}, 4),
	}
	d.run(f, func() {
		// Stop publication first (run closes done), then release transport before
		// waiting for in-flight I/O. Inspector APIs do not accept a context.
		_ = inspector.Close()
		f.wait()
	})
}

// dashboard confines all mutable UI state to its event-loop goroutine. run owns
// the initialized screen and ticker; cleanup owns the data source/workers.
type dashboard struct {
	s        tcell.Screen
	opts     Options
	ticker   *time.Ticker
	ticks    <-chan time.Time
	done     chan struct{}
	errorCh  chan fetchResult[error]
	queueCh  chan fetchResult[*asynq.QueueInfo]
	taskCh   chan fetchResult[*asynq.TaskInfo]
	queuesCh chan fetchResult[[]*asynq.QueueInfo]
	groupsCh chan fetchResult[[]*asynq.GroupInfo]
	tasksCh  chan fetchResult[[]*asynq.TaskInfo]
}

func newDashboard(s tcell.Screen, opts Options) *dashboard {
	ticker := time.NewTicker(opts.PollInterval)
	return &dashboard{
		s: s, opts: opts, ticker: ticker, ticks: ticker.C,
		done: make(chan struct{}), errorCh: make(chan fetchResult[error]),
		queueCh: make(chan fetchResult[*asynq.QueueInfo]), taskCh: make(chan fetchResult[*asynq.TaskInfo]),
		queuesCh: make(chan fetchResult[[]*asynq.QueueInfo]), groupsCh: make(chan fetchResult[[]*asynq.GroupInfo]),
		tasksCh: make(chan fetchResult[[]*asynq.TaskInfo]),
	}
}

// Only the event-loop owner closes done. Both a quit key and the deferred
// cleanup can request stopping, so repeated requests are harmless.
func stopDashboard(done chan struct{}) {
	select {
	case <-done:
	default:
		close(done)
	}
}

func (d *dashboard) run(f fetcher, cleanup func()) {
	s := d.s
	f = contextualFetcher{next: f, screen: s}
	s.SetStyle(baseStyle)
	state := State{}
	pageCapacity := taskPageSize(s)
	eventCh := make(chan tcell.Event)
	eventsStopped := make(chan struct{})
	go func() {
		defer close(eventsStopped)
		s.ChannelEvents(eventCh, d.done)
	}()
	defer func() {
		d.ticker.Stop()
		stopDashboard(d.done)
		<-eventsStopped
		cleanup()
		s.Fini()
	}()
	drawer := dashDrawer{s: s, opts: d.opts}
	h := keyEventHandler{
		s: s, state: &state, done: d.done, fetcher: f, drawer: &drawer,
		ticker: d.ticker, pollInterval: d.opts.PollInterval,
	}
	f.Fetch(&state)
	drawer.Draw(&state)
	for {
		s.Show()
		select {
		case <-d.done:
			return
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			switch ev := ev.(type) {
			case *tcell.EventResize:
				s.Sync()
				capacity := taskPageSize(s)
				details := state.view == viewTypeQueueDetails || (state.view == viewTypeHelp && state.prevView == viewTypeQueueDetails)
				if capacity != pageCapacity && details && !shouldShowGroupTable(&state) {
					state.pageNum = 1
					state.tasks = nil
					state.taskTableRowIdx = 0
					// Preserve an open modal's identity while replacing its underlying page.
					// Help defers admission until returning to the details view.
					if state.view == viewTypeQueueDetails {
						f.Fetch(&state)
						d.ticker.Reset(d.opts.PollInterval)
					}
				}
				pageCapacity = capacity
				drawer.Draw(&state)
				captureFetchContext(&state, s)
			case *tcell.EventKey:
				h.HandleKeyEvent(ev)
				captureFetchContext(&state, s)
			}
		case <-d.ticks:
			f.Fetch(&state)
		case result := <-d.queuesCh:
			if result.request != captureFetchContext(&state, s) {
				continue
			}
			queues := result.value
			state.queues = queues
			state.err = nil
			if len(queues) < state.queueTableRowIdx {
				state.queueTableRowIdx = len(queues)
			}
			drawer.Draw(&state)
		case result := <-d.queueCh:
			if result.request != captureFetchContext(&state, s) {
				continue
			}
			q := result.value
			state.selectedQueue = q
			state.err = nil
			drawer.Draw(&state)
		case result := <-d.groupsCh:
			if result.request != captureFetchContext(&state, s) {
				continue
			}
			groups := result.value
			state.groups = groups
			state.err = nil
			if len(groups) < state.groupTableRowIdx {
				state.groupTableRowIdx = len(groups)
			}
			drawer.Draw(&state)
		case result := <-d.tasksCh:
			if result.request != captureFetchContext(&state, s) {
				continue
			}
			tasks := result.value
			state.tasks = tasks
			state.err = nil
			if len(tasks) < state.taskTableRowIdx {
				state.taskTableRowIdx = len(tasks)
			}
			drawer.Draw(&state)
		case result := <-d.taskCh:
			if result.request != captureFetchContext(&state, s) {
				continue
			}
			task := result.value
			state.selectedTask = task
			state.err = nil
			drawer.Draw(&state)
		case result := <-d.errorCh:
			if result.request != captureFetchContext(&state, s) {
				continue
			}
			err := result.value
			if errors.Is(err, asynq.ErrTaskNotFound) {
				state.selectedTask = nil
			} else {
				state.err = err
			}
			drawer.Draw(&state)
		}
	}
}
