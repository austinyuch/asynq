package rdb

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/austinyuch/asynq/internal/timeutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Every fixture owns a new queue. Do not use setup: it flushes the database.
func line37Fixture(t testing.TB) (*RDB, string) {
	t.Helper()
	if useRedisCluster {
		t.Skip("standalone Redis contract; cluster transitions require separate evidence")
	}
	r := NewRDB(redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB, PoolSize: 1, MaxRetries: -1}))
	r.SetClock(timeutil.NewSimulatedClock(time.Now().Truncate(time.Second)))
	q := "line37-" + uuid.NewString()
	ctx := context.Background()
	foreign, e := r.client.SMembers(ctx, base.AllQueues).Result()
	if e != nil {
		t.Fatal(e)
	}
	sort.Strings(foreign)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var cur uint64
		for {
			keys, next, e := r.client.Scan(cleanup, cur, base.QueueKeyPrefix(q)+"*", 100).Result()
			if e != nil {
				t.Error(e)
				break
			}
			if len(keys) > 0 {
				if e := r.client.Del(cleanup, keys...).Err(); e != nil {
					t.Error(e)
				}
			}
			cur = next
			if cur == 0 {
				break
			}
		}
		var verifyCursor uint64
		for {
			keys, next, e := r.client.Scan(cleanup, verifyCursor, base.QueueKeyPrefix(q)+"*", 100).Result()
			if e != nil {
				t.Error(e)
				break
			}
			if len(keys) != 0 {
				t.Errorf("owned queue cleanup left %d keys", len(keys))
			}
			verifyCursor = next
			if verifyCursor == 0 {
				break
			}
		}
		if e := r.client.SRem(cleanup, base.AllQueues, q).Err(); e != nil {
			t.Error(e)
		}
		after, e := r.client.SMembers(cleanup, base.AllQueues).Result()
		if e != nil {
			t.Error(e)
			return
		}
		sort.Strings(after)
		if !reflect.DeepEqual(foreign, after) {
			t.Errorf("foreign queue membership changed: before=%v after=%v", foreign, after)
		}
	})
	return r, q
}
func line37Snapshot(t testing.TB, r *RDB, q string) map[string]string {
	t.Helper()
	ctx := context.Background()
	out := map[string]string{}
	var cur uint64
	for {
		keys, next, e := r.client.Scan(ctx, cur, base.QueueKeyPrefix(q)+"*", 100).Result()
		if e != nil {
			t.Fatal(e)
		}
		for _, k := range keys {
			v, e := r.client.Dump(ctx, k).Result()
			if e != nil {
				t.Fatal(e)
			}
			out[k] = v
		}
		cur = next
		if cur == 0 {
			break
		}
	}
	return out
}
func line37State(t testing.TB, r *RDB, q string, want map[string]string) {
	t.Helper()
	got := line37Snapshot(t, r, q)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exact owned data state differs: got keys %v want keys %v", got, want)
	}
}
func line37RedisError(t testing.TB, e error, text string) {
	t.Helper()
	var cause redis.Error
	if e == nil || !stderrors.As(e, &cause) || !strings.Contains(cause.Error(), text) {
		t.Fatalf("want Redis server cause %q, got %T %v", text, e, e)
	}
}
func line37SeedActive(t testing.TB, r *RDB, q string, msg *base.TaskMessage) {
	h.SeedActiveQueue(t, r.client, []*base.TaskMessage{msg}, q)
	h.SeedLease(t, r.client, []base.Z{{Message: msg, Score: time.Now().Add(time.Hour).Unix()}}, q)
}

