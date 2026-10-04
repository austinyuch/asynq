package asynq

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	"github.com/austinyuch/asynq/internal/rdb"
	"github.com/austinyuch/asynq/internal/testbroker"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// Recovery must perform the same real transition after the initial Redis outage.
// A bounded observer records retry errors without replacing the real operation.
func TestProcessorOutageSyncRecoveryContracts(t *testing.T) {
	if useRedisCluster {
		t.Skip("owned standalone processor outage fixture")
	}
	actions := []string{"completed", "archived", "done", "retry", "completed-expired", "archived-expired", "done-expired", "retry-expired"}
	rng := rand.New(rand.NewSource(20261004))
	for i := 0; i < 12; i++ {
		actions = append(actions, actions[rng.Intn(4)])
	}
	for _, disposition := range actions {
		t.Run(disposition, func(t *testing.T) {
			expired := strings.HasSuffix(disposition, "-expired")
			disposition = strings.TrimSuffix(disposition, "-expired")
			ctx := context.Background()
			client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
			real := rdb.NewRDB(client)
			queue := fmt.Sprintf("processor-outage-%d", time.Now().UnixNano())
			msg := h.NewTaskMessageWithQueue("outage-contract", []byte("preserve-payload"), queue)
			payload := make([]byte, 1+rng.Intn(64))
			_, _ = rng.Read(payload)
			msg.Payload = payload
			msg.Retention = 60
			msg.UniqueKey = base.UniqueKey(queue, msg.Type, msg.Payload)
			keys := []string{base.TaskKey(queue, msg.ID), msg.UniqueKey, base.PendingKey(queue), base.ActiveKey(queue), base.LeaseKey(queue), base.CompletedKey(queue), base.ArchivedKey(queue), base.RetryKey(queue), base.ProcessedTotalKey(queue), base.FailedTotalKey(queue)}
			for _, day := range []time.Time{time.Now(), time.Now().Add(24 * time.Hour)} {
				keys = append(keys, base.ProcessedKey(queue, day), base.FailedKey(queue, day))
			}
			t.Cleanup(func() {
				for _, key := range keys {
					if err := client.Del(ctx, key).Err(); err != nil {
						t.Error(err)
					}
				}
				if err := client.SRem(ctx, base.AllQueues, queue).Err(); err != nil {
					t.Error(err)
				}
				_ = real.Close()
			})
			if err := real.EnqueueUnique(ctx, msg, time.Minute); err != nil {
				t.Fatal(err)
			}
			active, deadline, err := real.Dequeue(queue)
			if err != nil {
				t.Fatal(err)
			}
			before := map[string]string{}
			for _, key := range keys {
				v, e := client.Dump(ctx, key).Result()
				if e == nil {
					before[key] = v
				} else if !errors.Is(e, redis.Nil) {
					t.Fatal(e)
				}
			}
			broker := testbroker.NewTestBroker(real)
			broker.Sleep()
			requests := make(chan *syncRequest, 1)
			p := newProcessor(processorParams{logger: testLogger, broker: broker, syncCh: requests, concurrency: 1, queues: map[string]int{queue: 1}})
			p.retryDelayFunc = func(int, error, *Task) time.Duration { return time.Minute }
			if expired {
				deadline = time.Now().Add(50 * time.Millisecond)
			}
			lease := base.NewLease(deadline)
			if disposition == "completed" {
				p.handleSucceededMessage(lease, active)
			} else if disposition == "done" {
				p.markAsDone(lease, active)
			} else if disposition == "retry" {
				p.retry(lease, active, errors.New("handler-retry"), true)
			} else {
				p.archive(lease, active, errors.New("handler-terminal"))
			}
			var request *syncRequest
			select {
			case request = <-requests:
			case <-time.After(time.Second):
				t.Fatal("outage lost recovery request")
			}
			for key, want := range before {
				got, e := client.Dump(ctx, key).Result()
				if e != nil || got != want {
					t.Fatalf("initial failure partially changed %s: %v", key, e)
				}
			}
			if !request.deadline.Equal(deadline) {
				t.Fatal("request deadline differs from original lease snapshot")
			}
			if expired {
				lease.Reset(deadline.Add(time.Minute))
				if !request.deadline.Equal(deadline) {
					t.Fatal("lease reset extended stored request")
				}
				broker.Wakeup()
				time.Sleep(time.Until(deadline) + 5*time.Millisecond)
				if e := request.fn(); e == nil {
					t.Fatal("stored request wrote after captured deadline")
				}
				for key, want := range before {
					v, e := client.Dump(ctx, key).Result()
					if e != nil || v != want {
						t.Fatalf("expired recovery changed %s: %v", key, e)
					}
				}
				return
			}
			broker.Wakeup()
			observed := make(chan error, 20)
			original := request.fn
			request.fn = func() error {
				e := original()
				select {
				case observed <- e:
				default:
				}
				return e
			}
			syncRequests := make(chan *syncRequest, 1)
			syncer := newSyncer(syncerParams{logger: testLogger, requestsCh: syncRequests, interval: 10 * time.Millisecond})
			var wg sync.WaitGroup
			syncer.start(&wg)
			t.Cleanup(func() { syncer.shutdown(); wg.Wait() })
			syncRequests <- request
			var retryErr error
			select {
			case retryErr = <-observed:
			case <-time.After(time.Second):
				t.Fatal("real syncer did not retry")
			}
			if retryErr != nil {
				t.Fatalf("real syncer cannot recover after Redis wakes: %v (context canceled=%v)", retryErr, errors.Is(retryErr, context.Canceled))
			}
			info, e := real.GetTaskInfo(queue, msg.ID)
			want := base.TaskStateCompleted
			if disposition == "archived" {
				want = base.TaskStateArchived
			}
			if disposition == "retry" {
				want = base.TaskStateRetry
			}
			if disposition == "done" {
				if info != nil || client.Exists(ctx, base.TaskKey(queue, msg.ID)).Val() != 0 {
					t.Fatal("done recovery retained task")
				}
			} else if e != nil || info == nil || info.State != want || string(info.Message.Payload) != string(msg.Payload) {
				t.Fatalf("recovery lost terminal state/payload: %+v %v", info, e)
			}
			owner, ownerErr := client.Get(ctx, msg.UniqueKey).Result()
			if disposition == "completed" || disposition == "done" {
				if !errors.Is(ownerErr, redis.Nil) {
					t.Fatalf("terminal recovery retained unique owner: %q %v", owner, ownerErr)
				}
			} else if ownerErr != nil || owner != msg.ID {
				t.Fatalf("retry/archive changed unique owner: %q %v", owner, ownerErr)
			}
			if client.LLen(ctx, base.ActiveKey(queue)).Val() != 0 || client.ZCard(ctx, base.LeaseKey(queue)).Val() != 0 {
				t.Fatal("recovery retained active/lease ownership")
			}
		})
	}
}

