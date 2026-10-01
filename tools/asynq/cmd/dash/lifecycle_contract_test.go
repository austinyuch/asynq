package dash

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
)

type lifecycleFrame struct {
	seq  int64
	text string
}
type lifecycleScreen struct {
	tcell.SimulationScreen
	frames    chan lifecycleFrame
	pumpEnded chan struct{}
	abortPump chan struct{}
	abortOnce sync.Once
	finiCount atomic.Int32
	syncCount atomic.Int32
	serial    atomic.Int64
}

func (s *lifecycleScreen) Show() {
	s.SimulationScreen.Show()
	cells, w, h := s.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := ' '
			cell := cells[y*w+x]
			if len(cell.Runes) > 0 {
				r = cell.Runes[0]
			}
			b.WriteRune(r)
		}
		b.WriteByte('\n')
	}
	select {
	case s.frames <- lifecycleFrame{s.serial.Add(1), b.String()}:
	default:
	}
}
func (s *lifecycleScreen) Sync()         { s.syncCount.Add(1); s.SimulationScreen.Sync() }
func (s *lifecycleScreen) Fini()         { s.finiCount.Add(1); s.SimulationScreen.Fini() }
func (s *lifecycleScreen) requestAbort() { s.abortOnce.Do(func() { close(s.abortPump) }) }
func (s *lifecycleScreen) ChannelEvents(ch chan<- tcell.Event, done <-chan struct{}) {
	derivedStop := make(chan struct{})
	relayEnded := make(chan struct{})
	go func() {
		defer close(relayEnded)
		select {
		case <-done:
		case <-s.abortPump:
		}
		close(derivedStop)
	}()
	defer func() { s.requestAbort(); <-relayEnded; close(s.pumpEnded) }()
	s.SimulationScreen.ChannelEvents(ch, derivedStop)
}

type lifecycleFetchRequest struct {
	view      viewType
	queue, id string
	state     asynq.TaskState
}
type lifecycleFetcher struct {
	requests chan lifecycleFetchRequest
	count    atomic.Int32
}

func (f *lifecycleFetcher) Fetch(s *State) {
	f.count.Add(1)
	q := ""
	if s.selectedQueue != nil {
		q = s.selectedQueue.Queue
	}
	f.requests <- lifecycleFetchRequest{s.view, q, s.taskID, s.taskState}
}

type lifecycleFixture struct {
	s            *lifecycleScreen
	d            *dashboard
	f            *lifecycleFetcher
	returned     chan struct{}
	cleanups     atomic.Int32
	cleanupOrder atomic.Bool
	ticks        chan time.Time
}

func newLifecycleFixture(t *testing.T) *lifecycleFixture {
	t.Helper()
	s := &lifecycleScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), frames: make(chan lifecycleFrame, 512), pumpEnded: make(chan struct{}), abortPump: make(chan struct{})}
	if e := s.Init(); e != nil {
		t.Fatal(e)
	}
	s.SetSize(512, 48)
	f := &lifecycleFetcher{requests: make(chan lifecycleFetchRequest, 512)}
	x := &lifecycleFixture{s: s, d: newDashboard(s, Options{PollInterval: time.Hour, DebugMode: true}), f: f, returned: make(chan struct{}), ticks: make(chan time.Time, 1)}
	x.d.ticks = x.ticks
	go func() {
		defer close(x.returned)
		x.d.run(f, func() {
			x.cleanups.Add(1)
			select {
			case <-s.pumpEnded:
				select {
				case <-x.d.done:
					x.cleanupOrder.Store(s.finiCount.Load() == 0)
				default:
				}
			default:
			}
		})
	}()
	t.Cleanup(func() {
		select {
		case <-x.returned:
		default:
			x.s.requestAbort()
			select {
			case <-x.returned:
			case <-time.After(2 * time.Second):
				t.Error("runner cleanup deadline")
			}
		}
	})
	x.frame(t, func(s string) bool { return strings.Contains(s, "=== Queues ===") })
	return x
}
func (x *lifecycleFixture) frame(t *testing.T, accept func(string) bool) string {
	return x.frameAfter(t, 0, accept)
}
func (x *lifecycleFixture) frameAfter(t *testing.T, after int64, accept func(string) bool) string {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	last := ""
	for {
		select {
		case s := <-x.s.frames:
			last = s.text
			if s.seq > after && accept(s.text) {
				return s.text
			}
		case <-x.returned:
			t.Fatal("runner exited before expected rendered frame")
		case <-timer.C:
			t.Fatalf("render handshake deadline; last frame: %s", last)
		}
	}
}
func (x *lifecycleFixture) key(t *testing.T, key tcell.Key, r rune, accept func(string) bool) string {
	t.Helper()
	after := x.s.serial.Load()
	sent := make(chan struct{})
	go func() { x.s.PostEventWait(tcell.NewEventKey(key, r, tcell.ModNone)); close(sent) }()
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("event queue admission deadline")
	}
	return x.frameAfter(t, after, accept)
}

