package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
)

// These contracts use real registered DB12 fixtures and command-owned clients.
// Table expectations come from named fixture data and independent Inspector
// reads; no production formatter or pagination helper supplies the oracle.
func TestTaskVisibilityNonemptyPages(t *testing.T) {
	for _, state := range []string{"active", "retry", "archived", "completed"} {
		t.Run(state, func(t *testing.T) {
			f := newAdministrationFixture(t)
			f.seed(t, f.neighbor, "pending", "", "healthy-neighbor")
			ctx := context.Background()
			failed := time.Unix(1700000000, 0)
			completed := time.Unix(1700001000, 0)
			var msgs []*base.TaskMessage
			var zs []base.Z
			for n := 0; n < 3; n++ {
				msg := h.NewTaskMessageWithQueue("visibility-type", []byte(fmt.Sprintf("payload-%d", n)), f.queue)
				msg.ID = fmt.Sprintf("visibility-%s-%d", state, n)
				msg.Retry = 9
				switch state {
				case "retry":
					msg = h.TaskMessageAfterRetry(*msg, fmt.Sprintf("failure-%d", n), failed)
				case "archived":
					msg = h.TaskMessageWithError(*msg, fmt.Sprintf("failure-%d", n), failed)
				case "completed":
					msg = h.TaskMessageWithCompletedAt(*msg, completed)
				}
				msgs = append(msgs, msg)
				zs = append(zs, base.Z{Message: msg, Score: 1700002000 + int64(n)})
			}
			switch state {
			case "active":
				h.SeedActiveQueue(t, f.client, msgs, f.queue)
				// DeleteQueue(force) rejects active tasks. Remove only this fixture's
				// canonical active pointers before the existing owned-queue cleanup.
				t.Cleanup(func() {
					if err := f.client.Del(ctx, base.ActiveKey(f.queue)).Err(); err != nil {
						t.Error(err)
					}
				})
			case "retry":
				h.SeedRetryQueue(t, f.client, zs, f.queue)
			case "archived":
				h.SeedArchivedQueue(t, f.client, zs, f.queue)
			case "completed":
				h.SeedCompletedQueue(t, f.client, zs, f.queue)
				for n, msg := range msgs {
					if err := f.client.HSet(ctx, base.TaskKey(f.queue, msg.ID), "result", fmt.Sprintf("result-%d", n)).Err(); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Validate the real fixture independently before using API page order.
			infos := map[string]*asynq.TaskInfo{}
			for n, msg := range msgs {
				info, e := f.inspector.GetTaskInfo(f.queue, msg.ID)
				if e != nil {
					t.Fatal(e)
				}
				if info.State.String() != state || info.Type != "visibility-type" || string(info.Payload) != fmt.Sprintf("payload-%d", n) || info.MaxRetry != 9 {
					t.Fatalf("fixture/API mismatch: %+v", info)
				}
				if state == "retry" && (info.LastErr != fmt.Sprintf("failure-%d", n) || info.Retried != 1 || !info.LastFailedAt.Equal(failed)) {
					t.Fatalf("retry fixture mismatch: %+v", info)
				}
				if state == "archived" && (info.LastErr != fmt.Sprintf("failure-%d", n) || !info.LastFailedAt.Equal(failed)) {
					t.Fatalf("archive fixture mismatch: %+v", info)
				}
				if state == "completed" && (string(info.Result) != fmt.Sprintf("result-%d", n) || !info.CompletedAt.Equal(completed)) {
					t.Fatalf("completed fixture mismatch: %+v", info)
				}
				infos[msg.ID] = info
			}
			before := taskFailureSnapshot(t, f)
			t.Cleanup(func() { taskFailureRequireUnchanged(t, f, before) })
			sizes := rand.New(rand.NewSource(20261019)).Perm(3)
			for _, zeroSize := range sizes {
				size := zeroSize + 1
				seen := map[string]bool{}
				for page := 1; page <= 4; page++ {
					opts := []asynq.ListOption{asynq.PageSize(size), asynq.Page(page)}
					var expected []*asynq.TaskInfo
					var e error
					switch state {
					case "active":
						expected, e = f.inspector.ListActiveTasks(f.queue, opts...)
					case "retry":
						expected, e = f.inspector.ListRetryTasks(f.queue, opts...)
					case "archived":
						expected, e = f.inspector.ListArchivedTasks(f.queue, opts...)
					case "completed":
						expected, e = f.inspector.ListCompletedTasks(f.queue, opts...)
					}
					if e != nil {
						t.Fatal(e)
					}
					out, e := invokeAdministration(t, taskListCmd, taskList, nil, map[string]string{"queue": f.queue, "state": state, "page": strconv.Itoa(page), "size": strconv.Itoa(size)})
					if e != nil {
						t.Fatal(e)
					}
					if len(expected) == 0 {
						if strings.TrimSpace(out) != fmt.Sprintf("No %s tasks in %q queue", state, f.queue) {
							t.Fatalf("empty page contract: %q", out)
						}
						continue
					}
					headers := map[string]string{"active": "ID Type Payload", "retry": "ID Type Payload Next Retry Last Error Last Failed Retried Max Retry", "archived": "ID Type Payload Last Failed Last Error", "completed": "ID Type Payload CompletedAt Result"}
					lines := strings.Split(strings.TrimSpace(out), "\n")
					if len(lines) != len(expected)+2 || strings.Join(strings.Fields(lines[0]), " ") != headers[state] {
						t.Fatalf("table/header contract: %q", out)
					}
					for n, info := range expected {
						if infos[info.ID] == nil || seen[info.ID] {
							t.Fatalf("unknown/repeated page ID %s size%d page%d", info.ID, size, page)
						}
						seen[info.ID] = true
						want := []string{info.ID, "visibility-type", string(infos[info.ID].Payload)}
						switch state {
						case "retry":
							want = append(want, "right", "now", infos[info.ID].LastErr)
							want = append(want, strings.Fields(failed.Format(time.UnixDate))...)
							want = append(want, "1", "9")
						case "archived":
							want = append(want, strings.Fields(failed.Format(time.UnixDate))...)
							want = append(want, infos[info.ID].LastErr)
						case "completed":
							want = append(want, strings.Fields(completed.Format(time.UnixDate))...)
							want = append(want, string(infos[info.ID].Result))
						}
						if !reflect.DeepEqual(strings.Fields(lines[n+2]), want) {
							t.Fatalf("state%s size%d page%d row%d got%v want%v", state, size, page, n, strings.Fields(lines[n+2]), want)
						}
					}
				}
				if len(seen) != len(msgs) {
					t.Fatalf("pagination union size%d got%v want3 fixture IDs", size, seen)
				}
			}
		})
	}
}

func TestTaskVisibilityRelativeSchedule(t *testing.T) {
	f := newAdministrationFixture(t)
	f.seed(t, f.neighbor, "pending", "", "healthy-neighbor")
	neighborBefore := taskFailureSnapshot(t, f)
	started := time.Now()
	out, e := invokeAdministration(t, taskEnqueueCmd, taskEnqueue, nil, map[string]string{"queue": f.queue, "id": "relative-visibility", "type_name": "relative-type", "payload": "relative-payload", "process_in": "2h"})
	after := time.Now()
	if e != nil || strings.TrimSpace(out) != fmt.Sprintf("Enqueued task relative-visibility to queue %s", f.queue) {
		t.Fatalf("enqueue e%v out%q", e, out)
	}
	info, e := f.inspector.GetTaskInfo(f.queue, "relative-visibility")
	if e != nil {
		t.Fatal(e)
	}
	// Redis stores seconds, so independently bound the enqueue interval at
	// second precision instead of reusing a production duration helper.
	for key, value := range neighborBefore {
		if strings.HasPrefix(key, base.QueueKeyPrefix(f.neighbor)) {
			got, err := f.client.Dump(context.Background(), key).Result()
			if err != nil || got != value {
				t.Fatalf("enqueue changed neighbor %s: %v", key, err)
			}
		}
	}
	if info.State != asynq.TaskStateScheduled || info.NextProcessAt.Before(started.Add(2*time.Hour).Truncate(time.Second)) || info.NextProcessAt.After(after.Add(2*time.Hour)) || info.Type != "relative-type" || string(info.Payload) != "relative-payload" {
		t.Fatalf("relative scheduling contract: %+v", info)
	}
	snapshot := taskFailureSnapshot(t, f)
	t.Cleanup(func() { taskFailureRequireUnchanged(t, f, snapshot) })
	out, e = invokeAdministration(t, taskInspectCmd, taskInspect, nil, map[string]string{"queue": f.queue, "id": info.ID})
	if e != nil {
		t.Fatal(e)
	}
	var next string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Next process time: ") {
			next = strings.TrimPrefix(line, "Next process time: ")
		}
	}
	prefix := info.NextProcessAt.Format(time.UnixDate) + " (in "
	if !strings.HasPrefix(next, prefix) || !strings.HasSuffix(next, ")") {
		t.Fatalf("future timestamp output: %q", next)
	}
	duration, e := time.ParseDuration(strings.TrimSuffix(strings.TrimPrefix(next, prefix), ")"))
	if e != nil || duration < 119*time.Minute || duration > 2*time.Hour {
		t.Fatalf("future relative duration %q e%v", next, e)
	}
}