func TestProcessorLeaseAndRequeueContracts(t *testing.T) {
	if useRedisCluster {
		t.Skip("owned standalone processor fixture")
	}
	for _, action := range []string{"expired-complete", "expired-done", "expired-retry", "expired-archive", "expired-requeue", "requeue-awake", "requeue-outage"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
			real := rdb.NewRDB(client)
			q := fmt.Sprintf("processor-lease-%d", time.Now().UnixNano())
			msg := h.NewTaskMessageWithQueue("lease-contract", []byte("preserved"), q)
			msg.UniqueKey = base.UniqueKey(q, msg.Type, msg.Payload)
			keys := []string{base.TaskKey(q, msg.ID), base.PendingKey(q), base.ActiveKey(q), base.LeaseKey(q), msg.UniqueKey}
			t.Cleanup(func() {
				for _, key := range keys {
					if e := client.Del(ctx, key).Err(); e != nil {
						t.Error(e)
					}
				}
				if e := client.SRem(ctx, base.AllQueues, q).Err(); e != nil {
					t.Error(e)
				}
				_ = real.Close()
			})
			if e := real.EnqueueUnique(ctx, msg, time.Minute); e != nil {
				t.Fatal(e)
			}
			active, deadline, e := real.Dequeue(q)
			if e != nil {
				t.Fatal(e)
			}
			before := map[string]string{}
			for _, key := range keys {
				v, e := client.Dump(ctx, key).Result()
				if e == nil {
					before[key] = v
				} else if !errors.Is(e, redis.Nil) {
					t.Fatal(e)
				}
			}
			broker := testbroker.NewTestBroker(real)
			requests := make(chan *syncRequest, 1)
			p := newProcessor(processorParams{logger: testLogger, broker: broker, syncCh: requests, concurrency: 1, queues: map[string]int{q: 1}})
			p.retryDelayFunc = func(int, error, *Task) time.Duration { return time.Minute }
			lease := base.NewLease(deadline)
			if action != "requeue-awake" && action != "requeue-outage" {
				lease = base.NewLease(time.Now().Add(-time.Second))
			}
			if action == "requeue-outage" {
				broker.Sleep()
			}
			switch action {
			case "expired-complete":
				p.markAsComplete(lease, active)
			case "expired-done":
				p.markAsDone(lease, active)
			case "expired-retry":
				p.retry(lease, active, errors.New("failure"), true)
			case "expired-archive":
				p.archive(lease, active, errors.New("failure"))
			default:
				p.requeue(lease, active)
			}
			if len(requests) != 0 {
				t.Fatal("expired lease or requeue queued unauthorized recovery")
			}
			if action == "requeue-awake" {
				info, e := real.GetTaskInfo(q, msg.ID)
				if e != nil || info.State != base.TaskStatePending || string(info.Message.Payload) != "preserved" {
					t.Fatalf("requeue state: %+v %v", info, e)
				}
				if client.LLen(ctx, base.ActiveKey(q)).Val() != 0 || client.ZCard(ctx, base.LeaseKey(q)).Val() != 0 || client.LLen(ctx, base.PendingKey(q)).Val() != 1 {
					t.Fatal("requeue ownership/index cardinality")
				}
				again, _, e := real.Dequeue(q)
				if e != nil || again.ID != msg.ID {
					t.Fatalf("requeued task not reclaimable: %+v %v", again, e)
				}
			} else {
				for key, want := range before {
					v, e := client.Dump(ctx, key).Result()
					if e != nil || v != want {
						t.Fatalf("protected active fixture changed: %s %v", key, e)
					}
				}
			}
			if owner, e := client.Get(ctx, msg.UniqueKey).Result(); e != nil || owner != msg.ID {
				t.Fatalf("lease/requeue altered owner: %s %v", owner, e)
			}
		})
	}
}

