package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	pb "github.com/austinyuch/asynq/internal/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

func TestTestutilComparisonContracts(t *testing.T) {
	rng := rand.New(rand.NewSource(6181))
	for size := 0; size <= 32; size++ {
		ids := make([]string, size)
		for i := range ids {
			ids[i] = fmt.Sprintf("id-%02d", i)
		}
		shuffled := append([]string(nil), ids...)
		rng.Shuffle(size, func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		workers, otherWorkers := []*base.WorkerInfo{}, []*base.WorkerInfo{}
		servers, otherServers := []*base.ServerInfo{}, []*base.ServerInfo{}
		events, otherEvents := []*base.SchedulerEnqueueEvent{}, []*base.SchedulerEnqueueEvent{}
		numeric, otherNumeric := []redis.Z{}, []redis.Z{}
		text, otherText := []redis.Z{}, []redis.Z{}
		for i, id := range ids {
			workers = append(workers, &base.WorkerInfo{ID: id, Queue: "healthy"})
			servers = append(servers, &base.ServerInfo{Host: fmt.Sprintf("host-%d", i%3), PID: i + 1, ServerID: id})
			events = append(events, &base.SchedulerEnqueueEvent{TaskID: id, EnqueuedAt: time.Unix(int64(i)-16, 123)})
			numeric = append(numeric, redis.Z{Member: i, Score: float64(i) - 16.5})
			// String member order deliberately differs from score order.
			text = append(text, redis.Z{Member: id, Score: float64(size - i)})
		}
		for _, id := range shuffled {
			var i int
			fmt.Sscanf(id, "id-%02d", &i)
			otherWorkers = append(otherWorkers, workers[i])
			otherServers = append(otherServers, servers[i])
			otherEvents = append(otherEvents, events[i])
			otherNumeric = append(otherNumeric, numeric[i])
			otherText = append(otherText, text[i])
		}
		for _, tc := range []struct {
			name   string
			a, b   interface{}
			option cmp.Option
		}{
			{"worker", workers, otherWorkers, SortWorkerInfoOpt}, {"server", servers, otherServers, SortServerInfoOpt}, {"event", events, otherEvents, SortSchedulerEnqueueEventOpt}, {"numeric-z", numeric, otherNumeric, SortRedisZSetEntryOpt}, {"string-z", text, otherText, SortRedisZSetEntryOpt},
		} {
			beforeA := fmt.Sprintf("%#v", tc.a)
			beforeB := fmt.Sprintf("%#v", tc.b)
			// Value serialization also checks pointed-to records rather than addresses.
			jsonA, _ := json.Marshal(tc.a)
			jsonB, _ := json.Marshal(tc.b)
			if !cmp.Equal(tc.a, tc.b, tc.option) {
				t.Fatalf("%s permutation size %d unequal", tc.name, size)
			}
			afterA, _ := json.Marshal(tc.a)
			afterB, _ := json.Marshal(tc.b)
			if beforeA != fmt.Sprintf("%#v", tc.a) || beforeB != fmt.Sprintf("%#v", tc.b) || !bytes.Equal(jsonA, afterA) || !bytes.Equal(jsonB, afterB) {
				t.Fatalf("%s comparison changed caller-owned input", tc.name)
			}
		}
		if size > 0 {
			changed := append([]*base.WorkerInfo(nil), otherWorkers...)
			copyWorker := *changed[0]
			copyWorker.Queue = "different"
			changed[0] = &copyWorker
			if cmp.Equal(workers, changed, SortWorkerInfoOpt) {
				t.Fatal("sort suppressed non-sort-field difference")
			}
			changedEvent := append([]*base.SchedulerEnqueueEvent(nil), otherEvents...)
			copyEvent := *changedEvent[0]
			copyEvent.TaskID = "different"
			changedEvent[0] = &copyEvent
			if cmp.Equal(events, changedEvent, SortSchedulerEnqueueEventOpt) {
				t.Fatal("event sort suppressed task identity difference")
			}
			changedZ := append([]redis.Z(nil), otherNumeric...)
			changedZ[0].Member = -1000
			if cmp.Equal(numeric, changedZ, SortRedisZSetEntryOpt) {
				t.Fatal("numeric sort suppressed member difference")
			}
		}
	}
}

func TestTestutilJSONContracts(t *testing.T) {
	data := JSON(map[string]interface{}{"text": "括號)\\\"", "nested": map[string]interface{}{"enabled": true}, "nil": nil})
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["text"] != "括號)\\\"" || got["nil"] != nil || got["nested"].(map[string]interface{})["enabled"] != true {
		t.Fatalf("JSON data changed: %v", got)
	}
	defer func() {
		v := recover()
		err, ok := v.(error)
		var unsupported *json.UnsupportedTypeError
		if !ok || !errors.As(err, &unsupported) {
			t.Fatalf("unsupported channel must panic with JSON type error: %v", v)
		}
	}()
	JSON(map[string]interface{}{"unsupported": make(chan int)})
}

func testutilCodecProperty(t *testing.T, payload []byte, second int64, nanos uint32) {
	t.Helper()
	second = second % 2000000000
	nanos %= 1000000000
	at := time.Unix(second, int64(nanos)).In(time.FixedZone("offset", 7*3600))
	original := *NewTaskMessageWithQueue("binary:contract", append([]byte(nil), payload...), "owned")
	original.Headers = map[string]string{"trace": "preserved"}
	original.Retried = 2
	original.GroupKey = "g"
	before := MustMarshal(t, &original)
	retry := TaskMessageAfterRetry(original, "failure", at)
	withError := TaskMessageWithError(original, "failure", at)
	completed := TaskMessageWithCompletedAt(original, at)
	if MustMarshal(t, &original) != before {
		t.Fatal("state helper mutated source fixture")
	}
	if retry.Retried != 3 || retry.LastFailedAt != second || retry.ErrorMsg != "failure" || withError.Retried != 2 || withError.LastFailedAt != second || completed.CompletedAt != second || completed.Retried != 2 {
		t.Fatal("retry/error/completion transition lost count or instant")
	}
	wire := MustMarshal(t, retry)
	var decoded pb.TaskMessage
	if err := proto.Unmarshal([]byte(wire), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Id != original.ID || decoded.Queue != "owned" || decoded.Type != "binary:contract" || decoded.Retried != 3 || decoded.LastFailedAt != second || decoded.GroupKey != "g" || decoded.Headers["trace"] != "preserved" || !bytes.Equal(decoded.Payload, payload) {
		t.Fatalf("wire-schema mismatch: %v", &decoded)
	}
	roundtrip := MustUnmarshal(t, wire)
	if !cmp.Equal(retry, roundtrip, cmp.Comparer(func(a, b []byte) bool { return bytes.Equal(a, b) })) {
		t.Fatalf("codec changed task: %s", cmp.Diff(retry, roundtrip))
	}
	if len(roundtrip.Payload) > 0 {
		roundtrip.Payload[0] ^= 0xff
		if bytes.Equal(roundtrip.Payload, original.Payload) {
			t.Fatal("decoder aliases input fixture payload")
		}
	}
}
func TestTestutilCodecProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(711))
	for i := 0; i < 200; i++ {
		p := make([]byte, rng.Intn(128))
		rng.Read(p)
		testutilCodecProperty(t, p, rng.Int63()-rng.Int63(), rng.Uint32())
	}
}
func FuzzTestutilCodecContracts(f *testing.F) {
	f.Add([]byte{0, 255, 10}, int64(-1), uint32(999999999))
	f.Add([]byte{}, int64(0), uint32(0))
	f.Fuzz(func(t *testing.T, p []byte, s int64, n uint32) {
		if len(p) > 4096 {
			t.Skip()
		}
		testutilCodecProperty(t, p, s, n)
	})
}

