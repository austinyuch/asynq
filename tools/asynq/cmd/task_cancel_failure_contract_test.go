package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	canonical "github.com/austinyuch/asynq/internal/errors"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// Redis PubSub is service-wide, not database-isolated. These tests require the
// dedicated endpoint's other publisher/observer suites to have terminated.
func cancelFailureSubscriber(t *testing.T, f *administrationFixture) *redis.PubSub {
	t.Helper()
	sub := f.client.Subscribe(context.Background(), base.CancelChannel)
	t.Cleanup(func() {
		if err := sub.Close(); err != nil {
			t.Errorf("close owned cancellation subscriber: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscription handshake: %v", err)
	}
	return sub
}
func cancelFailureRequirePublishedSequence(t *testing.T, f *administrationFixture, sub *redis.PubSub, ids []string, marker string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// The marker is a positive delivery barrier from an independent, already-warm
	// healthy connection. An absent failed-ID assertion never relies on a sleep.
	if err := f.client.Publish(ctx, base.CancelChannel, marker).Err(); err != nil {
		t.Fatal(err)
	}
	want := append(append([]string{}, ids...), marker)
	for n, id := range want {
		m, err := sub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatalf("publication %d missing: %v", n, err)
		}
		if m.Channel != base.CancelChannel || m.Payload != id {
			t.Fatalf("publication %d actual=(%q,%q) want=(%q,%q)", n, m.Channel, m.Payload, base.CancelChannel, id)
		}
	}
}
func cancelFailureRequireOwnership(t *testing.T, f *administrationFixture, before map[string]bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		after := queueStatsConnections(t, f)
		var extra []string
		for id := range after {
			if !before[id] {
				extra = append(extra, id)
			}
		}
		if len(extra) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("cancel command retained newly owned DB12 connection IDs: %v", extra)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func TestTaskCancelFailureAuthenticationNoPublication(t *testing.T) {
	f := taskFailureFixture(t)
	sub := cancelFailureSubscriber(t, f)
	beforeBytes := taskFailureSnapshot(t, f)
	beforeClients := queueStatsConnections(t, f)
	ids := []string{f.queue + "-auth-failed-first", f.queue + "-auth-failed-second"}
	viper.Set("username", "owned-cancel-invalid-user")
	viper.Set("password", "owned-cancel-invalid-password")
	// Released root formats the underlying Redis cause as a string. Compare its
	// public canonical API error; do not demand unavailable redis.Error identity.
	independent := asynq.NewInspector(getRedisConnOpt())
	t.Cleanup(func() {
		if e := independent.Close(); e != nil {
			t.Errorf("close independent AUTH inspector: %v", e)
		}
	})
	oracle := independent.CancelProcessing(ids[0])
	if oracle == nil || canonical.CanonicalCode(oracle) != canonical.Unknown || !strings.Contains(strings.ToLower(oracle.Error()), "wrongpass") {
		t.Fatalf("independent real AUTH oracle: %v", oracle)
	}
	out, err := invokeAdministration(t, taskCancelCmd, taskCancel, ids, nil)
	wantOut := strings.Repeat(fmt.Sprintf("error: could not send cancelation signal: %v\n", oracle), len(ids))
	if err == nil || canonical.CanonicalCode(err) != canonical.Unknown || err.Error() != oracle.Error() || out != wantOut {
		t.Fatalf("cancel AUTH status/output: got=%v out=%q want=%v out=%q", err, out, oracle, wantOut)
	}
	cancelFailureRequirePublishedSequence(t, f, sub, nil, f.queue+"-auth-delivery-barrier")
	cancelFailureRequireOwnership(t, f, beforeClients)
	taskFailureRequireUnchanged(t, f, beforeBytes)
}
func TestTaskCancelFailureSuccessSequenceProperty(t *testing.T) {
	f := taskFailureFixture(t)
	sub := cancelFailureSubscriber(t, f)
	beforeBytes := taskFailureSnapshot(t, f)
	rng := rand.New(rand.NewSource(20261024))
	for n := 0; n < 16; n++ {
		count := 1 + rng.Intn(5)
		ids := make([]string, count)
		var expected strings.Builder
		for j := range ids {
			ids[j] = fmt.Sprintf("%s-owned-cancel-%d-%d", f.queue, n, rng.Intn(3))
			fmt.Fprintf(&expected, "Sent cancelation signal for task %s\n", ids[j])
		}
		// Repeated IDs deliberately exercise exact multiplicity and ordering. These
		// are best-effort signals, not evidence that a nonexistent task was canceled.
		beforeClients := queueStatsConnections(t, f)
		out, err := invokeAdministration(t, taskCancelCmd, taskCancel, ids, nil)
		if err != nil || out != expected.String() {
			t.Fatalf("case%d cancel success: err=%v out=%q want=%q", n, err, out, expected.String())
		}
		cancelFailureRequirePublishedSequence(t, f, sub, ids, fmt.Sprintf("%s-case-%d-delivery-barrier", f.queue, n))
		cancelFailureRequireOwnership(t, f, beforeClients)
		taskFailureRequireUnchanged(t, f, beforeBytes)
	}
}

// No fabricated fuzz target: the assertions require live protocol observations.
