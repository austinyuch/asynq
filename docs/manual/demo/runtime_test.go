package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/redis/go-redis/v9"
)

const demoRuntimeChildEnv = "ASYNQ_DEMO_RUNTIME_CHILD"

// Normal test runs never seed the demo DB. main runs only in this explicitly
// selected child, with a parent deadline and a dedicated opted-in endpoint.
func TestDemoRuntimeChild(t *testing.T) {
	if os.Getenv(demoRuntimeChildEnv) == "" {
		t.Skip("parent-only subprocess helper")
	}
	if os.Getenv("ASYNQ_DEMO_TEST_REDIS_ADDR") == "" {
		t.Fatal("child requires explicitly opted-in Redis endpoint")
	}
	main()
}
func demoRuntimeClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("ASYNQ_DEMO_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ASYNQ_DEMO_TEST_REDIS_ADDR for exclusively owned DB13 runtime")
	}
	c := redis.NewClient(&redis.Options{Addr: addr, DB: demoDB})
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	n, err := c.DBSize(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("DB13 contains %d existing keys; no runtime writes or cleanup attempted", n)
	}
	return c
}
func demoRuntimeHash(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func demoRuntimeEvidenceDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("ASYNQ_DEMO_COVERAGE_DIR")
	if dir == "" {
		return ""
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
func demoRuntimeRun(t *testing.T, mode string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	args := []string{"-test.run=^TestDemoRuntimeChild$", "-test.v", "-redis_addr=" + os.Getenv("ASYNQ_DEMO_TEST_REDIS_ADDR")}
	dir := demoRuntimeEvidenceDir(t)
	profile := ""
	// Only a coverage-instrumented test executable accepts coverage output flags.
	if dir != "" && testing.CoverMode() != "" {
		childDir := filepath.Join(dir, mode+"-covdata")
		if err := os.MkdirAll(childDir, 0700); err != nil {
			t.Fatal(err)
		}
		profile = filepath.Join(dir, mode+".cover")
		args = append(args, "-test.coverprofile="+profile, "-test.gocoverdir="+childDir)
	}
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = append(os.Environ(), demoRuntimeChildEnv+"="+mode)
	output, err := cmd.CombinedOutput()
	if dir != "" {
		evidence := map[string]interface{}{"mode": mode, "args": args, "deadline_seconds": 25, "timeout": ctx.Err() != nil, "instrumented": testing.CoverMode() != "", "executable_sha256": demoRuntimeHash(t, os.Args[0]), "sources_sha256": map[string]string{"main.go": demoRuntimeHash(t, "main.go"), "runtime_test.go": demoRuntimeHash(t, "runtime_test.go")}, "profile_requested": profile, "exit_code": 0}
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				evidence["exit_code"] = e.ExitCode()
			} else {
				evidence["error"] = err.Error()
			}
		}
		if profile != "" {
			if b, e := os.ReadFile(profile); e == nil {
				sum := sha256.Sum256(b)
				evidence["profile_sha256"] = hex.EncodeToString(sum[:])
			} else {
				evidence["profile_absent"] = true
			}
		}
		b, e := json.MarshalIndent(evidence, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, mode+"-receipt.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, mode+".log"), output, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("demo child exceeded deadline: %v %s", ctx.Err(), output)
	}
	return string(output), err
}

type demoRuntimeKeyBackup struct {
	Dump       []byte `json:"dump_base64"`
	PTTLMillis int64  `json:"pttl_ms"`
}

var demoRuntimeDeleteIfOwned = redis.NewScript(`
local current = redis.call("DUMP", KEYS[1])
if not current then return 0 end
if current ~= ARGV[1] then return -1 end
return redis.call("DEL", KEYS[1])`)

func demoRuntimeCapture(t *testing.T, c *redis.Client) map[string]demoRuntimeKeyBackup {
	t.Helper()
	ctx := context.Background()
	backups := make(map[string]demoRuntimeKeyBackup)
	var cursor uint64
	for {
		keys, next, err := c.Scan(ctx, cursor, "", 100).Result()
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			dump, err := c.Dump(ctx, key).Bytes()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			ttl, err := c.PTTL(ctx, key).Result()
			if err != nil {
				t.Fatal(err)
			}
			backups[key] = demoRuntimeKeyBackup{Dump: dump, PTTLMillis: ttl.Milliseconds()}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return backups
}
func demoRuntimeNamespaceOwned(key string) bool {
	if key == base.AllQueues || key == base.AllServers || key == base.AllWorkers {
		return true
	}
	for _, q := range []string{"critical", "default", "low"} {
		if strings.HasPrefix(key, base.QueueKeyPrefix(q)) {
			return true
		}
	}
	return false
}
func demoRuntimeCleanup(t *testing.T, c *redis.Client) {
	t.Helper()
	backup := demoRuntimeCapture(t, c)
	if dir := demoRuntimeEvidenceDir(t); dir != "" {
		b, err := json.MarshalIndent(backup, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		if err = os.WriteFile(filepath.Join(dir, "positive-key-backup.json"), b, 0600); err != nil {
			t.Error(err)
			return
		}
	}
	// An unknown namespace indicates another operator wrote to the exclusive DB.
	// Preserve everything, including our fixture, rather than claim those keys.
	for key := range backup {
		if !demoRuntimeNamespaceOwned(key) {
			t.Errorf("foreign key %q detected; backup retained and cleanup refused", key)
			return
		}
	}
	for key, b := range backup {
		n, err := demoRuntimeDeleteIfOwned.Run(context.Background(), c, []string{key}, b.Dump).Int()
		if err != nil || n < 0 {
			t.Errorf("changed key %q retained: deleted=%d err=%v", key, n, err)
		}
	}
	if n, err := c.DBSize(context.Background()).Result(); err != nil || n != 0 {
		t.Errorf("DB13 cleanup left data: size=%d err=%v", n, err)
	}
}

func TestDemoRuntimePositive(t *testing.T) {
	c := demoRuntimeClient(t)
	t.Cleanup(func() { demoRuntimeCleanup(t, c) })
	output, err := demoRuntimeRun(t, "positive")
	if err != nil {
		t.Fatalf("demo child: %v %s", err, output)
	}
	if strings.Count(output, "  enqueued id=") != 8 || strings.Count(output, "processed email:welcome") != 3 || strings.Count(output, "processed image:resize") != 2 || strings.Count(output, "processed cleanup:tmp") != 1 {
		t.Fatalf("real workloads incomplete: %s", output)
	}
	for _, want := range []string{"duplicate rejected: cleanup:tmp", "card declined", "billing:charge", "report:generate", "next_process_at=", "user=1001", "user=1002", "user=1003", "width=640", "width=1280", "demo seed complete: data remains in DB 13"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing runtime output %q: %s", want, output)
		}
	}
	inspector := asynq.NewInspectorFromRedisClient(c)
	for _, tc := range []struct {
		q                                            string
		size, scheduled, archived, processed, failed int
	}{{"critical", 1, 0, 1, 4, 1}, {"default", 0, 0, 0, 2, 0}, {"low", 1, 1, 0, 1, 0}} {
		info, err := inspector.GetQueueInfo(tc.q)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size != tc.size || info.Pending != 0 || info.Active != 0 || info.Scheduled != tc.scheduled || info.Archived != tc.archived || info.Processed != tc.processed || info.Failed != tc.failed {
			t.Fatalf("independent stats %s=%+v", tc.q, info)
		}
		if !strings.Contains(output, fmt.Sprintf("%-10s", tc.q)) {
			t.Errorf("stats output omitted queue %s", tc.q)
		}
	}
	archived, err := inspector.ListArchivedTasks("critical")
	if err != nil || len(archived) != 1 {
		t.Fatalf("archived billing=%v %v", archived, err)
	}
	if archived[0].Type != "billing:charge" || archived[0].State != asynq.TaskStateArchived || archived[0].LastErr != "card declined" || archived[0].MaxRetry != 0 || !strings.Contains(string(archived[0].Payload), "INV-042") {
		t.Fatalf("billing contract=%+v", archived[0])
	}
	scheduled, err := inspector.ListScheduledTasks("low")
	if err != nil || len(scheduled) != 1 {
		t.Fatalf("scheduled report=%v %v", scheduled, err)
	}
	if scheduled[0].Type != "report:generate" || scheduled[0].State != asynq.TaskStateScheduled || !scheduled[0].NextProcessAt.After(time.Now().Add(time.Hour)) || !strings.Contains(string(scheduled[0].Payload), "2026-06") {
		t.Fatalf("report contract=%+v", scheduled[0])
	}
}

func TestDemoRuntimeNonemptyAdmission(t *testing.T) {
	c := demoRuntimeClient(t)
	ctx := context.Background()
	key := fmt.Sprintf("demo-runtime-sentinel-%d", time.Now().UnixNano())
	value := "must-survive-demo-admission"
	if err := c.Set(ctx, key, value, 0).Err(); err != nil {
		t.Fatal(err)
	}
	dump, err := c.Dump(ctx, key).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		n, err := demoRuntimeDeleteIfOwned.Run(ctx, c, []string{key}, dump).Int()
		if err != nil || n != 1 {
			t.Errorf("sentinel changed, preserved: %d %v", n, err)
		}
	})
	output, err := demoRuntimeRun(t, "nonempty")
	e, ok := err.(*exec.ExitError)
	if !ok || e.ExitCode() != 1 || !strings.Contains(output, errDemoDBNotEmpty.Error()) || strings.Contains(output, "Enqueue real workloads") || strings.Contains(output, "enqueued id=") {
		t.Fatalf("nonempty demo admission=%v %s", err, output)
	}
	now, err := c.Dump(ctx, key).Bytes()
	if err != nil || string(now) != string(dump) {
		t.Fatalf("sentinel mutated: %v", err)
	}
	if size, err := c.DBSize(ctx).Result(); err != nil || size != 1 {
		t.Fatalf("rejected demo wrote data: %d %v", size, err)
	}
}
