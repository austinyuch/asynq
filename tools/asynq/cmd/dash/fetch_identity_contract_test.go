package dash

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
)

// Admission snapshots contain values, never a pointer to event-loop State.
// These controlled completions test the real consumer/event-pump boundary;
// they do not claim identity wiring in the Redis-backed dataFetcher producer.
type identityRequest struct {
	context              fetchContext
	view                 viewType
	queue, group, taskID string
	state                asynq.TaskState
	page                 int
}
type identityHeldFetcher struct {
	requests chan identityRequest
	done     <-chan struct{}
}

func (f *identityHeldFetcher) Fetch(s *State) {
	request := identityRequest{context: s.request, view: s.view, state: s.taskState, page: s.pageNum, taskID: s.taskID}
	if s.selectedQueue != nil {
		request.queue = s.selectedQueue.Queue
	}
	if s.selectedGroup != nil {
		request.group = s.selectedGroup.Group
	}
	select {
	case f.requests <- request:
	case <-f.done:
	}
}
func newIdentityFixture(t *testing.T) (*lifecycleFixture, *identityHeldFetcher) {
	t.Helper()
	screen := &lifecycleScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), frames: make(chan lifecycleFrame, 512), pumpEnded: make(chan struct{}), abortPump: make(chan struct{})}
	if e := screen.Init(); e != nil {
		t.Fatal(e)
	}
	screen.SetSize(512, 48)
	x := &lifecycleFixture{s: screen, d: newDashboard(screen, Options{PollInterval: time.Hour, DebugMode: true}), returned: make(chan struct{}), ticks: make(chan time.Time, 1)}
	x.d.ticks = x.ticks
	fetcher := &identityHeldFetcher{requests: make(chan identityRequest, 64), done: x.d.done}
	// Register cancellation/join before the first asynchronous action or await.
	t.Cleanup(func() {
		screen.requestAbort()
		select {
		case <-x.returned:
		case <-time.After(2 * time.Second):
			t.Error("identity runner cleanup did not join")
		}
		select {
		case <-screen.pumpEnded:
		default:
			t.Error("identity event pump did not join")
		}
	})
	go func() {
		defer close(x.returned)
		x.d.run(fetcher, func() {
			x.cleanups.Add(1)
			select {
			case <-screen.pumpEnded:
				select {
				case <-x.d.done:
					x.cleanupOrder.Store(screen.finiCount.Load() == 0)
				default:
				}
			default:
			}
		})
	}()
	x.frame(t, hasText("=== Queues ==="))
	return x, fetcher
}
func identityAdmission(t *testing.T, f *identityHeldFetcher) identityRequest {
	t.Helper()
	select {
	case r := <-f.requests:
		return r
	case <-time.After(time.Second):
		t.Fatal("identity request admission deadline")
		return identityRequest{}
	}
}
func identityBarrier(t *testing.T, x *lifecycleFixture) string {
	t.Helper()
	return x.key(t, tcell.KeyCtrlL, 0, func(s string) bool { return strings.Contains(s, "=== Queue Summary ===") })
}
func identityName(t *testing.T, text, name string) {
	t.Helper()
	if !regexp.MustCompile(`(?m)^\s*Name\s+` + regexp.QuoteMeta(name) + `\s`).MatchString(text) {
		t.Fatalf("stale completion replaced current queue identity %q; actual frame:\n%s", name, text)
	}
}
func identitySwitchToB(t *testing.T) (*lifecycleFixture, *identityHeldFetcher, identityRequest, identityRequest) {
	t.Helper()
	x, f := newIdentityFixture(t)
	initial := identityAdmission(t, f)
	if initial.view != viewTypeQueues {
		t.Fatal("initial request not overview")
	}
	identitySend(t, x, x.d.queuesCh, initial.context, []*asynq.QueueInfo{{Queue: "identity-A", Active: 70, Pending: 70, Aggregating: 70}, {Queue: "identity-B", Active: 70, Pending: 70, Aggregating: 70}})
	x.frame(t, hasText("identity-A"))
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	old := identityAdmission(t, f)
	if old.queue != "identity-A" || old.state != asynq.TaskStateActive || old.page != 1 {
		t.Fatalf("A immutable snapshot: %+v", old)
	}
	x.key(t, tcell.KeyRune, 'q', hasText("=== Queues ==="))
	overview := identityAdmission(t, f)
	if overview.view != viewTypeQueues {
		t.Fatal("back request not overview")
	}
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=2 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	fresh := identityAdmission(t, f)
	if fresh.queue != "identity-B" || fresh.state != asynq.TaskStateActive || fresh.page != 1 || fresh.taskID != "" {
		t.Fatalf("B immutable snapshot: %+v", fresh)
	}
	return x, f, old, fresh
}
func TestFetchIdentityLateQueueCompletion(t *testing.T) {
	x, _, old, fresh := identitySwitchToB(t)
	identitySend(t, x, x.d.queueCh, fresh.context, &asynq.QueueInfo{Queue: fresh.queue, Active: 70, Pending: 70, Aggregating: 70})
	identityName(t, identityBarrier(t, x), fresh.queue)
	// Release A only after B's visible acknowledgement. The unbuffered send
	// admits the old result before a subsequent CtrlL/Show frame barrier.
	identitySend(t, x, x.d.queueCh, old.context, &asynq.QueueInfo{Queue: old.queue, Active: 70})
	identityName(t, identityBarrier(t, x), fresh.queue)
	x.exit(t, tcell.KeyCtrlC, 0)
}
func TestFetchIdentityLateTasksCompletion(t *testing.T) {
	x, _, old, fresh := identitySwitchToB(t)
	identitySend(t, x, x.d.tasksCh, fresh.context, []*asynq.TaskInfo{{ID: "B-identity-task", Queue: fresh.queue, Type: "B-task-type", State: asynq.TaskStateActive}})
	frame := identityBarrier(t, x)
	if !strings.Contains(frame, "B-identity-task") {
		t.Fatal("B fixture not visibly acknowledged")
	}
	identitySend(t, x, x.d.tasksCh, old.context, []*asynq.TaskInfo{{ID: "A-obsolete-task", Queue: old.queue, Type: "A-task-type", State: asynq.TaskStateActive}})
	frame = identityBarrier(t, x)
	if strings.Contains(frame, "A-obsolete-task") || !strings.Contains(frame, "B-identity-task") {
		t.Fatalf("old request task result contaminated new queue: actual frame:\n%s", frame)
	}
	x.exit(t, tcell.KeyCtrlC, 0)
}

