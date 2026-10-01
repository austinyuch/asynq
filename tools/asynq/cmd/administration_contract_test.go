package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	internalerrors "github.com/austinyuch/asynq/internal/errors"
	"github.com/austinyuch/asynq/internal/rdb"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type administrationFixture struct {
	client    *redis.Client
	inspector *asynq.Inspector
	queue     string
	neighbor  string
}

// Every fixture uses only unique queues in explicitly enabled, dedicated DB12.
// Cleanup deletes those queues; neither setup nor cleanup flushes the database.
func newAdministrationFixture(t *testing.T) *administrationFixture {
	t.Helper()
	addr := os.Getenv("ASYNQ_CLI_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ASYNQ_CLI_TEST_REDIS_ADDR for dedicated DB12 runtime")
	}
	isolatedCLI(t)
	for key, value := range map[string]interface{}{"uri": addr, "db": 12, "cluster": false, "password": "", "username": "", "tls": false, "tls_server": "", "insecure": false} {
		viper.Set(key, value)
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 12})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	inspector := createInspector()
	t.Cleanup(func() { _ = inspector.Close() })
	prefix := fmt.Sprintf("cli-admin-%d", time.Now().UnixNano())
	fixture := &administrationFixture{client: client, inspector: inspector, queue: prefix, neighbor: prefix + "-neighbor"}
	t.Cleanup(func() {
		for _, queue := range []string{fixture.queue, fixture.neighbor} {
			if err := inspector.DeleteQueue(queue, true); err != nil && !errors.Is(err, asynq.ErrQueueNotFound) {
				t.Errorf("cleanup owned queue %s: %v", queue, err)
			}
			// The consumed library's queue deletion leaves some group/completed
			// keys. Enumerate only this uniquely named queue's canonical scope.
			keys, err := client.Keys(context.Background(), base.QueueKeyPrefix(queue)+"*").Result()
			if err != nil {
				t.Errorf("enumerate owned cleanup keys: %v", err)
				continue
			}
			if len(keys) > 0 {
				if err := client.Del(context.Background(), keys...).Err(); err != nil {
					t.Errorf("remove owned fixture keys: %v", err)
				}
			}
		}
	})
	return fixture
}

func (f *administrationFixture) seed(t *testing.T, queue, state, group string, ids ...string) []*base.TaskMessage {
	t.Helper()
	msgs := make([]*base.TaskMessage, len(ids))
	entries := make([]base.Z, len(ids))
	for i, id := range ids {
		msg := h.NewTaskMessageWithQueue("admin-contract", []byte("payload:"+id), queue)
		msg.ID = id
		msg.GroupKey = group
		msgs[i] = msg
		entries[i] = base.Z{Message: msg, Score: time.Now().Add(time.Hour).Unix()}
	}
	switch state {
	case "pending":
		h.SeedPendingQueue(t, f.client, msgs, queue)
	case "scheduled":
		h.SeedScheduledQueue(t, f.client, entries, queue)
	case "retry":
		h.SeedRetryQueue(t, f.client, entries, queue)
	case "archived":
		h.SeedArchivedQueue(t, f.client, entries, queue)
	case "completed":
		h.SeedCompletedQueue(t, f.client, entries, queue)
	case "aggregating":
		h.SeedGroup(t, f.client, entries, queue, group)
	default:
		t.Fatalf("unsupported fixture state %s", state)
	}
	return msgs
}
func (f *administrationFixture) requireTask(t *testing.T, queue, id, state string) {
	t.Helper()
	info, err := f.inspector.GetTaskInfo(queue, id)
	if state == "deleted" {
		if !errors.Is(err, asynq.ErrTaskNotFound) {
			t.Fatalf("task %s should be deleted: info=%+v err=%v", id, info, err)
		}
		return
	}
	if err != nil || info.State.String() != state || string(info.Payload) != "payload:"+id {
		t.Fatalf("task %s want state=%s preserved payload: info=%+v err=%v", id, state, info, err)
	}
}
func invokeAdministration(t *testing.T, cmd *cobra.Command, run func(*cobra.Command, []string) error, args []string, flags map[string]string) (string, error) {
	t.Helper()
	for key, value := range flags {
		setCLIFlag(t, cmd, key, value)
	}
	var err error
	output := captureCLI(t, func() { err = run(cmd, args) })
	return output, err
}

