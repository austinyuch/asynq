package dash

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/redis/go-redis/v9"
)

func lifecycleAwait(t *testing.T, ch <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal(why)
	}
}

// Admission calls remain serial, as in the event loop. Entry and release
// channels hold all admitted work until the independent capacity oracle runs.
func TestFetchLifecycleAdmissionProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(20261016))
	for trial := 0; trial < 64; trial++ {
		t.Run(fmt.Sprint(trial), func(t *testing.T) {
			capacity := 4
			if trial > 0 {
				capacity = 1 + rng.Intn(4)
			}
			done := make(chan struct{})
			release := make(chan struct{})
			entered := make(chan struct{}, 32)
			f := &dataFetcher{done: done, slots: make(chan struct{}, capacity)}
			var started, active, maximum atomic.Int32
			work := func() {
				started.Add(1)
				n := active.Add(1)
				for old := maximum.Load(); n > old; old = maximum.Load() {
					if maximum.CompareAndSwap(old, n) {
						break
					}
				}
				entered <- struct{}{}
				<-release
				active.Add(-1)
			}
			var stopped, released, nextReleased bool
			var nextRelease chan struct{}
			var admissionReturns []<-chan struct{}
			t.Cleanup(func() {
				if !stopped {
					close(done)
				}
				if !released {
					close(release)
				}
				if nextRelease != nil && !nextReleased {
					close(nextRelease)
				}
				for _, returned := range admissionReturns {
					select {
					case <-returned:
					case <-time.After(time.Second):
						t.Error("saturated caller cleanup did not join")
						return
					}
				}
				f.wait()
			})
			for n := 0; n < capacity; n++ {
				f.launch(work)
				lifecycleAwait(t, entered, "admitted worker never entered")
			}
			// Launch on a single temporary event-loop caller to bound an accidental
			// blocking saturation implementation; cleanup releases any admitted work.
			for n := 0; n < 1+rng.Intn(6); n++ {
				returned := make(chan struct{})
				admissionReturns = append(admissionReturns, returned)
				go func() { f.launch(work); close(returned) }()
				lifecycleAwait(t, returned, "saturated admission blocked consumer")
			}
			if started.Load() != int32(capacity) || maximum.Load() != int32(capacity) || len(f.slots) != capacity {
				t.Fatalf("capacity exceeded: started%d max%d slots%d cap%d", started.Load(), maximum.Load(), len(f.slots), capacity)
			}
			close(release)
			released = true
			f.wait()
			if active.Load() != 0 || len(f.slots) != 0 {
				t.Fatal("completed work leaked active/slot ownership")
			}
			// A second stage proves released slots can be reused, rather than only
			// proving the initial admission filled the limiter.
			nextEntered := make(chan struct{})
			nextRelease = make(chan struct{})
			f.launch(func() { close(nextEntered); <-nextRelease })
			lifecycleAwait(t, nextEntered, "released capacity was not reusable")
			close(nextRelease)
			nextReleased = true
			f.wait()
			close(done)
			stopped = true
			var forbidden atomic.Int32
			f.launch(func() { forbidden.Add(1) })
			f.wait()
			if forbidden.Load() != 0 || len(f.slots) != 0 {
				t.Fatal("work admitted after done")
			}
		})
	}
}

func TestFetchLifecyclePublishContracts(t *testing.T) {
	value := &asynq.TaskInfo{ID: "identity", Type: "payload:contract", Payload: []byte{0, 255, 3}, State: asynq.TaskStateRetry}
	ch := make(chan *asynq.TaskInfo, 1)
	publish(ch, value, nil)
	if got := <-ch; got != value || !bytes.Equal(got.Payload, []byte{0, 255, 3}) {
		t.Fatal("publication lost pointer/payload/state identity")
	}
	done := make(chan struct{})
	close(done)
	for n := 0; n < 100; n++ {
		publish(ch, value, done)
		if len(ch) != 0 {
			t.Fatal("already-done publication beat cancellation")
		}
	}
	blocked := make(chan *asynq.TaskInfo)
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopNow := func() { stopOnce.Do(func() { close(stop) }) }
	entered := make(chan struct{})
	returned := make(chan struct{})
	go func() { close(entered); publish(blocked, value, stop); close(returned) }()
	t.Cleanup(func() {
		stopNow()
		select {
		case <-returned:
			return
		default:
		}
		select {
		case <-blocked:
		case <-time.After(time.Second):
		}
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Error("publisher cleanup could not join")
		}
	})
	lifecycleAwait(t, entered, "publisher never entered")
	stopNow()
	lifecycleAwait(t, returned, "cancellation did not unblock unbuffered publication")
	select {
	case <-blocked:
		t.Fatal("canceled publication delivered success")
	default:
	}
}

