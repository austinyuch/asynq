package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestQueueStatsFailureMixedResults(t *testing.T) {
	for _, kind := range []string{"inspect", "history"} {
		t.Run(kind, func(t *testing.T) {
			f := taskFailureFixture(t)
			started := time.Now().UTC()
			var healthyHistory []*asynq.DailyStats
			if kind == "history" {
				if e := f.client.Set(context.Background(), base.ProcessedKey(f.neighbor, started), 12, 0).Err(); e != nil {
					t.Fatal(e)
				}
				if e := f.client.Set(context.Background(), base.FailedKey(f.neighbor, started), 3, 0).Err(); e != nil {
					t.Fatal(e)
				}
				var e error
				healthyHistory, e = f.inspector.History(f.neighbor, 2)
				if e != nil {
					t.Fatal(e)
				}
			}
			before := taskFailureSnapshot(t, f)
			names := []string{f.queue + "-missing-first", f.neighbor, f.queue + "-missing-last"}
			rng := rand.New(rand.NewSource(20261009))
			rng.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
			var oracle error
			for _, name := range names {
				if kind == "inspect" {
					_, oracle = f.inspector.GetQueueInfo(name)
				} else {
					_, oracle = f.inspector.History(name, 2)
				}
				if oracle != nil {
					break
				}
			}
			cmd, run := queueInspectCmd, queueInspect
			if kind == "history" {
				cmd, run = queueHistoryCmd, queueHistory
			}
			flags := map[string]string{}
			if kind == "history" {
				flags["days"] = "2"
			}
			out, err := queueStatsInvoke(t, cmd, run, names, flags)
			if kind == "history" {
				queueStatsRequireSameUTCDate(t, started)
				queueStatsRequireHistoryRows(t, out, healthyHistory)
			} else if !strings.Contains(out, "Name:   "+f.neighbor+"\n") || !strings.Contains(out, "Task Count by State") {
				t.Fatalf("mixed inspect omitted healthy queue content: %q", out)
			}
			if !strings.Contains(out, "error: ") {
				t.Fatalf("mixed batch suppressed independent healthy/error output: %q", out)
			}
			if oracle == nil || err == nil || reflect.TypeOf(err) != reflect.TypeOf(oracle) || err.Error() != oracle.Error() {
				t.Fatalf("mixed batch must return first API error after healthy output: got=%v want=%v output=%q", err, oracle, out)
			}
			taskFailureRequireUnchanged(t, f, before)
		})
	}
}

func TestQueueStatsFailureCommandOwnership(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		run  func(*cobra.Command, []string) error
	}{
		{"list", queueListCmd, queueList}, {"inspect", queueInspectCmd, queueInspect}, {"history", queueHistoryCmd, queueHistory}, {"pause", queuePauseCmd, queuePause}, {"resume", queueUnpauseCmd, queueUnpause}, {"remove-guard", queueRemoveCmd, queueRemove}, {"stats", statsCmd, stats},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := taskFailureFixture(t)
			if _, err := f.inspector.GetQueueInfo(f.neighbor); err != nil {
				t.Fatal(err)
			}
			if tc.name == "resume" {
				if err := f.inspector.PauseQueue(f.neighbor); err != nil {
					t.Fatal(err)
				}
			}
			before := queueStatsConnections(t, f)
			args := []string{f.neighbor}
			flags := map[string]string{}
			switch tc.name {
			case "list", "stats":
				args = nil
			case "history":
				flags["days"] = "1"
			case "remove-guard":
				flags["force"] = "false"
			}
			_, err := invokeAdministration(t, tc.cmd, tc.run, args, flags)
			if tc.name == "remove-guard" {
				if err == nil {
					t.Fatal("nonempty queue removal unexpectedly succeeded")
				}
				f.requireTask(t, f.neighbor, "healthy-neighbor", "pending")
			} else if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for {
				n := queueStatsConnections(t, f)
				newIDs := []string{}
				for id := range n {
					if !before[id] {
						newIDs = append(newIDs, id)
					}
				}
				if len(newIDs) == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("command retained newly owned DB12 connection IDs: before=%v after=%v new=%v", before, n, newIDs)
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}

func TestQueueStatsFailureAuthenticationNoWrite(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		run  func(*cobra.Command, []string) error
	}{
		{"list", queueListCmd, queueList}, {"inspect", queueInspectCmd, queueInspect}, {"history", queueHistoryCmd, queueHistory}, {"pause", queuePauseCmd, queuePause}, {"resume", queueUnpauseCmd, queueUnpause}, {"remove", queueRemoveCmd, queueRemove}, {"stats", statsCmd, stats},
	}
	rng := rand.New(rand.NewSource(20261010))
	rng.Shuffle(len(cases), func(i, j int) { cases[i], cases[j] = cases[j], cases[i] })
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := taskFailureFixture(t)
			before := taskFailureSnapshot(t, f)
			viper.Set("username", "owned-nonexistent-user")
			viper.Set("password", "owned-invalid-password")
			args := []string{f.neighbor}
			flags := map[string]string{}
			if tc.name == "list" || tc.name == "stats" {
				args = nil
			}
			if tc.name == "history" {
				flags["days"] = "2"
			}
			if tc.name == "remove" {
				flags["force"] = "false"
			}
			out, err := invokeAdministration(t, tc.cmd, tc.run, args, flags)
			viper.Set("username", "")
			viper.Set("password", "")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "wrongpass") || strings.Contains(out, "Successfully") {
				t.Fatalf("authentication false success: out=%q err=%v", out, err)
			}
			if tc.name == "stats" || tc.name == "list" {
				if out != "" {
					t.Fatalf("failed read published success output: %q", out)
				}
			}
			taskFailureRequireUnchanged(t, f, before)
		})
	}
}

