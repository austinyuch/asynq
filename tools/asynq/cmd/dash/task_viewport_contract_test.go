package dash

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
)

type taskViewportFetcher struct{ requests chan fetchContext }

func (f *taskViewportFetcher) Fetch(state *State) { f.requests <- state.request }

type taskViewportFixture struct {
	*lifecycleFixture
	requests chan fetchContext
}

func newTaskViewportFixture(t *testing.T) *taskViewportFixture {
	t.Helper()
	screen := &lifecycleScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), frames: make(chan lifecycleFrame, 512), pumpEnded: make(chan struct{}), abortPump: make(chan struct{})}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(512, 48)
	requests := make(chan fetchContext, 64)
	x := &taskViewportFixture{lifecycleFixture: &lifecycleFixture{s: screen, d: newDashboard(screen, Options{PollInterval: time.Hour, DebugMode: true}), returned: make(chan struct{})}, requests: requests}
	// The owner stops the pump and joins before Fini; never close production done.
	t.Cleanup(func() {
		select {
		case <-x.returned:
		default:
			screen.requestAbort()
			select {
			case <-x.returned:
			case <-time.After(2 * time.Second):
				t.Error("viewport runner cleanup deadline")
			}
		}
	})
	go func() { defer close(x.returned); x.d.run(&taskViewportFetcher{requests}, func() {}) }()
	x.frame(t, hasText("=== Queues ==="))
	return x
}
func (x *taskViewportFixture) request(t *testing.T) fetchContext {
	t.Helper()
	select {
	case r := <-x.requests:
		return r
	case <-x.returned:
		t.Fatal("viewport owner exited before admission")
	case <-time.After(time.Second):
		t.Fatal("viewport admission deadline")
	}
	return fetchContext{}
}
func taskViewportSend[T any](t *testing.T, x *taskViewportFixture, ch chan<- fetchResult[T], request fetchContext, value T) {
	t.Helper()
	select {
	case ch <- fetchResult[T]{request: request, value: value}:
	case <-x.returned:
		t.Fatal("viewport exited before result")
	case <-time.After(time.Second):
		t.Fatal("viewport result admission deadline")
	}
}
func taskViewportTasks(first, last int) []*asynq.TaskInfo {
	var tasks []*asynq.TaskInfo
	for n := first; n <= last; n++ {
		tasks = append(tasks, &asynq.TaskInfo{ID: fmt.Sprintf("viewport-task-%03d", n), Queue: "viewport-owned", Type: "viewport-contract", State: asynq.TaskStateActive})
	}
	return tasks
}
func (x *taskViewportFixture) pageOne(t *testing.T) fetchContext {
	t.Helper()
	overview := x.request(t)
	taskViewportSend(t, x, x.d.queuesCh, overview, []*asynq.QueueInfo{{Queue: "viewport-owned", Active: 35}})
	x.frame(t, hasText("viewport-owned"))
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	request := x.request(t)
	if request.page != 1 || request.pageSize != 33 || request.taskID != "" {
		t.Fatalf("height48 admission: %+v", request)
	}
	taskViewportSend(t, x, x.d.tasksCh, request, taskViewportTasks(1, 33))
	x.frame(t, hasText("Showing 1-33 out of 35"))
	return request
}
func (x *taskViewportFixture) pageTwo(t *testing.T) fetchContext {
	t.Helper()
	x.pageOne(t)
	x.key(t, tcell.KeyRune, 'n', func(string) bool { return true })
	request := x.request(t)
	if request.page != 2 || request.pageSize != 33 {
		t.Fatalf("page2 admission: %+v", request)
	}
	taskViewportSend(t, x, x.d.tasksCh, request, taskViewportTasks(34, 35))
	x.frame(t, hasText("Showing 34-35 out of 35"))
	return request
}

var taskViewportRange = regexp.MustCompile(`Showing ([0-9]+)-([0-9]+) out of ([0-9]+)`)