func TestFetchLifecycleClosedInspectorCancellation(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if e := c.Close(); e != nil {
		t.Fatal(e)
	}
	i := asynq.NewInspectorFromRedisClient(c)
	queues := make(chan []*asynq.QueueInfo, 1)
	queue := make(chan *asynq.QueueInfo, 1)
	groups := make(chan []*asynq.GroupInfo, 1)
	tasks := make(chan []*asynq.TaskInfo, 1)
	task := make(chan *asynq.TaskInfo, 1)
	cases := []struct {
		name string
		call func(chan error, <-chan struct{})
	}{
		{"queues", func(e chan error, d <-chan struct{}) { fetchQueues(i, queues, e, Options{}, d) }},
		{"queue", func(e chan error, d <-chan struct{}) { fetchQueueInfo(i, "owned", queue, e, d) }},
		{"groups", func(e chan error, d <-chan struct{}) { fetchGroups(i, "owned", groups, e, d) }},
		{"aggregating", func(e chan error, d <-chan struct{}) { fetchAggregatingTasks(i, "owned", "group", 2, 1, tasks, e, d) }},
		{"task", func(e chan error, d <-chan struct{}) { fetchTaskInfo(i, "owned", "id", task, e, d) }},
	}
	for _, state := range []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStatePending, asynq.TaskStateScheduled, asynq.TaskStateRetry, asynq.TaskStateArchived, asynq.TaskStateCompleted} {
		state := state
		cases = append(cases, struct {
			name string
			call func(chan error, <-chan struct{})
		}{state.String(), func(e chan error, d <-chan struct{}) { fetchTasks(i, "owned", state, 2, 1, tasks, e, d) }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// First preserve the real lazy/closed-client error without any Redis call.
			errorsCh := make(chan error, 1)
			tc.call(errorsCh, nil)
			select {
			case e := <-errorsCh:
				if e == nil || !strings.Contains(e.Error(), redis.ErrClosed.Error()) {
					t.Fatalf("semantic transport error lost: %v", e)
				}
			default:
				t.Fatal("closed inspector did not publish error")
			}
			done := make(chan struct{})
			close(done)
			tc.call(errorsCh, done)
			if len(errorsCh) != 0 {
				t.Fatal("already-canceled helper published error")
			}
			unbuffered := make(chan error)
			stop := make(chan struct{})
			entered := make(chan struct{})
			returned := make(chan struct{})
			var stopOnce sync.Once
			stopNow := func() { stopOnce.Do(func() { close(stop) }) }
			go func() { close(entered); tc.call(unbuffered, stop); close(returned) }()
			// Cleanup offers a receiver if a broken publisher ignores cancellation;
			// this prevents the negative assertion from leaving a dropped goroutine.
			t.Cleanup(func() {
				stopNow()
				select {
				case <-returned:
					return
				default:
				}
				select {
				case <-unbuffered:
				case <-time.After(time.Second):
				}
				select {
				case <-returned:
				case <-time.After(time.Second):
					t.Error("helper cleanup could not join")
				}
			})
			lifecycleAwait(t, entered, "helper never entered")
			stopNow()
			lifecycleAwait(t, returned, "helper stuck publishing after cancellation")
			if len(queues)+len(queue)+len(groups)+len(tasks)+len(task) != 0 {
				t.Fatal("closed transport helper published success metadata")
			}
		})
	}
}

func FuzzFetchLifecyclePublishPayload(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 255, 1})
	f.Add([]byte("中文 payload"))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 1024 {
			t.Skip("bounded payload")
		}
		expected := append([]byte(nil), payload...)
		ch := make(chan []byte, 1)
		publish(ch, payload, nil)
		if !bytes.Equal(<-ch, expected) || !bytes.Equal(payload, expected) {
			t.Fatal("publication changed arbitrary binary payload")
		}
		done := make(chan struct{})
		close(done)
		publish(ch, payload, done)
		if len(ch) != 0 {
			t.Fatal("completed cancellation published arbitrary payload")
		}
	})
}