func TestQueueStatsFailureJSONPublicOracle(t *testing.T) {
	f := taskFailureFixture(t)
	states := []string{"pending", "scheduled", "retry", "archived", "completed", "aggregating"}
	for i, state := range states {
		f.seed(t, f.queue, state, "owned-group", fmt.Sprintf("state-%d", i))
	}
	before := taskFailureSnapshot(t, f)
	saved := jsonFlag
	jsonFlag = true
	t.Cleanup(func() { jsonFlag = saved })
	out, err := invokeAdministration(t, statsCmd, stats, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got FullStats
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	queues, err := f.inspector.Queues()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.QueueStats) != len(queues) {
		t.Fatalf("JSON queue membership got=%d want=%d", len(got.QueueStats), len(queues))
	}
	var want AggregateStats
	for _, q := range queues {
		info, e := f.inspector.GetQueueInfo(q)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, s := range got.QueueStats {
			if s.Queue == q {
				found = true
				if s.Size != info.Size || s.Active != info.Active || s.Pending != info.Pending || s.Scheduled != info.Scheduled || s.Retry != info.Retry || s.Archived != info.Archived || s.Completed != info.Completed || s.Aggregating != info.Aggregating || s.Paused != info.Paused {
					t.Fatalf("JSON per-queue disagrees with public Inspector: %+v vs %+v", s, info)
				}
			}
		}
		if !found {
			t.Fatalf("JSON omitted public queue %q", q)
		}
		want.Active += info.Active
		want.Pending += info.Pending
		want.Scheduled += info.Scheduled
		want.Retry += info.Retry
		want.Archived += info.Archived
		want.Completed += info.Completed
		want.Aggregating += info.Aggregating
		want.Processed += info.Processed
		want.Failed += info.Failed
	}
	want.Timestamp = got.Aggregate.Timestamp
	if !reflect.DeepEqual(want, got.Aggregate) {
		t.Fatalf("JSON aggregate got=%+v want=%+v", got.Aggregate, want)
	}
	rawInfo, infoErr := f.client.Info(context.Background(), "server").Result()
	if infoErr != nil {
		t.Fatal(infoErr)
	}
	if got.RedisInfo["redis_version"] == "" || !strings.Contains(rawInfo, "redis_version:"+got.RedisInfo["redis_version"]) {
		t.Fatalf("JSON Redis version differs from independent connection INFO: %q", got.RedisInfo["redis_version"])
	}
	taskFailureRequireUnchanged(t, f, before)
}

// Compare newly created IDs rather than total counts: prior unclosed commands
// may be finalized by the runtime while this command runs.
func queueStatsConnections(t *testing.T, f *administrationFixture) map[string]bool {
	t.Helper()
	raw, err := f.client.ClientList(context.Background()).Result()
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		var id string
		owned := false
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "id=") {
				id = strings.TrimPrefix(field, "id=")
			}
			if field == "db=12" {
				owned = true
			}
		}
		if owned {
			result[id] = true
		}
	}
	return result
}

func TestQueueStatsFailureHistoryPublicOracle(t *testing.T) {
	f := taskFailureFixture(t)
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		day := now.Add(-time.Duration(i) * 24 * time.Hour)
		if err := f.client.Set(context.Background(), base.ProcessedKey(f.queue, day), 12+i, 0).Err(); err != nil {
			t.Fatal(err)
		}
		if err := f.client.Set(context.Background(), base.FailedKey(f.queue, day), 3+i, 0).Err(); err != nil {
			t.Fatal(err)
		}
	}
	oracle, err := f.inspector.History(f.queue, 3)
	if err != nil {
		t.Fatal(err)
	}
	before := taskFailureSnapshot(t, f)
	out, err := queueStatsInvoke(t, queueHistoryCmd, queueHistory, []string{f.queue}, map[string]string{"days": "3"})
	queueStatsRequireSameUTCDate(t, now)
	if err != nil {
		t.Fatal(err)
	}
	queueStatsRequireHistoryRows(t, out, oracle)
	taskFailureRequireUnchanged(t, f, before)
}

func queueStatsInvoke(t *testing.T, cmd *cobra.Command, run func(*cobra.Command, []string) error, args []string, flags map[string]string) (string, error) {
	t.Helper()
	return invokeAdministration(t, cmd, func(c *cobra.Command, a []string) error {
		saved := color.Output
		color.Output = os.Stdout
		defer func() { color.Output = saved }()
		return run(c, a)
	}, args, flags)
}
func queueStatsRequireSameUTCDate(t *testing.T, started time.Time) {
	t.Helper()
	if started.UTC().Format("2006-01-02") != time.Now().UTC().Format("2006-01-02") {
		t.Skip("UTC date rolled over during real History observation; historical row claim not measured in this execution")
	}
}
func queueStatsRequireHistoryRows(t *testing.T, out string, oracle []*asynq.DailyStats) {
	t.Helper()
	normalized := strings.Join(strings.Fields(out), " ")
	if !strings.Contains(normalized, "date (UTC) processed failed error rate") {
		t.Fatalf("healthy history table header suppressed: %q", out)
	}
	for _, day := range oracle {
		rate := "N/A"
		if day.Processed != 0 {
			rate = fmt.Sprintf("%.2f%%", float64(day.Failed)/float64(day.Processed)*100)
		}
		row := fmt.Sprintf("%s %d %d %s", day.Date.UTC().Format("2006-01-02"), day.Processed, day.Failed, rate)
		if !strings.Contains(normalized, row) {
			t.Fatalf("history omitted independent public API row %q: %q", row, out)
		}
	}
}
