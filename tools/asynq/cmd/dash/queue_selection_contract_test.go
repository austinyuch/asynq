package dash

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
)

type queueSelectionRequest struct {
	queue     string
	page, row int
	taskID    string
	selected  *asynq.TaskInfo
}

// This fetcher deliberately completes no request. It snapshots admission from
// the real key pipeline, leaving a real loading interval for the next key.
type queueSelectionHeldFetcher struct{ requests []queueSelectionRequest }

func (f *queueSelectionHeldFetcher) Fetch(s *State) {
	q := ""
	if s.selectedQueue != nil {
		q = s.selectedQueue.Queue
	}
	f.requests = append(f.requests, queueSelectionRequest{q, s.pageNum, s.taskTableRowIdx, s.taskID, s.selectedTask})
}
func queueSelectionFixture(t *testing.T) (*keyEventHandler, *State, *queueSelectionHeldFetcher) {
	t.Helper()
	screen := renderingScreen(t, 120, 48)
	ticker := time.NewTicker(time.Hour)
	t.Cleanup(ticker.Stop)
	queues := []*asynq.QueueInfo{{Queue: "selection-A", Active: 1}, {Queue: "selection-B", Active: 1}}
	state := &State{view: viewTypeQueueDetails, queues: queues, selectedQueue: queues[0], queueTableRowIdx: 1, taskState: asynq.TaskStateActive, pageNum: 1, taskTableRowIdx: 1, tasks: []*asynq.TaskInfo{{ID: "A-owned-task", Queue: queues[0].Queue, Type: "selection-task", State: asynq.TaskStateActive}}}
	fetcher := &queueSelectionHeldFetcher{}
	return &keyEventHandler{s: screen, state: state, done: make(chan struct{}), fetcher: fetcher, drawer: &dashDrawer{s: screen}, ticker: ticker, pollInterval: time.Hour}, state, fetcher
}
func queueSelectionKey(h *keyEventHandler, key tcell.Key, r rune) {
	h.HandleKeyEvent(tcell.NewEventKey(key, r, tcell.ModNone))
}
func TestQueueSelectionLoadingEnter(t *testing.T) {
	h, s, f := queueSelectionFixture(t)
	// Queue A has a selected task row. Navigate through the actual public key
	// dispatch, then enter B while its held request has produced no task slice.
	queueSelectionKey(h, tcell.KeyRune, 'q')
	queueSelectionKey(h, tcell.KeyDown, 0)
	queueSelectionKey(h, tcell.KeyEnter, 0)
	if s.selectedQueue.Queue != "selection-B" || len(s.tasks) != 0 {
		t.Fatalf("loading fixture wrong: %+v", s)
	}

	if s.taskTableRowIdx != 0 || s.pageNum != 1 || s.taskID != "" || s.selectedTask != nil {
		t.Fatalf("new queue retains obsolete selection: %+v", s)
	}
	if len(f.requests) != 2 || f.requests[1].queue != "selection-B" || f.requests[1].row != 0 || f.requests[1].page != 1 || f.requests[1].taskID != "" || f.requests[1].selected != nil {
		t.Fatalf("request admitted stale queue/task selection: %+v", f.requests)
	}
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("original real key pipeline loading Enter panicked: %v", p)
			}
		}()
		queueSelectionKey(h, tcell.KeyEnter, 0)
	}()
}

func TestQueueSelectionSeededSwitches(t *testing.T) {
	rng := rand.New(rand.NewSource(20261021))
	for n := 0; n < 32; n++ {
		t.Run(fmt.Sprintf("sequence-%02d", n), func(t *testing.T) {
			h, s, f := queueSelectionFixture(t)
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("queue-switch key pipeline panic: %v", p)
				}
			}()
			for step := 0; step < 8; step++ {
				queueSelectionKey(h, tcell.KeyRune, 'q')
				target := rng.Intn(2)
				// Top-level queue rows are independently named, one-based selections.
				for attempt := 0; attempt < len(s.queues)+1 && s.queueTableRowIdx != target+1; attempt++ {
					queueSelectionKey(h, tcell.KeyDown, 0)
				}
				if s.queueTableRowIdx != target+1 {
					t.Fatalf("bounded queue selection failed: row%d target%d", s.queueTableRowIdx, target+1)
				}
				queueSelectionKey(h, tcell.KeyEnter, 0)
				want := []string{"selection-A", "selection-B"}[target]
				if s.selectedQueue.Queue != want || s.taskTableRowIdx != 0 || s.pageNum != 1 || s.taskID != "" || s.selectedTask != nil || len(s.tasks) != 0 {
					t.Fatalf("step%d wrong new-queue selection: %+v", step, s)
				}
				request := f.requests[len(f.requests)-1]
				if request.queue != want || request.row != 0 || request.page != 1 || request.taskID != "" || request.selected != nil {
					t.Fatalf("step%d stale fetch admission: %+v", step, request)
				}
				// Loading Enter must be inert and admit no task-modal request.
				admitted := len(f.requests)
				queueSelectionKey(h, tcell.KeyEnter, 0)
				if len(f.requests) != admitted || s.taskID != "" || s.selectedTask != nil {
					t.Fatal("loading Enter opened/fetched obsolete task")
				}
				// A fresh queue response then creates a selected row for the next switch.
				s.tasks = []*asynq.TaskInfo{{ID: fmt.Sprintf("%s-task-%d", want, step), Queue: want, Type: "selection-task", State: asynq.TaskStateActive}}
				queueSelectionKey(h, tcell.KeyDown, 0)
				if s.taskTableRowIdx != 1 {
					t.Fatal("fresh queue task row cannot be selected")
				}
			}
		})
	}
}
