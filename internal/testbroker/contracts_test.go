package testbroker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/redis/go-redis/v9"
)

// A spy implements the Broker delegation boundary, not a replacement Redis
// state machine. Every result and error has a distinct, observable identity.
type brokerContractCall struct {
	name string
	args []interface{}
}
type brokerContractSpy struct {
	base.Broker
	mu      sync.Mutex
	calls   []brokerContractCall
	failure error
	message *base.TaskMessage
	when    time.Time
	tasks   []*base.TaskMessage
	count   int
	pubsub  *redis.PubSub
	groups  []string
	setID   string
}

func newBrokerContractSpy() *brokerContractSpy {
	message := h.NewTaskMessageWithQueue("contract", []byte{0, 255}, "contract-queue")
	return &brokerContractSpy{failure: errors.New("downstream failure"), message: message, when: time.Unix(12345, 678).UTC(), tasks: []*base.TaskMessage{message}, count: 17, pubsub: &redis.PubSub{}, groups: []string{"g1", "g2"}, setID: "aggregation-set"}
}
func (s *brokerContractSpy) capture(name string, args ...interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, brokerContractCall{name, args})
}
func (s *brokerContractSpy) countCalls() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.calls) }
func (s *brokerContractSpy) Enqueue(ctx context.Context, msg *base.TaskMessage) error {
	s.capture("Enqueue", ctx, msg)
	return s.failure
}
func (s *brokerContractSpy) EnqueueUnique(ctx context.Context, msg *base.TaskMessage, ttl time.Duration) error {
	s.capture("EnqueueUnique", ctx, msg, ttl)
	return s.failure
}
func (s *brokerContractSpy) BatchEnqueue(ctx context.Context, items []base.BatchEnqueueItem) (int, error) {
	s.capture("BatchEnqueue", ctx, items)
	return s.count, s.failure
}
func (s *brokerContractSpy) Dequeue(qnames ...string) (*base.TaskMessage, time.Time, error) {
	s.capture("Dequeue", qnames)
	return s.message, s.when, s.failure
}
func (s *brokerContractSpy) Done(ctx context.Context, msg *base.TaskMessage) error {
	s.capture("Done", ctx, msg)
	return s.failure
}
func (s *brokerContractSpy) MarkAsComplete(ctx context.Context, msg *base.TaskMessage) error {
	s.capture("MarkAsComplete", ctx, msg)
	return s.failure
}
func (s *brokerContractSpy) Requeue(ctx context.Context, msg *base.TaskMessage) error {
	s.capture("Requeue", ctx, msg)
	return s.failure
}
func (s *brokerContractSpy) Schedule(ctx context.Context, msg *base.TaskMessage, processAt time.Time) error {
	s.capture("Schedule", ctx, msg, processAt)
	return s.failure
}
func (s *brokerContractSpy) ScheduleUnique(ctx context.Context, msg *base.TaskMessage, processAt time.Time, ttl time.Duration) error {
	s.capture("ScheduleUnique", ctx, msg, processAt, ttl)
	return s.failure
}
func (s *brokerContractSpy) Retry(ctx context.Context, msg *base.TaskMessage, processAt time.Time, errMsg string, isFailure bool) error {
	s.capture("Retry", ctx, msg, processAt, errMsg, isFailure)
	return s.failure
}
func (s *brokerContractSpy) Archive(ctx context.Context, msg *base.TaskMessage, errMsg string) error {
	s.capture("Archive", ctx, msg, errMsg)
	return s.failure
}
func (s *brokerContractSpy) ForwardIfReady(qnames ...string) error {
	s.capture("ForwardIfReady", qnames)
	return s.failure
}
func (s *brokerContractSpy) DeleteExpiredCompletedTasks(qname string, batchSize int) error {
	s.capture("DeleteExpiredCompletedTasks", qname, batchSize)
	return s.failure
}
func (s *brokerContractSpy) ListLeaseExpired(cutoff time.Time, qnames ...string) ([]*base.TaskMessage, error) {
	s.capture("ListLeaseExpired", cutoff, qnames)
	return s.tasks, s.failure
}
func (s *brokerContractSpy) ExtendLease(qname string, ids ...string) (time.Time, error) {
	s.capture("ExtendLease", qname, ids)
	return s.when, s.failure
}
func (s *brokerContractSpy) WriteServerState(info *base.ServerInfo, workers []*base.WorkerInfo, ttl time.Duration) error {
	s.capture("WriteServerState", info, workers, ttl)
	return s.failure
}
func (s *brokerContractSpy) ClearServerState(host string, pid int, serverID string) error {
	s.capture("ClearServerState", host, pid, serverID)
	return s.failure
}
func (s *brokerContractSpy) CancelationPubSub() (*redis.PubSub, error) {
	s.capture("CancelationPubSub")
	return s.pubsub, s.failure
}
func (s *brokerContractSpy) PublishCancelation(id string) error {
	s.capture("PublishCancelation", id)
	return s.failure
}
func (s *brokerContractSpy) WriteResult(qname, id string, data []byte) (int, error) {
	s.capture("WriteResult", qname, id, data)
	return s.count, s.failure
}
func (s *brokerContractSpy) Ping() error  { s.capture("Ping"); return s.failure }
func (s *brokerContractSpy) Close() error { s.capture("Close"); return s.failure }
func (s *brokerContractSpy) AddToGroup(ctx context.Context, msg *base.TaskMessage, gname string) error {
	s.capture("AddToGroup", ctx, msg, gname)
	return s.failure
}
func (s *brokerContractSpy) AddToGroupUnique(ctx context.Context, msg *base.TaskMessage, gname string, ttl time.Duration) error {
	s.capture("AddToGroupUnique", ctx, msg, gname, ttl)
	return s.failure
}
func (s *brokerContractSpy) ListGroups(qname string) ([]string, error) {
	s.capture("ListGroups", qname)
	return s.groups, s.failure
}
func (s *brokerContractSpy) AggregationCheck(qname, gname string, t time.Time, gracePeriod, maxDelay time.Duration, maxSize int) (aggregationSetID string, err error) {
	s.capture("AggregationCheck", qname, gname, t, gracePeriod, maxDelay, maxSize)
	return s.setID, s.failure
}
func (s *brokerContractSpy) ReadAggregationSet(qname, gname, aggregationSetID string) ([]*base.TaskMessage, time.Time, error) {
	s.capture("ReadAggregationSet", qname, gname, aggregationSetID)
	return s.tasks, s.when, s.failure
}
func (s *brokerContractSpy) DeleteAggregationSet(ctx context.Context, qname, gname, aggregationSetID string) error {
	s.capture("DeleteAggregationSet", ctx, qname, gname, aggregationSetID)
	return s.failure
}
func (s *brokerContractSpy) ReclaimStaleAggregationSets(qname string) error {
	s.capture("ReclaimStaleAggregationSets", qname)
	return s.failure
}

