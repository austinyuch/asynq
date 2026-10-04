package rdb

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	canonical "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// Task timestamps are integer Unix seconds (base.Z.Score). An unrepresentable
// persisted score must fail the inspection, preserve data, and leave a healthy
// neighboring task readable. These fixtures never FlushDB or replace shared
// indexes; they register only their own unique queue membership.
type inspectionContractFixture struct {
	r               *RDB
	queue           string
	healthy, broken *base.TaskMessage
	index           string
	keys            []string
}

func newInspectionContractFixture(t *testing.T, state string) *inspectionContractFixture {
	t.Helper()
	if useRedisCluster {
		t.Skip("owned standalone DB timestamp corruption fixture")
	}
	client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	q := fmt.Sprintf("inspection-contract-%d", time.Now().UnixNano())
	healthy := h.NewTaskMessageWithQueue("healthy-timestamp", []byte("healthy-payload"), q)
	broken := h.NewTaskMessageWithQueue("broken-timestamp", []byte("preserved-payload"), q)
	index := base.ScheduledKey(q)
	if state == "retry" {
		index = base.RetryKey(q)
	}
	f := &inspectionContractFixture{r: NewRDB(client), queue: q, healthy: healthy, broken: broken, index: index, keys: []string{index, base.TaskKey(q, healthy.ID), base.TaskKey(q, broken.ID)}}
	t.Cleanup(func() {
		ctx := context.Background()
		for _, key := range f.keys {
			if err := client.Del(ctx, key).Err(); err != nil {
				t.Error(err)
			}
		}
		if err := client.SRem(ctx, base.AllQueues, q).Err(); err != nil {
			t.Error(err)
		}
		if err := f.r.Close(); err != nil {
			t.Error(err)
		}
	})
	entries := []base.Z{{Message: healthy, Score: 2000000000}, {Message: broken, Score: 2000000100}}
	if state == "retry" {
		h.SeedRetryQueue(t, client, entries, q)
	} else {
		h.SeedScheduledQueue(t, client, entries, q)
	}
	return f
}
func (f *inspectionContractFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, key := range f.keys {
		v, err := f.r.client.Dump(context.Background(), key).Result()
		if err != nil {
			t.Fatal(err)
		}
		out[key] = v
	}
	return out
}
func (f *inspectionContractFixture) rejectUnrepresentableTimestamp(t *testing.T, state string, score float64) {
	t.Helper()
	if !math.IsInf(score, 0) && score >= math.MinInt64 && score < math.MaxInt64 {
		t.Fatalf("test input is representable: %v", score)
	}
	if err := f.r.client.ZAdd(context.Background(), f.index, redis.Z{Score: score, Member: f.broken.ID}).Err(); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t)
	info, err := f.r.GetTaskInfo(f.queue, f.broken.ID)
	if info != nil || canonical.CanonicalCode(err) != canonical.Internal {
		t.Fatalf("unrepresentable task timestamp accepted/misclassified: info=%+v err=%v", info, err)
	}
	var tasks []*base.TaskInfo
	if state == "retry" {
		tasks, err = f.r.ListRetry(f.queue, Pagination{Size: 10, Page: 0})
	} else {
		tasks, err = f.r.ListScheduled(f.queue, Pagination{Size: 10, Page: 0})
	}
	if tasks != nil || canonical.CanonicalCode(err) != canonical.Internal {
		t.Fatalf("partial timestamp list accepted/misclassified: tasks=%v err=%v", tasks, err)
	}
	if after := f.snapshot(t); !reflect.DeepEqual(before, after) {
		t.Fatal("failed inspection mutated persisted task/index bytes")
	}
	healthy, err := f.r.GetTaskInfo(f.queue, f.healthy.ID)
	wantState := base.TaskStateScheduled
	if state == "retry" {
		wantState = base.TaskStateRetry
	}
	if err != nil || healthy == nil || healthy.State != wantState || healthy.Message.ID != f.healthy.ID || healthy.Message.Type != f.healthy.Type || !bytes.Equal(healthy.Message.Payload, f.healthy.Payload) || !healthy.NextProcessAt.Equal(time.Unix(2000000000, 0)) {
		t.Fatalf("healthy neighbor changed: %+v %v", healthy, err)
	}
}
func TestInspectionUnrepresentableTimestampContracts(t *testing.T) {
	for _, state := range []string{"scheduled", "retry"} {
		t.Run(state, func(t *testing.T) {
			f := newInspectionContractFixture(t, state)
			for _, score := range []float64{math.Inf(1), math.Inf(-1), 1e100, -1e100, math.MaxFloat64, -math.MaxFloat64} {
				f.rejectUnrepresentableTimestamp(t, state, score)
			}
		})
	}
}
func TestInspectionTimestampDomainProperty(t *testing.T) {
	for _, state := range []string{"scheduled", "retry"} {
		t.Run(state, func(t *testing.T) {
			f := newInspectionContractFixture(t, state)
			// All generated scores exceed int64 even after decimal-string handling.
			property := func(exponent uint8, negative bool) bool {
				score := math.Pow10(40 + int(exponent%61))
				if negative {
					score = -score
				}
				f.rejectUnrepresentableTimestamp(t, state, score)
				return true
			}
			if err := quick.Check(property, &quick.Config{MaxCount: 40, Rand: rand.New(rand.NewSource(20261003))}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
