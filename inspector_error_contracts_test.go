package asynq

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
)

func TestInspectorErrorsClosedTransport(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	i := NewInspectorFromRedisClient(c)
	cases := []struct {
		name string
		call func() error
		wrap bool
	}{
		{"Queues", func() error {
			v, e := i.Queues()
			if v != nil {
				t.Error("queues on failure", v)
			}
			return e
		}, true},
		{"Groups", func() error {
			v, e := i.Groups("contract")
			if v != nil {
				t.Error("groups on failure", v)
			}
			return e
		}, true},
		{"QueueInfo", func() error {
			v, e := i.GetQueueInfo("contract")
			if v != nil {
				t.Error("queue info on failure", v)
			}
			return e
		}, true},
		{"History", func() error {
			v, e := i.History("contract", 2)
			if v != nil {
				t.Error("history on failure", v)
			}
			return e
		}, true},
		{"TaskInfo", func() error {
			v, e := i.GetTaskInfo("contract", "id")
			if v != nil {
				t.Error("task info on failure", v)
			}
			return e
		}, true},
		{"Pending", func() error {
			v, e := i.ListPendingTasks("contract")
			if v != nil {
				t.Error("pending on failure", v)
			}
			return e
		}, true},
		{"Active", func() error {
			v, e := i.ListActiveTasks("contract")
			if v != nil {
				t.Error("active on failure", v)
			}
			return e
		}, true},
		{"Aggregating", func() error {
			v, e := i.ListAggregatingTasks("contract", "group")
			if v != nil {
				t.Error("aggregating on failure", v)
			}
			return e
		}, true},
		{"Scheduled", func() error {
			v, e := i.ListScheduledTasks("contract")
			if v != nil {
				t.Error("scheduled on failure", v)
			}
			return e
		}, true},
		{"Retry", func() error {
			v, e := i.ListRetryTasks("contract")
			if v != nil {
				t.Error("retry on failure", v)
			}
			return e
		}, true},
		{"Archived", func() error {
			v, e := i.ListArchivedTasks("contract")
			if v != nil {
				t.Error("archived on failure", v)
			}
			return e
		}, true},
		{"Completed", func() error {
			v, e := i.ListCompletedTasks("contract")
			if v != nil {
				t.Error("completed on failure", v)
			}
			return e
		}, true},
		{"Update", func() error { return i.UpdateTaskPayload("contract", "id", []byte("new")) }, false},
		{"Delete", func() error { return i.DeleteTask("contract", "id") }, true},
		{"Run", func() error { return i.RunTask("contract", "id") }, true},
		{"Archive", func() error { return i.ArchiveTask("contract", "id") }, true},
		{"Servers", func() error {
			v, e := i.Servers()
			if v != nil {
				t.Error("servers on failure", v)
			}
			return e
		}, true},
		{"Scheduler", func() error {
			v, e := i.SchedulerEntries()
			if v != nil {
				t.Error("entries on failure", v)
			}
			return e
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil || errors.Is(err, ErrQueueNotFound) || errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("transport misclassified: %v", err)
			}
			// UpdateTaskPayload currently formats its cause with %v. This slice does
			// not invent a wrapping guarantee for that existing public behavior.
			if tc.wrap && !errors.Is(err, redis.ErrClosed) {
				t.Fatalf("closed cause lost: %v", err)
			}
			if !tc.wrap && !strings.Contains(err.Error(), redis.ErrClosed.Error()) {
				t.Fatalf("closed diagnostic lost: %v", err)
			}
		})
	}
}

type inspectorErrorFixture struct {
	r    redis.UniversalClient
	i    *Inspector
	q    string
	keys []string
}