func TestTaskAdministrationSingleTransitionsRuntime(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		cmd                     *cobra.Command
		run                     func(*cobra.Command, []string) error
		initial, final, message string
	}{
		{"archive pending", taskArchiveCmd, taskArchive, "pending", "archived", "task archived"},
		{"run scheduled", taskRunCmd, taskRun, "scheduled", "pending", "task is now pending"},
		{"delete archived", taskDeleteCmd, taskDelete, "archived", "deleted", "task deleted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdministrationFixture(t)
			f.seed(t, f.queue, tc.initial, "", "target", "same-queue-neighbor")
			f.seed(t, f.neighbor, tc.initial, "", "other-queue-neighbor")
			output, err := invokeAdministration(t, tc.cmd, tc.run, nil, map[string]string{"queue": f.queue, "id": "target"})
			if err != nil || !strings.Contains(output, tc.message) {
				t.Fatalf("operation err=%v output=%s", err, output)
			}
			f.requireTask(t, f.queue, "target", tc.final)
			f.requireTask(t, f.queue, "same-queue-neighbor", tc.initial)
			f.requireTask(t, f.neighbor, "other-queue-neighbor", tc.initial)
			_, err = invokeAdministration(t, tc.cmd, tc.run, nil, map[string]string{"queue": f.queue, "id": "missing"})
			if err == nil || !strings.Contains(err.Error(), "could not ") || !strings.Contains(err.Error(), "task not found") {
				t.Fatalf("missing task lost command context: %v", err)
			}
		})
	}
}

func TestTaskAdministrationBulkStateAndNeighborContractsRuntime(t *testing.T) {
	for _, operation := range []struct {
		name           string
		cmd            *cobra.Command
		run            func(*cobra.Command, []string) error
		states         []string
		final, message string
	}{
		{"archive", taskArchiveAllCmd, taskArchiveAll, []string{"pending", "scheduled", "retry", "aggregating"}, "archived", "2 tasks archived"},
		{"delete", taskDeleteAllCmd, taskDeleteAll, []string{"pending", "scheduled", "retry", "archived", "completed", "aggregating"}, "deleted", "2 tasks deleted"},
		{"run", taskRunAllCmd, taskRunAll, []string{"scheduled", "retry", "archived", "aggregating"}, "pending", "2 tasks are now pending"},
	} {
		for _, state := range operation.states {
			t.Run(operation.name+"/"+state, func(t *testing.T) {
				f := newAdministrationFixture(t)
				f.seed(t, f.queue, state, "selected-group", "target-a", "target-b")
				neighborState := "pending"
				if state == "pending" {
					neighborState = "scheduled"
				}
				f.seed(t, f.queue, neighborState, "", "other-state-neighbor")
				f.seed(t, f.neighbor, state, "selected-group", "other-queue-neighbor")
				if state == "aggregating" {
					f.seed(t, f.queue, state, "other-group", "other-group-neighbor")
				}
				output, err := invokeAdministration(t, operation.cmd, operation.run, nil, map[string]string{"queue": f.queue, "state": state, "group": "selected-group"})
				if err != nil || !strings.Contains(output, operation.message) {
					t.Fatalf("bulk operation err=%v output=%s", err, output)
				}
				for _, id := range []string{"target-a", "target-b"} {
					f.requireTask(t, f.queue, id, operation.final)
				}
				f.requireTask(t, f.queue, "other-state-neighbor", neighborState)
				f.requireTask(t, f.neighbor, "other-queue-neighbor", state)
				if state == "aggregating" {
					f.requireTask(t, f.queue, "other-group-neighbor", state)
				}
			})
		}
	}
}

