package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// Runtime tests are opt-in and own only a uniquely named queue. They never flush
// a shared database. DeleteQueue(force) removes this test's fixture at cleanup.
func TestCLIEnqueueInspectAndPaginationRuntime(t *testing.T) {
	addr := os.Getenv("ASYNQ_CLI_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ASYNQ_CLI_TEST_REDIS_ADDR for an isolated runtime")
	}
	isolatedCLI(t)
	viper.Set("uri", addr)
	viper.Set("db", 12)
	viper.Set("cluster", false)
	viper.Set("tls", false)
	viper.Set("tls_server", "")
	viper.Set("password", "")
	viper.Set("username", "")
	probe := redis.NewClient(&redis.Options{Addr: addr, DB: 12})
	t.Cleanup(func() { _ = probe.Close() })
	if err := probe.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	queue := fmt.Sprintf("cli-contract-%d", time.Now().UnixNano())
	inspector := createInspector()
	t.Cleanup(func() { _ = inspector.Close() })
	t.Cleanup(func() {
		if err := inspector.DeleteQueue(queue, true); err != nil {
			t.Errorf("remove owned fixture %s: %v", queue, err)
		}
		// Queue deletion does not remove this scheduled task's unique lock.
		key := base.UniqueKey(queue, "scheduled-contract", []byte("scheduled-payload"))
		if err := probe.Del(context.Background(), key).Err(); err != nil {
			t.Errorf("remove owned unique lock: %v", err)
		}
	})
	for i := 1; i <= 3; i++ {
		t.Run(fmt.Sprintf("enqueue-%d", i), func(t *testing.T) {
			id := fmt.Sprintf("contract-id-%d", i)
			for flag, value := range map[string]string{"type_name": "contract-task", "payload": fmt.Sprintf("payload-%d", i), "queue": queue, "id": id, "retry": "3", "timeout": "2m", "retention": "1h"} {
				setCLIFlag(t, taskEnqueueCmd, flag, value)
			}
			var enqueueErr error
			output := captureCLI(t, func() { enqueueErr = taskEnqueue(taskEnqueueCmd, nil) })
			if enqueueErr != nil || !strings.Contains(output, "Enqueued task "+id+" to queue "+queue) {
				t.Fatalf("enqueue error=%v output=%s", enqueueErr, output)
			}
			info, err := inspector.GetTaskInfo(queue, id)
			if err != nil {
				t.Fatal(err)
			}
			if info.Type != "contract-task" || string(info.Payload) != fmt.Sprintf("payload-%d", i) || info.MaxRetry != 3 || info.Timeout != 2*time.Minute || info.Retention != time.Hour || info.State != asynq.TaskStatePending {
				t.Fatalf("enqueue flag contract: %+v", info)
			}
		})
	}
	// FIFO task pages are disjoint and expose the selected ID and payload.
	for page := 1; page <= 3; page++ {
		t.Run(fmt.Sprintf("page-%d", page), func(t *testing.T) {
			var listErr error
			output := captureCLI(t, func() { listErr = listPendingTasks(queue, page, 1) })
			if listErr != nil {
				t.Fatal(listErr)
			}
			for id := 1; id <= 3; id++ {
				want := fmt.Sprintf("contract-id-%d", id)
				if strings.Contains(output, want) != (page == id) {
					t.Fatalf("page %d ID %s output=%s", page, want, output)
				}
			}
		})
	}
	t.Run("scheduled options", func(t *testing.T) {
		at := time.Now().Add(2 * time.Hour).Truncate(time.Second)
		deadline := at.Add(time.Hour)
		for flag, value := range map[string]string{"type_name": "scheduled-contract", "payload": "scheduled-payload", "queue": queue, "id": "scheduled-contract-id", "deadline": deadline.Format(time.RFC3339), "process_at": at.Format(time.RFC3339), "unique": "10m"} {
			setCLIFlag(t, taskEnqueueCmd, flag, value)
		}
		if err := taskEnqueue(taskEnqueueCmd, nil); err != nil {
			t.Fatal(err)
		}
		info, err := inspector.GetTaskInfo(queue, "scheduled-contract-id")
		if err != nil {
			t.Fatal(err)
		}
		if info.State != asynq.TaskStateScheduled || !info.NextProcessAt.Equal(at) || !info.Deadline.Equal(deadline) {
			t.Fatalf("schedule flags lost: %+v", info)
		}
	})
	for _, state := range []string{"active", "pending", "scheduled", "retry", "archived", "completed"} {
		t.Run("list-"+state, func(t *testing.T) {
			for flag, value := range map[string]string{"queue": queue, "state": state, "page": "1", "size": "30"} {
				setCLIFlag(t, taskListCmd, flag, value)
			}
			var err error
			captureCLI(t, func() { err = taskList(taskListCmd, nil) })
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("task inspection", func(t *testing.T) {
		setCLIFlag(t, taskInspectCmd, "queue", queue)
		setCLIFlag(t, taskInspectCmd, "id", "contract-id-1")
		var err error
		out := captureCLI(t, func() { err = taskInspect(taskInspectCmd, nil) })
		if err != nil || !strings.Contains(out, "contract-id-1") || !strings.Contains(out, "contract-task") || !strings.Contains(out, "pending") {
			t.Fatalf("inspect error=%v output=%s", err, out)
		}
	})
	t.Run("queue pause and resume", func(t *testing.T) {
		if err := queuePause(queuePauseCmd, []string{queue}); err != nil {
			t.Fatal(err)
		}
		info, err := inspector.GetQueueInfo(queue)
		if err != nil || !info.Paused {
			t.Fatalf("pause: %+v %v", info, err)
		}
		if err := queueUnpause(queueUnpauseCmd, []string{queue}); err != nil {
			t.Fatal(err)
		}
		info, err = inspector.GetQueueInfo(queue)
		if err != nil || info.Paused {
			t.Fatalf("resume: %+v %v", info, err)
		}
	})
	t.Run("queue inspection", func(t *testing.T) {
		var err error
		out := captureCLI(t, func() { err = queueInspect(queueInspectCmd, []string{queue}) })
		if err != nil || !strings.Contains(out, queue) {
			t.Fatalf("queue inspect error=%v output=%s", err, out)
		}
	})
}
