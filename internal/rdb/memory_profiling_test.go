package rdb

import (
	"os"
	"testing"

	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
)

// Process-wide environment changes require these cases to run sequentially.
func TestCurrentStatsMemoryProfilingEnvironment(t *testing.T) {
	const key = "DISABLE_MEMORY_USAGE_PROFILING"
	r := setup(t)
	t.Cleanup(func() { _ = r.Close() })
	msg := h.NewTaskMessageBuilder().SetType("memory-profiling-regression").Build()
	h.SeedPendingQueue(t, r.client, []*base.TaskMessage{msg}, msg.Queue)

	for _, tc := range []struct {
		name     string
		value    string
		unset    bool
		disabled bool
	}{
		{name: "unset", unset: true},
		{name: "empty"},
		{name: "false", value: "false"},
		{name: "true", value: "true", disabled: true},
		{name: "zero", value: "0", disabled: true},
		{name: "capital_false", value: "False", disabled: true},
		{name: "other", value: "enabled", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.value)
			if tc.unset {
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			}
			stats, err := r.CurrentStats(msg.Queue)
			if err != nil {
				t.Fatalf("CurrentStats: %v", err)
			}
			if stats.Size != 1 || stats.Pending != 1 {
				t.Fatalf("queue counts changed: size=%d pending=%d", stats.Size, stats.Pending)
			}
			if tc.disabled && stats.MemoryUsage != 0 {
				t.Errorf("disabled profiling: MemoryUsage=%d, want 0", stats.MemoryUsage)
			}
			if !tc.disabled && stats.MemoryUsage <= 0 {
				t.Errorf("enabled profiling: MemoryUsage=%d, want positive bytes for seeded queue", stats.MemoryUsage)
			}
		})
	}
}