func newInspectorErrorFixture(t *testing.T) *inspectorErrorFixture {
	t.Helper()
	if useRedisCluster {
		t.Skip("standalone owned-key failure fixture")
	}
	r := getRedisConnOpt(t).MakeRedisClient().(redis.UniversalClient)
	q := fmt.Sprintf("error-contract-%d", time.Now().UnixNano())
	f := &inspectorErrorFixture{r: r, i: NewInspectorFromRedisClient(r), q: q, keys: []string{base.PendingKey(q), base.ActiveKey(q), base.ScheduledKey(q), base.ArchivedKey(q), base.LeaseKey(q)}}
	t.Cleanup(func() {
		ctx := context.Background()
		for _, key := range f.keys {
			if err := r.Del(ctx, key).Err(); err != nil {
				t.Error(err)
			}
		}
		if err := r.SRem(ctx, base.AllQueues, q).Err(); err != nil {
			t.Error(err)
		}
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}
func (f *inspectorErrorFixture) seed(t *testing.T, state string) *base.TaskMessage {
	t.Helper()
	m := h.NewTaskMessageWithQueue("contract-"+state, []byte("preserved-"+state), f.q)
	f.keys = append(f.keys, base.TaskKey(f.q, m.ID))
	entry := []base.Z{{Message: m, Score: time.Now().Add(time.Hour).Unix()}}
	switch state {
	case "pending":
		h.SeedPendingQueue(t, f.r, []*base.TaskMessage{m}, f.q)
	case "active":
		h.SeedActiveQueue(t, f.r, []*base.TaskMessage{m}, f.q)
	case "scheduled":
		h.SeedScheduledQueue(t, f.r, entry, f.q)
	case "archived":
		h.SeedArchivedQueue(t, f.r, entry, f.q)
	default:
		t.Fatal(state)
	}
	return m
}
func (f *inspectorErrorFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	values := map[string]string{}
	for _, key := range f.keys {
		v, err := f.r.Dump(context.Background(), key).Result()
		if err == redis.Nil {
			values[key] = "<absent>"
		} else if err != nil {
			t.Fatal(err)
		} else {
			values[key] = v
		}
	}
	return values
}

func TestInspectorErrorsDomainIsolation(t *testing.T) {
	f := newInspectorErrorFixture(t)
	pending := f.seed(t, "pending")
	active := f.seed(t, "active")
	archived := f.seed(t, "archived")
	f.seed(t, "scheduled")
	before := f.snapshot(t)
	actions := []struct {
		name string
		call func(string, string) error
	}{
		{"Get", func(q, id string) error { _, e := f.i.GetTaskInfo(q, id); return e }},
		{"Update", func(q, id string) error { return f.i.UpdateTaskPayload(q, id, []byte("must-not-write")) }},
		{"Delete", f.i.DeleteTask}, {"Run", f.i.RunTask}, {"Archive", f.i.ArchiveTask},
	}
	rng := rand.New(rand.NewSource(20261002))
	for _, op := range actions {
		for trial := 0; trial < 12; trial++ {
			id := fmt.Sprintf("missing-%016x", rng.Uint64())
			if err := op.call(f.q+"-absent", id); !errors.Is(err, ErrQueueNotFound) || errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("%s missing queue classification: %v", op.name, err)
			}
			if err := op.call(f.q, id); !errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrQueueNotFound) {
				t.Fatalf("%s missing task classification: %v", op.name, err)
			}
		}
	}
	for _, op := range []struct {
		name, id string
		call     func(string, string) error
	}{{"update pending", pending.ID, actions[1].call}, {"delete active", active.ID, f.i.DeleteTask}, {"run pending", pending.ID, f.i.RunTask}, {"archive archived", archived.ID, f.i.ArchiveTask}} {
		err := op.call(f.q, op.id)
		if err == nil || errors.Is(err, ErrQueueNotFound) || errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("%s existing wrong state misclassified: %v", op.name, err)
		}
	}
	if !reflect.DeepEqual(before, f.snapshot(t)) {
		t.Fatal("rejected operations changed neighboring state/payload bytes")
	}
	info, err := f.i.GetTaskInfo(f.q, pending.ID)
	if err != nil || info.State != TaskStatePending || string(info.Payload) != "preserved-pending" {
		t.Fatalf("healthy neighbor unavailable: %+v %v", info, err)
	}
}

func TestInspectorErrorsLeaseIsolation(t *testing.T) {
	f := newInspectorErrorFixture(t)
	active := f.seed(t, "active")
	neighbor := f.seed(t, "pending")
	ctx := context.Background()
	// Listing active messages succeeds; only the subsequent lease lookup fails.
	if err := f.r.Set(ctx, base.LeaseKey(f.q), "wrong-type", 0).Err(); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t)
	tasks, err := f.i.ListActiveTasks(f.q)
	if err == nil || tasks != nil || !strings.Contains(strings.ToUpper(err.Error()), "WRONGTYPE") || errors.Is(err, ErrQueueNotFound) || errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("lease failure hidden/misclassified: %v %v", tasks, err)
	}
	if !reflect.DeepEqual(before, f.snapshot(t)) {
		t.Fatal("read failure mutated healthy neighbor or active task")
	}
	info, err := f.i.GetTaskInfo(f.q, neighbor.ID)
	if err != nil || info.State != TaskStatePending || string(info.Payload) != "preserved-pending" {
		t.Fatalf("healthy pending neighbor lost: %+v %v", info, err)
	}
	if info, err := f.i.GetTaskInfo(f.q, active.ID); err != nil || info.State != TaskStateActive {
		t.Fatalf("active task lost: %+v %v", info, err)
	}
}
