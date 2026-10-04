package asynq

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
)

func TestPeriodicFailurePathsContracts(t *testing.T) {
	sentinel := errors.New("configuration service unavailable")
	s := newScheduler(&SchedulerOpts{LogLevel: FatalLevel})
	cfg := &PeriodicTaskConfig{Cronspec: "@hourly", Task: NewTask("healthy", []byte("preserve"))}
	mgr := &PeriodicTaskManager{s: s, p: lifecycleProvider{}, m: make(map[string]string)}
	mgr.add([]*PeriodicTaskConfig{cfg})
	beforeMap := map[string]string{}
	for k, v := range mgr.m {
		beforeMap[k] = v
	}
	beforeEntries := s.cron.Entries()
	assertStable := func() {
		t.Helper()
		if !reflect.DeepEqual(mgr.m, beforeMap) || !reflect.DeepEqual(s.cron.Entries(), beforeEntries) {
			t.Fatal("failed update changed healthy schedule/mapping")
		}
	}
	for _, provider := range []PeriodicTaskConfigProvider{lifecycleProvider{err: sentinel}, lifecycleProvider{configs: []*PeriodicTaskConfig{nil}}, lifecycleProvider{configs: []*PeriodicTaskConfig{{Cronspec: "@hourly"}}}, lifecycleProvider{configs: []*PeriodicTaskConfig{{Task: NewTask("invalid", nil)}}}} {
		mgr.p = provider
		mgr.sync()
		assertStable()
	}
	mgr.add([]*PeriodicTaskConfig{{Cronspec: "invalid cron", Task: NewTask("invalid", nil)}})
	assertStable()
	mgr.remove(map[string]string{cfg.hash(): "unknown-entry"})
	assertStable()
	mgr.p = lifecycleProvider{err: sentinel}
	if err := mgr.Run(); !errors.Is(err, sentinel) {
		t.Fatalf("Run lost provider cause: %v", err)
	}
	assertStable()
	// Rejected scheduler startup must not launch a cron/heartbeat or sync goroutine.
	s.state.mu.Lock()
	s.state.value = srvStateClosed
	s.state.mu.Unlock()
	mgr.p = lifecycleProvider{}
	if err := mgr.Start(); err == nil || !strings.Contains(err.Error(), "already been stopped") {
		t.Fatalf("closed scheduler startup: %v", err)
	}
	assertStable()
	t.Run("uninitialized", func(t *testing.T) {
		defer func() {
			v := recover()
			if v == nil || !strings.Contains(v.(string), "uninitialized PeriodicTaskManager") {
				t.Fatalf("unexpected panic: %v", v)
			}
		}()
		(&PeriodicTaskManager{}).Start()
	})
}

type lifecycleRetryCall struct {
	ctx        context.Context
	msg        *base.TaskMessage
	at         time.Time
	err        string
	failure    bool
	contextErr error
}
type lifecycleBoundaryBroker struct {
	base.Broker
	msg   *base.TaskMessage
	retry chan lifecycleRetryCall
}

func (b *lifecycleBoundaryBroker) Dequeue(q ...string) (*base.TaskMessage, time.Time, error) {
	return b.msg, time.Now().Add(time.Minute), nil
}
func (b *lifecycleBoundaryBroker) Retry(ctx context.Context, msg *base.TaskMessage, at time.Time, e string, failure bool) error {
	b.retry <- lifecycleRetryCall{ctx, msg, at, e, failure, ctx.Err()}
	return nil
}

