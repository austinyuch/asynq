package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	"github.com/redis/go-redis/v9"
)

type exporterFixture struct {
	opt       asynq.RedisClientOpt
	client    *redis.Client
	inspector *asynq.Inspector
	queues    []string
}

func newExporterFixture(t *testing.T) *exporterFixture {
	t.Helper()
	addr := os.Getenv("ASYNQ_EXPORTER_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ASYNQ_EXPORTER_TEST_REDIS_ADDR for exclusively owned DB12")
	}
	f := &exporterFixture{opt: asynq.RedisClientOpt{Addr: addr, DB: 12}, client: redis.NewClient(&redis.Options{Addr: addr, DB: 12})}
	t.Cleanup(func() { f.client.Close() })
	if err := f.client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	f.inspector = asynq.NewInspector(f.opt)
	t.Cleanup(func() { f.inspector.Close() })
	prefix := fmt.Sprintf("exporter-contract-%d", time.Now().UnixNano())
	f.queues = []string{prefix, prefix + "-neighbor"}
	t.Cleanup(func() {
		for _, q := range f.queues {
			if e := f.inspector.DeleteQueue(q, true); e != nil && !errors.Is(e, asynq.ErrQueueNotFound) {
				t.Errorf("owned fixture cleanup: %v", e)
			}
		}
	})
	c := asynq.NewClient(f.opt)
	defer c.Close()
	for _, q := range f.queues {
		if _, e := c.Enqueue(asynq.NewTask("exporter-contract", []byte("preserved-payload")), asynq.Queue(q), asynq.TaskID("owned-task")); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := f.inspector.GetQueueInfo(f.queues[0]); e != nil {
		t.Fatal(e)
	}
	return f
}
func (f *exporterFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	keys := []string{base.AllQueues}
	for _, q := range f.queues {
		k, e := f.client.Keys(context.Background(), base.QueueKeyPrefix(q)+"*").Result()
		if e != nil {
			t.Fatal(e)
		}
		keys = append(keys, k...)
	}
	result := map[string]string{}
	for _, k := range keys {
		v, e := f.client.Dump(context.Background(), k).Result()
		if e != nil {
			t.Fatal(e)
		}
		result[k] = v
	}
	return result
}
func (f *exporterFixture) connections(t *testing.T) map[string]bool {
	t.Helper()
	raw, e := f.client.ClientList(context.Background()).Result()
	if e != nil {
		t.Fatal(e)
	}
	ids := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		id := ""
		owned := false
		for _, v := range strings.Fields(line) {
			if strings.HasPrefix(v, "id=") {
				id = strings.TrimPrefix(v, "id=")
			}
			if v == "db=12" {
				owned = true
			}
		}
		if owned {
			ids[id] = true
		}
	}
	return ids
}
func requireExporterConnectionsClosed(t *testing.T, f *exporterFixture, before map[string]bool) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for {
		now := f.connections(t)
		newIDs := []string{}
		for id := range now {
			if !before[id] {
				newIDs = append(newIDs, id)
			}
		}
		if len(newIDs) == 0 {
			return
		}
		if time.Now().After(until) {
			t.Fatalf("exporter retained owned transport IDs %v", newIDs)
		}
		time.Sleep(time.Millisecond)
	}
}
func requireExporterMetrics(t *testing.T, f *exporterFixture, body string) {
	t.Helper()
	for _, q := range f.queues {
		info, e := f.inspector.GetQueueInfo(q)
		if e != nil {
			t.Fatal(e)
		}
		want := fmt.Sprintf("asynq_queue_size{queue=%q} %d", q, info.Size)
		if !exporterExactMetric(body, strings.Fields(want)[0], float64(info.Size)) {
			t.Fatalf("independent Inspector metric missing %q", want)
		}
		for _, v := range []struct {
			state string
			n     int
		}{{"active", info.Active}, {"pending", info.Pending}, {"scheduled", info.Scheduled}, {"retry", info.Retry}, {"archived", info.Archived}, {"completed", info.Completed}} {
			want = fmt.Sprintf("asynq_tasks_enqueued_total{queue=%q,state=%q} %d", q, v.state, v.n)
			if !exporterExactMetric(body, strings.Fields(want)[0], float64(v.n)) {
				t.Fatalf("independent state metric missing %q", want)
			}
		}
	}
	if !strings.Contains(body, "go_goroutines ") || !strings.Contains(body, "process_start_time_seconds ") {
		t.Fatal("pedantic registry omitted standard collectors")
	}
}