var brokerContractMethods = []string{"Enqueue", "EnqueueUnique", "BatchEnqueue", "Dequeue", "Done", "MarkAsComplete", "Requeue", "Schedule", "ScheduleUnique", "Retry", "Archive", "ForwardIfReady", "DeleteExpiredCompletedTasks", "ListLeaseExpired", "ExtendLease", "WriteServerState", "ClearServerState", "CancelationPubSub", "PublishCancelation", "WriteResult", "Ping", "Close", "AddToGroup", "AddToGroupUnique", "ListGroups", "AggregationCheck", "ReadAggregationSet", "DeleteAggregationSet", "ReclaimStaleAggregationSets"}

func brokerContractValue(typ reflect.Type, index int, spy *brokerContractSpy) reflect.Value {
	switch typ {
	case reflect.TypeOf((*context.Context)(nil)).Elem():
		return reflect.ValueOf(context.WithValue(context.Background(), struct{}{}, "request-token"))
	case reflect.TypeOf((*base.TaskMessage)(nil)):
		return reflect.ValueOf(spy.message)
	case reflect.TypeOf(time.Time{}):
		return reflect.ValueOf(spy.when)
	case reflect.TypeOf(time.Duration(0)):
		return reflect.ValueOf(time.Duration(index+1) * time.Second)
	case reflect.TypeOf([]string{}):
		return reflect.ValueOf([]string{"queue-a", "queue-b"})
	case reflect.TypeOf([]byte{}):
		return reflect.ValueOf([]byte{0, 1, 255})
	case reflect.TypeOf([]base.BatchEnqueueItem{}):
		return reflect.ValueOf([]base.BatchEnqueueItem{{Msg: spy.message, ProcessAt: spy.when}})
	case reflect.TypeOf((*base.ServerInfo)(nil)):
		return reflect.ValueOf(&base.ServerInfo{Host: "host", PID: 123, ServerID: "server", Concurrency: 4})
	case reflect.TypeOf([]*base.WorkerInfo{}):
		return reflect.ValueOf([]*base.WorkerInfo{{ID: "worker", Type: "contract", Payload: []byte{1, 2}}})
	}
	switch typ.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("argument-%d", index))
	case reflect.Int:
		return reflect.ValueOf(index + 3)
	case reflect.Bool:
		return reflect.ValueOf(true)
	}
	panic("unhandled Broker argument type " + typ.String())
}
func brokerContractInvoke(method reflect.Value, args []reflect.Value) []reflect.Value {
	if method.Type().IsVariadic() {
		return method.CallSlice(args)
	}
	return method.Call(args)
}

