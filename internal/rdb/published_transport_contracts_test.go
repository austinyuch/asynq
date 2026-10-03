package rdb

import (
	"context"
	stderrors "errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	errors "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Successful publication must not hide the cause of a later transport failure.
// These tests use a distinct live observer and never flush the fixture database.
func TestPublishedQueueClosedTransportContracts(t *testing.T) {
	if useRedisCluster {
		t.Skip("owned standalone Redis fixture contract")
	}
	cases := []struct {
		name string
		code errors.Code
		call func(*testing.T, context.Context, *RDB, *base.TaskMessage) error
	}{
		{"Enqueue", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error { return r.Enqueue(c, m) }},
		{"EnqueueUnique", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error {
			return r.EnqueueUnique(c, m, time.Hour)
		}},
		{"AddToGroup", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error {
			return r.AddToGroup(c, m, "owned-group")
		}},
		{"AddToGroupUnique", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error {
			return r.AddToGroupUnique(c, m, "owned-group", time.Hour)
		}},
		{"Schedule", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error {
			return r.Schedule(c, m, time.Unix(2000000000, 0))
		}},
		{"ScheduleUnique", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error {
			return r.ScheduleUnique(c, m, time.Unix(2000000000, 0), time.Hour)
		}},
		{"BatchEnqueue", errors.Unknown, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error {
			n, e := r.BatchEnqueue(c, []base.BatchEnqueueItem{{Msg: m, ProcessAt: time.Unix(2000000000, 0)}})
			if n != 0 {
				t.Errorf("failed scheduled batch count=%d, want 0", n)
			}
			return e
		}},
		{"Requeue", errors.Internal, func(t *testing.T, c context.Context, r *RDB, m *base.TaskMessage) error { return r.Requeue(c, m) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			q := "published-transport-" + uuid.NewString()
			subject := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
			observer := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
			r := NewRDB(subject)
			closed := false
			t.Cleanup(func() {
				cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
				defer done()
				// UUID ownership prefix is fixed before any fixture writes; cleanup never scans foreign keys.
				keys := publishedTransportKeys(t, cleanupCtx, observer, q)
				for _, k := range keys {
					if !strings.HasPrefix(k, base.QueueKeyPrefix(q)) {
						t.Errorf("foreign cleanup key %q", k)
						continue
					}
					if e := observer.Del(cleanupCtx, k).Err(); e != nil {
						t.Errorf("cleanup key: %v", e)
					}
				}
				if e := observer.SRem(cleanupCtx, base.AllQueues, q).Err(); e != nil {
					t.Errorf("cleanup queue membership: %v", e)
				}
				if len(publishedTransportKeys(t, cleanupCtx, observer, q)) != 0 {
					t.Error("owned keys remain after cleanup")
				}
				if present, e := observer.SIsMember(cleanupCtx, base.AllQueues, q).Result(); e != nil || present {
					t.Errorf("owned membership remains: %v %v", present, e)
				}
				if !closed {
					if e := subject.Close(); e != nil {
						t.Errorf("subject cleanup: %v", e)
					}
				}
				if e := observer.Close(); e != nil {
					t.Errorf("observer cleanup: %v", e)
				}
			})
			if keys := publishedTransportKeys(t, ctx, observer, q); len(keys) != 0 {
				t.Fatalf("new UUID queue already exists: %v", keys)
			}
			seed := h.NewTaskMessageWithQueue("published:seed", []byte("seed"), q)
			seed.ID = uuid.NewString()
			if e := r.Enqueue(ctx, seed); e != nil {
				t.Fatalf("successful publication prerequisite: %v", e)
			}
			if present, e := observer.SIsMember(ctx, base.AllQueues, q).Result(); e != nil || !present {
				t.Fatalf("publication not independently visible: %v %v", present, e)
			}
			if ids, e := observer.LRange(ctx, base.PendingKey(q), 0, -1).Result(); e != nil || !reflect.DeepEqual(ids, []string{seed.ID}) {
				t.Fatalf("seed pending=%v err=%v", ids, e)
			}
			if state, e := observer.HGet(ctx, base.TaskKey(q, seed.ID), "state").Result(); e != nil || state != "pending" {
				t.Fatalf("seed state=%q err=%v", state, e)
			}
			if data, e := observer.HGet(ctx, base.TaskKey(q, seed.ID), "msg").Bytes(); e != nil || len(data) == 0 {
				t.Fatalf("seed encoded message missing: %v", e)
			}
			before := publishedTransportSnapshot(t, ctx, observer, q)
			queuesBefore, e := observer.SMembers(ctx, base.AllQueues).Result()
			if e != nil {
				t.Fatal(e)
			}
			sort.Strings(queuesBefore)
			if e := subject.Close(); e != nil {
				t.Fatal(e)
			}
			closed = true
			target := h.NewTaskMessageWithQueue("published:target", []byte("fresh"), q)
			target.ID = uuid.NewString()
			target.UniqueKey = base.UniqueKey(q, target.Type, target.Payload)
			err := tc.call(t, ctx, r, target)
			// Non-fatal assertions ensure the independent state oracle still runs on RED.
			if err == nil {
				t.Error("closed transport reported success")
			}
			var canonical *errors.Error
			if !stderrors.As(err, &canonical) || canonical.Code != tc.code || canonical.Op != errors.Op("rdb."+tc.name) {
				t.Errorf("canonical error=%#v want code=%v op=rdb.%s", canonical, tc.code, tc.name)
			}
			if !stderrors.Is(err, redis.ErrClosed) {
				t.Errorf("lost downstream transport cause: %v", err)
			}
			after := publishedTransportSnapshot(t, ctx, observer, q)
			if !reflect.DeepEqual(before, after) {
				t.Errorf("closed operation changed owned data: before=%#v after=%#v", before, after)
			}
			queuesAfter, e := observer.SMembers(ctx, base.AllQueues).Result()
			if e != nil {
				t.Fatal(e)
			}
			sort.Strings(queuesAfter)
			if !reflect.DeepEqual(queuesBefore, queuesAfter) {
				t.Errorf("queue index changed: %v => %v", queuesBefore, queuesAfter)
			}
			if n, e := observer.Exists(ctx, base.TaskKey(q, target.ID), target.UniqueKey).Result(); e != nil || n != 0 {
				t.Errorf("fresh target keys created: %d %v", n, e)
			}
		})
	}
}

func publishedTransportKeys(t *testing.T, ctx context.Context, c *redis.Client, q string) []string {
	t.Helper()
	var keys []string
	var cursor uint64
	for {
		page, next, e := c.Scan(ctx, cursor, base.QueueKeyPrefix(q)+"*", 100).Result()
		if e != nil {
			t.Fatal(e)
		}
		keys = append(keys, page...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	sort.Strings(keys)
	unique := keys[:0]
	for _, k := range keys {
		if len(unique) == 0 || unique[len(unique)-1] != k {
			unique = append(unique, k)
		}
	}
	return unique
}

// Values preserve arbitrary Redis bytes. Only unordered set membership is sorted;
// list order and sorted-set member/score pairs remain exact. TTL is not compared.
func publishedTransportSnapshot(t *testing.T, ctx context.Context, c *redis.Client, q string) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	for _, k := range publishedTransportKeys(t, ctx, c, q) {
		typ, e := c.Type(ctx, k).Result()
		if e != nil {
			t.Fatal(e)
		}
		var value interface{}
		switch typ {
		case "hash":
			value, e = c.HGetAll(ctx, k).Result()
		case "list":
			value, e = c.LRange(ctx, k, 0, -1).Result()
		case "set":
			var members []string
			members, e = c.SMembers(ctx, k).Result()
			sort.Strings(members)
			value = members
		case "string":
			value, e = c.Get(ctx, k).Result()
		case "zset":
			value, e = c.ZRangeWithScores(ctx, k, 0, -1).Result()
		default:
			t.Fatalf("unsupported owned key type %q for %q", typ, k)
		}
		if e != nil {
			t.Fatal(e)
		}
		out[k] = struct {
			Type  string
			Value interface{}
		}{typ, value}
	}
	return out
}
