package cmd

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/austinyuch/asynq/internal/rdb"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type cronServerOperation struct {
	name string
	cmd  *cobra.Command
	run  func(*cobra.Command, []string) error
}

func cronServerOperations() []cronServerOperation {
	return []cronServerOperation{{"cron-list", cronListCmd, cronList}, {"cron-history", cronHistoryCmd, cronHistory}, {"server-list", serverListCmd, serverList}}
}
func cronServerInvoke(t *testing.T, op cronServerOperation, ids []string) (string, error) {
	t.Helper()
	flags := map[string]string{}
	if op.name == "cron-history" {
		flags["page"] = "1"
		flags["size"] = "3"
	}
	return invokeAdministration(t, op.cmd, op.run, ids, flags)
}

// Assert the actual Redis protocol cause survives command formatting, independently
// of the user-facing prefix. Merely finding WRONGPASS in a string is insufficient.
func cronServerRequireRedisCause(t *testing.T, got, oracle error) {
	t.Helper()
	var expected redis.Error
	var actual redis.Error
	if oracle == nil || !errors.As(oracle, &expected) {
		t.Fatalf("independent API did not produce Redis cause: %T %v", oracle, oracle)
	}
	if got == nil || !errors.As(got, &actual) || reflect.TypeOf(actual) != reflect.TypeOf(expected) || actual.Error() != expected.Error() {
		t.Fatalf("command lost Redis cause: got=%T %v oracle=%T %v", got, got, oracle, oracle)
	}
}
func TestCronServerFailureAuthenticationCauseNoWrite(t *testing.T) {
	for _, op := range cronServerOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := taskFailureFixture(t)
			before := taskFailureSnapshot(t, f)
			viper.Set("username", "owned-cron-server-invalid-user")
			viper.Set("password", "owned-cron-server-invalid-password")
			// This independent API consumes the same real options but does not invoke the command.
			opt := getRedisConnOpt().(asynq.RedisClientOpt)
			i := asynq.NewInspector(opt)
			defer i.Close()
			var oracle error
			switch op.name {
			case "cron-list":
				_, oracle = i.SchedulerEntries()
			case "cron-history":
				_, oracle = i.ListSchedulerEnqueueEvents(f.queue + "-history")
			case "server-list":
				c := redis.NewClient(&redis.Options{Addr: opt.Addr, DB: opt.DB, Username: opt.Username, Password: opt.Password})
				defer c.Close()
				_, oracle = rdb.NewRDB(c).ListServers()
			}
			out, err := cronServerInvoke(t, op, []string{f.queue + "-history"})
			cronServerRequireRedisCause(t, err, oracle)
			for _, success := range []string{"No scheduler entries", "No running servers", "No scheduler enqueue events", "EnqueuedAt"} {
				if strings.Contains(out, success) {
					t.Fatalf("authentication failure printed success: %q", out)
				}
			}
			taskFailureRequireUnchanged(t, f, before)
		})
	}
}
func TestCronServerFailureHistoryMixedFirstErrorProperty(t *testing.T) {
	f := taskFailureFixture(t)
	raw := rdb.NewRDB(f.client)
	healthy := f.queue + "-history-healthy"
	badA := f.queue + "-history-wrongtype-a"
	badB := f.queue + "-history-malformed-protobuf"
	keys := []string{base.SchedulerHistoryKey(healthy), base.SchedulerHistoryKey(badA), base.SchedulerHistoryKey(badB)}
	t.Cleanup(func() {
		if e := f.client.Del(context.Background(), keys...).Err(); e != nil {
			t.Errorf("exact owned history cleanup: %v", e)
		}
	})
	if e := f.client.Set(context.Background(), base.SchedulerHistoryKey(badA), "owned-wrongtype:"+badA, 0).Err(); e != nil {
		t.Fatal(e)
	}
	// A valid ZSET with an invalid protobuf wire member produces a distinct
	// decoder error, unlike the first key's Redis WRONGTYPE protocol error.
	if e := f.client.ZAdd(context.Background(), base.SchedulerHistoryKey(badB), redis.Z{Score: 1800000000, Member: string([]byte{0xff})}).Err(); e != nil {
		t.Fatal(e)
	}
	_, errorA := f.inspector.ListSchedulerEnqueueEvents(badA)
	_, errorB := f.inspector.ListSchedulerEnqueueEvents(badB)
	if errorA == nil || errorB == nil || errorA.Error() == errorB.Error() {
		t.Fatalf("history fixtures must provide distinct actual API errors: A=%v B=%v", errorA, errorB)
	}
	event := &base.SchedulerEnqueueEvent{TaskID: f.queue + "-healthy-history-task", EnqueuedAt: time.Unix(1800000000, 0).UTC()}
	if e := raw.RecordSchedulerEnqueueEvent(healthy, event); e != nil {
		t.Fatal(e)
	}
	events, e := f.inspector.ListSchedulerEnqueueEvents(healthy, asynq.PageSize(3), asynq.Page(1))
	if e != nil || len(events) != 1 || events[0].TaskID != event.TaskID {
		t.Fatalf("healthy fixture API: %+v %v", events, e)
	}
	bytesBefore := map[string]string{}
	for _, key := range keys {
		v, e := f.client.Dump(context.Background(), key).Result()
		if e != nil {
			t.Fatal(e)
		}
		bytesBefore[key] = v
	}
	neighborBefore := taskFailureSnapshot(t, f)
	rng := rand.New(rand.NewSource(20261023))
	sequences := [][]string{{badA, healthy, badB}, {badB, healthy, badA}}
	for n := 0; n < 12; n++ {
		ids := []string{badA, healthy, badB}
		rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
		sequences = append(sequences, ids)
	}
	for n, ids := range sequences {
		var oracle error
		for _, id := range ids {
			_, e := f.inspector.ListSchedulerEnqueueEvents(id, asynq.PageSize(3), asynq.Page(1))
			if e != nil {
				oracle = e
				break
			}
		}
		out, err := cronServerInvoke(t, cronServerOperations()[1], ids)
		if !strings.Contains(out, "TaskID") || !strings.Contains(out, "EnqueuedAt") || !strings.Contains(out, events[0].TaskID) || !strings.Contains(out, events[0].EnqueuedAt.String()) || !strings.Contains(out, "error: ") {
			t.Fatalf("mixed history omitted healthy/error table case%d: %q", n, out)
		}
		if oracle == nil || err == nil || reflect.TypeOf(err) != reflect.TypeOf(oracle) || err.Error() != oracle.Error() {
			t.Fatalf("case%d did not return first API error: got%v want%v", n, err, oracle)
		}
		var expectedCause redis.Error
		if errors.As(oracle, &expectedCause) {
			cronServerRequireRedisCause(t, err, oracle)
		}
		for key, want := range bytesBefore {
			got, e := f.client.Dump(context.Background(), key).Result()
			ttl, te := f.client.PTTL(context.Background(), key).Result()
			if e != nil || te != nil || got != want || ttl != time.Duration(-1) {
				t.Fatalf("history read modified owned key%s: err%v/%v ttl%v", key, e, te, ttl)
			}
		}
		taskFailureRequireUnchanged(t, f, neighborBefore)
	}
}
func TestCronServerFailureOwnedConnectionsClosed(t *testing.T) {
	for _, op := range cronServerOperations() {
		t.Run(op.name, func(t *testing.T) {
			f := taskFailureFixture(t)
			f.requireTask(t, f.neighbor, "healthy-neighbor", "pending")
			// Warm independent fixture clients before observing newly command-owned IDs.
			if _, e := f.inspector.SchedulerEntries(); e != nil {
				t.Fatal(e)
			}
			if _, e := rdb.NewRDB(f.client).ListServers(); e != nil {
				t.Fatal(e)
			}
			beforeBytes := taskFailureSnapshot(t, f)
			before := queueStatsConnections(t, f)
			_, e := cronServerInvoke(t, op, []string{f.queue + "-empty-history"})
			if e != nil {
				t.Fatal(e)
			}
			deadline := time.Now().Add(time.Second)
			for {
				after := queueStatsConnections(t, f)
				var extra []string
				for id := range after {
					if !before[id] {
						extra = append(extra, id)
					}
				}
				if len(extra) == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("%s retained command-owned DB12 IDs: %v", op.name, extra)
				}
				time.Sleep(5 * time.Millisecond)
			}
			taskFailureRequireUnchanged(t, f, beforeBytes)
		})
	}
}

// No new fuzz target: these contracts require real Redis protocol/state oracles.