func TestBrokerAllAPIFailureAndDelegationContracts(t *testing.T) {
	// The base interface is the primary API inventory; additions must acquire a
	// contract rather than silently disappearing from this failure simulator.
	brokerInterface := reflect.TypeOf((*base.Broker)(nil)).Elem()
	if brokerInterface.NumMethod() != len(brokerContractMethods) {
		t.Fatalf("Broker inventory changed: %d vs %d", brokerInterface.NumMethod(), len(brokerContractMethods))
	}
	for _, name := range brokerContractMethods {
		t.Run(name, func(t *testing.T) {
			spy := newBrokerContractSpy()
			awake := NewTestBroker(spy)
			method := reflect.ValueOf(awake).MethodByName(name)
			var args []reflect.Value
			var wantedArgs []interface{}
			for i := 0; i < method.Type().NumIn(); i++ {
				v := brokerContractValue(method.Type().In(i), i, spy)
				args = append(args, v)
				wantedArgs = append(wantedArgs, v.Interface())
			}
			blocked := NewTestBroker(nil)
			blocked.Sleep()
			blocked.Sleep()
			results := brokerContractInvoke(reflect.ValueOf(blocked).MethodByName(name), args)
			for i, result := range results {
				if i == len(results)-1 {
					if result.Interface() != errRedisDown {
						t.Fatalf("sleeping error=%v", result.Interface())
					}
				} else if !result.IsZero() {
					t.Fatalf("sleeping result[%d]=%v", i, result.Interface())
				}
			}
			// A sleeping broker with a spy must also leave the underlying broker untouched.
			awake.Sleep()
			brokerContractInvoke(method, args)
			if spy.countCalls() != 0 {
				t.Fatal("sleeping delegated")
			}
			awake.Wakeup()
			awake.Wakeup()
			// Invoke the spy directly once to obtain its independently chosen return
			// identities, then clear only the observation log before exercising wrapper.
			wanted := brokerContractInvoke(reflect.ValueOf(spy).MethodByName(name), args)
			spy.mu.Lock()
			spy.calls = nil
			spy.mu.Unlock()
			got := brokerContractInvoke(method, args)
			if spy.countCalls() != 1 {
				t.Fatal("awake did not delegate exactly once")
			}
			spy.mu.Lock()
			call := spy.calls[0]
			spy.mu.Unlock()
			if call.name != name || !reflect.DeepEqual(call.args, wantedArgs) {
				t.Fatalf("arguments changed: %+v want=%v", call, wantedArgs)
			}
			for i, argument := range call.args {
				value := reflect.ValueOf(argument)
				if (value.Kind() == reflect.Ptr || value.Kind() == reflect.Slice) && value.Pointer() != args[i].Pointer() {
					t.Fatalf("argument[%d] identity changed", i)
				}
			}
			for i, result := range got {
				if !reflect.DeepEqual(result.Interface(), wanted[i].Interface()) {
					t.Fatalf("result[%d] changed", i)
				}
				if (result.Kind() == reflect.Ptr || result.Kind() == reflect.Slice) && result.Pointer() != wanted[i].Pointer() {
					t.Fatalf("result[%d] pointer identity changed", i)
				}
			}
			if got[len(got)-1].Interface() != spy.failure {
				t.Fatal("downstream error identity changed")
			}
			// Successful downstream operations retain the same nonzero return
			// values and must not acquire an outage error after recovery.
			spy.failure = nil
			before := spy.countCalls()
			success := brokerContractInvoke(method, args)
			if success[len(success)-1].Interface() != nil || spy.countCalls() != before+1 {
				t.Fatal("successful downstream operation altered after recovery")
			}
			for i := 0; i < len(success)-1; i++ {
				if !reflect.DeepEqual(success[i].Interface(), wanted[i].Interface()) {
					t.Fatalf("successful result[%d] changed", i)
				}
			}
		})
	}
}

func brokerStateSequence(ops []byte) bool {
	spy := newBrokerContractSpy()
	b := NewTestBroker(spy)
	sleeping := false
	calls := 0
	for _, op := range ops {
		switch op % 4 {
		case 0:
			b.Sleep()
			sleeping = true
		case 1:
			b.Wakeup()
			sleeping = false
		case 2:
			err := b.Ping()
			if sleeping {
				if err != errRedisDown {
					return false
				}
			} else {
				calls++
				if err != spy.failure {
					return false
				}
			}
		case 3:
			count, err := b.WriteResult("queue", "id", []byte{op})
			if sleeping {
				if count != 0 || err != errRedisDown {
					return false
				}
			} else {
				calls++
				if count != spy.count || err != spy.failure {
					return false
				}
			}
		}
		if spy.countCalls() != calls {
			return false
		}
	}
	return true
}
func TestBrokerStateSequenceProperty(t *testing.T) {
	if err := quick.Check(brokerStateSequence, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261001))}); err != nil {
		t.Fatal(err)
	}
}
func FuzzBrokerStateSequence(f *testing.F) {
	f.Add([]byte{2, 0, 2, 3, 1, 3})
	f.Add([]byte{0, 0, 1, 1, 2, 3})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 4096 {
			t.Skip("bounded transition trace")
		}
		if !brokerStateSequence(ops) {
			t.Fatal("outage transition/delegation invariant violated")
		}
	})
}
func TestBrokerConcurrentOutageTransitions(t *testing.T) {
	spy := newBrokerContractSpy()
	b := NewTestBroker(spy)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				b.Sleep()
				err := b.Ping()
				if err != errRedisDown && err != spy.failure {
					t.Errorf("invalid concurrent ping error: %v", err)
				}
				b.Wakeup()
			}
		}()
	}
	wg.Wait()
	b.Wakeup()
	before := spy.countCalls()
	if b.Ping() != spy.failure || spy.countCalls() != before+1 {
		t.Fatal("final recovery did not delegate")
	}
}