func TestTaskAdministrationValidationLeavesDataRuntime(t *testing.T) {
	for _, operation := range []struct {
		name string
		cmd  *cobra.Command
		run  func(*cobra.Command, []string) error
	}{
		{"archive", taskArchiveAllCmd, taskArchiveAll}, {"delete", taskDeleteAllCmd, taskDeleteAll}, {"run", taskRunAllCmd, taskRunAll},
	} {
		for _, state := range []string{"unsupported", "aggregating"} {
			t.Run(operation.name+"/"+state, func(t *testing.T) {
				f := newAdministrationFixture(t)
				f.seed(t, f.queue, "pending", "", "untouched")
				_, err := invokeAdministration(t, operation.cmd, operation.run, nil, map[string]string{"queue": f.queue, "state": state, "group": ""})
				want := `unsupported state "unsupported"`
				if state == "aggregating" {
					want = "flag --group is required"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("validation err=%v want %s", err, want)
				}
				f.requireTask(t, f.queue, "untouched", "pending")
			})
		}
	}
}

func TestQueueAdministrationIsolationRuntime(t *testing.T) {
	f := newAdministrationFixture(t)
	f.seed(t, f.queue, "pending", "", "target")
	f.seed(t, f.neighbor, "pending", "", "neighbor")
	output, err := invokeAdministration(t, queueRemoveCmd, queueRemove, []string{f.queue}, map[string]string{"force": "false"})
	if err == nil || !strings.Contains(output, "--force") {
		t.Fatalf("nonempty removal should require force: %v %s", err, output)
	}
	f.requireTask(t, f.queue, "target", "pending")
	f.requireTask(t, f.neighbor, "neighbor", "pending")
	// An invalid blank queue does not stop later valid queues; return the first error.
	for _, op := range []struct {
		cmd    *cobra.Command
		run    func(*cobra.Command, []string) error
		paused bool
	}{{queuePauseCmd, queuePause, true}, {queueUnpauseCmd, queueUnpause, false}} {
		_, err := invokeAdministration(t, op.cmd, op.run, []string{"", f.queue}, nil)
		if err == nil || !strings.Contains(err.Error(), "queue") {
			t.Fatalf("invalid queue cause lost: %v", err)
		}
		info, err := f.inspector.GetQueueInfo(f.queue)
		if err != nil || info.Paused != op.paused {
			t.Fatalf("valid queue not processed after failure: %+v %v", info, err)
		}
		neighbor, err := f.inspector.GetQueueInfo(f.neighbor)
		if err != nil || neighbor.Paused {
			t.Fatalf("neighbor pause state changed: %+v %v", neighbor, err)
		}
	}
	output, err = invokeAdministration(t, queueRemoveCmd, queueRemove, []string{f.queue + "-missing", f.queue}, map[string]string{"force": "true"})
	if err == nil || !strings.Contains(output, "Successfully removed queue") {
		t.Fatalf("mixed removal output=%s err=%v", output, err)
	}
	if info, err := f.inspector.GetQueueInfo(f.queue); info != nil || !internalerrors.IsQueueNotFound(err) {
		t.Fatalf("forced queue removal incomplete: %v", err)
	}
	f.requireTask(t, f.neighbor, "neighbor", "pending")
}