func TestOriginalLuaCorruptActiveTransitions(t *testing.T) {
	for _, op := range []string{"done", "done-unique", "complete", "complete-unique", "requeue", "retry", "archive"} {
		for _, fault := range []string{"missing-active", "missing-lease", "missing-hash", "existing-completed"} {
			if fault == "missing-hash" && op != "done" && op != "done-unique" {
				continue
			}
			if fault == "existing-completed" && op != "complete" && op != "complete-unique" {
				continue
			}
			t.Run(op+"/"+fault, func(t *testing.T) {
				r, q := line37Fixture(t)
				ctx := context.Background()
				msg := h.NewTaskMessageWithQueue("line-contract", []byte("opaque"), q)
				msg.Retention = 3600
				line37SeedActive(t, r, q, msg)
				if strings.HasSuffix(op, "-unique") {
					msg.UniqueKey = base.UniqueKey(q, msg.Type, msg.Payload)
					if e := r.client.Set(ctx, msg.UniqueKey, msg.ID, 0).Err(); e != nil {
						t.Fatal(e)
					}
				}
				switch fault {
				case "missing-active":
					if e := r.client.Del(ctx, base.ActiveKey(q)).Err(); e != nil {
						t.Fatal(e)
					}
				case "missing-lease":
					if e := r.client.Del(ctx, base.LeaseKey(q)).Err(); e != nil {
						t.Fatal(e)
					}
				case "missing-hash":
					if e := r.client.Del(ctx, base.TaskKey(q, msg.ID)).Err(); e != nil {
						t.Fatal(e)
					}
				case "existing-completed":
					h.SeedCompletedQueue(t, r.client, []base.Z{{Message: msg, Score: r.clock.Now().Add(time.Hour).Unix()}}, q)
				}
				want := line37Snapshot(t, r, q)
				// Redis Lua errors do not roll back preceding writes. Preserve exact expected partial state.
				if fault != "missing-active" {
					delete(want, base.ActiveKey(q))
				}
				if fault == "missing-hash" || fault == "existing-completed" {
					delete(want, base.LeaseKey(q))
				}
				var e error
				switch op {
				case "done", "done-unique":
					e = r.Done(ctx, msg)
				case "complete", "complete-unique":
					e = r.MarkAsComplete(ctx, msg)
				case "requeue":
					e = r.Requeue(ctx, msg)
				case "retry":
					e = r.Retry(ctx, msg, time.Now().Add(time.Hour), "failure", true)
				case "archive":
					e = r.Archive(ctx, msg, "failure")
				}
				expected := "NOT FOUND"
				if fault == "existing-completed" {
					expected = "INTERNAL"
				}
				line37RedisError(t, e, expected)
				line37State(t, r, q, want)
			})
		}
	}
}

func TestOriginalLuaProcessedCounterBoundary(t *testing.T) {
	for _, op := range []string{"done-unique", "complete", "complete-unique", "retry", "archive"} {
		t.Run(op, func(t *testing.T) {
			r, q := line37Fixture(t)
			ctx := context.Background()
			msg := h.NewTaskMessageWithQueue("boundary", []byte("opaque"), q)
			msg.Retention = 3600
			line37SeedActive(t, r, q, msg)
			if strings.HasSuffix(op, "-unique") {
				msg.UniqueKey = base.UniqueKey(q, msg.Type, msg.Payload)
				if e := r.client.Set(ctx, msg.UniqueKey, msg.ID, 0).Err(); e != nil {
					t.Fatal(e)
				}
			}
			if e := r.client.Set(ctx, base.ProcessedTotalKey(q), int64(math.MaxInt64), 0).Err(); e != nil {
				t.Fatal(e)
			}
			if e := r.client.Set(ctx, base.FailedTotalKey(q), int64(math.MaxInt64), 0).Err(); e != nil {
				t.Fatal(e)
			}
			var e error
			switch op {
			case "done-unique":
				e = r.Done(ctx, msg)
			case "complete", "complete-unique":
				e = r.MarkAsComplete(ctx, msg)
			case "retry":
				e = r.Retry(ctx, msg, time.Now().Add(time.Hour), "failure", true)
			case "archive":
				e = r.Archive(ctx, msg, "failure")
			}
			if e != nil {
				t.Fatal(e)
			}
			n, e := r.client.Get(ctx, base.ProcessedTotalKey(q)).Int64()
			if e != nil || n != 1 {
				t.Fatalf("processed wrap: %d %v", n, e)
			}
			if op == "retry" || op == "archive" {
				n, e = r.client.Get(ctx, base.FailedTotalKey(q)).Int64()
				if e != nil || n != 1 {
					t.Fatalf("failed wrap: %d %v", n, e)
				}
			} else {
				n, e = r.client.Get(ctx, base.FailedTotalKey(q)).Int64()
				if e != nil || n != math.MaxInt64 {
					t.Fatalf("unrelated failed counter changed: %d %v", n, e)
				}
			}
			if n, e := r.client.LLen(ctx, base.ActiveKey(q)).Result(); e != nil || n != 0 {
				t.Fatalf("active remains %d %v", n, e)
			}
			if n, e := r.client.ZCard(ctx, base.LeaseKey(q)).Result(); e != nil || n != 0 {
				t.Fatalf("lease remains %d %v", n, e)
			}
			if msg.UniqueKey != "" {
				if n, e := r.client.Exists(ctx, msg.UniqueKey).Result(); e != nil || n != 0 {
					t.Fatalf("unique lock retained %d %v", n, e)
				}
			}
			if op == "retry" || op == "archive" {
				key, state := base.RetryKey(q), "retry"
				if op == "archive" {
					key, state = base.ArchivedKey(q), "archived"
				}
				if n, e := r.client.ZCard(ctx, key).Result(); e != nil || n != 1 {
					t.Fatalf("transition index %d %v", n, e)
				}
				if s, e := r.client.HGet(ctx, base.TaskKey(q, msg.ID), "state").Result(); e != nil || s != state {
					t.Fatalf("state %q %v", s, e)
				}
			}
		})
	}
}

