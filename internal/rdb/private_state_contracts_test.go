package rdb

import (
	"math/rand"
	"testing"

	"github.com/austinyuch/asynq/internal/base"
)

type unsupportedListStateCase struct {
	state base.TaskState
	want  string
}

// These are known valid task states. No arbitrary TaskState value reaches String.
var unsupportedListStateCases = []unsupportedListStateCase{
	{base.TaskStateScheduled, "unsupported task state: scheduled"},
	{base.TaskStateRetry, "unsupported task state: retry"},
	{base.TaskStateArchived, "unsupported task state: archived"},
	{base.TaskStateCompleted, "unsupported task state: completed"},
	{base.TaskStateAggregating, "unsupported task state: aggregating"},
}

func assertUnsupportedListState(tb testing.TB, c unsupportedListStateCase, page, size int) {
	tb.Helper()
	// A nil client is intentional: the exact string panic must precede Redis I/O.
	// Merely recovering a nil-pointer panic would fail the type/string assertions.
	completed := false
	defer func() {
		got := recover()
		if completed {
			tb.Fatal("unsupported list state returned instead of panicking")
		}
		message, ok := got.(string)
		if !ok {
			tb.Fatalf("unsupported list state panic type = %T; want string", got)
		}
		if message != c.want {
			tb.Fatalf("unsupported list state panic = %q; want %q", message, c.want)
		}
	}()
	_, _ = (&RDB{}).listMessages("private-state-contract", c.state, Pagination{Page: page, Size: size})
	completed = true
}

func TestPrivateListMessagesUnsupportedState(t *testing.T) {
	for _, c := range unsupportedListStateCases {
		t.Run(c.want, func(t *testing.T) { assertUnsupportedListState(t, c, 0, 20) })
	}
}

func TestPrivateListMessagesUnsupportedStateProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(20261004))
	// The deterministic cycle gives each state exactly twenty varied pagination cases.
	for i := 0; i < 100; i++ {
		page, size := int(rng.Int31())-(1<<30), int(rng.Int31())-(1<<30)
		assertUnsupportedListState(t, unsupportedListStateCases[i%len(unsupportedListStateCases)], page, size)
	}
}

func FuzzPrivateListMessagesUnsupportedState(f *testing.F) {
	for i := range unsupportedListStateCases {
		f.Add(uint8(i), 0, 20)
	}
	f.Add(uint8(255), -1, 0)
	f.Fuzz(func(t *testing.T, selector uint8, page, size int) {
		assertUnsupportedListState(t, unsupportedListStateCases[int(selector)%len(unsupportedListStateCases)], page, size)
	})
}
