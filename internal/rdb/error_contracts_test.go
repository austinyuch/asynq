package rdb

import (
	"context"
	stderrors "errors"
	"math/rand"
	"testing"
	"testing/quick"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	errors "github.com/austinyuch/asynq/internal/errors"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// Losing the transport must not report successful reads or mutations. The
// same public APIs are called by the worker and Inspector paths.
func TestClosedClientOperationContracts(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: redisAddr, DB: redisDB})
	r := NewRDB(client)
	if r.Client() != client {
		t.Fatal("Client does not preserve supplied connection")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	msg := h.NewTaskMessageWithQueue("contracts:closed", nil, "contracts")
	p := Pagination{Size: 10, Page: 0}
	now := time.Now()
	cases := []struct {
		name string
		call func() error
		raw  bool
	}{
		{"Ping", func() error { return r.Ping() }, true},
		{"RedisInfo", func() error { _, e := r.RedisInfo(); return e }, true},
		{"RedisClusterInfo", func() error { _, e := r.RedisClusterInfo(); return e }, true},
		{"AllQueues", func() error { _, e := r.AllQueues(); return e }, true},
		{"CurrentStats", func() error { _, e := r.CurrentStats(msg.Queue); return e }, false},
		{"HistoricalStats", func() error { _, e := r.HistoricalStats(msg.Queue, 2); return e }, false},
		{"GetTaskInfo", func() error { _, e := r.GetTaskInfo(msg.Queue, msg.ID); return e }, false},
		{"GroupStats", func() error { _, e := r.GroupStats(msg.Queue); return e }, false},
		{"ListPending", func() error { _, e := r.ListPending(msg.Queue, p); return e }, false},
		{"ListActive", func() error { _, e := r.ListActive(msg.Queue, p); return e }, false},
		{"ListScheduled", func() error { _, e := r.ListScheduled(msg.Queue, p); return e }, false},
		{"ListRetry", func() error { _, e := r.ListRetry(msg.Queue, p); return e }, false},
		{"ListArchived", func() error { _, e := r.ListArchived(msg.Queue, p); return e }, false},
		{"ListCompleted", func() error { _, e := r.ListCompleted(msg.Queue, p); return e }, false},
		{"ListAggregating", func() error { _, e := r.ListAggregating(msg.Queue, "group", p); return e }, false},
		{"RunTask", func() error { return r.RunTask(msg.Queue, msg.ID) }, false},
		{"ArchiveTask", func() error { return r.ArchiveTask(msg.Queue, msg.ID) }, false},
		{"UpdateTaskPayload", func() error { return r.UpdateTaskPayload(msg.Queue, msg.ID, []byte("new")) }, false},
		{"DeleteTask", func() error { return r.DeleteTask(msg.Queue, msg.ID) }, false},
		{"RunAllScheduledTasks", func() error { _, e := r.RunAllScheduledTasks(msg.Queue); return e }, false},
		{"RunAllRetryTasks", func() error { _, e := r.RunAllRetryTasks(msg.Queue); return e }, false},
		{"RunAllArchivedTasks", func() error { _, e := r.RunAllArchivedTasks(msg.Queue); return e }, false},
		{"RunAllAggregatingTasks", func() error { _, e := r.RunAllAggregatingTasks(msg.Queue, "group"); return e }, false},
		{"ArchiveAllPendingTasks", func() error { _, e := r.ArchiveAllPendingTasks(msg.Queue); return e }, false},
		{"ArchiveAllScheduledTasks", func() error { _, e := r.ArchiveAllScheduledTasks(msg.Queue); return e }, false},
		{"ArchiveAllRetryTasks", func() error { _, e := r.ArchiveAllRetryTasks(msg.Queue); return e }, false},
		{"ArchiveAllAggregatingTasks", func() error { _, e := r.ArchiveAllAggregatingTasks(msg.Queue, "group"); return e }, false},
		{"DeleteAllPendingTasks", func() error { _, e := r.DeleteAllPendingTasks(msg.Queue); return e }, false},
		{"DeleteAllScheduledTasks", func() error { _, e := r.DeleteAllScheduledTasks(msg.Queue); return e }, false},
		{"DeleteAllRetryTasks", func() error { _, e := r.DeleteAllRetryTasks(msg.Queue); return e }, false},
		{"DeleteAllArchivedTasks", func() error { _, e := r.DeleteAllArchivedTasks(msg.Queue); return e }, false},
		{"DeleteAllCompletedTasks", func() error { _, e := r.DeleteAllCompletedTasks(msg.Queue); return e }, false},
		{"DeleteAllAggregatingTasks", func() error { _, e := r.DeleteAllAggregatingTasks(msg.Queue, "group"); return e }, false},
		{"RemoveQueue", func() error { return r.RemoveQueue(msg.Queue, false) }, true},
		{"ListServers", func() error { _, e := r.ListServers(); return e }, true},
		{"ListWorkers", func() error { _, e := r.ListWorkers(); return e }, false},
		{"ListSchedulerEntries", func() error { _, e := r.ListSchedulerEntries(); return e }, true},
		{"ListSchedulerEnqueueEvents", func() error { _, e := r.ListSchedulerEnqueueEvents("entry", p); return e }, true},
		{"Pause", func() error { return r.Pause(msg.Queue) }, true},
		{"Unpause", func() error { return r.Unpause(msg.Queue) }, true},
		{"Enqueue", func() error { return r.Enqueue(ctx, msg) }, false},
		{"BatchEnqueue", func() error { _, e := r.BatchEnqueue(ctx, []base.BatchEnqueueItem{{Msg: msg}}); return e }, false},
		{"EnqueueUnique", func() error { return r.EnqueueUnique(ctx, msg, time.Minute) }, false},
		{"Dequeue", func() error { _, _, e := r.Dequeue(msg.Queue); return e }, false},
		{"Done", func() error { return r.Done(ctx, msg) }, false},
		{"MarkAsComplete", func() error { return r.MarkAsComplete(ctx, msg) }, false},
		{"Requeue", func() error { return r.Requeue(ctx, msg) }, false},
		{"AddToGroup", func() error { return r.AddToGroup(ctx, msg, "group") }, false},
		{"AddToGroupUnique", func() error { return r.AddToGroupUnique(ctx, msg, "group", time.Minute) }, false},
		{"Schedule", func() error { return r.Schedule(ctx, msg, now) }, false},
		{"ScheduleUnique", func() error { return r.ScheduleUnique(ctx, msg, now, time.Minute) }, false},
		{"Retry", func() error { return r.Retry(ctx, msg, now, "failure", true) }, false},
		{"Archive", func() error { return r.Archive(ctx, msg, "failure") }, false},
		{"ForwardIfReady", func() error { return r.ForwardIfReady(msg.Queue) }, false},
		{"ListGroups", func() error { _, e := r.ListGroups(msg.Queue); return e }, false},
		{"AggregationCheck", func() error {
			_, e := r.AggregationCheck(msg.Queue, "group", now, time.Minute, time.Hour, 10)
			return e
		}, false},
		{"ReadAggregationSet", func() error { _, _, e := r.ReadAggregationSet(msg.Queue, "group", "set"); return e }, false},
		{"DeleteAggregationSet", func() error { return r.DeleteAggregationSet(ctx, msg.Queue, "group", "set") }, false},
		{"ReclaimStaleAggregationSets", func() error { return r.ReclaimStaleAggregationSets(msg.Queue) }, false},
		{"DeleteExpiredCompletedTasks", func() error { return r.DeleteExpiredCompletedTasks(msg.Queue, 10) }, false},
		{"ListLeaseExpired", func() error { _, e := r.ListLeaseExpired(now, msg.Queue); return e }, false},
		{"ExtendLease", func() error { _, e := r.ExtendLease(msg.Queue, msg.ID); return e }, true},
		{"WriteServerState", func() error { return r.WriteServerState(&base.ServerInfo{}, nil, time.Minute) }, false},
		{"ClearServerState", func() error { return r.ClearServerState("host", 1, "server") }, false},
		{"WriteSchedulerEntries", func() error { return r.WriteSchedulerEntries("scheduler", nil, time.Minute) }, false},
		{"ClearSchedulerEntries", func() error { return r.ClearSchedulerEntries("scheduler") }, false},
		{"CancelationPubSub", func() error { _, e := r.CancelationPubSub(); return e }, false},
		{"PublishCancelation", func() error { return r.PublishCancelation(msg.ID) }, false},
		{"RecordSchedulerEnqueueEvent", func() error {
			return r.RecordSchedulerEnqueueEvent("entry", &base.SchedulerEnqueueEvent{EnqueuedAt: now})
		}, false},
		{"ClearSchedulerHistory", func() error { return r.ClearSchedulerHistory("entry") }, false},
		{"WriteResult", func() error { _, e := r.WriteResult(msg.Queue, msg.ID, []byte("result")); return e }, false},
	}
	expectedCodes := map[string]errors.Code{
		"Ping":                        errors.Unknown,
		"RedisInfo":                   errors.Unknown,
		"RedisClusterInfo":            errors.Unknown,
		"AllQueues":                   errors.Unknown,
		"CurrentStats":                errors.Unknown,
		"HistoricalStats":             errors.Unknown,
		"GetTaskInfo":                 errors.Unknown,
		"GroupStats":                  errors.Unknown,
		"ListPending":                 errors.Unknown,
		"ListActive":                  errors.Unknown,
		"ListScheduled":               errors.Unknown,
		"ListRetry":                   errors.Unknown,
		"ListArchived":                errors.Unknown,
		"ListCompleted":               errors.Unknown,
		"ListAggregating":             errors.Unknown,
		"RunTask":                     errors.Unknown,
		"ArchiveTask":                 errors.Unknown,
		"UpdateTaskPayload":           errors.Unknown,
		"DeleteTask":                  errors.Unknown,
		"RunAllScheduledTasks":        errors.Unknown,
		"RunAllRetryTasks":            errors.Unknown,
		"RunAllArchivedTasks":         errors.Unknown,
		"RunAllAggregatingTasks":      errors.Unknown,
		"ArchiveAllPendingTasks":      errors.Unknown,
		"ArchiveAllScheduledTasks":    errors.Internal,
		"ArchiveAllRetryTasks":        errors.Internal,
		"ArchiveAllAggregatingTasks":  errors.Unknown,
		"DeleteAllPendingTasks":       errors.Unknown,
		"DeleteAllScheduledTasks":     errors.Unknown,
		"DeleteAllRetryTasks":         errors.Unknown,
		"DeleteAllArchivedTasks":      errors.Unknown,
		"DeleteAllCompletedTasks":     errors.Unknown,
		"DeleteAllAggregatingTasks":   errors.Unknown,
		"RemoveQueue":                 errors.Unknown,
		"ListServers":                 errors.Unknown,
		"ListWorkers":                 errors.Unknown,
		"ListSchedulerEntries":        errors.Unknown,
		"ListSchedulerEnqueueEvents":  errors.Unknown,
		"Pause":                       errors.Unknown,
		"Unpause":                     errors.Unknown,
		"Enqueue":                     errors.Unknown,
		"BatchEnqueue":                errors.Unknown,
		"EnqueueUnique":               errors.Unknown,
		"Dequeue":                     errors.Unknown,
		"Done":                        errors.Internal,
		"MarkAsComplete":              errors.Internal,
		"Requeue":                     errors.Internal,
		"AddToGroup":                  errors.Unknown,
		"AddToGroupUnique":            errors.Unknown,
		"Schedule":                    errors.Unknown,
		"ScheduleUnique":              errors.Unknown,
		"Retry":                       errors.Internal,
		"Archive":                     errors.Internal,
		"ForwardIfReady":              errors.Internal,
		"ListGroups":                  errors.Unknown,
		"AggregationCheck":            errors.Unknown,
		"ReadAggregationSet":          errors.Unknown,
		"DeleteAggregationSet":        errors.Internal,
		"ReclaimStaleAggregationSets": errors.Internal,
		"DeleteExpiredCompletedTasks": errors.Internal,
		"ListLeaseExpired":            errors.Internal,
		"ExtendLease":                 errors.Unknown,
		"WriteServerState":            errors.Unknown,
		"ClearServerState":            errors.Internal,
		"WriteSchedulerEntries":       errors.Unknown,
		"ClearSchedulerEntries":       errors.Unknown,
		"CancelationPubSub":           errors.Unknown,
		"PublishCancelation":          errors.Unknown,
		"RecordSchedulerEnqueueEvent": errors.Internal,
		"ClearSchedulerHistory":       errors.Unknown,
		"WriteResult":                 errors.Unknown,
	}
	// These tested routes still format the Redis cause instead of wrapping it.
	formattedCauses := map[string]bool{
		"CancelationPubSub":           true,
		"DeleteExpiredCompletedTasks": true,
		"Dequeue":                     true,
		"ForwardIfReady":              true,
		"ListLeaseExpired":            true,
		"PublishCancelation":          true,
		"ReadAggregationSet":          true,
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("closed connection reported success")
			}
			if tc.raw {
				if !stderrors.Is(err, redis.ErrClosed) {
					t.Fatalf("lost transport cause: %v", err)
				}
			} else {
				var canonical *errors.Error
				want, classified := expectedCodes[tc.name]
				if !classified || !stderrors.As(err, &canonical) || canonical.Code != want {
					t.Fatalf("canonical transport code=%v want=%v: %v", errors.CanonicalCode(err), want, err)
				}
				if !formattedCauses[tc.name] && !stderrors.Is(err, redis.ErrClosed) {
					t.Fatalf("lost wrapped Redis transport cause: %v", err)
				}
			}
		})
	}
}

