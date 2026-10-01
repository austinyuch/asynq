package asynq

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq/internal/base"
)

// Embed only the interface: unexpected persistence operations panic, instead of
// silently returning the success that the contract is intended to verify.
type aggregatorFailureBroker struct {
	base.Broker
	groups       map[string][]string
	stages       map[string]string
	messages     map[string][]*base.TaskMessage
	sets         map[string]bool
	deadline     time.Time
	calls        []string
	accepted     map[string]*base.TaskMessage
	contexts     []context.Context
	callContexts []aggregatorCallContext
}

type aggregatorCallContext struct {
	err         error
	deadline    time.Time
	hasDeadline bool
}

func (b *aggregatorFailureBroker) observeContext(ctx context.Context) {
	deadline, ok := ctx.Deadline()
	b.callContexts = append(b.callContexts, aggregatorCallContext{ctx.Err(), deadline, ok})
}

var aggregatorBoundaryError = errors.New("owned aggregation boundary unavailable")

func (b *aggregatorFailureBroker) ListGroups(q string) ([]string, error) {
	b.calls = append(b.calls, "list:"+q)
	if q == "bad-list" {
		return nil, aggregatorBoundaryError
	}
	return b.groups[q], nil
}
func (b *aggregatorFailureBroker) AggregationCheck(q, g string, at time.Time, grace, maxDelay time.Duration, size int) (string, error) {
	b.calls = append(b.calls, "check:"+g)
	if b.stages[g] == "check" {
		return "", aggregatorBoundaryError
	}
	if b.stages[g] == "not-ready" {
		return "", nil
	}
	return "set-" + g, nil
}
func (b *aggregatorFailureBroker) ReadAggregationSet(q, g, set string) ([]*base.TaskMessage, time.Time, error) {
	b.calls = append(b.calls, "read:"+g)
	if set != "set-"+g {
		panic("wrong set identity")
	}
	if b.stages[g] == "read" {
		return nil, time.Time{}, aggregatorBoundaryError
	}
	return b.messages[g], b.deadline, nil
}
func (b *aggregatorFailureBroker) Enqueue(ctx context.Context, msg *base.TaskMessage) error {
	g := msg.Headers["group"]
	b.calls = append(b.calls, "enqueue:"+g)
	b.contexts = append(b.contexts, ctx)
	b.observeContext(ctx)
	if b.stages[g] == "enqueue" {
		return aggregatorBoundaryError
	}
	copyMsg := *msg
	copyMsg.Payload = append([]byte(nil), msg.Payload...)
	copyMsg.Headers = map[string]string{}
	for k, v := range msg.Headers {
		copyMsg.Headers[k] = v
	}
	b.accepted[g] = &copyMsg
	return nil
}
func (b *aggregatorFailureBroker) DeleteAggregationSet(ctx context.Context, q, g, set string) error {
	b.calls = append(b.calls, "delete:"+g)
	b.contexts = append(b.contexts, ctx)
	b.observeContext(ctx)
	if set != "set-"+g {
		panic("delete wrong aggregation set")
	}
	if b.stages[g] == "delete" {
		return aggregatorBoundaryError
	}
	delete(b.sets, g)
	return nil
}

func TestAggregatorFailureIsolationProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(20261015))
	for trial := 0; trial < 64; trial++ {
		b := &aggregatorFailureBroker{groups: map[string][]string{}, stages: map[string]string{}, messages: map[string][]*base.TaskMessage{}, sets: map[string]bool{}, accepted: map[string]*base.TaskMessage{}, deadline: time.Now().Add(time.Hour)}
		stages := []string{"check", "not-ready", "read", "enqueue", "delete", "healthy"}
		rng.Shuffle(len(stages), func(i, j int) { stages[i], stages[j] = stages[j], stages[i] })
		stages = append(stages, "healthy-tail")
		for i, stage := range stages {
			g := fmt.Sprintf("group-%d-%s", i, stage)
			b.groups["mixed"] = append(b.groups["mixed"], g)
			b.stages[g] = stage
			b.sets[g] = true
			for n := 0; n < 1+rng.Intn(4); n++ {
				b.messages[g] = append(b.messages[g], &base.TaskMessage{ID: fmt.Sprintf("source-%s-%d", g, n), Type: "item:contract", Payload: []byte(fmt.Sprintf("payload/%s/%d", g, n)), Headers: map[string]string{"source": "preserved", "ordinal": fmt.Sprint(n)}, Queue: "mixed", GroupKey: g})
			}
		}
		tail := "healthy-next-queue"
		b.groups["next"] = []string{tail}
		b.stages[tail] = "healthy"
		b.sets[tail] = true
		b.messages[tail] = []*base.TaskMessage{{ID: "next-source", Type: "item:contract", Payload: []byte("next-queue-payload"), Headers: map[string]string{"source": "preserved"}, Queue: "next", GroupKey: tail}}
		original := make(map[string][]*base.TaskMessage)
		for g, msgs := range b.messages {
			for _, m := range msgs {
				copyMsg := *m
				copyMsg.Payload = append([]byte(nil), m.Payload...)
				copyMsg.Headers = map[string]string{}
				for k, v := range m.Headers {
					copyMsg.Headers[k] = v
				}
				original[g] = append(original[g], &copyMsg)
			}
		}
		aggregated := map[string]int{}
		a := newAggregator(aggregatorParams{logger: testLogger, broker: b, queues: []string{"bad-list", "mixed", "next"}, gracePeriod: time.Second, maxDelay: time.Minute, maxSize: 10, groupAggregator: GroupAggregatorFunc(func(g string, tasks []*Task) *Task {
			aggregated[g]++
			if len(tasks) != len(original[g]) {
				t.Fatalf("group %s dropped source tasks", g)
			}
			payloads := []string{}
			for i, task := range tasks {
				want := original[g][i]
				if task.Type() != want.Type || string(task.Payload()) != string(want.Payload) || !reflect.DeepEqual(task.Headers(), want.Headers) {
					t.Fatalf("group conversion lost type/payload/headers: %s", g)
				}
				task.Headers()["source"] = "callback-local"
				payloads = append(payloads, string(task.Payload()))
			}
			return NewTaskWithHeaders("aggregate:contract", []byte(strings.Join(payloads, "|")), map[string]string{"group": g, "result": "preserved"}, Queue("wrong-task-queue"), MaxRetry(7), TaskID("aggregate-"+g))
		})})
		a.sema <- struct{}{}
		a.aggregate(time.Now())
		if len(a.sema) != 0 {
			t.Fatal("aggregation failed to release semaphore")
		}
		if !reflect.DeepEqual(original, b.messages) {
			t.Fatal("aggregation or callback header changes mutated input messages")
		}
		expectedCalls := []string{"list:bad-list", "list:mixed"}
		ordered := append(append([]string(nil), b.groups["mixed"]...), tail)
		for _, g := range ordered {
			stage := b.stages[g]
			if g == tail {
				expectedCalls = append(expectedCalls, "list:next")
			}
			expectedCalls = append(expectedCalls, "check:"+g)
			if stage == "check" || stage == "not-ready" {
				continue
			}
			expectedCalls = append(expectedCalls, "read:"+g)
			if stage == "read" {
				continue
			}
			expectedCalls = append(expectedCalls, "enqueue:"+g)
			if stage == "enqueue" {
				continue
			}
			expectedCalls = append(expectedCalls, "delete:"+g)
		}
		if !reflect.DeepEqual(b.calls, expectedCalls) {
			t.Fatalf("failure crossed boundary or blocked healthy neighbor: got%v want%v", b.calls, expectedCalls)
		}
		for _, g := range ordered {
			stage := b.stages[g]
			wantGA := 0
			if stage != "check" && stage != "not-ready" && stage != "read" {
				wantGA = 1
			}
			if aggregated[g] != wantGA {
				t.Fatalf("aggregate callback count %s=%d want%d", g, aggregated[g], wantGA)
			}
			msg, accepted := b.accepted[g]
			wantAccepted := stage == "delete" || strings.HasPrefix(stage, "healthy")
			if accepted != wantAccepted {
				t.Fatalf("accepted %s=%v", g, accepted)
			}
			wantRetained := !strings.HasPrefix(stage, "healthy")
			if b.sets[g] != wantRetained {
				t.Fatalf("set retention %s=%v want%v", g, b.sets[g], wantRetained)
			}
			if accepted {
				queue := "mixed"
				if g == tail {
					queue = "next"
				}
				payloads := []string{}
				for _, m := range original[g] {
					payloads = append(payloads, string(m.Payload))
				}
				if msg.Type != "aggregate:contract" || msg.ID != "aggregate-"+g || msg.Queue != queue || msg.GroupKey != "" || msg.Retry != 7 || string(msg.Payload) != strings.Join(payloads, "|") || !reflect.DeepEqual(msg.Headers, map[string]string{"group": g, "result": "preserved"}) {
					t.Fatalf("real Client enqueue wire contract lost aggregation %s: %+v", g, msg)
				}
			}
		}
		for _, call := range b.callContexts {
			if call.err != nil || !call.hasDeadline || !call.deadline.Equal(b.deadline) {
				t.Fatalf("operation received canceled or incorrect-deadline context: err=%v deadline=%v present=%v", call.err, call.deadline, call.hasDeadline)
			}
		}
		for _, ctx := range b.contexts {
			deadline, ok := ctx.Deadline()
			if !ok || !deadline.Equal(b.deadline) || !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("request context deadline/cancel leaked: %v %v %v", deadline, ok, ctx.Err())
			}
		}
	}
}

func TestAggregatorSaturatedSemaphoreSkips(t *testing.T) {
	b := &aggregatorFailureBroker{}
	a := newAggregator(aggregatorParams{logger: testLogger, broker: b, queues: []string{"unexpected"}, gracePeriod: time.Second, groupAggregator: GroupAggregatorFunc(func(string, []*Task) *Task { t.Fatal("saturated aggregator callback ran"); return nil })})
	for i := 0; i < cap(a.sema); i++ {
		a.sema <- struct{}{}
	}
	for i := 0; i < 10; i++ {
		a.exec(time.Now())
	}
	if len(a.sema) != cap(a.sema) || len(b.calls) != 0 {
		t.Fatalf("saturated execution changed capacity or broker state: %d %v", len(a.sema), b.calls)
	}
}
