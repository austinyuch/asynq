package cmd

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func taskFailureSnapshot(t *testing.T, f *administrationFixture) map[string]string {
	t.Helper()
	ctx := context.Background()
	keys := []string{base.AllQueues}
	for _, q := range []string{f.queue, f.neighbor} {
		owned, e := f.client.Keys(ctx, base.QueueKeyPrefix(q)+"*").Result()
		if e != nil {
			t.Fatal(e)
		}
		keys = append(keys, owned...)
	}
	result := map[string]string{}
	for _, key := range keys {
		v, e := f.client.Dump(ctx, key).Result()
		if e != nil {
			t.Fatal(e)
		}
		result[key] = v
	}
	return result
}
func taskFailureRequireUnchanged(t *testing.T, f *administrationFixture, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, taskFailureSnapshot(t, f)) {
		t.Fatal("failed/read command changed owned queue/healthy-neighbor bytes")
	}
	f.requireTask(t, f.neighbor, "healthy-neighbor", "pending")
}
func taskFailureFixture(t *testing.T) *administrationFixture {
	t.Helper()
	f := newAdministrationFixture(t)
	f.seed(t, f.neighbor, "pending", "", "healthy-neighbor")
	if e := f.client.SAdd(context.Background(), base.AllQueues, f.queue).Err(); e != nil {
		t.Fatal(e)
	}
	h.SeedGroup(t, f.client, nil, f.queue, "empty-owned-group")
	return f
}
func TestTaskFailureListEmptyAndAuthenticationProperty(t *testing.T) {
	f := taskFailureFixture(t)
	before := taskFailureSnapshot(t, f)
	states := []string{"active", "pending", "scheduled", "retry", "archived", "completed", "aggregating"}
	rng := rand.New(rand.NewSource(20261007))
	rng.Shuffle(len(states), func(i, j int) { states[i], states[j] = states[j], states[i] })
	for _, state := range states {
		flags := map[string]string{"queue": f.queue, "state": state, "page": "1", "size": "3", "group": "empty-owned-group"}
		out, e := invokeAdministration(t, taskListCmd, taskList, nil, flags)
		want := fmt.Sprintf("No %s tasks in %q queue\n", state, f.queue)
		if state == "aggregating" {
			want = "No aggregating tasks in group \"empty-owned-group\" \n"
		}
		if e != nil || out != want {
			t.Fatalf("empty %s lost precise message: out=%q err=%v want=%q", state, out, e, want)
		}
		viper.Set("username", "owned-nonexistent-user")
		viper.Set("password", "owned-invalid-password")
		out, e = invokeAdministration(t, taskListCmd, taskList, nil, flags)
		if e == nil || !strings.Contains(strings.ToLower(e.Error()), "wrongpass") || out != "" {
			t.Fatalf("authentication emitted false success %s: out=%q err=%v", state, out, e)
		}
		viper.Set("username", "")
		viper.Set("password", "")
		taskFailureRequireUnchanged(t, f, before)
	}
}
func TestTaskFailureMissingAndBulkValidationContracts(t *testing.T) {
	f := taskFailureFixture(t)
	before := taskFailureSnapshot(t, f)
	singles := []struct {
		name   string
		cmd    *cobra.Command
		run    func(*cobra.Command, []string) error
		prefix string
	}{{"inspect", taskInspectCmd, taskInspect, "could not get task info: "}, {"archive", taskArchiveCmd, taskArchive, "could not archive task: "}, {"delete", taskDeleteCmd, taskDelete, "could not delete task: "}, {"run", taskRunCmd, taskRun, "could not run task: "}}
	for _, tc := range singles {
		out, e := invokeAdministration(t, tc.cmd, tc.run, nil, map[string]string{"queue": f.queue, "id": "missing-owned-task"})
		if e == nil || e.Error() != tc.prefix+"asynq: "+asynq.ErrTaskNotFound.Error() || !errors.Is(e, asynq.ErrTaskNotFound) || out != "" {
			t.Fatalf("missing %s: out=%q err=%v", tc.name, out, e)
		}
		taskFailureRequireUnchanged(t, f, before)
	}
	bulk := []struct {
		cmd *cobra.Command
		run func(*cobra.Command, []string) error
	}{{taskArchiveAllCmd, taskArchiveAll}, {taskDeleteAllCmd, taskDeleteAll}, {taskRunAllCmd, taskRunAll}}
	for _, tc := range bulk {
		state := "pending"
		if tc.cmd == taskRunAllCmd {
			state = "scheduled"
		}
		missingQueue := f.queue + "-missing-owned"
		var oracle error
		switch tc.cmd {
		case taskArchiveAllCmd:
			_, oracle = f.inspector.ArchiveAllPendingTasks(missingQueue)
		case taskDeleteAllCmd:
			_, oracle = f.inspector.DeleteAllPendingTasks(missingQueue)
		case taskRunAllCmd:
			_, oracle = f.inspector.RunAllScheduledTasks(missingQueue)
		}
		out, e := invokeAdministration(t, tc.cmd, tc.run, nil, map[string]string{"queue": missingQueue, "state": state, "group": ""})
		if oracle == nil || e == nil || reflect.TypeOf(e) != reflect.TypeOf(oracle) || e.Error() != oracle.Error() || out != "" {
			t.Fatalf("missing queue error differs from independent Inspector oracle or published success: %q got=%v want=%v", out, e, oracle)
		}
		taskFailureRequireUnchanged(t, f, before)
		for _, state := range []string{"not-a-task-state", "aggregating"} {
			out, e := invokeAdministration(t, tc.cmd, tc.run, nil, map[string]string{"queue": f.queue, "state": state, "group": ""})
			want := fmt.Sprintf("unsupported state %q", state)
			if state == "aggregating" {
				want = "flag --group is required for aggregating tasks"
			}
			if e == nil || e.Error() != want || out != "" {
				t.Fatalf("bulk validation: out=%q err=%v want=%q", out, e, want)
			}
			taskFailureRequireUnchanged(t, f, before)
		}
	}
	out, e := invokeAdministration(t, taskListCmd, taskList, nil, map[string]string{"queue": f.queue, "state": "aggregating", "group": ""})
	if e == nil || e.Error() != "flag --group is required for listing aggregating tasks" || out != "" {
		t.Fatalf("list missing group: %q %v", out, e)
	}
}
func TestTaskFailureGroupEnqueueConflictOptionsContracts(t *testing.T) {
	f := taskFailureFixture(t)
	deadline := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	flags := map[string]string{"queue": f.queue, "type_name": "group-options-contract", "payload": "preserved-payload", "group": "owned-enqueue-group", "id": "fixed-owned-id", "timeout": "2s", "deadline": deadline.Format(time.RFC3339), "retry": "7", "retention": "1m"}
	out, e := invokeAdministration(t, taskEnqueueCmd, taskEnqueue, nil, flags)
	want := fmt.Sprintf("Enqueued task fixed-owned-id to queue %s\n", f.queue)
	if e != nil || out != want {
		t.Fatalf("enqueue output=%q err=%v", out, e)
	}
	info, e := f.inspector.GetTaskInfo(f.queue, "fixed-owned-id")
	if e != nil || info == nil || info.State != asynq.TaskStateAggregating || info.Group != "owned-enqueue-group" || info.Timeout != 2*time.Second || !info.Deadline.Equal(deadline) || info.MaxRetry != 7 || info.Retention != time.Minute || string(info.Payload) != "preserved-payload" {
		t.Fatalf("public enqueue options lost: %+v %v", info, e)
	}
	before := taskFailureSnapshot(t, f)
	out, e = invokeAdministration(t, taskEnqueueCmd, taskEnqueue, nil, flags)
	if e == nil || e.Error() != "could not enqueue task: "+asynq.ErrTaskIDConflict.Error() || !errors.Is(e, asynq.ErrTaskIDConflict) || out != "" {
		t.Fatalf("ID conflict emitted success: %q %v", out, e)
	}
	taskFailureRequireUnchanged(t, f, before)
}
func TestTaskFailurePrettyPrintLastErrorContracts(t *testing.T) {
	isolatedCLI(t)
	info := &asynq.TaskInfo{ID: "owned-pretty-id", Queue: "owned-pretty-queue", Type: "owned-type", State: asynq.TaskStateRetry, Retried: 2, MaxRetry: 7, LastErr: "owned-failure-text", LastFailedAt: time.Now().Add(-3 * time.Second)}
	out := captureCLI(t, func() {
		original := color.Output
		color.Output = os.Stdout
		defer func() { color.Output = original }()
		printTaskInfo(info)
	})
	for _, want := range []string{"Queue:   owned-pretty-queue\n", "ID:      owned-pretty-id\n", "State:   retry\n", "Retried: 2/7\n", "Next process time: n/a\n", "Last Failure\n", "Error message: owned-failure-text\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("prettyprint lost %q: %q", want, out)
		}
	}
	info.LastErr = ""
	out = captureCLI(t, func() {
		original := color.Output
		color.Output = os.Stdout
		defer func() { color.Output = original }()
		printTaskInfo(info)
	})
	if strings.Contains(out, "Last Failure") || strings.Contains(out, "Error message:") {
		t.Fatalf("empty last error fabricated failure: %q", out)
	}
}