func TestTestutilFatalChild(t *testing.T) {
	switch os.Getenv("ASYNQ_TESTUTIL_FATAL_CASE") {
	case "marshal":
		MustMarshal(t, nil)
	case "unmarshal":
		MustUnmarshal(t, string([]byte{255}))
	default:
		t.Skip("parent-selected error subprocess")
	}
	t.Fatal("helper returned after fatal error")
}
func TestTestutilFatalContracts(t *testing.T) {
	for _, tc := range []struct{ mode, want string }{{"marshal", "cannot encode nil message"}, {"unmarshal", "cannot parse invalid wire-format data"}} {
		t.Run(tc.mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			args := []string{"-test.run=^TestTestutilFatalChild$", "-test.v"}
			if dir := os.Getenv("ASYNQ_TESTUTIL_COVERAGE_DIR"); dir != "" && testing.CoverMode() != "" {
				dir, err := filepath.Abs(dir)
				if err != nil {
					t.Fatal(err)
				}
				covdir := filepath.Join(dir, tc.mode+"-covdata")
				if err := os.MkdirAll(covdir, 0700); err != nil {
					t.Fatal(err)
				}
				args = append(args, "-test.coverprofile="+filepath.Join(dir, tc.mode+".cover"), "-test.gocoverdir="+covdir)
			}
			cmd := exec.CommandContext(ctx, os.Args[0], args...)
			cmd.Env = append(os.Environ(), "ASYNQ_TESTUTIL_FATAL_CASE="+tc.mode)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("fatal helper deadline: %v", ctx.Err())
			}
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 1 || !strings.Contains(string(out), tc.want) || strings.Contains(string(out), "helper returned after fatal error") {
				t.Fatalf("fatal contract: %v %s", err, out)
			}
		})
	}
}