func TestTaskViewportResizeInvalidatesRowsAndRefetches(t *testing.T) {
	for _, height := range []int{49, 50} {
		t.Run(fmt.Sprintf("height%d", height), func(t *testing.T) {
			x := newTaskViewportFixture(t)
			old := x.pageTwo(t)
			after := x.s.serial.Load()
			x.s.SetSize(512, height)
			x.s.PostEventWait(tcell.NewEventResize(512, height))
			frame := x.frameAfter(t, after, func(string) bool { return true })
			for _, m := range taskViewportRange.FindAllStringSubmatch(frame, -1) {
				end, _ := strconv.Atoi(m[2])
				total, _ := strconv.Atoi(m[3])
				if end > total {
					t.Errorf("viewport range exceeds actual total: %s", m[0])
				}
			}
			if !strings.Contains(frame, "pageNum=1") {
				t.Error("capacity change did not normalize task page to1")
			}
			if strings.Contains(frame, "viewport-task-034") || strings.Contains(frame, "viewport-task-035") {
				t.Error("resize loading frame retained obsolete page2 rows")
			}
			var fresh fetchContext
			// Show completes only after the resize handler returns; immediate Fetch must
			// already have admitted its immutable request, no deadline-as-oracle needed.
			select {
			case fresh = <-x.requests:
			default:
				t.Error("capacity change did not immediately admit fresh task fetch")
				return
			}
			if fresh.page != 1 || fresh.pageSize != height-15 || fresh.epoch <= old.epoch {
				t.Fatalf("normalized resize admission: %+v old%+v", fresh, old)
			}
			last := fresh.pageSize
			if last > 35 {
				last = 35
			}
			taskViewportSend(t, x, x.d.tasksCh, fresh, taskViewportTasks(1, last))
			x.frame(t, hasText("viewport-task-001"))
		})
	}
}
func TestTaskViewportPageChangesCannotEnterObsoleteRows(t *testing.T) {
	for _, direction := range []rune{'n', 'p'} {
		t.Run(string(direction), func(t *testing.T) {
			x := newTaskViewportFixture(t)
			if direction == 'n' {
				x.pageOne(t)
			} else {
				x.pageTwo(t)
			}
			x.key(t, tcell.KeyDown, 0, hasText("taskTableRowIdx=1 "))
			frame := x.key(t, tcell.KeyRune, direction, func(string) bool { return true })
			request := x.request(t)
			wantPage := 2
			if direction == 'p' {
				wantPage = 1
			}
			if request.page != wantPage || request.taskID != "" {
				t.Fatalf("page change admission: %+v", request)
			}
			if strings.Contains(frame, "viewport-task-") {
				t.Error("page loading frame retained obsolete rows")
			}
			if !strings.Contains(frame, "taskTableRowIdx=0 ") {
				t.Error("page loading frame retained selected task row")
			}
			x.key(t, tcell.KeyDown, 0, func(string) bool { return true })
			frame = x.key(t, tcell.KeyEnter, 0, func(string) bool { return true })
			if !strings.Contains(frame, "taskID= ") || !strings.Contains(frame, "selectedTask=nil ") {
				t.Error("Enter opened an obsolete task before new page delivery")
			}
			select {
			case unexpected := <-x.requests:
				t.Errorf("loading Enter admitted obsolete modal request: %+v", unexpected)
			default:
			}
		})
	}
}

func TestTaskViewportModalPageKeysRemainInert(t *testing.T) {
	for _, direction := range []rune{'n', 'p'} {
		t.Run(string(direction), func(t *testing.T) {
			x := newTaskViewportFixture(t)
			if direction == 'n' {
				x.pageOne(t)
			} else {
				x.pageTwo(t)
			}
			x.key(t, tcell.KeyDown, 0, hasText("taskTableRowIdx=1 "))
			id := "viewport-task-001"
			if direction == 'p' {
				id = "viewport-task-034"
			}
			x.key(t, tcell.KeyEnter, 0, hasText("taskID="+id+" "))
			modal := x.request(t)
			frame := x.key(t, tcell.KeyRune, direction, func(string) bool { return true })
			if !strings.Contains(frame, "taskID="+id+" ") || !strings.Contains(frame, "selectedTask={ID:"+id+"} ") {
				t.Error("modal page key changed selected identity")
			}
			select {
			case unexpected := <-x.requests:
				t.Errorf("modal page key admitted background page transition: got%+v modal%+v", unexpected, modal)
			default:
			}
		})
	}
}
func TestTaskViewportModalResizeKeepsIdentityAndRefetchesUnderlyingRows(t *testing.T) {
	x := newTaskViewportFixture(t)
	x.pageTwo(t)
	x.key(t, tcell.KeyDown, 0, hasText("taskTableRowIdx=1 "))
	id := "viewport-task-034"
	x.key(t, tcell.KeyEnter, 0, hasText("taskID="+id+" "))
	modal := x.request(t)
	after := x.s.serial.Load()
	x.s.SetSize(512, 49)
	x.s.PostEventWait(tcell.NewEventResize(512, 49))
	frame := x.frameAfter(t, after, func(string) bool { return true })
	if !strings.Contains(frame, "taskID="+id+" ") || !strings.Contains(frame, "selectedTask={ID:"+id+"} ") {
		t.Error("modal resize lost selected task identity")
	}
	if !strings.Contains(frame, "pageNum=1") {
		t.Error("modal resize did not normalize underlying task page")
	}
	select {
	case request := <-x.requests:
		if request.page != 1 || request.pageSize != 34 || request.taskID != id || request.epoch <= modal.epoch {
			t.Errorf("modal resize admission lost underlying viewport/identity: %+v", request)
		}
	default:
		t.Error("modal resize did not refetch underlying task viewport")
	}
}