// Publication runs in the test goroutine, so no sender can survive a timeout.
// These selects are deliberately independent of production publish.
func lifecycleSend[T any](t *testing.T, x *lifecycleFixture, ch chan<- T, value T) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case ch <- value:
	case <-x.s.abortPump:
		t.Fatal("publication aborted")
	case <-x.returned:
		t.Fatal("runner exited before publication")
	case <-timer.C:
		t.Fatal("result admission deadline")
	}
}
func (x *lifecycleFixture) exit(t *testing.T, key tcell.Key, r rune) {
	t.Helper()
	x.s.PostEventWait(tcell.NewEventKey(key, r, tcell.ModNone))
	select {
	case <-x.returned:
	case <-time.After(2 * time.Second):
		t.Fatal("quit did not return")
	}
	if x.cleanups.Load() != 1 || x.s.finiCount.Load() != 1 || !x.cleanupOrder.Load() {
		t.Fatalf("cleanup/pump/Fini ordering: cleanup=%d Fini=%d order=%v", x.cleanups.Load(), x.s.finiCount.Load(), x.cleanupOrder.Load())
	}
	select {
	case <-x.s.pumpEnded:
	default:
		t.Fatal("runner returned with live event pump")
	}
	returned := make(chan struct{})
	go func() { x.s.PostEventWait(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)); close(returned) }()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("post-exit event producer leaked")
	}
}
func hasText(text string) func(string) bool {
	return func(s string) bool { return strings.Contains(s, text) }
}