func TestProcessorShutdownRequeuesOwnedTask(t *testing.T) {
	if useRedisCluster {
		t.Skip("owned standalone shutdown fixture")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	real := rdb.NewRDB(client)
	q := fmt.Sprintf("processor-shutdown-%d", time.Now().UnixNano())
	msg := h.NewTaskMessageWithQueue("blocked-handler", []byte("preserved"), q)
	msg.UniqueKey = base.UniqueKey(q, msg.Type, msg.Payload)
	keys := []string{base.TaskKey(q, msg.ID), msg.UniqueKey, base.PendingKey(q), base.ActiveKey(q), base.LeaseKey(q)}
	t.Cleanup(func() {
		for _, key := range keys {
			if e := client.Del(ctx, key).Err(); e != nil {
				t.Error(e)
			}
		}
		if e := client.SRem(ctx, base.AllQueues, q).Err(); e != nil {
			t.Error(e)
		}
		_ = real.Close()
	})
	if e := real.EnqueueUnique(ctx, msg, time.Minute); e != nil {
		t.Fatal(e)
	}
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	handlerDone := make(chan struct{})
	requests := make(chan *syncRequest, 1)
	p := newProcessor(processorParams{logger: testLogger, broker: real, baseCtxFn: context.Background, concurrency: 1, queues: map[string]int{q: 1}, syncCh: requests, cancelations: base.NewCancelations(), shutdownTimeout: 20 * time.Millisecond, starting: make(chan *workerInfo, 1), finished: make(chan *base.TaskMessage, 1)})
	p.handler = HandlerFunc(func(ctx context.Context, _ *Task) error {
		defer close(handlerDone)
		started <- ctx
		<-release
		return nil
	})
	var wg sync.WaitGroup
	p.start(&wg)
	var workerCtx context.Context
	select {
	case workerCtx = <-started:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("worker never started")
	}
	shutdownDone := make(chan struct{})
	go func() { p.shutdown(); wg.Wait(); close(shutdownDone) }()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("shutdown exceeded bounded timeout")
	}
	select {
	case <-workerCtx.Done():
	case <-time.After(time.Second):
		close(release)
		t.Fatal("requeued worker context not canceled")
	}
	close(release)
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler cleanup timed out")
	}
	info, e := real.GetTaskInfo(q, msg.ID)
	if e != nil || info.State != base.TaskStatePending || string(info.Message.Payload) != "preserved" {
		t.Fatalf("shutdown changed task: %+v %v", info, e)
	}
	if client.LLen(ctx, base.PendingKey(q)).Val() != 1 || client.LLen(ctx, base.ActiveKey(q)).Val() != 0 || client.ZCard(ctx, base.LeaseKey(q)).Val() != 0 || len(requests) != 0 {
		t.Fatal("shutdown lost/duplicated task ownership")
	}
	if owner, e := client.Get(ctx, msg.UniqueKey).Result(); e != nil || owner != msg.ID {
		t.Fatalf("shutdown altered unique owner: %q %v", owner, e)
	}
}