// Domain counts govern navigation even while the visible page is loading.
// The oracle enumerates page starts, independently of the drawer's rows.
func taskViewportNavigationModel(t *testing.T, height, total, page, rows, stateIndex int, grouped bool) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, height)
	states := []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStatePending, asynq.TaskStateScheduled, asynq.TaskStateRetry, asynq.TaskStateArchived, asynq.TaskStateCompleted, asynq.TaskStateAggregating}
	queue := &asynq.QueueInfo{Active: total, Pending: total, Scheduled: total, Retry: total, Archived: total, Completed: total, Aggregating: total}
	state := &State{selectedQueue: queue, taskState: states[stateIndex%len(states)], pageNum: page, tasks: make([]*asynq.TaskInfo, rows)}
	if grouped {
		state.taskState = asynq.TaskStateAggregating
		state.selectedGroup = &asynq.GroupInfo{Size: total}
		queue.Aggregating = total + 99
	}
	capacity := height - 15
	if capacity < 1 {
		capacity = 1
	}
	last := 1
	for first := capacity; first < total; first += capacity {
		last++
	}
	want := page < last
	if got := isNextTaskPageAvailable(screen, state); got != want {
		t.Fatalf("navigation depends on rendered rows or foreign count: height=%d total=%d page=%d rows=%d state=%v grouped=%v got=%v want=%v", height, total, page, rows, state.taskState, grouped, got, want)
	}
}
func TestTaskViewportNavigationCountModelPBT(t *testing.T) {
	rng := rand.New(rand.NewSource(22022))
	for n := 0; n < 256; n++ {
		taskViewportNavigationModel(t, 1+rng.Intn(90), rng.Intn(180), 1+rng.Intn(190), rng.Intn(50), rng.Intn(7), n%2 == 0)
	}
}
func FuzzTaskViewportNavigationCountModel(f *testing.F) {
	f.Add(uint8(49), uint8(35), uint8(2), uint8(0), uint8(0), false)
	f.Add(uint8(48), uint8(35), uint8(2), uint8(2), uint8(6), true)
	f.Fuzz(func(t *testing.T, height, total, page, rows, state uint8, grouped bool) {
		taskViewportNavigationModel(t, 1+int(height)%90, int(total), 1+int(page), int(rows)%50, int(state)%7, grouped)
	})
}

func TestTaskViewportWidthOnlyResizePreservesPage(t *testing.T) {
	x := newTaskViewportFixture(t)
	old := x.pageTwo(t)
	serial := x.s.serial.Load()
	x.s.SetSize(500, 48)
	if err := x.s.PostEvent(tcell.NewEventResize(500, 48)); err != nil {
		t.Fatal(err)
	}
	frame := x.frameAfter(t, serial, func(string) bool { return true })
	if !strings.Contains(frame, "pageNum=2 ") || !strings.Contains(frame, "viewport-task-034") || !strings.Contains(frame, "viewport-task-035") {
		t.Fatalf("width-only resize changed task page: %s", frame)
	}
	select {
	case got := <-x.requests:
		t.Fatalf("width-only resize fetched: old%+v new%+v", old, got)
	default:
	}
}

func TestTaskViewportHelpResizeDefersAndRejectsOldRows(t *testing.T) {
	x := newTaskViewportFixture(t)
	old := x.pageTwo(t)
	x.key(t, tcell.KeyRune, '?', hasText("=== Help ==="))
	serial := x.s.serial.Load()
	x.s.SetSize(512, 49)
	if err := x.s.PostEvent(tcell.NewEventResize(512, 49)); err != nil {
		t.Fatal(err)
	}
	frame := x.frameAfter(t, serial, hasText("=== Help ==="))
	if !strings.Contains(frame, "pageNum=1 ") || !strings.Contains(frame, "len(tasks)=0 ") {
		t.Fatalf("help resize retained underlying page: %s", frame)
	}
	select {
	case got := <-x.requests:
		t.Fatalf("help resize prematurely fetched %+v", got)
	default:
	}
	x.key(t, tcell.KeyRune, 'q', hasText("=== Queue Summary ==="))
	fresh := x.request(t)
	if fresh.page != 1 || fresh.pageSize != 34 || fresh.epoch <= old.epoch {
		t.Fatalf("return from help admission %+v", fresh)
	}
	taskViewportSend(t, x, x.d.tasksCh, old, taskViewportTasks(34, 35))
	// A current error is a FIFO owner-frame barrier after the stale task delivery.
	taskViewportSend(t, x, x.d.errorCh, fresh, fmt.Errorf("viewport-help-barrier"))
	frame = x.frame(t, hasText("viewport-help-barrier"))
	if strings.Contains(frame, "viewport-task-034") || strings.Contains(frame, "viewport-task-035") || !strings.Contains(frame, "len(tasks)=0 ") {
		t.Fatalf("old identity accepted after help resize: %s", frame)
	}
	taskViewportSend(t, x, x.d.tasksCh, fresh, taskViewportTasks(1, 34))
	x.frame(t, hasText("viewport-task-001"))
}