// Independent test publication must remain bounded even when acceptance is
// mutated. It never calls production publish or legacy compatibility adapters.
func identitySend[T any](t *testing.T, x *lifecycleFixture, ch chan<- fetchResult[T], request fetchContext, value T) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case ch <- fetchResult[T]{request: request, value: value}:
	case <-x.returned:
		t.Fatal("identity runner returned before completion")
	case <-x.s.abortPump:
		t.Fatal("identity completion aborted")
	case <-timer.C:
		t.Fatal("identity completion admission deadline")
	}
}
func identityTask(t *testing.T, x *lifecycleFixture, r identityRequest, id string) {
	t.Helper()
	identitySend(t, x, x.d.tasksCh, r.context, []*asynq.TaskInfo{{ID: id, Queue: r.queue, Type: "identity-task", State: r.state}})
}
func TestFetchIdentityQueueReturnEpoch(t *testing.T) {
	x, f, old, _ := identitySwitchToB(t)
	x.key(t, tcell.KeyRune, 'q', hasText("=== Queues ==="))
	identityAdmission(t, f)
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=0 "))
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	fresh := identityAdmission(t, f)
	if fresh.queue != old.queue || fresh.context.epoch == old.context.epoch {
		t.Fatal("A-B-A did not create a new logical visit")
	}
	identityTask(t, x, fresh, "A-new-visit")
	frame := identityBarrier(t, x)
	if !strings.Contains(frame, "A-new-visit") {
		t.Fatal("new A fixture not visible")
	}
	identityTask(t, x, old, "A-old-visit")
	frame = identityBarrier(t, x)
	if !strings.Contains(frame, "A-new-visit") || strings.Contains(frame, "A-old-visit") {
		t.Fatal("returning to A accepted old visit result")
	}
	x.exit(t, tcell.KeyCtrlC, 0)
}
func TestFetchIdentityStateAndPage(t *testing.T) {
	x, f, _, active := identitySwitchToB(t)
	x.key(t, tcell.KeyRight, 0, hasText("taskState=pending "))
	pending := identityAdmission(t, f)
	identityTask(t, x, pending, "pending-current")
	identityBarrier(t, x)
	identityTask(t, x, active, "active-obsolete")
	frame := identityBarrier(t, x)
	if !strings.Contains(frame, "pending-current") || strings.Contains(frame, "active-obsolete") {
		t.Fatal("old state result polluted pending tab")
	}
	// Next-page admission is observed through immutable request, not a sleep or
	// PostEventWait processing assumption. Current completion supplies redraw.
	x.s.PostEventWait(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	page := identityAdmission(t, f)
	if page.page != 2 {
		t.Fatalf("next-page snapshot %+v", page)
	}
	identityTask(t, x, page, "page-two-current")
	identityBarrier(t, x)
	identityTask(t, x, pending, "page-one-obsolete")
	frame = identityBarrier(t, x)
	if !strings.Contains(frame, "page-two-current") || strings.Contains(frame, "page-one-obsolete") {
		t.Fatal("old page result polluted page two")
	}
	x.exit(t, tcell.KeyCtrlC, 0)
}
func TestFetchIdentityModalAndSameContextPoll(t *testing.T) {
	x, f, _, current := identitySwitchToB(t)
	identityTask(t, x, current, "modal-first")
	identityBarrier(t, x)
	x.key(t, tcell.KeyDown, 0, hasText("taskTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("taskID=modal-first "))
	firstModal := identityAdmission(t, f)
	identitySend(t, x, x.d.taskCh, firstModal.context, &asynq.TaskInfo{ID: "modal-first", Queue: current.queue, Type: "identity-task", State: asynq.TaskStateActive})
	identityBarrier(t, x)
	x.key(t, tcell.KeyRune, 'q', hasText("taskID= "))
	// Closing a modal starts a replacement request for the non-modal view.
	fresh := identityAdmission(t, f)
	identityTask(t, x, fresh, "modal-second")
	identityBarrier(t, x)
	x.key(t, tcell.KeyEnter, 0, hasText("taskID=modal-second "))
	secondModal := identityAdmission(t, f)
	identitySend(t, x, x.d.taskCh, secondModal.context, &asynq.TaskInfo{ID: "modal-second", Queue: current.queue, Type: "identity-task", State: asynq.TaskStateActive})
	identityBarrier(t, x)
	identitySend[error](t, x, x.d.errorCh, firstModal.context, asynq.ErrTaskNotFound)
	frame := identityBarrier(t, x)
	if !strings.Contains(frame, "selectedTask={ID:modal-second}") {
		t.Fatal("late old modal NotFound cleared current modal")
	}
	x.ticks <- time.Now()
	poll := identityAdmission(t, f)
	if poll.context != secondModal.context {
		t.Fatal("unchanged-context poll invalidated a slow request")
	}
	identitySend(t, x, x.d.taskCh, secondModal.context, &asynq.TaskInfo{ID: "modal-second", Queue: current.queue, Type: "accepted-slow-poll", State: asynq.TaskStateActive})
	frame = identityBarrier(t, x)
	if !strings.Contains(frame, "accepted-slow-poll") {
		t.Fatal("same-context slow completion was rejected")
	}
	x.exit(t, tcell.KeyCtrlC, 0)
}

// A value-level logical query model tracks epochs independently of the
// production identity equality implementation. It deliberately ignores row
// highlights and non-detail fields and caps the task page capacity at one.
func identityEpochModel(t *testing.T, input []byte) {
	t.Helper()
	screen := renderingScreen(t, 100, 48)
	s := &State{}
	previous := "overview"
	epoch := uint64(0)
	height := 48
	if len(input) > 64 {
		input = input[:64]
	}
	for _, op := range input {
		switch op % 9 {
		case 0:
			s.view = viewTypeQueues
		case 1:
			s.view = viewTypeQueueDetails
			s.selectedQueue = &asynq.QueueInfo{Queue: "model-A"}
			s.pageNum = 1
			s.taskState = asynq.TaskStateActive
		case 2:
			s.selectedQueue = &asynq.QueueInfo{Queue: "model-B"}
		case 3:
			s.pageNum++
		case 4:
			s.taskState = asynq.TaskStatePending
		case 5:
			s.selectedGroup = &asynq.GroupInfo{Group: "model-group"}
		case 6:
			s.taskID = "model-modal"
		case 7:
			s.taskTableRowIdx++
		case 8:
			height = 1 + int(op/9)%45
			screen.SetSize(100, height)
		}
		logical := "overview"
		if s.view == viewTypeQueueDetails {
			q, g := "", ""
			if s.selectedQueue != nil {
				q = s.selectedQueue.Queue
			}
			if s.selectedGroup != nil {
				g = s.selectedGroup.Group
			}
			capacity := height - 15
			if capacity < 1 {
				capacity = 1
			}
			logical = fmt.Sprintf("details|%q|%q|%q|%d|%d|%d", q, g, s.taskID, s.taskState, s.pageNum, capacity)
		}
		if logical != previous {
			epoch++
			previous = logical
		}
		expected := fetchContext{view: s.view, epoch: epoch}
		if s.view == viewTypeQueueDetails {
			if s.selectedQueue != nil {
				expected.queue = s.selectedQueue.Queue
			}
			if s.selectedGroup != nil {
				expected.group = s.selectedGroup.Group
			}
			expected.taskState, expected.taskID, expected.page = s.taskState, s.taskID, s.pageNum
			expected.pageSize = height - 15
			if expected.pageSize < 1 {
				expected.pageSize = 1
			}
		}
		got := captureFetchContext(s, screen)
		if got != expected {
			t.Fatalf("request fields got%+v want%+v", got, expected)
		}
		if got.epoch != epoch {
			t.Fatalf("epoch model operation%d got%d want%d logical%s", op, got.epoch, epoch, logical)
		}
		again := captureFetchContext(s, screen)
		if got != again {
			t.Fatal("unchanged query/poll changed epoch")
		}
	}
}
func TestFetchIdentitySeededEpochModel(t *testing.T) {
	rng := rand.New(rand.NewSource(20261022))
	for n := 0; n < 128; n++ {
		input := make([]byte, 48)
		rng.Read(input)
		t.Run(fmt.Sprintf("sample-%03d", n), func(t *testing.T) { identityEpochModel(t, input) })
	}
}
func FuzzFetchIdentityEpochModel(f *testing.F) {
	f.Add([]byte{1, 2, 1, 0, 1})
	f.Add([]byte{1, 3, 4, 5, 6, 7, 8})
	f.Fuzz(func(t *testing.T, input []byte) { identityEpochModel(t, input) })
}

func TestFetchIdentityResizeReturnEpoch(t *testing.T) {
	x, f, _, old := identitySwitchToB(t)
	for _, height := range []int{50, 48} {
		after := x.s.serial.Load()
		x.s.SetSize(512, height)
		x.s.PostEventWait(tcell.NewEventResize(512, height))
		x.frameAfter(t, after, hasText("=== Queue Summary ==="))
	}
	identityTask(t, x, old, "old-before-resize")
	if strings.Contains(identityBarrier(t, x), "old-before-resize") {
		t.Fatal("resize ABA admitted old request")
	}
	x.ticks <- time.Now()
	current := identityAdmission(t, f)
	if current.context.epoch == old.context.epoch {
		t.Fatal("resize return lost visit epoch")
	}
	identityTask(t, x, current, "current-after-resize")
	if !strings.Contains(identityBarrier(t, x), "current-after-resize") {
		t.Fatal("post-resize request unavailable")
	}
	x.exit(t, tcell.KeyCtrlC, 0)
}