func TestTaskFailureBulkAuthenticationProperty(t *testing.T) {
	f := taskFailureFixture(t)
	before := taskFailureSnapshot(t, f)
	ops := []struct {
		cmd    *cobra.Command
		run    func(*cobra.Command, []string) error
		states []string
	}{{taskArchiveAllCmd, taskArchiveAll, []string{"pending", "scheduled", "retry", "aggregating"}}, {taskDeleteAllCmd, taskDeleteAll, []string{"pending", "scheduled", "retry", "archived", "completed", "aggregating"}}, {taskRunAllCmd, taskRunAll, []string{"scheduled", "retry", "archived", "aggregating"}}}
	rng := rand.New(rand.NewSource(20261008))
	rng.Shuffle(len(ops), func(i, j int) { ops[i], ops[j] = ops[j], ops[i] })
	viper.Set("username", "owned-nonexistent-user")
	viper.Set("password", "owned-invalid-password")
	for _, op := range ops {
		for _, state := range op.states {
			out, e := invokeAdministration(t, op.cmd, op.run, nil, map[string]string{"queue": f.queue, "state": state, "group": "empty-owned-group"})
			if e == nil || !strings.Contains(strings.ToLower(e.Error()), "wrongpass") || out != "" {
				t.Fatalf("bulk auth false success %s: out=%q err=%v", state, out, e)
			}
			taskFailureRequireUnchanged(t, f, before)
		}
	}
	viper.Set("username", "")
	viper.Set("password", "")
}