type taskViewportPageRecorder struct{ pages []int }

func (r *taskViewportPageRecorder) Fetch(s *State) { r.pages = append(r.pages, s.pageNum) }

type taskViewportNoopDrawer struct{}

func (taskViewportNoopDrawer) Draw(*State) {}

func TestTaskViewportExtremeAndEmptyNavigationBounds(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 15)
	maxInt := int(^uint(0) >> 1)
	for _, total := range []int{maxInt, 0, -7} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			state := &State{view: viewTypeQueueDetails, selectedQueue: &asynq.QueueInfo{Active: total}, taskState: asynq.TaskStateActive, pageNum: maxInt - 1}
			recorder := &taskViewportPageRecorder{}
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			h := &keyEventHandler{s: s, state: state, fetcher: recorder, drawer: taskViewportNoopDrawer{}, ticker: ticker, pollInterval: time.Hour}
			wantLast := 1
			if total > 0 {
				wantLast = maxInt
			}
			if got := lastTaskPage(s, state); got != wantLast {
				t.Fatalf("cap1 count%d last=%d want%d", total, got, wantLast)
			}
			h.nextPage()
			if state.pageNum != wantLast {
				t.Fatalf("n clamped to%d want%d", state.pageNum, wantLast)
			}
			before := len(recorder.pages)
			h.nextPage()
			if state.pageNum != wantLast || len(recorder.pages) != before {
				t.Fatalf("last-page n overflow/admitted: page%d requests%v", state.pageNum, recorder.pages)
			}
			h.prevPage()
			wantPrev := 1
			if total > 0 {
				wantPrev = maxInt - 1
			}
			if state.pageNum != wantPrev {
				t.Fatalf("p=%d want%d", state.pageNum, wantPrev)
			}
			state.pageNum = -9
			h.prevPage()
			if state.pageNum != 1 {
				t.Fatalf("negative page did not clamp to1: %d", state.pageNum)
			}
		})
	}
}

func TestTaskViewportSelectedGroupResizeUsesGroupCount(t *testing.T) {
	x := newTaskViewportFixture(t)
	overview := x.request(t)
	taskViewportSend(t, x, x.d.queuesCh, overview, []*asynq.QueueInfo{{Queue: "viewport-owned", Aggregating: 999}})
	x.frame(t, hasText("viewport-owned"))
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	x.request(t)
	x.key(t, tcell.KeyRight, 0, hasText("taskState=pending "))
	x.request(t)
	x.key(t, tcell.KeyRight, 0, hasText("taskState=aggregating "))
	groupRequest := x.request(t)
	taskViewportSend(t, x, x.d.groupsCh, groupRequest, []*asynq.GroupInfo{{Group: "viewport-group", Size: 35}})
	x.frame(t, hasText("viewport-group"))
	x.key(t, tcell.KeyDown, 0, hasText("groupTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, func(string) bool { return true })
	first := x.request(t)
	taskViewportSend(t, x, x.d.tasksCh, first, taskViewportTasks(1, 33))
	x.frame(t, hasText("Showing 1-33 out of 35"))
	x.key(t, tcell.KeyRune, 'n', func(string) bool { return true })
	old := x.request(t)
	taskViewportSend(t, x, x.d.tasksCh, old, taskViewportTasks(34, 35))
	x.frame(t, hasText("Showing 34-35 out of 35"))
	serial := x.s.serial.Load()
	x.s.SetSize(512, 49)
	if err := x.s.PostEvent(tcell.NewEventResize(512, 49)); err != nil {
		t.Fatal(err)
	}
	frame := x.frameAfter(t, serial, func(string) bool { return true })
	fresh := x.request(t)
	if fresh.group != "viewport-group" || fresh.page != 1 || fresh.pageSize != 34 || fresh.epoch <= old.epoch || !strings.Contains(frame, "len(tasks)=0 ") {
		t.Fatalf("selected-group resize frame/admission invalid: %+v %s", fresh, frame)
	}
	taskViewportSend(t, x, x.d.tasksCh, fresh, taskViewportTasks(1, 34))
	x.frame(t, hasText("Showing 1-34 out of 35"))
}
