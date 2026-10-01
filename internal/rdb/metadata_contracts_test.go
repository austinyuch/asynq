package rdb

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	"github.com/austinyuch/asynq/internal/timeutil"
	"github.com/redis/go-redis/v9"
)

type metadataFixture struct {
	r        *RDB
	c        *redis.Client
	prefix   string
	keys     []string
	members  map[string][]interface{}
	baseline map[string]string
}

func newMetadataFixture(t *testing.T) *metadataFixture {
	t.Helper()
	addr := os.Getenv("ASYNQ_METADATA_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ASYNQ_METADATA_TEST_REDIS_ADDR for an exclusive task-owned Redis database")
	}
	db := 11
	if value := os.Getenv("ASYNQ_METADATA_TEST_REDIS_DB"); value != "" {
		var err error
		db, err = strconv.Atoi(value)
		if err != nil || db < 1 || db > 15 {
			t.Fatalf("ASYNQ_METADATA_TEST_REDIS_DB must name a dedicated database between 1 and 15: %q", value)
		}
	}
	c := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	f := &metadataFixture{r: NewRDB(c), c: c, prefix: fmt.Sprintf("metadata-%d", time.Now().UnixNano()), members: map[string][]interface{}{}, baseline: map[string]string{}}
	ctx := context.Background()
	t.Cleanup(func() {
		for index, members := range f.members {
			if len(members) > 0 {
				if e := c.ZRem(ctx, index, members...).Err(); e != nil {
					t.Error(e)
				}
			}
		}
		if len(f.keys) > 0 {
			if e := c.Del(ctx, f.keys...).Err(); e != nil {
				t.Error(e)
			}
		}
		for key, want := range f.baseline {
			got, e := c.Dump(ctx, key).Result()
			if e != nil || got != want {
				t.Errorf("historical key changed %s: %v", key, e)
			}
		}
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	if e := c.Ping(ctx).Err(); e != nil {
		t.Fatal(e)
	}
	for _, index := range []string{base.AllServers, base.AllWorkers, base.AllSchedulers} {
		kind, e := c.Type(ctx, index).Result()
		if e != nil || kind != "none" {
			t.Fatalf("metadata index not exclusively empty %s: kind%s e%v", index, kind, e)
		}
	}
	keys, e := c.Keys(ctx, "*").Result()
	if e != nil {
		t.Fatal(e)
	}
	for _, key := range keys {
		v, e := c.Dump(ctx, key).Result()
		if e != nil {
			t.Fatal(e)
		}
		f.baseline[key] = v
	}
	f.r.SetClock(timeutil.NewSimulatedClock(time.Unix(1900000000, 0).UTC()))
	return f
}
func (f *metadataFixture) member(t *testing.T, index, key string, score int64) {
	t.Helper()
	f.members[index] = append(f.members[index], key)
	f.keys = append(f.keys, key)
	if e := f.c.ZAdd(context.Background(), index, redis.Z{Member: key, Score: float64(score)}).Err(); e != nil {
		t.Fatal(e)
	}
}
func metadataBytes(t *testing.T, b []byte, e error) []byte {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestMetadataRegistryIsolationAndExpiry(t *testing.T) {
	f := newMetadataFixture(t)
	ctx := context.Background()
	now := int64(1900000000)
	rng := rand.New(rand.NewSource(20261020))
	for iteration := 0; iteration < 12; iteration++ {
		name := fmt.Sprintf("%s-%02d", f.prefix, iteration)
		healthyScore := now + int64(rng.Intn(2))
		if iteration < 2 {
			healthyScore = now + int64(iteration)
		}
		server := &base.ServerInfo{Host: name, PID: 7, ServerID: "healthy", Concurrency: 3, Queues: map[string]int{"queue": 2}, Status: "active", Started: time.Unix(1800000000, 0).UTC()}
		worker := &base.WorkerInfo{Host: name, PID: 7, ServerID: "healthy", ID: name + "-task", Type: "metadata-task", Payload: []byte("payload"), Queue: "queue", Started: server.Started, Deadline: time.Unix(now+60, 0).UTC()}
		entry := &base.SchedulerEntry{ID: name + "-entry", Spec: "@hourly", Type: "scheduled-task", Payload: []byte("scheduled-payload"), Opts: []string{"queue=queue"}, Prev: server.Started, Next: time.Unix(now+3600, 0).UTC()}
		for _, kind := range []string{"servers", "workers", "schedulers"} {
			t.Run(fmt.Sprintf("%02d-%s", iteration, kind), func(t *testing.T) {
				var index string
				key := func(label string) string { return "" }
				switch kind {
				case "servers":
					index = base.AllServers
					key = func(label string) string { return base.ServerInfoKey(name, 7, label) }
				case "workers":
					index = base.AllWorkers
					key = func(label string) string { return base.WorkersKey(name, 7, label) }
				case "schedulers":
					index = base.AllSchedulers
					key = func(label string) string { return base.SchedulerEntriesKey(name + label) }
				}
				// Preregister every cleanup identity before the corresponding write.
				for _, label := range []string{"healthy", "stale"} {
					score := healthyScore
					if label == "stale" {
						score = now - 1
					}
					f.member(t, index, key(label), score)
				}
				var encoded []byte
				switch kind {
				case "servers":
					b, e := base.EncodeServerInfo(server)
					encoded = metadataBytes(t, b, e)
					if e = f.c.Set(ctx, key("healthy"), encoded, 0).Err(); e != nil {
						t.Fatal(e)
					}
				case "workers":
					b, e := base.EncodeWorkerInfo(worker)
					encoded = metadataBytes(t, b, e)
					if e = f.c.HSet(ctx, key("healthy"), "good", encoded).Err(); e != nil {
						t.Fatal(e)
					}
				case "schedulers":
					b, e := base.EncodeSchedulerEntry(entry)
					encoded = metadataBytes(t, b, e)
					if e = f.c.RPush(ctx, key("healthy"), encoded).Err(); e != nil {
						t.Fatal(e)
					}
				}
				if e := f.c.Set(ctx, key("stale"), "retained-stale-data", 0).Err(); e != nil {
					t.Fatal(e)
				}
				data := map[string]string{}
				for _, label := range []string{"healthy", "stale"} {
					v, e := f.c.Dump(ctx, key(label)).Result()
					if e != nil {
						t.Fatal(e)
					}
					data[key(label)] = v
				}
				switch kind {
				case "servers":
					got, e := f.r.ListServers()
					if e != nil || !reflect.DeepEqual(got, []*base.ServerInfo{server}) {
						t.Fatalf("healthy server lost/invalid admitted got%v e%v", got, e)
					}
				case "workers":
					got, e := f.r.ListWorkers()
					if e != nil || !reflect.DeepEqual(got, []*base.WorkerInfo{worker}) {
						t.Fatalf("healthy worker lost/invalid admitted got%v e%v", got, e)
					}
				case "schedulers":
					got, e := f.r.ListSchedulerEntries()
					if e != nil || !reflect.DeepEqual(got, []*base.SchedulerEntry{entry}) {
						t.Fatalf("healthy scheduler lost/invalid admitted got%v e%v", got, e)
					}
				}
				want := map[string]float64{key("healthy"): float64(healthyScore)}
				zs, e := f.c.ZRangeWithScores(ctx, index, 0, -1).Result()
				if e != nil {
					t.Fatal(e)
				}
				got := map[string]float64{}
				for _, z := range zs {
					got[z.Member.(string)] = z.Score
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("exact owned expiry boundary got%v want%v", got, want)
				}
				for key, want := range data {
					got, e := f.c.Dump(ctx, key).Result()
					if e != nil || got != want {
						t.Fatalf("registry read altered data %s e%v", key, e)
					}
				}
				// Remove only this generated case before the next independent case.
				if e := f.c.ZRem(ctx, index, f.members[index]...).Err(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestMetadataHistoryPaginationAndFailure(t *testing.T) {
	f := newMetadataFixture(t)
	ctx := context.Background()
	id := f.prefix + "-history"
	key := base.SchedulerHistoryKey(id)
	f.keys = append(f.keys, key)
	expected := []*base.SchedulerEnqueueEvent{}
	for n := 0; n < 5; n++ {
		event := &base.SchedulerEnqueueEvent{TaskID: fmt.Sprintf("task-%d", n), EnqueuedAt: time.Unix(1800000000+int64(n), 0).UTC()}
		expected = append([]*base.SchedulerEnqueueEvent{event}, expected...)
		if e := f.r.RecordSchedulerEnqueueEvent(id, event); e != nil {
			t.Fatal(e)
		}
	}
	before, e := f.c.Dump(ctx, key).Result()
	if e != nil {
		t.Fatal(e)
	}
	for size := 1; size <= 3; size++ {
		var union []*base.SchedulerEnqueueEvent
		for page := 0; page < 5; page++ {
			got, e := f.r.ListSchedulerEnqueueEvents(id, Pagination{Size: size, Page: page})
			if e != nil {
				t.Fatal(e)
			}
			start := page * size
			end := start + size
			if end > len(expected) {
				end = len(expected)
			}
			if start > len(expected) {
				start = len(expected)
			}
			if len(got) != end-start || !reflect.DeepEqual(append([]*base.SchedulerEnqueueEvent{}, got...), expected[start:end]) {
				t.Fatalf("history page size%d page%d got%v want%v", size, page, got, expected[start:end])
			}
			union = append(union, got...)
		}
		if !reflect.DeepEqual(union, expected) {
			t.Fatal("history pages lost/duplicated events")
		}
	}
	after, e := f.c.Dump(ctx, key).Result()
	if e != nil || before != after {
		t.Fatal("history listing mutated events")
	}
	corrupt := []byte{0, 255}
	if e := f.c.ZAdd(ctx, key, redis.Z{Member: string(corrupt), Score: 1800000002.5}).Err(); e != nil {
		t.Fatal(e)
	}
	corruptBefore, err := f.c.Dump(ctx, key).Result()
	if err != nil {
		t.Fatal(err)
	}
	_, decodeErr := base.DecodeSchedulerEnqueueEvent(corrupt)
	got, e := f.r.ListSchedulerEnqueueEvents(id, Pagination{Size: 10})
	if decodeErr == nil || e == nil || e.Error() != decodeErr.Error() || got != nil {
		t.Fatalf("malformed history must fail without partial success got%v e%v expected%v", got, e, decodeErr)
	}
	corruptAfter, err := f.c.Dump(ctx, key).Result()
	if err != nil || corruptBefore != corruptAfter {
		t.Fatal("malformed history read mutated data")
	}
	if e := f.c.Del(ctx, key).Err(); e != nil {
		t.Fatal(e)
	}
	if e := f.c.Set(ctx, key, "owned-wrongtype", 0).Err(); e != nil {
		t.Fatal(e)
	}
	got, e = f.r.ListSchedulerEnqueueEvents(id, Pagination{Size: 10})
	if e == nil || got != nil {
		t.Fatalf("wrongtype history got%v e%v", got, e)
	}
}

func TestMetadataClosedTransportCause(t *testing.T) {
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	r := NewRDB(c)
	if e := r.Close(); e != nil {
		t.Fatal(e)
	}
	for name, call := range map[string]func() error{"servers": func() error { _, e := r.ListServers(); return e }, "workers": func() error { _, e := r.ListWorkers(); return e }, "schedulers": func() error { _, e := r.ListSchedulerEntries(); return e }, "history": func() error { _, e := r.ListSchedulerEnqueueEvents("closed-owned", Pagination{Size: 1}); return e }} {
		t.Run(name, func(t *testing.T) {
			if e := call(); !errors.Is(e, redis.ErrClosed) {
				t.Fatalf("transport cause lost: %v", e)
			}
		})
	}
}