func TestOriginalLuaCorruptInspectorIndexes(t *testing.T) {
	for _, op := range []string{"run", "archive", "delete"} {
		for _, state := range []string{"pending", "aggregating", "retry"} {
			if op == "run" && state == "pending" {
				continue
			}
			t.Run(op+"/"+state, func(t *testing.T) {
				r, q := line37Fixture(t)
				ctx := context.Background()
				msg := h.NewTaskMessageWithQueue("corrupt", []byte("opaque"), q)
				group := "group-" + uuid.NewString()
				if e := r.client.SAdd(ctx, base.AllQueues, q).Err(); e != nil {
					t.Fatal(e)
				}
				if e := r.client.HSet(ctx, base.TaskKey(q, msg.ID), "msg", h.MustMarshal(t, msg), "state", state, "group", group).Err(); e != nil {
					t.Fatal(e)
				}
				// Task hash declares a state; its corresponding list/zset deliberately has no member.
				if e := r.client.SAdd(ctx, base.AllGroups(q), group).Err(); e != nil {
					t.Fatal(e)
				}
				before := line37Snapshot(t, r, q)
				var e error
				switch op {
				case "run":
					e = r.RunTask(q, msg.ID)
				case "archive":
					e = r.ArchiveTask(q, msg.ID)
				case "delete":
					e = r.DeleteTask(q, msg.ID)
				}
				text := "zset"
				if state == "pending" {
					text = "list"
				}
				line37RedisError(t, e, text)
				line37State(t, r, q, before)
			})
		}
	}
}

