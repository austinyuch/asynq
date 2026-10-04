package rdb

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	canonical "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
)

type removalFixture struct {
	r      *RDB
	keys   []string
	queues []string
}

func newRemovalFixture(t *testing.T) *removalFixture {
	t.Helper()
	var client redis.UniversalClient
	if useRedisCluster {
		client = redis.NewClusterClient(&redis.ClusterOptions{Addrs: strings.Split(redisClusterAddrs, ",")})
	} else {
		client = redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	}
	f := &removalFixture{r: NewRDB(client)}
	t.Cleanup(func() {
		ctx := context.Background()
		// Each key is registered at creation, including keys expected to be deleted
		// by RemoveQueue. Separate DEL commands also support different cluster slots.
		for _, key := range f.keys {
			if err := client.Del(ctx, key).Err(); err != nil {
				t.Error(err)
			}
		}
		for _, q := range f.queues {
			if err := client.SRem(ctx, base.AllQueues, q).Err(); err != nil {
				t.Error(err)
			}
		}
		if err := f.r.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}
func (f *removalFixture) queue(t *testing.T) string {
	t.Helper()
	q := fmt.Sprintf("removal-%d-%d", time.Now().UnixNano(), len(f.queues))
	f.queues = append(f.queues, q)
	f.keys = append(f.keys, base.PendingKey(q), base.ActiveKey(q), base.ScheduledKey(q), base.RetryKey(q), base.ArchivedKey(q), base.CompletedKey(q), base.LeaseKey(q), base.AllGroups(q), base.AllAggregationSets(q))
	return q
}
func (f *removalFixture) seed(t *testing.T, q, state string, n int) []*base.TaskMessage {
	t.Helper()
	msgs := make([]*base.TaskMessage, n)
	entries := make([]base.Z, n)
	for j := range msgs {
		m := h.NewTaskMessageWithQueue("removal-"+state, []byte(fmt.Sprintf("payload-%d", j)), q)
		m.UniqueKey = base.UniqueKey(q, m.Type, m.Payload)
		msgs[j] = m
		entries[j] = base.Z{Message: m, Score: int64(j + 1)}
		f.keys = append(f.keys, base.TaskKey(q, m.ID), m.UniqueKey)
		if err := f.r.client.Set(context.Background(), m.UniqueKey, m.ID, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
	}
	switch state {
	case "pending":
		h.SeedPendingQueue(t, f.r.client, msgs, q)
	case "active":
		h.SeedActiveQueue(t, f.r.client, msgs, q)
	case "scheduled":
		h.SeedScheduledQueue(t, f.r.client, entries, q)
	case "retry":
		h.SeedRetryQueue(t, f.r.client, entries, q)
	case "archived":
		h.SeedArchivedQueue(t, f.r.client, entries, q)
	case "completed":
		h.SeedCompletedQueue(t, f.r.client, entries, q)
	case "group":
		f.keys = append(f.keys, base.GroupKey(q, "group"))
		h.SeedGroup(t, f.r.client, entries, q, "group")
	case "staged":
		key := base.AggregationSetKey(q, "stage", "set")
		f.keys = append(f.keys, key)
		h.SeedAggregationSet(t, f.r.client, entries, q, "stage", "set")
		if err := f.r.client.ZAdd(context.Background(), base.AllAggregationSets(q), redis.Z{Score: float64(time.Now().Add(time.Minute).Unix()), Member: key}).Err(); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unknown state", state)
	}
	return msgs
}
func removalSnapshot(t *testing.T, r *RDB, keys []string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, key := range keys {
		value, err := r.client.Dump(context.Background(), key).Result()
		if err == redis.Nil {
			out[key] = "<absent>"
		} else if err != nil {
			t.Fatal(err)
		} else {
			kind, err := r.client.Type(context.Background(), key).Result()
			if err != nil {
				t.Fatal(err)
			}
			// DUMP can change set iteration order without changing persisted members.
			// Quote sorted strings so arbitrary member bytes remain distinguishable.
			if kind == "set" {
				members, err := r.client.SMembers(context.Background(), key).Result()
				if err != nil {
					t.Fatal(err)
				}
				sort.Strings(members)
				out[key] = "set:" + fmt.Sprintf("%q", members)
			} else {
				out[key] = value
			}
		}
	}
	return out
}
func TestQueueRemovalContractsNonForce(t *testing.T) {
	f := newRemovalFixture(t)
	for _, state := range []string{"pending", "active", "scheduled", "retry", "archived", "completed", "group", "staged"} {
		t.Run(state, func(t *testing.T) {
			q := f.queue(t)
			f.seed(t, q, state, 2)
			keys := append([]string{base.AllQueues}, f.keys...)
			before := removalSnapshot(t, f.r, keys)
			err := f.r.RemoveQueue(q, false)
			if !canonical.IsQueueNotEmpty(err) {
				t.Fatalf("%s removal=%v want QueueNotEmpty", state, err)
			}
			if after := removalSnapshot(t, f.r, keys); !reflect.DeepEqual(before, after) {
				t.Fatalf("non-force changed %s persisted bytes", state)
			}
		})
	}
	// Stale indexes still describe a nonempty queue to administrators. They may
	// reference empty groups/sets, and must not be silently discarded non-force.
	for _, index := range []string{"group-index", "set-index"} {
		q := f.queue(t)
		ctx := context.Background()
		if err := f.r.client.SAdd(ctx, base.AllQueues, q).Err(); err != nil {
			t.Fatal(err)
		}
		if index == "group-index" {
			if err := f.r.client.SAdd(ctx, base.AllGroups(q), "empty-group").Err(); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := f.r.client.ZAdd(ctx, base.AllAggregationSets(q), redis.Z{Score: 1, Member: base.AggregationSetKey(q, "empty", "set")}).Err(); err != nil {
				t.Fatal(err)
			}
		}
		before := removalSnapshot(t, f.r, append([]string{base.AllQueues}, f.keys...))
		if err := f.r.RemoveQueue(q, false); !canonical.IsQueueNotEmpty(err) {
			t.Fatalf("index-only accepted: %v", err)
		}
		if !reflect.DeepEqual(before, removalSnapshot(t, f.r, append([]string{base.AllQueues}, f.keys...))) {
			t.Fatal("index-only changed")
		}
	}
	q := f.queue(t)
	if err := f.r.client.SAdd(context.Background(), base.AllQueues, q).Err(); err != nil {
		t.Fatal(err)
	}
	if err := f.r.RemoveQueue(q, false); err != nil {
		t.Fatalf("empty rejected: %v", err)
	}
}
func TestQueueRemovalContractsForceIsolation(t *testing.T) {
	f := newRemovalFixture(t)
	rng := rand.New(rand.NewSource(20261001))
	ctx := context.Background()
	for trial := 0; trial < 8; trial++ {
		q := f.queue(t)
		neighbor := f.queue(t)
		neighborStart := len(f.keys)
		neighborMsgs := f.seed(t, neighbor, "group", 2)
		neighborKeys := append([]string(nil), f.keys[neighborStart:]...)
		neighborKeys = append(neighborKeys, base.GroupKey(neighbor, "group"), base.AllGroups(neighbor))
		before := removalSnapshot(t, f.r, neighborKeys)
		var msgs []*base.TaskMessage
		targetStart := len(f.keys)
		for _, state := range []string{"pending", "scheduled", "retry", "archived", "completed", "group", "staged"} {
			msgs = append(msgs, f.seed(t, q, state, 1+rng.Intn(3))...)
		}
		// A stale task still points to its former lock after a newer owner acquired it.
		replaced := msgs[0].UniqueKey
		if err := f.r.client.Set(ctx, replaced, "new-owner", time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		// Both a poisoned set reference and task lock metadata point at a foreign
		// queue in a different slot. They must be ignored before any Redis access.
		foreignLock := neighborMsgs[0].UniqueKey
		// Matching task ID cannot authorize access across queue boundaries.
		if err := f.r.client.Set(ctx, foreignLock, msgs[1].ID, time.Minute).Err(); err != nil {
			t.Fatal(err)
		}
		before = removalSnapshot(t, f.r, neighborKeys)
		if err := f.r.client.HSet(ctx, base.TaskKey(q, msgs[1].ID), "unique_key", foreignLock).Err(); err != nil {
			t.Fatal(err)
		}
		if err := f.r.client.ZAdd(ctx, base.AllAggregationSets(q), redis.Z{Score: 1, Member: base.GroupKey(neighbor, "group")}).Err(); err != nil {
			t.Fatal(err)
		}
		// All reachable owned task, state, group, set and lock keys share
		// the real cluster hash slot; foreign refs deliberately need not.
		if useRedisCluster {
			slot, err := f.r.client.ClusterKeySlot(ctx, base.PendingKey(q)).Result()
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range f.keys[targetStart:] {
				got, err := f.r.client.ClusterKeySlot(ctx, key).Result()
				if err != nil || got != slot {
					t.Fatalf("owned key slot %s=%d want=%d err=%v", key, got, slot, err)
				}
			}
		}
		// Unindexed keys cannot safely be discovered or claimed by removal.
		unknown := base.TaskKey(q, "unindexed")
		f.keys = append(f.keys, unknown)
		if err := f.r.client.Set(ctx, unknown, "unowned-by-index", 0).Err(); err != nil {
			t.Fatal(err)
		}
		if err := f.r.RemoveQueue(q, true); err != nil {
			t.Fatal(err)
		}
		if got := f.r.client.Get(ctx, unknown).Val(); got != "unowned-by-index" {
			t.Fatal("unindexed key was deleted")
		}
		for _, m := range msgs {
			if f.r.client.Exists(ctx, base.TaskKey(q, m.ID)).Val() != 0 {
				t.Fatalf("task orphan %s", m.ID)
			}
		}
		for _, key := range []string{base.PendingKey(q), base.ActiveKey(q), base.ScheduledKey(q), base.RetryKey(q), base.ArchivedKey(q), base.CompletedKey(q), base.LeaseKey(q), base.AllGroups(q), base.AllAggregationSets(q), base.GroupKey(q, "group"), base.AggregationSetKey(q, "stage", "set")} {
			if f.r.client.Exists(ctx, key).Val() != 0 {
				t.Fatalf("state orphan %s", key)
			}
		}
		for _, key := range f.keys[targetStart:] {
			if strings.Contains(key, ":unique:") && key != replaced && key != msgs[1].UniqueKey {
				if f.r.client.Exists(ctx, key).Val() != 0 {
					t.Fatalf("lock orphan %s", key)
				}
			}
		}
		if got := f.r.client.Get(ctx, replaced).Val(); got != "new-owner" {
			t.Fatalf("new owner lock deleted: %q", got)
		}
		if !reflect.DeepEqual(before, removalSnapshot(t, f.r, neighborKeys)) {
			t.Fatal("foreign queue bytes changed")
		}
		if f.r.client.SIsMember(ctx, base.AllQueues, q).Val() || !f.r.client.SIsMember(ctx, base.AllQueues, neighbor).Val() {
			t.Fatal("queue index isolation failed")
		}
	}
}
func TestQueueRemovalContractsActiveRefusal(t *testing.T) {
	f := newRemovalFixture(t)
	q := f.queue(t)
	f.seed(t, q, "active", 1)
	f.seed(t, q, "completed", 2)
	f.seed(t, q, "group", 2)
	f.seed(t, q, "staged", 2)
	keys := append([]string{base.AllQueues}, f.keys...)
	before := removalSnapshot(t, f.r, keys)
	if err := f.r.RemoveQueue(q, true); canonical.CanonicalCode(err) != canonical.FailedPrecondition {
		t.Fatalf("active removal=%v", err)
	}
	if !reflect.DeepEqual(before, removalSnapshot(t, f.r, keys)) {
		t.Fatal("active refusal partially mutated queue")
	}
}

// Corrupt task metadata must not claim unrelated string keys even when they
// are in the same hash slot and contain exactly the removed task's ID.
func TestQueueRemovalContractsCorruptReference(t *testing.T) {
	f := newRemovalFixture(t)
	q := f.queue(t)
	msg := f.seed(t, q, "completed", 1)[0]
	ctx := context.Background()
	nonlock := base.TaskKey(q, "unindexed-string")
	f.keys = append(f.keys, nonlock)
	if err := f.r.client.Set(ctx, nonlock, msg.ID, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := f.r.client.HSet(ctx, base.TaskKey(q, msg.ID), "unique_key", nonlock).Err(); err != nil {
		t.Fatal(err)
	}
	before := removalSnapshot(t, f.r, []string{nonlock})
	if err := f.r.RemoveQueue(q, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, removalSnapshot(t, f.r, []string{nonlock})) {
		t.Fatal("corrupt unique metadata deleted same-queue nonlock")
	}
	if f.r.client.Exists(ctx, base.TaskKey(q, msg.ID)).Val() != 0 || f.r.client.Exists(ctx, base.CompletedKey(q)).Val() != 0 {
		t.Fatal("reachable completed task was not removed")
	}
}