func TestInspectorWrongTypeContracts(t *testing.T) {
	ctx := context.Background()
	queue := "error-contracts"
	p := Pagination{Size: 10}
	now := time.Now()
	cases := []struct {
		name, key string
		call      func(*RDB) error
	}{
		{"stats", base.PendingKey(queue), func(r *RDB) error { _, e := r.CurrentStats(queue); return e }},
		{"history", base.ProcessedKey(queue, now), func(r *RDB) error { _, e := r.HistoricalStats(queue, 1); return e }},
		{"groups", base.AllGroups(queue), func(r *RDB) error { _, e := r.GroupStats(queue); return e }},
		{"pending", base.PendingKey(queue), func(r *RDB) error { _, e := r.ListPending(queue, p); return e }},
		{"active", base.ActiveKey(queue), func(r *RDB) error { _, e := r.ListActive(queue, p); return e }},
		{"scheduled", base.ScheduledKey(queue), func(r *RDB) error { _, e := r.ListScheduled(queue, p); return e }},
		{"retry", base.RetryKey(queue), func(r *RDB) error { _, e := r.ListRetry(queue, p); return e }},
		{"archived", base.ArchivedKey(queue), func(r *RDB) error { _, e := r.ListArchived(queue, p); return e }},
		{"completed", base.CompletedKey(queue), func(r *RDB) error { _, e := r.ListCompleted(queue, p); return e }},
		{"aggregating", base.GroupKey(queue, "group"), func(r *RDB) error { _, e := r.ListAggregating(queue, "group", p); return e }},
		{"run scheduled", base.ScheduledKey(queue), func(r *RDB) error { _, e := r.RunAllScheduledTasks(queue); return e }},
		{"archive scheduled", base.ScheduledKey(queue), func(r *RDB) error { _, e := r.ArchiveAllScheduledTasks(queue); return e }},
		{"run group", base.GroupKey(queue, "group"), func(r *RDB) error { _, e := r.RunAllAggregatingTasks(queue, "group"); return e }},
		{"archive group", base.GroupKey(queue, "group"), func(r *RDB) error { _, e := r.ArchiveAllAggregatingTasks(queue, "group"); return e }},
		{"archive pending", base.PendingKey(queue), func(r *RDB) error { _, e := r.ArchiveAllPendingTasks(queue); return e }},
		{"delete pending", base.PendingKey(queue), func(r *RDB) error { _, e := r.DeleteAllPendingTasks(queue); return e }},
		{"delete scheduled", base.ScheduledKey(queue), func(r *RDB) error { _, e := r.DeleteAllScheduledTasks(queue); return e }},
		{"delete group", base.GroupKey(queue, "group"), func(r *RDB) error { _, e := r.DeleteAllAggregatingTasks(queue, "group"); return e }},
		{"task info", base.TaskKey(queue, "id"), func(r *RDB) error { _, e := r.GetTaskInfo(queue, "id"); return e }},
		{"run task", base.TaskKey(queue, "id"), func(r *RDB) error { return r.RunTask(queue, "id") }},
		{"archive task", base.TaskKey(queue, "id"), func(r *RDB) error { return r.ArchiveTask(queue, "id") }},
		{"delete task", base.TaskKey(queue, "id"), func(r *RDB) error { return r.DeleteTask(queue, "id") }},
		{"remove queue", base.PendingKey(queue), func(r *RDB) error { return r.RemoveQueue(queue, false) }},
		{"write worker index", base.AllWorkers, func(r *RDB) error { return r.WriteServerState(&base.ServerInfo{}, nil, time.Minute) }},
		{"clear worker index", base.AllWorkers, func(r *RDB) error { return r.ClearServerState("host", 1, "server") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := setup(t)
			defer func() { _ = r.Close() }()
			h.SeedRedisSet(t, r.client, base.AllQueues, []string{queue})
			if tc.name == "history" {
				if err := r.client.RPush(ctx, tc.key, "invalid-key-type").Err(); err != nil {
					t.Fatal(err)
				}
			} else if err := r.client.Set(ctx, tc.key, "invalid-key-type", 0).Err(); err != nil {
				t.Fatal(err)
			}
			err := tc.call(r)
			if err == nil {
				t.Fatal("wrong-type fixture reported success")
			}
			code := errors.CanonicalCode(err)
			if code != errors.Unknown && code != errors.Internal {
				t.Fatalf("wrong-type should remain a storage error; code=%v err=%v", code, err)
			}
		})
	}
}