func TestExporterInstancesMetricsAndMuxIsolation(t *testing.T) {
	f := newExporterFixture(t)
	snapshot := f.snapshot(t)
	before := f.connections(t)
	saved := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	http.DefaultServeMux.HandleFunc("/default-only", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "sentinel") })
	t.Cleanup(func() { http.DefaultServeMux = saved })
	for i := 0; i < 2; i++ {
		e := newExporter(f.opt, "127.0.0.1:0")
		t.Cleanup(func() { e.inspector.Close() })
		if e.server.Handler == http.DefaultServeMux {
			t.Fatal("exporter captured global mux")
		}
		if e.server.ReadHeaderTimeout != 10*time.Second || e.server.ReadTimeout != 30*time.Second || e.server.WriteTimeout != 60*time.Second || e.server.IdleTimeout != 120*time.Second {
			t.Fatalf("server timeout policy changed: %+v", e.server)
		}
		w := httptest.NewRecorder()
		e.server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://owned/metrics", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("metrics HTTP status=%d: %s", w.Code, w.Body)
		}
		requireExporterMetrics(t, f, w.Body.String())
		absent := httptest.NewRecorder()
		e.server.Handler.ServeHTTP(absent, httptest.NewRequest(http.MethodGet, "http://owned/default-only", nil))
		if absent.Code != 404 {
			t.Fatal("private mux inherited default handler")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := e.run(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("already canceled run: %v", err)
		}
	}
	untouched := httptest.NewRecorder()
	http.DefaultServeMux.ServeHTTP(untouched, httptest.NewRequest(http.MethodGet, "http://owned/metrics", nil))
	if untouched.Code != 404 {
		t.Fatal("exporter registered /metrics globally")
	}
	requireExporterConnectionsClosed(t, f, before)
	if !reflect.DeepEqual(snapshot, f.snapshot(t)) {
		t.Fatal("metrics mutated owned/neighbor task bytes")
	}
}
func TestExporterServeGracefulHandlerBarrier(t *testing.T) {
	f := newExporterFixture(t)
	before := f.connections(t)
	e := newExporter(f.opt, "127.0.0.1:0")
	t.Cleanup(func() { e.inspector.Close() })
	if err := exporterInspectorError(e.inspector); err != nil {
		t.Fatal(err)
	}
	h := newExporterHarness(t, e, true)
	h.waitEntered(t)
	h.cancel()
	select {
	case err := <-h.done:
		h.runJoined = true
		t.Fatalf("returned before active handler finished: %v", err)
	default:
	}
	h.releaseHandler()
	if err := h.waitServe(t); err != nil {
		t.Fatal(err)
	}
	if err := <-h.handlerErr; err != nil {
		t.Fatalf("Inspector closed before active handler barrier: %v", err)
	}
	if err := h.waitHTTP(t); err != nil {
		t.Fatal(err)
	}
	if err := exporterInspectorError(e.inspector); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("owned Inspector did not report exact closed transport after shutdown: %v", err)
	}
	requireExporterConnectionsClosed(t, f, before)
}
func TestExporterCanceledBindAndTimeoutContracts(t *testing.T) {
	f := newExporterFixture(t)
	t.Run("bind", func(t *testing.T) {
		before := f.connections(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		e := newExporter(f.opt, ln.Addr().String())
		t.Cleanup(func() { e.inspector.Close() })
		err = e.run(context.Background())
		var op *net.OpError
		if !errors.As(err, &op) || !strings.Contains(err.Error(), "listen metrics:") {
			t.Fatalf("bind cause lost: %v", err)
		}
		requireExporterConnectionsClosed(t, f, before)
	})
	t.Run("pre-canceled-listener", func(t *testing.T) {
		before := f.connections(t)
		e := newExporter(f.opt, "127.0.0.1:0")
		t.Cleanup(func() { e.inspector.Close() })
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err = e.serve(ctx, ln); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		conn, err := net.DialTimeout("tcp", ln.Addr().String(), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			t.Fatal("pre-canceled owned listener remained open")
		}
		requireExporterConnectionsClosed(t, f, before)
	})
	t.Run("shutdown-deadline", func(t *testing.T) {
		before := f.connections(t)
		e := newExporter(f.opt, "127.0.0.1:0")
		t.Cleanup(func() { e.inspector.Close() })
		e.shutdownTimeout = 20 * time.Millisecond
		h := newExporterHarness(t, e, false)
		h.waitEntered(t)
		h.cancel()
		if err := h.waitServe(t); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown cause lost: %v", err)
		}
		select {
		case <-h.handlerDone:
			t.Fatal("test did not retain independent handler until release")
		default:
		}
		h.releaseHandler()
		select {
		case <-h.handlerDone:
		case <-time.After(time.Second):
			t.Fatal("test handler did not join")
		}
		_ = h.waitHTTP(t)
		requireExporterConnectionsClosed(t, f, before)
	})
}

