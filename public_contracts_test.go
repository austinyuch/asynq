package asynq

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	asynqcontext "github.com/austinyuch/asynq/internal/context"
	"github.com/redis/go-redis/v9"
)

func TestPublicTaskAccessorsContract(t *testing.T) {
	options := []Option{Queue("mail"), MaxRetry(2)}
	payload := []byte{0, 255}
	task := NewTask("mail:send", payload, options...)
	if task.Type() != "mail:send" || !bytes.Equal(task.Payload(), payload) || !reflect.DeepEqual(task.Options(), options) || task.Headers() != nil || task.ResultWriter() != nil {
		t.Fatal("new task accessors lost supplied data or gained a worker result writer")
	}
	headers := map[string]string{"trace": "original"}
	task = NewTaskWithHeaders("mail:send", payload, headers, options...)
	headers["trace"] = "changed"
	if task.Headers()["trace"] != "original" || !reflect.DeepEqual(task.Options(), options) {
		t.Fatal("task did not retain its headers snapshot/options")
	}
	if NewTaskWithHeaders("empty", nil, nil).Headers() != nil {
		t.Fatal("nil headers must remain absent")
	}
	writer := &ResultWriter{id: "task-id"}
	workerTask := newTask("worker", payload, writer)
	if workerTask.ResultWriter() != writer || writer.TaskID() != "task-id" || workerTask.Type() != "worker" || !bytes.Equal(workerTask.Payload(), payload) || workerTask.Headers() == nil {
		t.Fatal("worker result writer/task ID identity lost")
	}
}

func TestPublicTaskContextContract(t *testing.T) {
	msg := &base.TaskMessage{ID: "stable-id", Queue: "mail", Retry: 9, Retried: 3}
	ctx, cancel := asynqcontext.New(context.Background(), msg, time.Now().Add(time.Hour))
	defer cancel()
	for _, getter := range []struct {
		fn   func(context.Context) (string, bool)
		want string
	}{{GetTaskID, "stable-id"}, {GetQueueName, "mail"}} {
		if got, ok := getter.fn(ctx); !ok || got != getter.want {
			t.Fatalf("task context value=%q present=%v", got, ok)
		}
		if got, ok := getter.fn(context.Background()); ok || got != "" {
			t.Fatal("ordinary context fabricated task metadata")
		}
	}
	for _, getter := range []struct {
		fn   func(context.Context) (int, bool)
		want int
	}{{GetRetryCount, 3}, {GetMaxRetry, 9}} {
		if got, ok := getter.fn(ctx); !ok || got != getter.want {
			t.Fatalf("task retry metadata=%d present=%v", got, ok)
		}
		if got, ok := getter.fn(context.Background()); ok || got != 0 {
			t.Fatal("ordinary context fabricated retry metadata")
		}
	}
	cancel()
	if id, ok := GetTaskID(ctx); !ok || id != "stable-id" {
		t.Fatal("cancellation must not erase task identity")
	}
}

func FuzzPublicTaskContextMetadata(f *testing.F) {
	f.Add("id", "mail", 9, 3)
	f.Add("", "", 0, 0)
	f.Fuzz(func(t *testing.T, id, queue string, maxRetry, retried int) {
		ctx, cancel := asynqcontext.New(context.Background(), &base.TaskMessage{ID: id, Queue: queue, Retry: maxRetry, Retried: retried}, time.Now().Add(time.Minute))
		cancel()
		if got, ok := GetTaskID(ctx); !ok || got != id {
			t.Fatal("canceled task context lost ID presence/value")
		}
		if got, ok := GetQueueName(ctx); !ok || got != queue {
			t.Fatal("canceled task context lost queue presence/value")
		}
		if got, ok := GetMaxRetry(ctx); !ok || got != maxRetry {
			t.Fatal("canceled task context lost max retry presence/value")
		}
		if got, ok := GetRetryCount(ctx); !ok || got != retried {
			t.Fatal("canceled task context lost retry count presence/value")
		}
	})
}

