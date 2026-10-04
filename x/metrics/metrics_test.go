package metrics

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/redis/go-redis/v9"
)

var metricsRedisAddr = flag.String("redis_addr", "localhost:6379", "Redis address for metrics integration tests")
var metricsRedisDB = flag.Int("redis_db", 15, "Dedicated Redis database for metrics integration tests")

// Module-wide -redis_db also configures concurrently running rate tests.
// Explicit generic flags require a distinct metrics database override.
func resolveMetricsRedisDB(override string, generic int, explicit bool) (int, error) {
	if override == "" {
		if explicit {
			return 0, fmt.Errorf("explicit -redis_db requires a distinct ASYNQ_METRICS_TEST_REDIS_DB to isolate metrics from rate tests")
		}
		return generic, nil
	}
	db, err := strconv.Atoi(override)
	if err != nil || db < 0 || db > 15 {
		return 0, fmt.Errorf("ASYNQ_METRICS_TEST_REDIS_DB must be an integer in [0,15]")
	}
	rateDB := 14 // rate package default when -redis_db is not supplied.
	if explicit {
		rateDB = generic
	}
	if db == rateDB {
		return 0, fmt.Errorf("ASYNQ_METRICS_TEST_REDIS_DB must differ from rate test database (explicit -redis_db or default 14) to isolate concurrent packages")
	}
	return db, nil
}

func TestMetricsDatabaseIsolation(t *testing.T) {
	cases := []struct {
		name, override string
		generic        int
		explicit       bool
		want           int
		wantErr        bool
	}{
		{"default metrics DB", "", 15, false, 15, false},
		{"dedicated override", "10", 15, false, 10, false},
		{"explicit generic without override", "", 14, true, 0, true},
		{"distinct explicit databases", "10", 9, true, 10, false},
		{"same explicit database rejected", "10", 10, true, 0, true},
		{"implicit rate default collision rejected", "14", 15, false, 0, true},
		{"invalid override", "bad", 15, false, 0, true},
		{"negative override", "-1", 15, false, 0, true},
		{"out of range override", "16", 15, false, 0, true},
		{"zero valid override", "0", 15, false, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveMetricsRedisDB(tc.override, tc.generic, tc.explicit)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("db=%d err=%v want db=%d error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func metricsFixture(t *testing.T) (*QueueMetricsCollector, *redis.Client) {
	t.Helper()
	explicitGenericDB := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "redis_db" {
			explicitGenericDB = true
		}
	})
	db, err := resolveMetricsRedisDB(os.Getenv("ASYNQ_METRICS_TEST_REDIS_DB"), *metricsRedisDB, explicitGenericDB)
	if err != nil {
		t.Fatal(err)
	}
	opt := asynq.RedisClientOpt{Addr: *metricsRedisAddr, DB: db}
	client := redis.NewClient(&redis.Options{Addr: opt.Addr, DB: opt.DB})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	h.FlushDB(t, client)
	inspector := asynq.NewInspector(opt)
	t.Cleanup(func() { _ = inspector.Close(); _ = client.Close() })
	return NewQueueMetricsCollector(inspector), client
}

func gathered(t *testing.T, collector *QueueMetricsCollector) map[string]*dto.MetricFamily {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]*dto.MetricFamily)
	for _, family := range families {
		result[family.GetName()] = family
	}
	return result
}

func metric(t *testing.T, families map[string]*dto.MetricFamily, name, queue, state string, kind dto.MetricType) float64 {
	t.Helper()
	family := families[name]
	if family == nil {
		t.Fatalf("missing family %s", name)
	}
	if family.GetType() != kind {
		t.Fatalf("%s type=%s want %s", name, family.GetType(), kind)
	}
	for _, value := range family.Metric {
		labels := make(map[string]string)
		for _, label := range value.Label {
			labels[label.GetName()] = label.GetValue()
		}
		if labels["queue"] == queue && labels["state"] == state {
			wantLabels := 1
			if state != "" {
				wantLabels = 2
			}
			if len(labels) != wantLabels {
				t.Fatalf("%s unexpected labels %v", name, labels)
			}
			if kind == dto.MetricType_COUNTER {
				return value.GetCounter().GetValue()
			}
			return value.GetGauge().GetValue()
		}
	}
	t.Fatalf("missing %s queue=%q state=%q", name, queue, state)
	return 0
}

