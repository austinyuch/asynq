package asynq

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	"github.com/austinyuch/asynq/internal/log"
	"github.com/austinyuch/asynq/internal/rdb"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/austinyuch/asynq/internal/timeutil"
	"github.com/redis/go-redis/v9"
)

type heartbeatContractLog struct{ entries []string }

func (l *heartbeatContractLog) Debug(v ...interface{}) {
	l.entries = append(l.entries, fmt.Sprint(v...))
}
func (l *heartbeatContractLog) Info(v ...interface{})  { l.Debug(v...) }
func (l *heartbeatContractLog) Warn(v ...interface{})  { l.Debug(v...) }
func (l *heartbeatContractLog) Error(v ...interface{}) { l.Debug(v...) }
func (l *heartbeatContractLog) Fatal(v ...interface{}) { l.Debug(v...) }

// The two observations deliberately straddle an existing lease deadline.
// This models time passing during the real Redis round-trip, without replacing
// the broker or changing the returned extension deadline.
type heartbeatContractClock struct {
	first, later time.Time
	calls        int
}

func (c *heartbeatContractClock) Now() time.Time {
	c.calls++
	if c.calls == 1 {
		return c.first
	}
	return c.later
}

func heartbeatClosedContract(t testing.TB, seed int64, payload []byte) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	clock := timeutil.NewSimulatedClock(now)
	logger := &heartbeatContractLog{}
	hb := newHeartbeater(heartbeaterParams{logger: log.NewLogger(logger), broker: rdb.NewRDB(client), interval: time.Second, state: &serverState{value: srvStateActive}, queues: map[string]int{"healthy": 1}})
	rng := rand.New(rand.NewSource(seed))
	width := 1 + rng.Intn(16)
	deadlines := map[string]time.Time{}
	original := map[string]base.TaskMessage{}
	for i := 0; i < width; i++ {
		msg := h.NewTaskMessageWithQueue("closed-transport", append([]byte(nil), payload...), fmt.Sprintf("closed-%d", i%3))
		msg.ID = fmt.Sprintf("closed-%d-%d", seed, i)
		// Equality is valid; a closed transport must not invalidate it locally.
		deadline := now.Add(time.Duration(i) * time.Second)
		hb.workers[msg.ID] = &workerInfo{msg: msg, started: now.Add(-time.Second), deadline: now.Add(time.Hour), lease: h.NewLeaseWithClock(deadline, clock)}
		deadlines[msg.ID] = deadline
		copy := *msg
		copy.Payload = append([]byte(nil), msg.Payload...)
		original[msg.ID] = copy
	}
	hb.beat()
	entries := strings.Join(logger.entries, "\n")
	if !strings.Contains(entries, "Failed to write server state data:") || strings.Count(entries, "Failed to extend lease for tasks") != min(width, 3) {
		t.Fatalf("missing actual closed transport diagnostics: %s", entries)
	}
	for id, w := range hb.workers {
		if !w.lease.Deadline().Equal(deadlines[id]) || !reflect.DeepEqual(*w.msg, original[id]) {
			t.Fatalf("closed transport changed task/lease %s", id)
		}
		select {
		case <-w.lease.Done():
			t.Fatalf("valid lease notified on transport failure %s", id)
		default:
		}
	}
	if len(hb.workers) != width {
		t.Fatalf("worker loss: got %d want %d", len(hb.workers), width)
	}
}

func TestHeartbeatFailureClosedTransportProperties(t *testing.T) {
	for seed := int64(0); seed < 16; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) { heartbeatClosedContract(t, seed, []byte{0, byte(seed), 255}) })
	}
}
func FuzzHeartbeatFailureClosedTransport(f *testing.F) {
	f.Add(int64(0), []byte("payload"))
	f.Add(int64(23), []byte{0, 255})
	f.Fuzz(func(t *testing.T, seed int64, payload []byte) {
		if len(payload) > 4096 {
			t.Skip()
		}
		heartbeatClosedContract(t, seed, payload)
	})
}