func taskFailureDB12Connections(t *testing.T, f *administrationFixture) int {
	t.Helper()
	clients, e := f.client.ClientList(context.Background()).Result()
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, line := range strings.Split(clients, "\n") {
		for _, field := range strings.Fields(line) {
			if field == "db=12" {
				count++
				break
			}
		}
	}
	return count
}
func TestTaskFailureCommandClosesOwnedConnections(t *testing.T) {
	f := taskFailureFixture(t)
	f.requireTask(t, f.neighbor, "healthy-neighbor", "pending")
	baseline := taskFailureDB12Connections(t, f)
	operations := []func(){func() {
		out, e := invokeAdministration(t, taskListCmd, taskList, nil, map[string]string{"queue": f.queue, "state": "pending", "page": "1", "size": "3"})
		if e != nil || out != fmt.Sprintf("No pending tasks in %q queue\n", f.queue) {
			t.Fatalf("list before connection oracle: %q %v", out, e)
		}
	}, func() {
		out, e := invokeAdministration(t, taskEnqueueCmd, taskEnqueue, nil, map[string]string{"queue": f.queue, "type_name": "connection-contract", "payload": "preserved", "id": "owned-connection-task"})
		if e != nil || out != fmt.Sprintf("Enqueued task owned-connection-task to queue %s\n", f.queue) {
			t.Fatalf("enqueue before connection oracle: %q %v", out, e)
		}
	}}
	for _, op := range operations {
		op()
		deadline := time.Now().Add(time.Second)
		last := 0
		for {
			last = taskFailureDB12Connections(t, f)
			if last == baseline {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("command retained owned DB12 connection: baseline=%d after=%d", baseline, last)
			}
			timer := time.NewTimer(5 * time.Millisecond)
			<-timer.C
		}
	}
}