func TestOriginalLuaMissingAndPreservedMetadata(t *testing.T) {
	t.Run("memory-sample-zero-original-symbol", func(t *testing.T) {
		r, q := line37Fixture(t)
		keys := []string{base.ActiveKey(q), base.PendingKey(q), base.ScheduledKey(q), base.RetryKey(q), base.ArchivedKey(q), base.CompletedKey(q), base.AllGroups(q)}
		before := line37Snapshot(t, r, q)
		_, e := memoryUsageCmd.Run(context.Background(), r.client, keys, base.TaskKeyPrefix(q), 0, 1, base.GroupKeyPrefix(q)).Result()
		line37RedisError(t, e, "sample size must be a positive number")
		line37State(t, r, q, before)
	})
	t.Run("history-missing-original-api", func(t *testing.T) {
		r, q := line37Fixture(t)
		if e := r.client.SAdd(context.Background(), base.AllQueues, q).Err(); e != nil {
			t.Fatal(e)
		}
		before := line37Snapshot(t, r, q)
		s, e := r.HistoricalStats(q, 1)
		if e != nil || len(s) != 1 || s[0].Processed != 0 || s[0].Failed != 0 {
			t.Fatalf("empty history: %v %v", s, e)
		}
		line37State(t, r, q, before)
	})
	t.Run("missing-update-original-symbol", func(t *testing.T) {
		r, q := line37Fixture(t)
		before := line37Snapshot(t, r, q)
		n, e := updateTaskPayloadCmd.Run(context.Background(), r.client, []string{base.TaskKey(q, uuid.NewString())}, []byte("opaque")).Int64()
		if e != nil || n != 0 {
			t.Fatalf("missing task: %d %v", n, e)
		}
		line37State(t, r, q, before)
	})
	rng := rand.New(rand.NewSource(20261004))
	for i := 0; i < 100; i++ {
		payload := make([]byte, rng.Intn(256))
		_, _ = rng.Read(payload)
		t.Run(fmt.Sprintf("payload-property-%03d", i), func(t *testing.T) {
			line37PayloadProperty(t, payload)
		})
	}
}

func line37PayloadProperty(t *testing.T, payload []byte) {
	t.Helper()
	r, q := line37Fixture(t)
	ctx := context.Background()
	msg := h.NewTaskMessageWithQueue("payload-property", []byte("before"), q)
	h.SeedScheduledQueue(t, r.client, []base.Z{{Message: msg, Score: time.Now().Add(time.Hour).Unix()}}, q)
	key := base.TaskKey(q, msg.ID)
	if e := r.client.HSet(ctx, key, "pending_since", "123", "group", "own-group", "unique_key", base.UniqueKey(q, msg.Type, msg.Payload)).Err(); e != nil {
		t.Fatal(e)
	}
	if e := r.client.Expire(ctx, key, time.Hour).Err(); e != nil {
		t.Fatal(e)
	}
	before := line37Snapshot(t, r, q)
	beforeFields, e := r.client.HGetAll(ctx, key).Result()
	if e != nil {
		t.Fatal(e)
	}
	ttlBefore, e := r.client.PTTL(ctx, key).Result()
	if e != nil {
		t.Fatal(e)
	}
	if e := r.UpdateTaskPayload(q, msg.ID, payload); e != nil {
		t.Fatal(e)
	}
	afterFields, e := r.client.HGetAll(ctx, key).Result()
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range []string{"state", "pending_since", "group", "unique_key"} {
		if beforeFields[f] != afterFields[f] {
			t.Fatalf("metadata %s altered", f)
		}
	}
	updated := h.MustUnmarshal(t, afterFields["msg"])
	expected := h.MustUnmarshal(t, beforeFields["msg"])
	expected.Payload = append([]byte(nil), payload...)
	// Canonical protobuf round-trip treats nil and empty bytes identically.
	expected = h.MustUnmarshal(t, h.MustMarshal(t, expected))
	if !bytes.Equal(updated.Payload, payload) || !reflect.DeepEqual(updated, expected) {
		t.Fatal("opaque payload or complete task message not preserved")
	}
	delete(before, key)
	after := line37Snapshot(t, r, q)
	delete(after, key)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated owned data mutated")
	}
	ttlAfter, e := r.client.PTTL(ctx, key).Result()
	if e != nil || ttlAfter <= 0 || ttlAfter > ttlBefore || ttlBefore-ttlAfter > 10*time.Second {
		t.Fatalf("TTL not preserved: %v -> %v %v", ttlBefore, ttlAfter, e)
	}
}

// Native Go fuzz engine coverage guides inputs. Each invocation owns and cleans
// a UUID queue; the finite domain excludes payloads longer than4096 bytes.
func FuzzOriginalLuaPayloadMetadata(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte{0})
	f.Add([]byte{0xff, 0xfe, 0x80, 0x00})
	binary := make([]byte, 255)
	for i := range binary {
		binary[i] = byte(i)
	}
	f.Add(binary)
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 4096 {
			t.Skip("bounded payload domain: maximum4096 bytes")
		}
		line37PayloadProperty(t, payload)
	})
}