func TestPublicOptionDescriptors(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		opt   Option
		kind  OptionType
		value interface{}
		text  string
	}{
		{MaxRetry(-1), MaxRetryOpt, 0, "MaxRetry(0)"}, {MaxRetry(3), MaxRetryOpt, 3, "MaxRetry(3)"},
		{Queue("mail"), QueueOpt, "mail", `Queue("mail")`}, {TaskID("id"), TaskIDOpt, "id", `TaskID("id")`},
		{Timeout(2 * time.Second), TimeoutOpt, 2 * time.Second, "Timeout(2s)"},
		{Deadline(at), DeadlineOpt, at, "Deadline(Thu Oct  1 00:00:00 UTC 2026)"},
		{Unique(time.Second), UniqueOpt, time.Second, "Unique(1s)"},
		{ProcessAt(at), ProcessAtOpt, at, "ProcessAt(Thu Oct  1 00:00:00 UTC 2026)"},
		{ProcessIn(time.Minute), ProcessInOpt, time.Minute, "ProcessIn(1m0s)"},
		{Retention(time.Hour), RetentionOpt, time.Hour, "Retention(1h0m0s)"},
		{Group("batch"), GroupOpt, "batch", `Group("batch")`},
		{Header("trace", "value"), HeaderOpt, [2]string{"trace", "value"}, `Header(["trace","value"])`},
	}
	for _, tc := range tests {
		if tc.opt.Type() != tc.kind || !reflect.DeepEqual(tc.opt.Value(), tc.value) || tc.opt.String() != tc.text {
			t.Errorf("option descriptor %T: %v/%v/%q", tc.opt, tc.opt.Type(), tc.opt.Value(), tc.opt.String())
		}
		// Scheduler serialization supports this documented set of option types.
		if tc.kind == TaskIDOpt || tc.kind == GroupOpt {
			continue
		}
		got, err := parseOption(tc.opt.String())
		if err != nil || got.Type() != tc.kind || !reflect.DeepEqual(got.Value(), tc.value) {
			t.Errorf("serialized option %q failed semantic roundtrip: %v/%v", tc.text, got, err)
		}
	}
	for _, s := range []string{`Queue(unquoted)`, `MaxRetry(no)`, `MaxRetry(`, `Timeout(no)`, `Deadline(no)`, `Unique(no)`, `ProcessAt(no)`, `ProcessIn(no)`, `Retention(no)`, `Header(no)`, `Unsupported(x)`} {
		if got, err := parseOption(s); err == nil || got != nil {
			t.Errorf("invalid serialized option %q accepted", s)
		}
	}
}

func TestPublicOptionPrecedenceAndValidation(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	got, err := composeOptions(MaxRetry(1), MaxRetry(7), Queue("old"), Queue("new"), TaskID("old"), TaskID("new"), Timeout(time.Second), Timeout(2*time.Second), Deadline(at.Add(time.Hour)), Deadline(at), Unique(time.Second), Unique(2*time.Second), Retention(time.Second), Retention(time.Minute), Group("old"), Group("new"), Header("a", "old"), Header("b", "kept"), Header("a", "new"), ProcessIn(time.Hour), ProcessAt(at))
	if err != nil {
		t.Fatal(err)
	}
	if got.retry != 7 || got.queue != "new" || got.taskID != "new" || got.timeout != 2*time.Second || !got.deadline.Equal(at) || got.uniqueTTL != 2*time.Second || got.retention != time.Minute || got.group != "new" || got.headers["a"] != "new" || got.headers["b"] != "kept" || !got.processAt.Equal(at) {
		t.Fatalf("last option failed to override its own field: %+v", got)
	}
	before := time.Now()
	relative, err := composeOptions(ProcessAt(at), ProcessIn(time.Minute))
	after := time.Now()
	if err != nil || relative.processAt.Before(before.Add(time.Minute)) || relative.processAt.After(after.Add(time.Minute)) {
		t.Fatal("last relative schedule failed to override absolute schedule")
	}
	for _, opt := range []Option{Queue("\u2003\n"), TaskID("\t"), Group(" "), Unique(time.Second - time.Nanosecond), Unique(-time.Second)} {
		if _, err := composeOptions(opt); err == nil {
			t.Errorf("invalid option %v accepted", opt)
		}
	}
	defaults, err := composeOptions()
	if err != nil || defaults.queue != base.DefaultQueueName || defaults.retry != defaultMaxRetry || defaults.taskID == "" || defaults.headers == nil {
		t.Fatal("default option contract failed")
	}
}

func TestPublicOptionPrecedenceProperty(t *testing.T) {
	property := func(a, b uint16) bool {
		got, err := composeOptions(MaxRetry(int(a)), MaxRetry(int(b)))
		return err == nil && got.retry == int(b)
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261003))}); err != nil {
		t.Fatal(err)
	}
}