func TestAdministrativeReadSurfacesRuntime(t *testing.T) {
	f := newAdministrationFixture(t)
	f.seed(t, f.queue, "pending", "", "pending")
	f.seed(t, f.queue, "aggregating", "batch", "grouped")
	f.seed(t, f.neighbor, "scheduled", "", "neighbor")
	out, err := invokeAdministration(t, groupListCmd, groupLists, nil, map[string]string{"queue": f.queue})
	if err != nil || strings.TrimSpace(out) != "batch" {
		t.Fatalf("group list: %s %v", out, err)
	}
	out, err = invokeAdministration(t, groupListCmd, groupLists, nil, map[string]string{"queue": f.neighbor})
	if err != nil || !strings.Contains(out, "No groups found") {
		t.Fatalf("empty groups: %s %v", out, err)
	}
	out, err = invokeAdministration(t, taskListCmd, taskList, nil, map[string]string{"queue": f.queue, "state": "aggregating", "group": "batch", "page": "1", "size": "1"})
	if err != nil || !strings.Contains(out, "grouped") || strings.Contains(out, "neighbor") {
		t.Fatalf("aggregate task selection: %s %v", out, err)
	}
	out, err = invokeAdministration(t, queueListCmd, queueList, nil, nil)
	if err != nil || !strings.Contains(out, f.queue) || !strings.Contains(out, f.neighbor) {
		t.Fatalf("queue list: %s %v", out, err)
	}
	out, err = invokeAdministration(t, queueInspectCmd, queueInspect, []string{f.queue + "-missing", f.queue}, nil)
	if err == nil || !strings.Contains(out, "error:") || !strings.Contains(out, f.queue) || !strings.Contains(out, separator) {
		t.Fatalf("queue inspection continuation: %s %v", out, err)
	}
	out, err = invokeAdministration(t, queueHistoryCmd, queueHistory, []string{f.queue + "-missing", f.queue}, map[string]string{"days": "2"})
	if err == nil || !strings.Contains(out, "error:") || !strings.Contains(out, "date (UTC)") {
		t.Fatalf("history continuation: %s %v", out, err)
	}
	// Both JSON and human-readable stats must agree with independent queue state.
	for _, format := range []string{"true", "false"} {
		t.Run("stats-json-"+format, func(t *testing.T) {
			out, err := invokeAdministration(t, statsCmd, stats, nil, map[string]string{"json": format})
			if err != nil {
				t.Fatal(err)
			}
			if format == "true" {
				var got FullStats
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatal(err)
				}
				if got.Aggregate.Pending != 1 || got.Aggregate.Aggregating != 1 || got.Aggregate.Scheduled != 1 || len(got.QueueStats) != 2 {
					t.Fatalf("JSON stats lost state counts: %+v", got)
				}
			} else {
				for _, want := range []string{f.queue, f.neighbor, "pending", "aggregating", "scheduled", "version"} {
					if !strings.Contains(out, want) {
						t.Errorf("text stats missing %q: %s", want, out)
					}
				}
			}
		})
	}
	f.requireTask(t, f.queue, "pending", "pending")
	f.requireTask(t, f.queue, "grouped", "aggregating")
	f.requireTask(t, f.neighbor, "neighbor", "scheduled")
}

func TestAdministrativeAuthenticationAndTLSFailuresRuntime(t *testing.T) {
	f := newAdministrationFixture(t)
	viper.Set("password", "owned-invalid-password")
	viper.Set("username", "owned-nonexistent-user")
	_, err := invokeAdministration(t, queueListCmd, queueList, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "could not fetch list of queues:") || !strings.Contains(strings.ToLower(err.Error()), "wrongpass") {
		t.Fatalf("authentication failure lost cause: %v", err)
	}

	viper.Set("password", "")
	viper.Set("username", "")
	// A controlled plaintext endpoint replies immediately, making a real TLS
	// negotiation failure deterministic instead of relying on Redis timeouts.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = io.WriteString(conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	viper.Set("uri", listener.Addr().String())
	viper.Set("tls_server", "localhost")
	opt := getRedisConnOpt().(asynq.RedisClientOpt)
	if opt.TLSConfig == nil || opt.TLSConfig.ServerName != "localhost" {
		t.Fatal("TLS server flag did not select TLS")
	}
	_, err = invokeAdministration(t, queueListCmd, queueList, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "could not fetch list of queues:") || !strings.Contains(err.Error(), "tls:") {
		t.Fatalf("expected real TLS negotiation failure, got %v", err)
	}
	viper.Set("tls_server", "")
	if err := f.client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("TLS test affected healthy plaintext endpoint: %v", err)
	}
}