func TestDashboardLifecycleNavigationProperty(t *testing.T) {
	for _, quit := range []struct {
		name string
		key  tcell.Key
		r    rune
	}{{"top-q", tcell.KeyRune, 'q'}, {"control-c", tcell.KeyCtrlC, 0}} {
		t.Run(quit.name, func(t *testing.T) {
			x := newLifecycleFixture(t)
			queues := []*asynq.QueueInfo{{Queue: "owned-alpha", Active: 2, Size: 2}, {Queue: "owned-beta", Active: 1, Size: 1}}
			lifecycleSend(t, x, x.d.queuesCh, queues)
			x.frame(t, hasText("owned-beta"))
			// Fixed-seed user navigation model: the header is row zero; selection cycles
			// through two known queue rows. This oracle is independent of handler code.
			rng := rand.New(rand.NewSource(20261006))
			selected := 0
			for n := 0; n < 32; n++ {
				down := rng.Intn(2) == 0
				if down {
					selected = (selected + 1) % 3
				} else {
					selected = (selected + 2) % 3
				}
				r := 'j'
				if !down {
					r = 'k'
				}
				x.key(t, tcell.KeyRune, r, hasText(fmt.Sprintf("queueTableRowIdx=%d ", selected)))
			}
			x.key(t, tcell.KeyRune, '?', hasText("=== Help ==="))
			x.key(t, tcell.KeyRune, 'q', hasText("=== Queues ==="))
			for selected != 1 {
				selected = (selected + 1) % 3
				x.key(t, tcell.KeyRune, 'j', hasText(fmt.Sprintf("queueTableRowIdx=%d ", selected)))
			}
			x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
			tasks := []*asynq.TaskInfo{{ID: "owned-task-one", Queue: "owned-alpha", Type: "owned-type", Payload: []byte("owned-payload"), State: asynq.TaskStateActive}, {ID: "owned-task-two", Queue: "owned-alpha", Type: "owned-type", State: asynq.TaskStateActive}}
			lifecycleSend(t, x, x.d.tasksCh, tasks)
			x.frame(t, hasText("owned-task-two"))
			x.key(t, tcell.KeyRune, 'j', hasText("taskTableRowIdx=1 "))
			x.key(t, tcell.KeyEnter, 0, hasText("=== Task Info ==="))
			x.key(t, tcell.KeyEscape, 0, func(s string) bool {
				return strings.Contains(s, "=== Queue Summary ===") && !strings.Contains(s, "=== Task Info ===")
			})
			x.key(t, tcell.KeyRune, 'q', hasText("=== Queues ==="))
			x.exit(t, quit.key, quit.r)
		})
	}
}