func TestHeartbeatFailureRealLeaseBoundaries(t *testing.T) {
	addr := os.Getenv("ASYNQ_HEARTBEAT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("opt-in task-owned empty DB2 Redis contract")
	}
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 2})
	t.Cleanup(func() { _ = client.Close() })
	size, err := client.DBSize(ctx).Result()
	if err != nil || size != 0 {
		t.Fatalf("DB2 must initially be empty: size=%d err=%v", size, err)
	}
	for seed := int64(0); seed < 12; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			now := time.Now().UTC()
			old := now.Add(time.Duration(1+seed) * time.Second)
			logger := &heartbeatContractLog{}
			broker := rdb.NewRDB(client)
			broker.SetClock(timeutil.NewSimulatedClock(now))
			expectedExtension := now.Add(rdb.LeaseDuration)
			hb := newHeartbeater(heartbeaterParams{logger: log.NewLogger(logger), broker: broker, interval: time.Minute, state: &serverState{value: srvStateActive}, queues: map[string]int{"contract": 1}})
			hb.started = now.Add(-time.Minute)
			q := fmt.Sprintf("heartbeat-owned-%s-%d", hb.serverID, seed)
			hb.queues = map[string]int{q: 1}
			msgs := []*base.TaskMessage{}
			entries := []base.Z{}
			for i := 0; i < 3; i++ {
				msg := h.NewTaskMessageWithQueue("lease-boundary", []byte{0, byte(seed), byte(i), 255}, q)
				msg.ID = fmt.Sprintf("heartbeat-%d-%d", seed, i)
				msgs = append(msgs, msg)
				entries = append(entries, base.Z{Message: msg, Score: old.Unix()})
			}
			serverKey := base.ServerInfoKey(hb.host, hb.pid, hb.serverID)
			workersKey := base.WorkersKey(hb.host, hb.pid, hb.serverID)
			keys := []string{base.ActiveKey(q), base.LeaseKey(q), serverKey, workersKey}
			for _, msg := range msgs {
				keys = append(keys, base.TaskKey(q, msg.ID))
			}
			// Register exact owned cleanup before the first fixture publication.
			t.Cleanup(func() {
				if err := client.ZRem(ctx, base.AllServers, serverKey).Err(); err != nil {
					t.Error(err)
				}
				if err := client.ZRem(ctx, base.AllWorkers, workersKey).Err(); err != nil {
					t.Error(err)
				}
				if err := client.SRem(ctx, base.AllQueues, q).Err(); err != nil {
					t.Error(err)
				}
				if err := client.Del(ctx, keys...).Err(); err != nil {
					t.Error(err)
				}
				if n, err := client.DBSize(ctx).Result(); err != nil || n != 0 {
					t.Errorf("exact cleanup DB2 not empty: %d %v", n, err)
				}
			})
			h.SeedActiveQueue(t, client, msgs, q)
			h.SeedLease(t, client, entries, q)
			dumps := map[string]string{}
			for _, msg := range msgs {
				dump, err := client.Dump(ctx, base.TaskKey(q, msg.ID)).Result()
				if err != nil {
					t.Fatal(err)
				}
				dumps[msg.ID] = dump
			}
			expired := h.NewLeaseWithClock(now.Add(-time.Nanosecond), timeutil.NewSimulatedClock(now))
			crossing := h.NewLeaseWithClock(old, &heartbeatContractClock{first: old, later: old.Add(time.Nanosecond)})
			healthy := h.NewLeaseWithClock(old, timeutil.NewSimulatedClock(now))
			leases := []*base.Lease{expired, crossing, healthy}
			immutableMessages := make(map[string]base.TaskMessage)
			immutableHost, immutablePID := hb.host, hb.pid
			immutableStarted, immutableDeadline := now, now.Add(time.Hour)
			for _, msg := range msgs {
				copy := *msg
				copy.Payload = append([]byte(nil), msg.Payload...)
				immutableMessages[msg.ID] = copy
			}
			for i, msg := range msgs {
				hb.workers[msg.ID] = &workerInfo{msg: msg, started: now, deadline: now.Add(time.Hour), lease: leases[i]}
			}
			hb.beat()
			select {
			case <-expired.Done():
			default:
				t.Fatal("expired lease notification absent")
			}
			if !crossing.Deadline().Equal(old) {
				t.Fatalf("already expired local lease resurrected: %s", crossing.Deadline())
			}
			if !healthy.Deadline().Equal(expectedExtension) {
				t.Fatal("healthy neighbor did not extend")
			}
			if !strings.Contains(strings.Join(logger.entries, "\n"), "Lease reset failed for "+msgs[1].ID) {
				t.Fatal("reset refusal diagnostic absent")
			}
			for i, msg := range msgs {
				score, err := client.ZScore(ctx, base.LeaseKey(q), msg.ID).Result()
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 && score != float64(old.Unix()) {
					t.Fatal("expired lease extended in Redis")
				}
				if i > 0 && score != float64(expectedExtension.Unix()) {
					t.Fatal("real Redis extension absent")
				}
				dump, err := client.Dump(ctx, base.TaskKey(q, msg.ID)).Result()
				if err != nil || !bytes.Equal([]byte(dump), []byte(dumps[msg.ID])) {
					t.Fatalf("task hash changed %s %v", msg.ID, err)
				}
			}
			workers, err := broker.ListWorkers()
			if err != nil {
				t.Fatal(err)
			}
			if len(workers) != 3 {
				t.Fatalf("worker metadata lost including expired worker: %d", len(workers))
			}
			byID := map[string]*base.WorkerInfo{}
			for _, w := range workers {
				byID[w.ID] = w
			}
			for _, msg := range msgs {
				expected := immutableMessages[msg.ID]
				if !reflect.DeepEqual(*msg, expected) {
					t.Fatalf("worker input mutated %s", msg.ID)
				}
				w := byID[msg.ID]
				if w == nil || w.Queue != q || w.Type != expected.Type || !bytes.Equal(w.Payload, expected.Payload) || w.ServerID != hb.serverID || w.Host != immutableHost || w.PID != immutablePID || !w.Started.Equal(immutableStarted) || !w.Deadline.Equal(immutableDeadline) {
					t.Fatalf("typed worker metadata mismatch: %s %+v", msg.ID, w)
				}
			}
		})
	}
}
