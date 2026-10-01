package dash

import (
	"fmt"
	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func reverseNavigationSequence(t *testing.T, input []byte) {
	t.Helper()
	screen := renderingScreen(t, 120, 48)
	ticker := time.NewTicker(time.Hour)
	t.Cleanup(ticker.Stop)
	// Independent user-visible reverse order, not production taskStates.
	order := []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStateCompleted, asynq.TaskStateArchived, asynq.TaskStateRetry, asynq.TaskStateScheduled, asynq.TaskStateAggregating, asynq.TaskStatePending}
	s := &State{view: viewTypeQueueDetails, selectedQueue: &asynq.QueueInfo{Queue: "reverse-owned"}, taskState: order[0], pageNum: 1}
	h := &keyEventHandler{s: screen, state: s, fetcher: &taskViewportPageRecorder{}, drawer: taskViewportNoopDrawer{}, ticker: ticker, pollInterval: time.Hour}
	install := func() {
		s.tasks = taskViewportTasks(1, 3)
		s.groups = []*asynq.GroupInfo{{Group: "r-a"}, {Group: "r-b"}, {Group: "r-c"}}
	}
	install()
	index, row := 0, 0
	if len(input) > 64 {
		input = input[:64]
	}
	for step, b := range input {
		switch b % 3 {
		case 0:
			h.HandleKeyEvent(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
			row = (row + 3) % 4
		case 1:
			h.HandleKeyEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
			row = (row + 1) % 4
		case 2:
			s.pageNum = 2
			h.HandleKeyEvent(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
			index = (index + 1) % 7
			row = 0
			if s.taskState != order[index] || s.pageNum != 1 || s.taskTableRowIdx != 0 || s.selectedGroup != nil || len(s.tasks) != 0 {
				t.Fatalf("step%d reverse-state retained old data: %+v", step, s)
			}
			install()
			s.groupTableRowIdx = 0
		}
		got := s.taskTableRowIdx
		if s.taskState == asynq.TaskStateAggregating {
			got = s.groupTableRowIdx
		}
		if got != row {
			t.Fatalf("step%d row%d want%d state%s", step, got, row, s.taskState)
		}
	}
}
func TestReverseNavigationSeededModel(t *testing.T) {
	r := rand.New(rand.NewSource(20261022))
	for n := 0; n < 128; n++ {
		input := make([]byte, 64)
		r.Read(input)
		t.Run(fmt.Sprint(n), func(t *testing.T) { reverseNavigationSequence(t, input) })
	}
}
func FuzzReverseNavigationKeySequence(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2})
	f.Fuzz(func(t *testing.T, input []byte) { reverseNavigationSequence(t, input) })
}
func TestReverseNavigationOverviewUpWrap(t *testing.T) {
	x := newTaskViewportFixture(t)
	r := x.request(t)
	taskViewportSend(t, x, x.d.queuesCh, r, []*asynq.QueueInfo{{Queue: "reverse-first"}, {Queue: "reverse-last"}})
	x.frame(t, hasText("reverse-first"))
	for _, want := range []int{2, 1, 0, 2} {
		frame := x.key(t, tcell.KeyUp, 0, func(string) bool { return true })
		if !strings.Contains(frame, fmt.Sprintf("queueTableRowIdx=%d ", want)) {
			t.Fatalf("up row mismatch want%d actual frame: %s", want, frame)
		}
	}
}

