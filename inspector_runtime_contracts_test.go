package asynq

import (
	"context"
	"errors"
	"fmt"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/austinyuch/asynq/internal/rdb"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// Fixtures use unique names and exact cleanup; they never FlushDB or delete an
// inventory of keys. Existing neighboring queues and registry entries survive.
func inspectorRuntimeFixture(t *testing.T) (redis.UniversalClient, *Inspector, *rdb.RDB, string) {
	t.Helper()
	if useRedisCluster {
		t.Skip("standalone fixture")
	}
	r := getRedisConnOpt(t).MakeRedisClient().(redis.UniversalClient)
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	i := NewInspectorFromRedisClient(r)
	id := fmt.Sprintf("runtime-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if err := i.DeleteQueue(id, true); err != nil && !errors.Is(err, ErrQueueNotFound) {
			t.Error(err)
		}
	})
	return r, i, rdb.NewRDB(r), id
}

func TestInspectorRuntimeGroupTransitions(t *testing.T) {
	r, i, _, q := inspectorRuntimeFixture(t)
	rng := rand.New(rand.NewSource(20261001))
	ops := []struct {
		name    string
		call    func(string, string) (int, error)
		state   TaskState
		deleted bool
	}{
		{"delete", i.DeleteAllAggregatingTasks, 0, true}, {"run", i.RunAllAggregatingTasks, TaskStatePending, false}, {"archive", i.ArchiveAllAggregatingTasks, TaskStateArchived, false}}
	for _, op := range ops {
		if _, err := op.call("", "g"); err == nil {
			t.Fatalf("%s invalid queue", op.name)
		}
		for trial := 0; trial < 12; trial++ {
			group := fmt.Sprintf("%s-%d", op.name, trial)
			neighbor := group + "-neighbor"
			n := 1 + rng.Intn(8)
			entries := make([]base.Z, n)
			for j := range entries {
				m := h.NewTaskMessageWithQueue("contract", []byte(fmt.Sprintf("payload-%d", j)), q)
				m.GroupKey = group
				entries[j] = base.Z{Message: m, Score: int64(j + 1)}
			}
			other := h.NewTaskMessageWithQueue("neighbor", []byte("keep"), q)
			other.GroupKey = neighbor
			t.Cleanup(func() {
				if _, err := i.DeleteAllAggregatingTasks(q, group); err != nil {
					t.Error(err)
				}
				if _, err := i.DeleteAllAggregatingTasks(q, neighbor); err != nil {
					t.Error(err)
				}
			})
			h.SeedGroup(t, r, entries, q, group)
			h.SeedGroup(t, r, []base.Z{{Message: other, Score: 1}}, q, neighbor)
			count, err := op.call(q, group)
			if err != nil || count != n {
				t.Fatalf("%s count=%d err=%v want=%d", op.name, count, err, n)
			}
			for _, entry := range entries {
				info, err := i.GetTaskInfo(q, entry.Message.ID)
				if op.deleted {
					if !errors.Is(err, ErrTaskNotFound) {
						t.Fatalf("deleted task visible: %v %v", info, err)
					}
				} else if err != nil || info.State != op.state || string(info.Payload) != string(entry.Message.Payload) {
					t.Fatalf("%s state=%+v err=%v", op.name, info, err)
				}
			}
			info, err := i.GetTaskInfo(q, other.ID)
			if err != nil || info.State != TaskStateAggregating || string(info.Payload) != "keep" {
				t.Fatalf("neighbor changed: %+v %v", info, err)
			}
			count, err = op.call(q, group)
			if err != nil || count != 0 {
				t.Fatalf("repeat %s=%d %v", op.name, count, err)
			}
		}
	}
}

func TestInspectorRuntimeQueueControl(t *testing.T) {
	r, i, broker, q := inspectorRuntimeFixture(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if err := r.Del(ctx, base.PausedKey(q)).Err(); err != nil {
			t.Error(err)
		}
	})
	if err := i.PauseQueue(""); err == nil {
		t.Fatal("invalid pause accepted")
	}
	if err := i.UnpauseQueue(""); err == nil {
		t.Fatal("invalid resume accepted")
	}
	if err := i.PauseQueue(q); err != nil {
		t.Fatal(err)
	}
	if r.Exists(ctx, base.PausedKey(q)).Val() != 1 || r.Exists(ctx, base.PausedKey(q+"-neighbor")).Val() != 0 {
		t.Fatal("pause isolation failed")
	}
	if err := i.PauseQueue(q); err == nil {
		t.Fatal("repeated pause accepted")
	}
	if err := i.UnpauseQueue(q); err != nil {
		t.Fatal(err)
	}
	if r.Exists(ctx, base.PausedKey(q)).Val() != 0 {
		t.Fatal("pause retained")
	}
	if err := i.UnpauseQueue(q); err == nil {
		t.Fatal("repeated resume accepted")
	}
	sub, err := broker.CancelationPubSub()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	id := q + "-task"
	// A foreign cancellation deliberately arrives first on the shared channel.
	if err := i.CancelProcessing(q + "-unrelated"); err != nil {
		t.Fatal(err)
	}
	if err := i.CancelProcessing(id); err != nil {
		t.Fatal(err)
	}
	receiveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// CancelChannel is shared across logical Redis DBs. Ignore unrelated IDs
	// under the original deadline; serial endpoint gates still protect legacy
	// subscribers which expect an exact global message set.
	for {
		msg, err := sub.ReceiveMessage(receiveCtx)
		if err != nil {
			t.Fatalf("own cancellation signal missing: %v", err)
		}
		if msg.Payload == id {
			if msg.Channel != base.CancelChannel {
				t.Fatalf("unexpected channel: %s", msg.Channel)
			}
			break
		}
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := i.CancelProcessing(q + "-missing"); err != nil {
		t.Fatal(err)
	}
}