func TestExporterProcessHelper(t *testing.T) {
	if os.Getenv("ASYNQ_EXPORTER_CHILD") != "1" {
		return
	}
	main()
	fmt.Println("EXPORTER_CHILD_RETURNED")
}
func exporterChild(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, exe, append([]string{"-test.run=^TestExporterProcessHelper$"}, args...)...)
	cmd.Env = append(os.Environ(), "ASYNQ_EXPORTER_CHILD=1")
	return cmd
}
func TestExporterChildFlagsAndSignal(t *testing.T) {
	f := newExporterFixture(t)
	t.Run("malformed", func(t *testing.T) {
		cmd := exporterChild(t, "-port=not-an-int")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "invalid value") {
			t.Fatalf("malformed flag must exit before runtime: %v %s", err, out)
		}
	})
	t.Run("bind-failure", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		cmd := exporterChild(t, "-redis-addr="+f.opt.Addr, "-redis-db=12", fmt.Sprintf("-port=%d", port))
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "listen metrics:") {
			t.Fatalf("real bind failure: %v %s", err, out)
		}
	})
	t.Run("SIGTERM", func(t *testing.T) {
		before := f.connections(t)
		args := []string{"-redis-addr=" + f.opt.Addr, "-redis-db=12", "-port=0"}
		if testing.CoverMode() != "" && os.Getenv("ASYNQ_EXPORTER_COVERAGE_DIR") != "" {
			dir := os.Getenv("ASYNQ_EXPORTER_COVERAGE_DIR")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			args = append(args, "-test.coverprofile="+filepath.Join(dir, "sigterm.cover"))
		}
		cmd := exporterChild(t, args...)
		pipe, err := cmd.StderrPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		joined := false
		t.Cleanup(func() {
			if !joined {
				cmd.Process.Kill()
				<-done
			}
		})
		ready := make(chan string, 1)
		scanned := make(chan struct{})
		go func() {
			defer close(scanned)
			scan := bufio.NewScanner(pipe)
			for scan.Scan() {
				line := scan.Text()
				if strings.Contains(line, "listening on ") {
					fields := strings.Fields(line)
					ready <- fields[len(fields)-1]
				}
			}
		}()
		t.Cleanup(func() {
			pipe.Close()
			select {
			case <-scanned:
			case <-time.After(time.Second):
				t.Error("child stderr reader did not join")
			}
		})
		var addr string
		select {
		case addr = <-ready:
		case err := <-done:
			joined = true
			t.Fatalf("child exited before ready: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("child listener deadline")
		}
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Timeout: time.Second}
		resp, err := client.Get("http://127.0.0.1:" + port + "/metrics")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		requireExporterMetrics(t, f, string(body))
		if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			joined = true
			if err != nil {
				t.Fatalf("SIGTERM abrupt exit: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("SIGTERM shutdown deadline")
		}
		<-scanned
		if !strings.Contains(stdout.String(), "EXPORTER_CHILD_RETURNED") {
			t.Fatal("main did not return normally after SIGTERM")
		}
		requireExporterConnectionsClosed(t, f, before)
	})
}