// Current error frames form an owner-loop barrier after each stale receive.
func reverseNavigationBarrier(t *testing.T, x *taskViewportFixture, r fetchContext, label string) string {
	t.Helper()
	taskViewportSend(t, x, x.d.errorCh, r, fmt.Errorf("reverse-barrier-%s", label))
	return x.frame(t, hasText("reverse-barrier-"+label))
}
func TestReverseNavigationStaleOverviewRejected(t *testing.T) {
	x := newTaskViewportFixture(t)
	r := x.request(t)
	taskViewportSend(t, x, x.d.queuesCh, r, []*asynq.QueueInfo{{Queue: "reverse-healthy"}})
	x.frame(t, hasText("reverse-healthy"))
	old := r
	old.epoch++
	taskViewportSend(t, x, x.d.queuesCh, old, []*asynq.QueueInfo{{Queue: "reverse-stale"}})
	frame := reverseNavigationBarrier(t, x, r, "overview")
	if !strings.Contains(frame, "reverse-healthy") || strings.Contains(frame, "reverse-stale") {
		t.Fatalf("stale replaced queue: %s", frame)
	}
}
func TestReverseNavigationStaleDetailsRejected(t *testing.T) {
	for _, kind := range []string{"queue", "groups", "tasks", "task"} {
		t.Run(kind, func(t *testing.T) {
			x := newTaskViewportFixture(t)
			r := x.pageOne(t)
			if kind == "task" {
				x.key(t, tcell.KeyDown, 0, hasText("taskTableRowIdx=1 "))
				x.key(t, tcell.KeyEnter, 0, hasText("=== Task Info ==="))
				r = x.request(t)
				taskViewportSend(t, x, x.d.taskCh, r, &asynq.TaskInfo{ID: r.taskID, Queue: r.queue, Type: "reverse-current-modal", State: asynq.TaskStateActive})
				x.frame(t, hasText("reverse-current-modal"))
			}
			old := r
			old.queue = "reverse-other-queue"
			switch kind {
			case "queue":
				taskViewportSend(t, x, x.d.queueCh, old, &asynq.QueueInfo{Queue: "reverse-stale-queue", Active: 99})
			case "groups":
				taskViewportSend(t, x, x.d.groupsCh, old, []*asynq.GroupInfo{{Group: "reverse-stale-group", Size: 99}})
			case "tasks":
				taskViewportSend(t, x, x.d.tasksCh, old, []*asynq.TaskInfo{{ID: "reverse-stale-task", Queue: r.queue, Type: "reverse-stale-type", State: asynq.TaskStateActive}})
			case "task":
				taskViewportSend(t, x, x.d.taskCh, old, &asynq.TaskInfo{ID: r.taskID, Queue: r.queue, Type: "reverse-stale-modal", State: asynq.TaskStateActive})
			}
			frame := reverseNavigationBarrier(t, x, r, kind)
			if strings.Contains(frame, "reverse-stale") {
				t.Fatalf("stale %s accepted: %s", kind, frame)
			}
			switch kind {
			case "queue":
				if !strings.Contains(frame, "viewport-owned") {
					t.Fatal("healthy queue lost")
				}
			case "groups":
				if !strings.Contains(frame, "len(groups)=0 ") {
					t.Fatal("stale groups accepted")
				}
			case "tasks":
				if !strings.Contains(frame, "viewport-task-001") || !strings.Contains(frame, "len(tasks)=33 ") {
					t.Fatal("healthy rows replaced")
				}
			case "task":
				if !strings.Contains(frame, "reverse-current-modal") {
					t.Fatal("healthy modal replaced")
				}
			}
		})
	}
}

func TestReverseNavigationStaleVisibleGroupsRejected(t *testing.T) {
	x := newTaskViewportFixture(t)
	x.pageOne(t)
	x.key(t, tcell.KeyRight, 0, hasText("taskState=pending "))
	x.request(t)
	x.key(t, tcell.KeyRight, 0, hasText("taskState=aggregating "))
	old := x.request(t)
	taskViewportSend(t, x, x.d.groupsCh, old, []*asynq.GroupInfo{{Group: "reverse-prior-group", Size: 2}})
	x.frame(t, hasText("reverse-prior-group"))
	// Leave and return through real state keys: same queue/state but a new epoch.
	x.key(t, tcell.KeyRight, 0, hasText("taskState=scheduled "))
	x.request(t)
	x.key(t, tcell.KeyLeft, 0, hasText("taskState=aggregating "))
	current := x.request(t)
	if current.epoch <= old.epoch || current.taskState != old.taskState || current.queue != old.queue {
		t.Fatalf("real reverse state did not create new admission: old%+v current%+v", old, current)
	}
	taskViewportSend(t, x, x.d.groupsCh, current, []*asynq.GroupInfo{{Group: "reverse-healthy-group-a", Size: 3}, {Group: "reverse-healthy-group-b", Size: 4}})
	x.frame(t, hasText("reverse-healthy-group-a"))
	taskViewportSend(t, x, x.d.groupsCh, old, []*asynq.GroupInfo{{Group: "reverse-stale-visible-group", Size: 99}})
	frame := reverseNavigationBarrier(t, x, current, "visible-groups")
	if strings.Contains(frame, "reverse-stale-visible-group") || !strings.Contains(frame, "reverse-healthy-group-a") || !strings.Contains(frame, "reverse-healthy-group-b") || !strings.Contains(frame, "len(groups)=2 ") {
		t.Fatalf("stale group delivery replaced healthy rendered rows: %s", frame)
	}
	taskViewportSend(t, x, x.d.groupsCh, current, []*asynq.GroupInfo{{Group: "reverse-fresh-group", Size: 5}})
	frame = x.frame(t, hasText("reverse-fresh-group"))
	if strings.Contains(frame, "reverse-healthy-group-a") || !strings.Contains(frame, "len(groups)=1 ") {
		t.Fatalf("current group refresh not accepted: %s", frame)
	}
}