func TestPublicTaskInfoStateContract(t *testing.T) {
	at := time.Unix(1700000000, 0)
	payload, result := []byte("payload"), []byte("result")
	msg := &base.TaskMessage{ID: "id", Queue: "q", Type: "kind", Payload: payload, Headers: map[string]string{"trace": "value"}, Retry: 9, Retried: 3, ErrorMsg: "cause", GroupKey: "g", Timeout: 2, Deadline: at.Unix(), Retention: 4, LastFailedAt: at.Unix(), CompletedAt: at.Unix()}
	for i, name := range []string{"active", "pending", "scheduled", "retry", "archived", "completed", "aggregating"} {
		info := newTaskInfo(msg, base.TaskState(i+1), at, result)
		if info.State.String() != name || info.ID != "id" || info.Queue != "q" || info.Type != "kind" || !bytes.Equal(info.Payload, payload) || !bytes.Equal(info.Result, result) || info.Headers["trace"] != "value" || info.MaxRetry != 9 || info.Retried != 3 || info.LastErr != "cause" || info.Group != "g" || info.Timeout != 2*time.Second || info.Retention != 4*time.Second || !info.Deadline.Equal(at) || !info.LastFailedAt.Equal(at) || !info.CompletedAt.Equal(at) || !info.NextProcessAt.Equal(at) {
			t.Fatalf("task info state %s lost metadata: %+v", name, info)
		}
	}
	zero := newTaskInfo(&base.TaskMessage{}, base.TaskStatePending, time.Time{}, nil)
	if !zero.Deadline.IsZero() || !zero.LastFailedAt.IsZero() || !zero.CompletedAt.IsZero() {
		t.Fatal("absent timestamps must remain zero time")
	}
	for _, fn := range []func(){func() { _ = TaskState(0).String() }, func() { _ = newTaskInfo(msg, base.TaskState(0), at, nil) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid state must panic")
				}
			}()
			fn()
		}()
	}
}

type publicResultBroker struct {
	base.Broker
	calls     int
	queue, id string
	data      []byte
	err       error
}

func (b *publicResultBroker) WriteResult(queue, id string, data []byte) (int, error) {
	b.calls++
	b.queue = queue
	b.id = id
	b.data = append([]byte(nil), data...)
	if b.err != nil {
		return 0, b.err
	}
	return len(data), nil
}

func TestPublicResultWriterContract(t *testing.T) {
	b := &publicResultBroker{}
	w := &ResultWriter{id: "id", qname: "q", broker: b, ctx: context.Background()}
	payload := []byte{0, 255}
	if n, err := w.Write(payload); err != nil || n != len(payload) || b.queue != "q" || b.id != "id" || !bytes.Equal(b.data, payload) {
		t.Fatal("result writer did not preserve task identity/payload/write count")
	}
	sentinel := errors.New("storage failure")
	b.err = sentinel
	if n, err := w.Write(payload); n != 0 || !errors.Is(err, sentinel) {
		t.Fatal("result writer discarded storage error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.ctx = ctx
	before := b.calls
	if n, err := w.Write(payload); n != 0 || !errors.Is(err, context.Canceled) || b.calls != before {
		t.Fatal("canceled writer touched broker or lost cancellation cause")
	}
}

func TestPublicRedisClientsLazyConfiguration(t *testing.T) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "test.invalid"}
	regular := RedisClientOpt{Network: "unix", Addr: "/unreachable/asynq-contract.sock", Username: "user", Password: "password", DB: 14, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 3 * time.Second, PoolSize: 2, TLSConfig: tlsConfig}.MakeRedisClient().(*redis.Client)
	defer regular.Close()
	sentinel := RedisFailoverClientOpt{MasterName: "master", SentinelAddrs: []string{"127.0.0.1:1"}, SentinelUsername: "sentinel-user", SentinelPassword: "sentinel-password", Username: "user", Password: "password", DB: 14, DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 3 * time.Second, PoolSize: 2, TLSConfig: tlsConfig}.MakeRedisClient().(*redis.Client)
	defer sentinel.Close()
	for _, c := range []*redis.Client{regular, sentinel} {
		o := c.Options()
		if o.Username != "user" || o.Password != "password" || o.DB != 14 || o.DialTimeout != time.Second || o.ReadTimeout != 2*time.Second || o.WriteTimeout != 3*time.Second || o.PoolSize != 2 || o.TLSConfig != tlsConfig {
			t.Fatal("redis credentials/database/timeout/TLS configuration lost")
		}
		if c.PoolStats().TotalConns != 0 {
			t.Fatal("redis client constructor unexpectedly opened a connection")
		}
	}
	if regular.Options().Network != "unix" || regular.Options().Addr != "/unreachable/asynq-contract.sock" {
		t.Fatal("unix socket configuration lost")
	}
	cluster := RedisClusterClientOpt{Addrs: []string{"127.0.0.1:1"}, MaxRedirects: 7, Username: "user", Password: "password", DialTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 3 * time.Second, TLSConfig: tlsConfig}.MakeRedisClient().(*redis.ClusterClient)
	defer cluster.Close()
	o := cluster.Options()
	if !reflect.DeepEqual(o.Addrs, []string{"127.0.0.1:1"}) || o.MaxRedirects != 7 || o.Username != "user" || o.Password != "password" || o.DialTimeout != time.Second || o.ReadTimeout != 2*time.Second || o.WriteTimeout != 3*time.Second || o.TLSConfig != tlsConfig || cluster.PoolStats().TotalConns != 0 {
		t.Fatal("lazy cluster configuration lost seed addresses/redirects/credentials/TLS")
	}
}