func exporterInspectorError(i *asynq.Inspector) error { _, err := i.Queues(); return err }
func TestExporterAcceptErrorHandlerBarrier(t *testing.T) {
	f := newExporterFixture(t)
	before := f.connections(t)
	e := newExporter(f.opt, "127.0.0.1:0")
	t.Cleanup(func() { e.inspector.Close() })
	h := newExporterHarness(t, e, true)
	h.waitEntered(t)
	h.listener.Close()
	select {
	case err := <-h.done:
		h.runJoined = true
		t.Fatalf("accept error bypassed active-handler barrier: %v", err)
	default:
	}
	h.releaseHandler()
	if err := h.waitServe(t); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("accept cause lost: %v", err)
	}
	if err := <-h.handlerErr; err != nil {
		t.Fatalf("Inspector closed before accept-error handler barrier: %v", err)
	}
	if err := h.waitHTTP(t); err != nil {
		t.Fatal(err)
	}
	requireExporterConnectionsClosed(t, f, before)
}

// Compare complete series and parse the value token; value 1 must not match 10.
func exporterExactMetric(body, series string, value float64) bool {
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != series {
			continue
		}
		got, err := strconv.ParseFloat(fields[1], 64)
		return err == nil && got == value
	}
	return false
}

type exporterHarness struct {
	e                             *exporter
	listener                      net.Listener
	cancel                        context.CancelFunc
	done, response                chan error
	entered, release, handlerDone chan struct{}
	handlerErr                    chan error
	releaseOnce                   sync.Once
	runJoined, httpJoined         bool
}

func newExporterHarness(t *testing.T, e *exporter, checkInspector bool) *exporterHarness {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := &exporterHarness{e: e, listener: ln, cancel: cancel, done: make(chan error, 1), response: make(chan error, 1), entered: make(chan struct{}), release: make(chan struct{}), handlerDone: make(chan struct{}), handlerErr: make(chan error, 1)}
	// Register joins before launching either goroutine or making a fatal assertion.
	t.Cleanup(func() {
		h.releaseHandler()
		cancel()
		ln.Close()
		e.server.Close()
		if !h.runJoined {
			select {
			case <-h.done:
				h.runJoined = true
			case <-time.After(3 * time.Second):
				t.Error("cleanup failed to join owned Serve")
			}
		}
		if !h.httpJoined {
			select {
			case <-h.response:
				h.httpJoined = true
			case <-time.After(3 * time.Second):
				t.Error("cleanup failed to join owned HTTP client")
			}
		}
		// An HTTP call rejected before acceptance never starts the handler.
		select {
		case <-h.entered:
			select {
			case <-h.handlerDone:
			case <-time.After(3 * time.Second):
				t.Error("cleanup failed to join entered handler")
			}
		default:
		}
	})
	e.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(h.entered)
		defer close(h.handlerDone)
		<-h.release
		if checkInspector {
			h.handlerErr <- exporterInspectorError(e.inspector)
		}
		fmt.Fprint(w, "joined")
	})
	go func() { h.done <- e.serve(ctx, ln) }()
	go func() {
		c := &http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://" + ln.Addr().String())
		if resp != nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil {
				err = readErr
			}
			if err == nil && string(body) != "joined" {
				err = fmt.Errorf("handler body=%q", body)
			}
		}
		h.response <- err
	}()
	return h
}
func (h *exporterHarness) releaseHandler() { h.releaseOnce.Do(func() { close(h.release) }) }
func (h *exporterHarness) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-h.entered:
	case err := <-h.done:
		h.runJoined = true
		t.Fatalf("Serve returned before handler entry: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("owned handler did not enter")
	}
}
func (h *exporterHarness) waitServe(t *testing.T) error {
	t.Helper()
	select {
	case err := <-h.done:
		h.runJoined = true
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("owned Serve did not join")
		return nil
	}
}
func (h *exporterHarness) waitHTTP(t *testing.T) error {
	t.Helper()
	select {
	case err := <-h.response:
		h.httpJoined = true
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("owned HTTP client did not join")
		return nil
	}
}

func TestExporterMetricOracleRejectsPrefixValues(t *testing.T) {
	series := `asynq_queue_size{queue="owned"}`
	for _, tc := range []struct {
		body string
		want bool
	}{{series + " 1\n", true}, {series + " 1.0\n", true}, {series + " 10\n", false}, {series + " 1e1\n", false}, {series + " 1garbage\n", false}, {`asynq_queue_size{queue="other"} 1` + "\n", false}} {
		if got := exporterExactMetric(tc.body, series, 1); got != tc.want {
			t.Fatalf("metric value oracle body=%q got=%t want=%t", tc.body, got, tc.want)
		}
	}
}