func TestDashboardLifecycleResultChannelsAndResize(t *testing.T) {
	x := newLifecycleFixture(t)
	queues := []*asynq.QueueInfo{{Queue: "owned-alpha", Active: 2}, {Queue: "owned-beta", Active: 2}}
	lifecycleSend(t, x, x.d.queuesCh, queues)
	x.frame(t, hasText("owned-beta"))
	x.key(t, tcell.KeyRune, 'j', hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyRune, 'j', hasText("queueTableRowIdx=2 "))
	lifecycleSend(t, x, x.d.queuesCh, queues[:1])
	x.frame(t, hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	lifecycleSend(t, x, x.d.queueCh, &asynq.QueueInfo{Queue: "queue-update-oracle", Active: 2})
	x.frame(t, hasText("queue-update-oracle"))
	tasks := []*asynq.TaskInfo{{ID: "result-one", Queue: "queue-update-oracle", Type: "oracle-type", State: asynq.TaskStateActive}, {ID: "result-two", Queue: "queue-update-oracle", Type: "oracle-type", State: asynq.TaskStateActive}}
	lifecycleSend(t, x, x.d.tasksCh, tasks)
	x.frame(t, hasText("result-two"))
	x.key(t, tcell.KeyRune, 'j', hasText("taskTableRowIdx=1 "))
	x.key(t, tcell.KeyRune, 'j', hasText("taskTableRowIdx=2 "))
	lifecycleSend(t, x, x.d.tasksCh, tasks[:1])
	x.frame(t, hasText("taskTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Task Info ==="))
	lifecycleSend(t, x, x.d.taskCh, &asynq.TaskInfo{ID: "result-one", Queue: "queue-update-oracle", Type: "updated-modal-oracle", State: asynq.TaskStateActive})
	x.frame(t, hasText("updated-modal-oracle"))
	lifecycleSend(t, x, x.d.errorCh, asynq.ErrTaskNotFound)
	x.frame(t, hasText("no longer exists"))
	lifecycleSend(t, x, x.d.taskCh, tasks[0])
	x.frame(t, hasText("oracle-type"))
	lifecycleSend(t, x, x.d.errorCh, errors.New("owned-transport-error"))
	x.frame(t, hasText("owned-transport-error"))
	x.key(t, tcell.KeyEscape, 0, func(s string) bool {
		return !strings.Contains(s, "=== Task Info ===") && strings.Contains(s, "=== Queue Summary ===")
	})
	groups := []*asynq.GroupInfo{{Group: "group-one", Size: 1}, {Group: "group-two", Size: 2}}
	// The groups result is valid independent of the current visible tab; force a
	// follow-up help render as acknowledgement without exposing shared State.
	lifecycleSend(t, x, x.d.groupsCh, groups)
	x.frame(t, hasText("len(groups)=2 "))
	x.key(t, tcell.KeyRight, 0, hasText("taskState=pending "))
	x.key(t, tcell.KeyRight, 0, hasText("taskState=aggregating "))
	x.key(t, tcell.KeyRune, 'j', hasText("groupTableRowIdx=1 "))
	x.key(t, tcell.KeyRune, 'j', hasText("groupTableRowIdx=2 "))
	lifecycleSend(t, x, x.d.groupsCh, groups[:1])
	x.frame(t, hasText("groupTableRowIdx=1 "))
	previous := x.f.count.Load()
	tickAfter := x.s.serial.Load()
	x.ticks <- time.Now()
	x.frameAfter(t, tickAfter, func(string) bool { return x.f.count.Load() == previous+1 })
	after := x.s.serial.Load()
	x.s.PostEventWait(tcell.NewEventResize(512, 48))
	x.frameAfter(t, after, func(string) bool { return x.s.syncCount.Load() > 0 })
	x.exit(t, tcell.KeyCtrlC, 0)
}

func TestDashboardLifecycleClosedEventPumpReturns(t *testing.T) {
	x := newLifecycleFixture(t)
	// Abort only the actual ChannelEvents pump via its derived stop channel.
	// The runner alone closes dashboard.done and finalizes the screen.
	x.s.requestAbort()
	select {
	case <-x.returned:
	case <-time.After(2 * time.Second):
		t.Fatal("closed event source spun instead of returning")
	}
	if x.cleanups.Load() != 1 || x.s.finiCount.Load() != 1 || !x.cleanupOrder.Load() {
		t.Fatal("closed-event shutdown lost cleanup ordering")
	}
}

func TestDashboardLifecycleRealInspectorWiring(t *testing.T) {
	// A separate canonical fixture Inspector supplies the oracle. Production
	// runWithScreen creates and owns its own Inspector/managed fetcher transport.
	fixture := newFetchContractFixture(t)
	before := fixture.snapshot(t)
	screen := &lifecycleScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), frames: make(chan lifecycleFrame, 512), pumpEnded: make(chan struct{}), abortPump: make(chan struct{})}
	if e := screen.Init(); e != nil {
		t.Fatal(e)
	}
	screen.SetSize(512, 48)
	x := &lifecycleFixture{s: screen, returned: make(chan struct{})}
	go func() {
		defer close(x.returned)
		runWithScreen(screen, Options{PollInterval: time.Hour, RedisConnOpt: asynq.RedisClientOpt{Addr: os.Getenv("ASYNQ_DASH_TEST_REDIS_ADDR"), DB: 12}})
	}()
	t.Cleanup(func() {
		select {
		case <-x.returned:
		default:
			screen.requestAbort()
			select {
			case <-x.returned:
			case <-time.After(2 * time.Second):
				t.Error("real wiring cleanup deadline")
			}
		}
	})
	x.frame(t, func(s string) bool {
		return strings.Contains(s, fixture.queues[0]) && strings.Contains(s, fixture.queues[1])
	})
	screen.PostEventWait(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
	select {
	case <-x.returned:
	case <-time.After(2 * time.Second):
		t.Fatal("real owned Inspector runner did not shut down")
	}
	if screen.finiCount.Load() != 1 {
		t.Fatal("real wiring finalized screen other than once")
	}
	select {
	case <-screen.pumpEnded:
	default:
		t.Fatal("real wiring leaked event pump")
	}
	if !reflect.DeepEqual(before, fixture.snapshot(t)) {
		t.Fatal("real dashboard changed independent fixture task/index bytes")
	}
}