func TestProcessorCancellationFailurePathsContracts(t *testing.T) {
	for _, mode := range []string{"before-handler", "running-handler"} {
		t.Run(mode, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "before-handler" {
				cancel()
			}
			msg := &base.TaskMessage{ID: mode, Type: "canceled-contract", Queue: "owned", Payload: []byte("payload"), Retry: 3, Retried: 1, Timeout: 60}
			broker := &lifecycleBoundaryBroker{msg: msg, retry: make(chan lifecycleRetryCall, 1)}
			finished := make(chan *base.TaskMessage) // receive synchronizes with deferred worker cleanup
			p := newProcessor(processorParams{logger: testLogger, broker: broker, baseCtxFn: func() context.Context { return parent }, concurrency: 1, queues: map[string]int{"owned": 1}, cancelations: base.NewCancelations(), starting: make(chan *workerInfo, 1), finished: finished, syncCh: make(chan *syncRequest, 1), retryDelayFunc: func(n int, e error, task *Task) time.Duration {
				if n != 1 || !errors.Is(e, context.Canceled) || task.Type() != msg.Type || string(task.Payload()) != "payload" {
					t.Error("retry delay lost cancellation/task arguments")
				}
				return time.Minute
			}, isFailureFunc: func(e error) bool { return errors.Is(e, context.Canceled) }})
			entered := make(chan struct{})
			handlerDone := make(chan struct{})
			var calls atomic.Int32
			release := make(chan struct{})
			returned := make(chan struct{})
			t.Cleanup(func() {
				cancel()
				close(release)
				if calls.Load() > 0 {
					select {
					case <-returned:
					case <-time.After(time.Second):
						t.Error("handler cleanup timed out")
					}
				}
			})
			p.handler = HandlerFunc(func(ctx context.Context, _ *Task) error {
				calls.Add(1)
				close(entered)
				<-ctx.Done()
				close(handlerDone)
				<-release
				close(returned)
				return nil
			})
			p.exec()
			if mode == "running-handler" {
				select {
				case <-entered:
					cancel()
				case <-time.After(time.Second):
					t.Fatal("handler did not start")
				}
			}
			var retryContext context.Context
			select {
			case call := <-broker.retry:
				retryContext = call.ctx
				if call.msg != msg || call.err != context.Canceled.Error() || !call.failure || call.contextErr != nil {
					t.Fatalf("retry lost cancellation semantics: %+v", call)
				}
				if d := time.Until(call.at); d < 59*time.Second || d > 61*time.Second {
					t.Fatalf("retry schedule delta %v", d)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not retry")
			}
			select {
			case got := <-finished:
				if got != msg {
					t.Fatal("completion lost identity")
				}
			case <-time.After(time.Second):
				t.Fatal("worker did not finish")
			}
			// exec's final defer sends finished before releasing sema; acquire the released
			// token as a synchronization handshake, rather than timing-based polling.
			select {
			case p.sema <- struct{}{}:
				<-p.sema
			case <-time.After(time.Second):
				t.Fatal("worker leaked semaphore")
			}
			select {
			case <-retryContext.Done():
			default:
				t.Fatal("retry context not released after worker completion")
			}
			if _, ok := p.cancelations.Get(msg.ID); ok {
				t.Fatal("worker leaked cancellation registry")
			}
			want := int32(0)
			if mode == "running-handler" {
				want = 1
				select {
				case <-handlerDone:
				case <-time.After(time.Second):
					t.Fatal("handler context not canceled")
				}
			}
			if calls.Load() != want {
				t.Fatalf("handler calls %d want %d", calls.Load(), want)
			}
		})
	}
}

func TestBatchAndURIValidationFailurePathsContracts(t *testing.T) {
	c := &Client{} // storage tripwire; unsupported requests must fail before broker use.
	if got := c.BatchEnqueueContext(context.Background(), nil); len(got) != 0 {
		t.Fatalf("empty batch = %v", got)
	}
	for _, opt := range []Option{Group("group"), Unique(time.Minute)} {
		task := NewTask("valid", []byte("preserve"))
		result := c.BatchEnqueueContext(context.Background(), []*Task{task}, opt)
		if len(result) != 1 || result[0].Err == nil || !strings.Contains(result[0].Err.Error(), "batch enqueue does not support") {
			t.Fatalf("unsupported batch option %v: %+v", opt, result)
		}
		if task.Type() != "valid" || string(task.Payload()) != "preserve" {
			t.Fatal("rejection mutated task")
		}
	}
	if opt, err := ParseRedisURI("redis://host/%zz"); err == nil || opt != nil {
		t.Fatalf("invalid URI escaped parsing: %v %v", opt, err)
	}
}

type lifecycleBatchBroker struct {
	base.Broker
	items []base.BatchEnqueueItem
}

func (b *lifecycleBatchBroker) BatchEnqueue(ctx context.Context, items []base.BatchEnqueueItem) (int, error) {
	b.items = items
	return len(items), nil
}
func TestBatchDeadlineTimeoutBoundaryContracts(t *testing.T) {
	deadline := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	b := &lifecycleBatchBroker{}
	c := &Client{broker: b}
	task := NewTask("bounded", []byte("unaltered"))
	got := c.BatchEnqueueContext(context.Background(), []*Task{task}, Deadline(deadline), Timeout(37*time.Second), TaskID("fixed-id"), Queue("owned"))
	if len(b.items) != 1 || len(got) != 1 || got[0].Err != nil {
		t.Fatalf("batch result %+v items%v", got, b.items)
	}
	msg := b.items[0].Msg
	if msg.Deadline != deadline.Unix() || msg.Timeout != 37 || msg.ID != "fixed-id" || msg.Queue != "owned" || msg.Type != "bounded" || string(msg.Payload) != "unaltered" {
		t.Fatalf("batch options lost: %+v", msg)
	}
	if got[0].TaskInfo.ID != "fixed-id" || got[0].TaskInfo.State != TaskStatePending || got[0].TaskInfo.Timeout != 37*time.Second || !got[0].TaskInfo.Deadline.Equal(deadline) {
		t.Fatalf("public metadata mismatch: %+v", got[0].TaskInfo)
	}
}
