package rate

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	asynqcontext "github.com/austinyuch/asynq/internal/context"
	"github.com/redis/go-redis/v9"
)

// Release only needs task metadata. A task's unavailable transport must retain
// the original Redis cause even when the valid context has no lease deadline.
func semaphoreReleaseClosedContract(t *testing.T, id string, withoutDeadline bool) {
	t.Helper()
	sema := NewSemaphore(asynq.RedisClientOpt{Addr: "127.0.0.1:1"}, "release-owned-closed-transport", 1)
	t.Cleanup(func() { _ = sema.Close() })
	client, ok := sema.rc.(*redis.Client)
	if !ok {
		t.Fatalf("expected real standalone client, got %T", sema.rc)
	}
	if err := sema.Close(); err != nil {
		t.Fatalf("close real lazy client: %v", err)
	}
	message := &base.TaskMessage{ID: id, Queue: "release-owned-queue"}
	ctx, cancel := asynqcontext.New(context.Background(), message, time.Now().Add(time.Hour))
	t.Cleanup(cancel)
	if withoutDeadline {
		ctx = context.WithoutCancel(ctx)
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("metadata fixture unexpectedly has deadline")
		}
	}
	gotID, ok := asynqcontext.GetTaskID(ctx)
	if !ok || gotID != id || ctx.Err() != nil {
		t.Fatalf("valid task fixture lost identity: %q %v %v", gotID, ok, ctx.Err())
	}
	// The independently observed transport cause supplies the error oracle.
	transportErr := client.ZRem(ctx, "independent-closed-client-oracle", id).Err()
	if !errors.Is(transportErr, redis.ErrClosed) {
		t.Fatalf("real closed client cause: %v", transportErr)
	}
	err := sema.Release(ctx)
	if !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("Release must preserve closed transport identity: task%q got%v", id, err)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		t.Fatalf("transport failure replaced by fabricated lease failure: %v", err)
	}
	afterID, afterOK := asynqcontext.GetTaskID(ctx)
	if afterID != id || !afterOK || message.ID != id || ctx.Err() != nil {
		t.Fatalf("Release changed task/context input: %q %v %+v", afterID, ctx.Err(), message)
	}
}
func TestSemaphoreReleaseClosedTransportContract(t *testing.T) {
	for _, noDeadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("without-deadline=%v", noDeadline), func(t *testing.T) { semaphoreReleaseClosedContract(t, "release-task-contract", noDeadline) })
	}
}
func TestSemaphoreReleaseClosedTransportSeededProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(20261023))
	for n := 0; n < 128; n++ {
		bytes := make([]byte, 16)
		rng.Read(bytes)
		t.Run(fmt.Sprintf("task-%03d", n), func(t *testing.T) { semaphoreReleaseClosedContract(t, fmt.Sprintf("release-%x", bytes), n%2 == 0) })
	}
}
func FuzzSemaphoreReleaseClosedTransportIdentity(f *testing.F) {
	f.Add([]byte("task-one"), true)
	f.Add([]byte{0, 1, 255}, false)
	f.Fuzz(func(t *testing.T, task []byte, withoutDeadline bool) {
		if len(task) > 256 {
			task = task[:256]
		}
		semaphoreReleaseClosedContract(t, fmt.Sprintf("release-%x", task), withoutDeadline)
	})
}