func TestQueueMetricsCollectorContract(t *testing.T) {
	t.Setenv("DISABLE_MEMORY_USAGE_PROFILING", "false")
	collector, client := metricsFixture(t)
	ctx := context.Background()
	// Distinct cardinalities catch accidental swaps between task states.
	queue := "metrics-contract"
	msgs := func(n int) []*base.TaskMessage {
		result := make([]*base.TaskMessage, n)
		for i := range result {
			result[i] = h.NewTaskMessageWithQueue("metrics:test", nil, queue)
		}
		return result
	}
	entries := func(n int) []base.Z {
		result := make([]base.Z, n)
		for i, msg := range msgs(n) {
			result[i] = base.Z{Message: msg, Score: time.Now().Add(time.Hour).Unix()}
		}
		return result
	}
	h.SeedActiveQueue(t, client, msgs(1), queue)
	pending := msgs(2)
	h.SeedPendingQueue(t, client, pending, queue)
	for _, msg := range pending {
		if err := client.HSet(ctx, base.TaskKey(queue, msg.ID), "pending_since", time.Now().Add(-5*time.Second).UnixNano()).Err(); err != nil {
			t.Fatal(err)
		}
	}
	h.SeedScheduledQueue(t, client, entries(3), queue)
	h.SeedRetryQueue(t, client, entries(4), queue)
	h.SeedArchivedQueue(t, client, entries(5), queue)
	h.SeedCompletedQueue(t, client, entries(6), queue)
	if err := client.Set(ctx, base.ProcessedTotalKey(queue), 31, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := client.Set(ctx, base.FailedTotalKey(queue), 7, 0).Err(); err != nil {
		t.Fatal(err)
	}
	families := gathered(t, collector)
	if len(families) != 7 {
		t.Fatalf("families=%d want 7", len(families))
	}
	for i, state := range []string{"active", "pending", "scheduled", "retry", "archived", "completed"} {
		if got := metric(t, families, "asynq_tasks_enqueued_total", queue, state, dto.MetricType_GAUGE); got != float64(i+1) {
			t.Errorf("state %s=%v want %d", state, got, i+1)
		}
	}
	for name, want := range map[string]float64{"asynq_queue_size": 21, "asynq_queue_paused_total": 0} {
		if got := metric(t, families, name, queue, "", dto.MetricType_GAUGE); got != want {
			t.Errorf("%s=%v want %v", name, got, want)
		}
	}
	for name, want := range map[string]float64{"asynq_tasks_processed_total": 31, "asynq_tasks_failed_total": 7} {
		if got := metric(t, families, name, queue, "", dto.MetricType_COUNTER); got != want {
			t.Errorf("%s=%v want %v", name, got, want)
		}
	}
	if got := metric(t, families, "asynq_queue_latency_seconds", queue, "", dto.MetricType_GAUGE); got < 5 || got > 60 {
		t.Errorf("latency seconds=%v want [5,60] for task pending since five seconds ago", got)
	}
	if got := metric(t, families, "asynq_queue_memory_usage_approx_bytes", queue, "", dto.MetricType_GAUGE); got <= 0 {
		t.Errorf("nonpositive memory %v for populated queue", got)
	}
	if err := collector.inspector.PauseQueue(queue); err != nil {
		t.Fatal(err)
	}
	if got := metric(t, gathered(t, collector), "asynq_queue_paused_total", queue, "", dto.MetricType_GAUGE); got != 1 {
		t.Errorf("paused=%v want 1", got)
	}
}

func TestQueueMetricsCollectorEmpty(t *testing.T) {
	collector, _ := metricsFixture(t)
	infos, err := collector.collectQueueInfo()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 0 {
		t.Fatalf("empty Redis returned %d queue infos", len(infos))
	}
	if families := gathered(t, collector); len(families) != 0 {
		t.Fatalf("empty Redis exported %d families", len(families))
	}
}

func TestQueueMetricsCollectorErrors(t *testing.T) {
	t.Run("queue names unavailable", func(t *testing.T) {
		collector, _ := metricsFixture(t)
		if err := collector.inspector.Close(); err != nil {
			t.Fatal(err)
		}
		infos, err := collector.collectQueueInfo()
		if err == nil || !strings.Contains(err.Error(), "failed to get queue names:") {
			t.Fatalf("infos=%v err=%v", infos, err)
		}
		if families := gathered(t, collector); len(families) != 0 {
			t.Fatalf("failed collector emitted %d families", len(families))
		}
	})
	t.Run("queue info wrong type", func(t *testing.T) {
		collector, client := metricsFixture(t)
		queue := "broken-metrics"
		h.SeedRedisSet(t, client, base.AllQueues, []string{queue})
		if err := client.Set(context.Background(), base.PendingKey(queue), "wrong-type", 0).Err(); err != nil {
			t.Fatal(err)
		}
		infos, err := collector.collectQueueInfo()
		if infos != nil || err == nil || !strings.Contains(err.Error(), "failed to get queue info:") {
			t.Fatalf("infos=%v err=%v", infos, err)
		}
		if families := gathered(t, collector); len(families) != 0 {
			t.Fatalf("failed collector emitted %d families", len(families))
		}
	})
}

func TestQueueMetricsCollectorCardinalityProperties(t *testing.T) {
	collector, client := metricsFixture(t)
	// One shared scrape must preserve each queue's independently varied counts.
	for n := 0; n < 5; n++ {
		queue := fmt.Sprintf("metrics-property-%d", n)
		msgs := make([]*base.TaskMessage, n)
		for i := range msgs {
			msgs[i] = h.NewTaskMessageWithQueue("metrics:property", nil, queue)
		}
		h.SeedPendingQueue(t, client, msgs, queue)
		if n%2 == 1 {
			if err := collector.inspector.PauseQueue(queue); err != nil {
				t.Fatal(err)
			}
		}
	}
	families := gathered(t, collector)
	for n := 0; n < 5; n++ {
		queue := fmt.Sprintf("metrics-property-%d", n)
		for _, name := range []string{"asynq_queue_size", "asynq_tasks_enqueued_total"} {
			state := ""
			if name == "asynq_tasks_enqueued_total" {
				state = "pending"
			}
			if got := metric(t, families, name, queue, state, dto.MetricType_GAUGE); got != float64(n) {
				t.Errorf("%s queue=%s got=%v want=%d", name, queue, got, n)
			}
		}
		if got := metric(t, families, "asynq_queue_paused_total", queue, "", dto.MetricType_GAUGE); got != float64(n%2) {
			t.Errorf("queue=%s paused=%v", queue, got)
		}
	}
}

// Vary cardinality and pause state independently while retaining other queues.
// A fixed seed makes each property failure reproducible.
func TestQueueMetricsCollectorGeneratedProperties(t *testing.T) {
	collector, client := metricsFixture(t)
	iteration := 0
	property := func(raw uint8, paused bool) bool {
		count := int(raw % 12)
		queue := fmt.Sprintf("metrics-generated-%d", iteration)
		iteration++
		messages := make([]*base.TaskMessage, count)
		for i := range messages {
			messages[i] = h.NewTaskMessageWithQueue("metrics:generated", nil, queue)
		}
		h.SeedPendingQueue(t, client, messages, queue)
		if paused {
			if err := collector.inspector.PauseQueue(queue); err != nil {
				t.Fatal(err)
			}
		}
		families := gathered(t, collector)
		size := metric(t, families, "asynq_queue_size", queue, "", dto.MetricType_GAUGE)
		pending := metric(t, families, "asynq_tasks_enqueued_total", queue, "pending", dto.MetricType_GAUGE)
		pausedValue := metric(t, families, "asynq_queue_paused_total", queue, "", dto.MetricType_GAUGE)
		wantPaused := 0.0
		if paused {
			wantPaused = 1
		}
		return size == float64(count) && pending == size && pausedValue == wantPaused
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 20, Rand: rand.New(rand.NewSource(20261001))}); err != nil {
		t.Fatal(err)
	}
}