func TestInspectorRuntimeServers(t *testing.T) {
	_, i, broker, id := inspectorRuntimeFixture(t)
	started := time.Now().UTC().Truncate(time.Second)
	info := &base.ServerInfo{Host: "contract-host", PID: 42, ServerID: id, Concurrency: 3, Queues: map[string]int{"a": 2, "b": 1}, StrictPriority: true, Status: "active", Started: started}
	worker := &base.WorkerInfo{ServerID: id, ID: "worker", Type: "contract", Payload: []byte("payload"), Queue: "a", Started: started, Deadline: started.Add(time.Minute)}
	orphan := &base.WorkerInfo{ServerID: id + "-absent", ID: "orphan", Type: "orphan", Queue: "b"}
	t.Cleanup(func() {
		if err := broker.ClearServerState(info.Host, info.PID, id); err != nil {
			t.Error(err)
		}
	})
	if err := broker.WriteServerState(info, []*base.WorkerInfo{worker, orphan}, time.Minute); err != nil {
		t.Fatal(err)
	}
	servers, err := i.Servers()
	if err != nil {
		t.Fatal(err)
	}
	var got *ServerInfo
	for _, s := range servers {
		if s.ID == id {
			got = s
		}
	}
	if got == nil {
		t.Fatal("server missing")
	}
	if got.Host != info.Host || got.PID != info.PID || got.Concurrency != info.Concurrency || !reflect.DeepEqual(got.Queues, info.Queues) || got.StrictPriority != info.StrictPriority || got.Status != info.Status || !got.Started.Equal(started) {
		t.Fatalf("metadata=%+v", got)
	}
	if len(got.ActiveWorkers) != 1 {
		t.Fatalf("orphan leaked: %+v", got.ActiveWorkers)
	}
	w := got.ActiveWorkers[0]
	if w.TaskID != worker.ID || w.TaskType != worker.Type || string(w.TaskPayload) != string(worker.Payload) || w.Queue != worker.Queue || !w.Started.Equal(worker.Started) || !w.Deadline.Equal(worker.Deadline) {
		t.Fatalf("worker=%+v", w)
	}
}

func TestInspectorRuntimeSchedulerEvents(t *testing.T) {
	r, i, broker, id := inspectorRuntimeFixture(t)
	ctx := context.Background()
	epoch := time.Now().UTC().Truncate(time.Second)
	const n = 37
	t.Cleanup(func() {
		if err := r.Del(ctx, base.SchedulerHistoryKey(id), base.SchedulerHistoryKey(id+"-bad")).Err(); err != nil {
			t.Error(err)
		}
	})
	for _, j := range rand.New(rand.NewSource(42)).Perm(n) {
		if err := broker.RecordSchedulerEnqueueEvent(id, &base.SchedulerEnqueueEvent{TaskID: fmt.Sprintf("task-%02d", j), EnqueuedAt: epoch.Add(time.Duration(j) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := i.ListSchedulerEnqueueEvents(id)
	if err != nil || len(first) != 30 {
		t.Fatalf("default=%d %v", len(first), err)
	}
	for size := 1; size <= 9; size++ {
		var all []*SchedulerEnqueueEvent
		for page := 1; page < 50; page++ {
			events, err := i.ListSchedulerEnqueueEvents(id, PageSize(size), Page(page))
			if err != nil {
				t.Fatal(err)
			}
			all = append(all, events...)
			if len(events) < size {
				break
			}
		}
		if len(all) != n {
			t.Fatalf("size %d union=%d", size, len(all))
		}
		for j, event := range all {
			want := n - 1 - j
			if event.TaskID != fmt.Sprintf("task-%02d", want) || !event.EnqueuedAt.Equal(epoch.Add(time.Duration(want)*time.Second)) {
				t.Fatalf("size %d order[%d]=%+v", size, j, event)
			}
		}
	}
	if events, err := i.ListSchedulerEnqueueEvents(id + "-missing"); err != nil || len(events) != 0 {
		t.Fatalf("missing=%v %v", events, err)
	}
	if err := r.Set(ctx, base.SchedulerHistoryKey(id+"-bad"), "wrong-type", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if events, err := i.ListSchedulerEnqueueEvents(id + "-bad"); err == nil || events != nil {
		t.Fatalf("error hidden: %v %v", events, err)
	}
}