func TestPayloadUpdateStateContracts(t *testing.T) {
	r := setup(t)
	defer func() { _ = r.Close() }()
	ctx := context.Background()
	queue := "payload-contracts"
	msg := h.NewTaskMessageWithQueue("contracts:payload", []byte("before"), queue)
	if err := r.Schedule(ctx, msg, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	payload := []byte{0, 1, 255, 'x'}
	if err := r.UpdateTaskPayload(queue, msg.ID, payload); err != nil {
		t.Fatal(err)
	}
	info, err := r.GetTaskInfo(queue, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(info.Message.Payload) != string(payload) || info.State != base.TaskStateScheduled {
		t.Fatalf("update changed state or payload: %+v", info)
	}
	if err := r.RunTask(queue, msg.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateTaskPayload(queue, msg.ID, []byte("must-not-write")); errors.CanonicalCode(err) != errors.FailedPrecondition {
		t.Fatalf("pending update=%v", err)
	}
	info, err = r.GetTaskInfo(queue, msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(info.Message.Payload) != string(payload) {
		t.Fatal("rejected update mutated payload")
	}
	if err := r.UpdateTaskPayload(queue, "missing", nil); err == nil {
		t.Fatal("missing task update succeeded")
	}
	if err := r.UpdateTaskPayload("missing", msg.ID, nil); !errors.IsQueueNotFound(err) {
		t.Fatalf("missing queue update=%v", err)
	}
}

func TestHistoricalStatsValidationContracts(t *testing.T) {
	r := setup(t)
	defer func() { _ = r.Close() }()
	for _, days := range []int{-1, 0} {
		stats, err := r.HistoricalStats("missing", days)
		if stats != nil || errors.CanonicalCode(err) != errors.FailedPrecondition {
			t.Fatalf("days=%d stats=%v error=%v", days, stats, err)
		}
	}
	stats, err := r.HistoricalStats("missing", 1)
	if stats != nil || errors.CanonicalCode(err) != errors.NotFound {
		t.Fatalf("missing queue stats=%v err=%v", stats, err)
	}
	for _, list := range []func(string, Pagination) ([]*base.TaskInfo, error){r.ListCompleted, func(q string, p Pagination) ([]*base.TaskInfo, error) { return r.ListAggregating(q, "group", p) }} {
		infos, err := list("missing", Pagination{Size: 10})
		if infos != nil || errors.CanonicalCode(err) != errors.NotFound {
			t.Fatalf("missing queue infos=%v error=%v", infos, err)
		}
	}
	if _, err := r.DeleteAllCompletedTasks("missing"); errors.CanonicalCode(err) != errors.NotFound {
		t.Fatalf("missing queue delete=%v", err)
	}
}

func TestCorruptTaskInspectionContracts(t *testing.T) {
	for _, state := range []base.TaskState{base.TaskStatePending, base.TaskStateScheduled} {
		t.Run(state.String(), func(t *testing.T) {
			r := setup(t)
			defer func() { _ = r.Close() }()
			ctx := context.Background()
			queue := "corrupt-contracts"
			healthy := h.NewTaskMessageWithQueue("healthy", nil, queue)
			broken := h.NewTaskMessageWithQueue("broken", nil, queue)
			if state == base.TaskStatePending {
				h.SeedPendingQueue(t, r.client, []*base.TaskMessage{broken, healthy}, queue)
			} else {
				h.SeedScheduledQueue(t, r.client, []base.Z{{Message: broken, Score: 1}, {Message: healthy, Score: 2}}, queue)
			}
			if err := r.client.HSet(ctx, base.TaskKey(queue, broken.ID), "msg", []byte{255}).Err(); err != nil {
				t.Fatal(err)
			}
			if _, err := r.WriteResult(queue, healthy.ID, []byte("result")); err != nil {
				t.Fatal(err)
			}
			var infos []*base.TaskInfo
			var err error
			if state == base.TaskStatePending {
				infos, err = r.ListPending(queue, Pagination{Size: 10})
			} else {
				infos, err = r.ListScheduled(queue, Pagination{Size: 10})
			}
			if err != nil || len(infos) != 1 || infos[0].Message.ID != healthy.ID || string(infos[0].Result) != "result" {
				t.Fatalf("healthy task not preserved: infos=%v err=%v", infos, err)
			}
			info, err := r.GetTaskInfo(queue, broken.ID)
			if info != nil || errors.CanonicalCode(err) != errors.Internal {
				t.Fatalf("corrupt task info=%v err=%v", info, err)
			}
			if err := r.client.HSet(ctx, base.TaskKey(queue, healthy.ID), "state", "unsupported").Err(); err != nil {
				t.Fatal(err)
			}
			info, err = r.GetTaskInfo(queue, healthy.ID)
			if info != nil || errors.CanonicalCode(err) != errors.FailedPrecondition {
				t.Fatalf("corrupt task state info=%v err=%v", info, err)
			}
		})
	}
}

func TestInvalidUTF8TaskWriteContracts(t *testing.T) {
	r := setup(t)
	defer func() { _ = r.Close() }()
	ctx := context.Background()
	now := time.Now()
	invalid := h.NewTaskMessageWithQueue(string([]byte{255}), nil, "invalid-encoding")
	calls := []func() error{
		func() error { return r.Enqueue(ctx, invalid) },
		func() error { _, e := r.BatchEnqueue(ctx, []base.BatchEnqueueItem{{Msg: invalid}}); return e },
		func() error { return r.EnqueueUnique(ctx, invalid, time.Minute) },
		func() error { return r.AddToGroup(ctx, invalid, "group") },
		func() error { return r.AddToGroupUnique(ctx, invalid, "group", time.Minute) },
		func() error { return r.Schedule(ctx, invalid, now) },
		func() error { return r.ScheduleUnique(ctx, invalid, now, time.Minute) },
		func() error { return r.MarkAsComplete(ctx, invalid) },
		func() error { return r.Retry(ctx, invalid, now, "failure", true) },
		func() error { return r.Archive(ctx, invalid, "failure") },
	}
	for i, call := range calls {
		if err := call(); err == nil {
			t.Errorf("writer %d accepted invalid UTF8 task type", i)
		}
	}
	queues, err := r.AllQueues()
	if err != nil {
		t.Fatal(err)
	}
	if len(queues) != 0 {
		t.Fatalf("rejected writes published queue state: %v", queues)
	}
}

func TestRegistryCorruptionIsolationContracts(t *testing.T) {
	r := setup(t)
	defer func() { _ = r.Close() }()
	ctx := context.Background()
	expires := time.Now().Add(time.Hour).Unix()
	info := &base.ServerInfo{Host: "good", PID: 1, ServerID: "server"}
	worker := &base.WorkerInfo{ID: "task", Type: "healthy", Queue: "queue"}
	if err := r.WriteServerState(info, []*base.WorkerInfo{worker}, time.Hour); err != nil {
		t.Fatal(err)
	}
	serverKey := base.ServerInfoKey(info.Host, info.PID, info.ServerID)
	workersKey := base.WorkersKey(info.Host, info.PID, info.ServerID)
	h.SeedRedisZSets(t, r.client, map[string][]redis.Z{
		base.AllServers: {{Member: serverKey, Score: float64(expires)}, {Member: base.ServerInfoKey("corrupt", 1, "server"), Score: float64(expires)}, {Member: base.ServerInfoKey("missing", 1, "server"), Score: float64(expires)}},
		base.AllWorkers: {{Member: workersKey, Score: float64(expires)}, {Member: base.WorkersKey("wrong-type", 1, "server"), Score: float64(expires)}},
	})
	if err := r.client.Set(ctx, base.ServerInfoKey("corrupt", 1, "server"), []byte{255}, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.client.Set(ctx, base.WorkersKey("wrong-type", 1, "server"), "not-a-hash", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.client.HSet(ctx, workersKey, "corrupt", []byte{255}).Err(); err != nil {
		t.Fatal(err)
	}
	servers, err := r.ListServers()
	if err != nil || len(servers) != 1 || servers[0].Host != info.Host {
		t.Fatalf("servers=%v err=%v", servers, err)
	}
	workers, err := r.ListWorkers()
	if err != nil || len(workers) != 1 || workers[0].ID != worker.ID {
		t.Fatalf("workers=%v err=%v", workers, err)
	}
	entry := &base.SchedulerEntry{ID: "healthy"}
	if err := r.WriteSchedulerEntries("scheduler", []*base.SchedulerEntry{entry}, time.Hour); err != nil {
		t.Fatal(err)
	}
	key := base.SchedulerEntriesKey("scheduler")
	if err := r.client.RPush(ctx, key, []byte{255}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.client.ZAdd(ctx, base.AllSchedulers, redis.Z{Member: base.SchedulerEntriesKey("wrong-type"), Score: float64(expires)}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := r.client.Set(ctx, base.SchedulerEntriesKey("wrong-type"), "not-a-list", 0).Err(); err != nil {
		t.Fatal(err)
	}
	entries, err := r.ListSchedulerEntries()
	if err != nil || len(entries) != 1 || entries[0].ID != entry.ID {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if err := r.ClearSchedulerHistory("entry"); err != nil {
		t.Fatal(err)
	}
	if err := r.client.ZAdd(ctx, base.SchedulerHistoryKey("entry"), redis.Z{Member: string([]byte{255}), Score: 1}).Err(); err != nil {
		t.Fatal(err)
	}
	if events, err := r.ListSchedulerEnqueueEvents("entry", Pagination{Size: 10}); err == nil || events != nil {
		t.Fatalf("corrupt history events=%v err=%v", events, err)
	}
}

func TestQueuePublicationFailureContracts(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	calls := []struct {
		name string
		call func(*RDB, *base.TaskMessage) error
	}{
		{"enqueue", func(r *RDB, m *base.TaskMessage) error { return r.Enqueue(ctx, m) }},
		{"unique enqueue", func(r *RDB, m *base.TaskMessage) error { return r.EnqueueUnique(ctx, m, time.Minute) }},
		{"group", func(r *RDB, m *base.TaskMessage) error { return r.AddToGroup(ctx, m, "group") }},
		{"unique group", func(r *RDB, m *base.TaskMessage) error { return r.AddToGroupUnique(ctx, m, "group", time.Minute) }},
		{"scheduled", func(r *RDB, m *base.TaskMessage) error { return r.Schedule(ctx, m, now) }},
		{"unique scheduled", func(r *RDB, m *base.TaskMessage) error { return r.ScheduleUnique(ctx, m, now, time.Minute) }},
		{"batch", func(r *RDB, m *base.TaskMessage) error {
			_, e := r.BatchEnqueue(ctx, []base.BatchEnqueueItem{{Msg: m}})
			return e
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			r := setup(t)
			defer func() { _ = r.Close() }()
			msg := h.NewTaskMessageWithQueue("publication:test", nil, "publication")
			msg.UniqueKey = base.UniqueKey(msg.Queue, msg.Type, msg.Payload)
			if err := r.client.Set(ctx, base.AllQueues, "wrong-type", 0).Err(); err != nil {
				t.Fatal(err)
			}
			if err := tc.call(r, msg); err == nil {
				t.Fatal("failed queue publication reported success")
			}
			// Pipeline enqueue is intentionally non-atomic. After repairing its index,
			// retrying with a fresh task must republish rather than trust a stale cache.
			if err := r.client.Del(ctx, base.AllQueues).Err(); err != nil {
				t.Fatal(err)
			}
			fresh := h.NewTaskMessageWithQueue("publication:retry", nil, msg.Queue)
			fresh.UniqueKey = base.UniqueKey(fresh.Queue, fresh.Type, fresh.Payload)
			if err := tc.call(r, fresh); err != nil {
				t.Fatal(err)
			}
			queues, err := r.AllQueues()
			if err != nil || len(queues) != 1 || queues[0] != msg.Queue {
				t.Fatalf("queue not republished: %v %v", queues, err)
			}
		})
	}
}

func TestCorruptLeaseAndAggregationContracts(t *testing.T) {
	t.Run("dequeue", func(t *testing.T) {
		r := setup(t)
		defer func() { _ = r.Close() }()
		msg := h.NewTaskMessageWithQueue("corrupt", nil, "dequeue-corrupt")
		h.SeedPendingQueue(t, r.client, []*base.TaskMessage{msg}, msg.Queue)
		if err := r.client.HSet(context.Background(), base.TaskKey(msg.Queue, msg.ID), "msg", []byte{255}).Err(); err != nil {
			t.Fatal(err)
		}
		got, _, err := r.Dequeue(msg.Queue)
		if got != nil || errors.CanonicalCode(err) != errors.Internal {
			t.Fatalf("dequeued=%v err=%v", got, err)
		}
	})
	t.Run("expired lease", func(t *testing.T) {
		r := setup(t)
		defer func() { _ = r.Close() }()
		msg := h.NewTaskMessageWithQueue("corrupt", nil, "lease-corrupt")
		h.SeedActiveQueue(t, r.client, []*base.TaskMessage{msg}, msg.Queue)
		h.SeedLease(t, r.client, []base.Z{{Message: msg, Score: 1}}, msg.Queue)
		if err := r.client.HSet(context.Background(), base.TaskKey(msg.Queue, msg.ID), "msg", []byte{255}).Err(); err != nil {
			t.Fatal(err)
		}
		msgs, err := r.ListLeaseExpired(time.Now(), msg.Queue)
		if msgs != nil || errors.CanonicalCode(err) != errors.Internal {
			t.Fatalf("expired=%v err=%v", msgs, err)
		}
	})
	t.Run("aggregation message", func(t *testing.T) {
		r := setup(t)
		defer func() { _ = r.Close() }()
		msg := h.NewTaskMessageWithQueue("corrupt", nil, "aggregation-corrupt")
		h.SeedAggregationSet(t, r.client, []base.Z{{Message: msg, Score: 1}}, msg.Queue, "group", "set")
		if err := r.client.HSet(context.Background(), base.TaskKey(msg.Queue, msg.ID), "msg", []byte{255}).Err(); err != nil {
			t.Fatal(err)
		}
		msgs, _, err := r.ReadAggregationSet(msg.Queue, "group", "set")
		if msgs != nil || errors.CanonicalCode(err) != errors.Internal {
			t.Fatalf("aggregation=%v err=%v", msgs, err)
		}
	})
	t.Run("aggregation deadline missing", func(t *testing.T) {
		r := setup(t)
		defer func() { _ = r.Close() }()
		msgs, _, err := r.ReadAggregationSet("missing", "group", "set")
		if msgs != nil || errors.CanonicalCode(err) != errors.Unknown {
			t.Fatalf("aggregation=%v err=%v", msgs, err)
		}
	})
}

func TestInvalidUTF8RegistryWrites(t *testing.T) {
	r := setup(t)
	defer func() { _ = r.Close() }()
	invalid := string([]byte{255})
	if err := r.WriteServerState(&base.ServerInfo{Host: invalid}, nil, time.Minute); errors.CanonicalCode(err) != errors.Internal {
		t.Fatalf("server encoding=%v", err)
	}
	if err := r.RecordSchedulerEnqueueEvent("entry", &base.SchedulerEnqueueEvent{TaskID: invalid, EnqueuedAt: time.Now()}); errors.CanonicalCode(err) != errors.Internal {
		t.Fatalf("event encoding=%v", err)
	}
	info := &base.ServerInfo{Host: "valid", PID: 1, ServerID: "server"}
	if err := r.WriteServerState(info, []*base.WorkerInfo{{ID: "bad", Type: invalid}, {ID: "good", Type: "valid"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	workers, err := r.ListWorkers()
	if err != nil || len(workers) != 1 || workers[0].ID != "good" {
		t.Fatalf("workers=%v err=%v", workers, err)
	}
	if err := r.WriteSchedulerEntries("scheduler", []*base.SchedulerEntry{{ID: invalid}, {ID: "good"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	entries, err := r.ListSchedulerEntries()
	if err != nil || len(entries) != 1 || entries[0].ID != "good" {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
}

// Arbitrary bytes after an illegal protobuf field-zero tag must never displace
// the healthy task beside it. The seed makes generated failures reproducible.
func TestGeneratedCorruptTaskIsolation(t *testing.T) {
	r := setup(t)
	defer func() { _ = r.Close() }()
	queue := "generated-corruption"
	healthy := h.NewTaskMessageWithQueue("healthy", []byte("preserve"), queue)
	broken := h.NewTaskMessageWithQueue("broken", nil, queue)
	h.SeedPendingQueue(t, r.client, []*base.TaskMessage{broken, healthy}, queue)
	property := func(bytes []byte) bool {
		if len(bytes) > 64 {
			bytes = bytes[:64]
		}
		invalid := append([]byte{0}, bytes...)
		if err := r.client.HSet(context.Background(), base.TaskKey(queue, broken.ID), "msg", invalid).Err(); err != nil {
			t.Fatal(err)
		}
		info, err := r.GetTaskInfo(queue, broken.ID)
		if info != nil || errors.CanonicalCode(err) != errors.Internal {
			return false
		}
		infos, err := r.ListPending(queue, Pagination{Size: 10})
		return err == nil && len(infos) == 1 && infos[0].Message.ID == healthy.ID && string(infos[0].Message.Payload) == "preserve"
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 20, Rand: rand.New(rand.NewSource(20261001))}); err != nil {
		t.Fatal(err)
	}
}

func TestRealClusterIntrospectionContracts(t *testing.T) {
	if !useRedisCluster {
		t.Skip("requires explicitly configured real Redis Cluster; standalone coverage excludes cluster success paths")
	}
	r := setup(t)
	defer func() { _ = r.Close() }()
	info, err := r.RedisClusterInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info["cluster_state"] != "ok" || info["cluster_slots_assigned"] != "16384" {
		t.Fatalf("cluster is not fully assigned/healthy: %v", info)
	}
	for _, queue := range []string{"default", "critical", "custom"} {
		slot, err := r.ClusterKeySlot(queue)
		if err != nil {
			t.Fatal(err)
		}
		if slot < 0 || slot >= 16384 {
			t.Fatalf("queue %s invalid slot %d", queue, slot)
		}
		nodes, err := r.ClusterNodes(queue)
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) == 0 {
			t.Fatalf("queue %s slot %d has no owner", queue, slot)
		}
		for _, node := range nodes {
			if node.ID == "" || node.Addr == "" {
				t.Fatalf("queue %s incomplete owning node %+v", queue, node)
			}
		}
	}
}