func TestAdministrativeSchedulerAndServerOrderingRuntime(t *testing.T) {
	f := newAdministrationFixture(t)
	raw := rdb.NewRDB(f.client)
	schedulerID := f.queue + "-scheduler"
	entries := []*base.SchedulerEntry{
		{ID: f.queue + "-later", Spec: "@every 2h", Type: "later", Payload: []byte("later-payload"), Opts: []string{asynq.Queue(f.queue).String()}, Next: time.Now().Add(time.Hour)},
		{ID: f.queue + "-earlier", Spec: "@every 1h", Type: "earlier", Payload: []byte("earlier-payload"), Next: time.Now().Add(-time.Hour), Prev: time.Now().Add(-time.Hour)},
	}
	if err := raw.WriteSchedulerEntries(schedulerID, entries, time.Minute); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := raw.ClearSchedulerEntries(schedulerID); err != nil {
			t.Errorf("cleanup scheduler: %v", err)
		}
	})
	for _, entry := range entries {
		event := &base.SchedulerEnqueueEvent{TaskID: entry.ID + "-task", EnqueuedAt: time.Now()}
		if err := raw.RecordSchedulerEnqueueEvent(entry.ID, event); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := raw.ClearSchedulerHistory(entry.ID); err != nil {
				t.Errorf("cleanup history: %v", err)
			}
		})
	}
	out, err := invokeAdministration(t, cronListCmd, cronList, nil, nil)
	if err != nil || strings.Index(out, "earlier-payload") < 0 || strings.Index(out, "earlier-payload") > strings.Index(out, "later-payload") || !strings.Contains(out, "N/A") || !strings.Contains(out, "Now") {
		t.Fatalf("cron ordering/timestamps: %s %v", out, err)
	}
	out, err = invokeAdministration(t, cronHistoryCmd, cronHistory, []string{entries[0].ID + "-missing", entries[0].ID}, map[string]string{"page": "1", "size": "1"})
	if err != nil || !strings.Contains(out, "No scheduler enqueue events") || !strings.Contains(out, entries[0].ID+"-task") {
		t.Fatalf("scheduler history: %s %v", out, err)
	}
	servers := []*base.ServerInfo{
		{Host: "z-" + f.queue, PID: 20, ServerID: "server-z", Concurrency: 3, Queues: map[string]int{f.queue: 1}, Status: "active", Started: time.Now().Add(-time.Hour)},
		{Host: "a-" + f.queue, PID: 30, ServerID: "server-a", Concurrency: 2, Queues: map[string]int{f.queue: 2}, Status: "stopped", Started: time.Now().Add(-time.Hour)},
	}
	for _, server := range servers {
		if err := raw.WriteServerState(server, nil, time.Minute); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := raw.ClearServerState(server.Host, server.PID, server.ServerID); err != nil {
				t.Errorf("cleanup server: %v", err)
			}
		})
	}
	out, err = invokeAdministration(t, serverListCmd, serverList, nil, nil)
	if err != nil || strings.Index(out, servers[1].Host) < 0 || strings.Index(out, servers[1].Host) > strings.Index(out, servers[0].Host) || !strings.Contains(out, "0/3") || !strings.Contains(out, "stopped") {
		t.Fatalf("server sorting/state: %s %v", out, err)
	}
}

func TestTaskCancellationSignalScopeRuntime(t *testing.T) {
	f := newAdministrationFixture(t)
	f.seed(t, f.queue, "pending", "", "untouched")
	sub := f.client.Subscribe(context.Background(), base.CancelChannel)
	t.Cleanup(func() { _ = sub.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	ids := []string{f.queue + "-cancel-a", f.queue + "-cancel-b"}
	out, err := invokeAdministration(t, taskCancelCmd, taskCancel, ids, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !strings.Contains(out, "Sent cancelation signal for task "+id) {
			t.Fatalf("signal attribution missing: %s", out)
		}
		for {
			message, err := sub.ReceiveMessage(ctx)
			if err != nil {
				t.Fatalf("missing cancel message for %s: %v", id, err)
			}
			if strings.HasPrefix(message.Payload, f.queue) {
				if message.Payload != id {
					t.Fatalf("signal payload=%q want=%q", message.Payload, id)
				}
				break
			}
		}
	}
	f.requireTask(t, f.queue, "untouched", "pending")
}
