package asynq

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// Nil persistence is a tripwire: invalid input must fail before touching storage.
func TestInspectorValidationContracts(t *testing.T) {
	i := &Inspector{}
	cases := []struct {
		name string
		call func(string) error
	}{
		{"GetQueueInfo", func(q string) error { _, e := i.GetQueueInfo(q); return e }},
		{"History", func(q string) error { _, e := i.History(q, 1); return e }},
		{"ListPendingTasks", func(q string) error { _, e := i.ListPendingTasks(q); return e }},
		{"ListActiveTasks", func(q string) error { _, e := i.ListActiveTasks(q); return e }},
		{"ListAggregatingTasks", func(q string) error { _, e := i.ListAggregatingTasks(q, "group"); return e }},
		{"ListScheduledTasks", func(q string) error { _, e := i.ListScheduledTasks(q); return e }},
		{"ListRetryTasks", func(q string) error { _, e := i.ListRetryTasks(q); return e }},
		{"ListArchivedTasks", func(q string) error { _, e := i.ListArchivedTasks(q); return e }},
		{"ListCompletedTasks", func(q string) error { _, e := i.ListCompletedTasks(q); return e }},
		{"DeleteAllPendingTasks", func(q string) error { _, e := i.DeleteAllPendingTasks(q); return e }},
		{"DeleteAllScheduledTasks", func(q string) error { _, e := i.DeleteAllScheduledTasks(q); return e }},
		{"DeleteAllRetryTasks", func(q string) error { _, e := i.DeleteAllRetryTasks(q); return e }},
		{"DeleteAllArchivedTasks", func(q string) error { _, e := i.DeleteAllArchivedTasks(q); return e }},
		{"DeleteAllCompletedTasks", func(q string) error { _, e := i.DeleteAllCompletedTasks(q); return e }},
		{"UpdateTaskPayload", func(q string) error { return i.UpdateTaskPayload(q, "id", []byte("payload")) }},
		{"DeleteTask", func(q string) error { return i.DeleteTask(q, "id") }},
		{"RunAllScheduledTasks", func(q string) error { _, e := i.RunAllScheduledTasks(q); return e }},
		{"RunAllRetryTasks", func(q string) error { _, e := i.RunAllRetryTasks(q); return e }},
		{"RunAllArchivedTasks", func(q string) error { _, e := i.RunAllArchivedTasks(q); return e }},
		{"RunTask", func(q string) error { return i.RunTask(q, "id") }},
		{"ArchiveAllPendingTasks", func(q string) error { _, e := i.ArchiveAllPendingTasks(q); return e }},
		{"ArchiveAllScheduledTasks", func(q string) error { _, e := i.ArchiveAllScheduledTasks(q); return e }},
		{"ArchiveAllRetryTasks", func(q string) error { _, e := i.ArchiveAllRetryTasks(q); return e }},
		{"ArchiveTask", func(q string) error { return i.ArchiveTask(q, "id") }},
	}
	rng := rand.New(rand.NewSource(20261006))
	invalid := []string{"", " ", "\t\n", "\u2003", "\u00a0"}
	alphabet := []rune{' ', '\t', '\n', '\r', '\u2003', '\u00a0'}
	for n := 0; n < 50; n++ {
		v := make([]rune, 1+rng.Intn(16))
		for k := range v {
			v[k] = alphabet[rng.Intn(len(alphabet))]
		}
		invalid = append(invalid, string(v))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, q := range invalid {
				if e := tc.call(q); e == nil || !strings.Contains(e.Error(), "queue name must contain one or more characters") {
					t.Fatalf("invalid queue %q: %v", q, e)
				}
			}
		})
	}
}
func TestInspectorPaginationClampContracts(t *testing.T) {
	rng := rand.New(rand.NewSource(20261007))
	for n := 0; n < 300; n++ {
		size, page := -1-rng.Intn(10000), -1-rng.Intn(10000)
		opts := []ListOption{PageSize(size), Page(page)}
		before := append([]ListOption(nil), opts...)
		got := composeListOptions(opts...)
		if got.pageSize != 0 || got.pageNum != 1 {
			t.Fatalf("negative pagination = %+v", got)
		}
		if !reflect.DeepEqual(opts, before) {
			t.Fatal("pagination mutated caller options")
		}
		positive := 1 + rng.Intn(10000)
		got = composeListOptions(PageSize(positive), Page(positive))
		if got.pageSize != positive || got.pageNum != positive {
			t.Fatalf("positive pagination = %+v", got)
		}
	}
	// Zero is accepted as supplied; only negative pages are documented to clamp.
	if got := composeListOptions(Page(0)); got.pageNum != 0 {
		t.Fatalf("zero page changed: %+v", got)
	}
}
