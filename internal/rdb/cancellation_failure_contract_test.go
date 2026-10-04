package rdb

import (
	"context"
	stderrors "errors"
	"math/rand"
	"testing"

	"github.com/austinyuch/asynq/internal/base"
	internalerrors "github.com/austinyuch/asynq/internal/errors"
	"github.com/redis/go-redis/v9"
)

// A closed real client rejects Publish before any socket is opened. The direct
// command supplies the independent cause oracle; callers must retain it.
func requireCancellationClosedCause(t testing.TB, id string) {
	t.Helper()
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	oracle := c.Publish(context.Background(), base.CancelChannel, id).Err()
	if !stderrors.Is(oracle, redis.ErrClosed) {
		t.Fatalf("closed client oracle: %v", oracle)
	}
	err := NewRDB(c).PublishCancelation(id)
	if !stderrors.Is(err, oracle) {
		t.Fatalf("lost publish cause: got=%T %v oracle=%v", err, err, oracle)
	}
	var canonical *internalerrors.Error
	if !stderrors.As(err, &canonical) || canonical.Code != internalerrors.Unknown || canonical.Op != "rdb.PublishCancelation" {
		t.Fatalf("canonical error contract: %v", err)
	}
	wantText := internalerrors.Unknown.String() + ": redis pubsub publish error: " + oracle.Error()
	if err.Error() != wantText {
		t.Fatalf("publish diagnostic: %v", err)
	}
}

func TestCancellationClosedCauseProperty(t *testing.T) {
	r := rand.New(rand.NewSource(20261024))
	for n := 0; n < 128; n++ {
		b := make([]byte, r.Intn(64))
		_, _ = r.Read(b)
		requireCancellationClosedCause(t, string(b))
	}
}

func FuzzCancellationClosedCause(f *testing.F) {
	for _, id := range []string{"", "owned-task", "\x00\xff", "任務"} {
		f.Add(id)
	}
	f.Fuzz(func(t *testing.T, id string) {
		if len(id) > 4096 {
			t.Skip("bounded task-ID input")
		}
		requireCancellationClosedCause(t, id)
	})
}
