package dash

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/austinyuch/asynq/internal/base"
	h "github.com/austinyuch/asynq/internal/testutil"
	"github.com/gdamore/tcell/v2"
	"github.com/redis/go-redis/v9"
)

type fetchContractFixture struct {
	client    *redis.Client
	inspector *asynq.Inspector
	queues    []string
	ids       map[asynq.TaskState][]string
	group     string
}

func newFetchContractFixture(t *testing.T) *fetchContractFixture {
	t.Helper()
	addr := os.Getenv("ASYNQ_DASH_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ASYNQ_DASH_TEST_REDIS_ADDR for exclusively owned DB12")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 12})
	f := &fetchContractFixture{client: client, inspector: asynq.NewInspectorFromRedisClient(client), ids: map[asynq.TaskState][]string{}, group: "owned-group"}
	prefix := fmt.Sprintf("dash-fetch-%d", time.Now().UnixNano())
	f.queues = []string{prefix + "-z", prefix + "-a"}
	t.Cleanup(func() {
		ctx := context.Background()
		for _, q := range f.queues {
			keys, e := client.Keys(ctx, base.QueueKeyPrefix(q)+"*").Result()
			if e != nil {
				t.Error(e)
			}
			if len(keys) > 0 {
				if e := client.Del(ctx, keys...).Err(); e != nil {
					t.Error(e)
				}
			}
			if e := client.SRem(ctx, base.AllQueues, q).Err(); e != nil {
				t.Error(e)
			}
		}
		_ = client.Close()
	})
	ctx := context.Background()
	if e := client.Ping(ctx).Err(); e != nil {
		t.Fatal(e)
	}
	for _, q := range f.queues {
		if e := client.SAdd(ctx, base.AllQueues, q).Err(); e != nil {
			t.Fatal(e)
		}
	}
	for _, state := range []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStatePending, asynq.TaskStateScheduled, asynq.TaskStateRetry, asynq.TaskStateArchived, asynq.TaskStateCompleted, asynq.TaskStateAggregating} {
		var msgs []*base.TaskMessage
		var entries []base.Z
		for n := 0; n < 5; n++ {
			msg := h.NewTaskMessageWithQueue("fetch-contract", []byte(fmt.Sprintf("payload-%s-%d", state, n)), f.queues[0])
			msg.GroupKey = f.group
			msgs = append(msgs, msg)
			entries = append(entries, base.Z{Message: msg, Score: 2000000000 + int64(n)})
			f.ids[state] = append(f.ids[state], msg.ID)
		}
		switch state {
		case asynq.TaskStateActive:
			h.SeedActiveQueue(t, client, msgs, f.queues[0])
		case asynq.TaskStatePending:
			h.SeedPendingQueue(t, client, msgs, f.queues[0])
		case asynq.TaskStateScheduled:
			h.SeedScheduledQueue(t, client, entries, f.queues[0])
		case asynq.TaskStateRetry:
			h.SeedRetryQueue(t, client, entries, f.queues[0])
		case asynq.TaskStateArchived:
			h.SeedArchivedQueue(t, client, entries, f.queues[0])
		case asynq.TaskStateCompleted:
			h.SeedCompletedQueue(t, client, entries, f.queues[0])
		case asynq.TaskStateAggregating:
			h.SeedGroup(t, client, entries, f.queues[0], f.group)
		}
	}
	return f
}
func fetchReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(time.Second):
		t.Fatal("fetch did not deliver within deadline")
		var zero T
		return zero
	}
}
func fetchWantTasks(t *testing.T, i *asynq.Inspector, q string, state asynq.TaskState, size, page int) []*asynq.TaskInfo {
	t.Helper()
	opts := []asynq.ListOption{asynq.PageSize(size), asynq.Page(page)}
	var result []*asynq.TaskInfo
	var e error
	switch state {
	case asynq.TaskStateActive:
		result, e = i.ListActiveTasks(q, opts...)
	case asynq.TaskStatePending:
		result, e = i.ListPendingTasks(q, opts...)
	case asynq.TaskStateScheduled:
		result, e = i.ListScheduledTasks(q, opts...)
	case asynq.TaskStateRetry:
		result, e = i.ListRetryTasks(q, opts...)
	case asynq.TaskStateArchived:
		result, e = i.ListArchivedTasks(q, opts...)
	case asynq.TaskStateCompleted:
		result, e = i.ListCompletedTasks(q, opts...)
	}
	if e != nil {
		t.Fatal(e)
	}
	return result
}
func fetchAssertTasks(t *testing.T, f *fetchContractFixture, state asynq.TaskState, got, want []*asynq.TaskInfo) {
	t.Helper()
	normalizedGot, normalizedWant := make([]asynq.TaskInfo, len(got)), make([]asynq.TaskInfo, len(want))
	for n, v := range got {
		normalizedGot[n] = *v
		if state == asynq.TaskStatePending {
			if time.Since(v.NextProcessAt) < 0 || time.Since(v.NextProcessAt) > time.Second {
				t.Fatalf("pending availability is not current: %v", v.NextProcessAt)
			}
			normalizedGot[n].NextProcessAt = time.Time{}
		}
	}
	for n, v := range want {
		normalizedWant[n] = *v
		if state == asynq.TaskStatePending {
			normalizedWant[n].NextProcessAt = time.Time{}
		}
	}
	if !reflect.DeepEqual(normalizedGot, normalizedWant) {
		t.Fatalf("fetch pagination/data differs from independent Inspector: got=%+v want=%+v", got, want)
	}
	for _, task := range got {
		seededIndex := -1
		for n, id := range f.ids[state] {
			if id == task.ID {
				seededIndex = n
				break
			}
		}
		if seededIndex < 0 || task.Type != "fetch-contract" || string(task.Payload) != fmt.Sprintf("payload-%s-%d", state, seededIndex) {
			t.Fatalf("fetch did not preserve independently seeded task identity/payload: %+v", task)
		}
		info, e := f.inspector.GetTaskInfo(f.queues[0], task.ID)
		if e != nil || info.State != state || !reflect.DeepEqual(info.Payload, task.Payload) {
			t.Fatalf("seeded ID/state/payload mismatch: %+v %v", info, e)
		}
	}
}
func TestDashboardFetchRuntimeContracts(t *testing.T) {
	f := newFetchContractFixture(t)
	before := f.snapshot(t)
	defer func() {
		if !reflect.DeepEqual(before, f.snapshot(t)) {
			t.Error("fetch changed owned fixture bytes")
		}
	}()
	errCh := make(chan error, 10)
	queuesCh := make(chan []*asynq.QueueInfo, 1)
	fetchQueues(f.inspector, queuesCh, errCh, Options{})
	queues := fetchReceive(t, queuesCh)
	names := []string{}
	for _, q := range queues {
		names = append(names, q.Queue)
	}
	wantNames := append([]string(nil), f.queues...)
	sort.Strings(wantNames)
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("queue ordering/scope: %v want %v", names, wantNames)
	}
	queueCh := make(chan *asynq.QueueInfo, 1)
	fetchQueueInfo(f.inspector, f.queues[0], queueCh, errCh)
	q := fetchReceive(t, queueCh)
	if q.Queue != f.queues[0] || q.Pending != 5 || q.Active != 5 || q.Scheduled != 5 || q.Retry != 5 || q.Archived != 5 || q.Completed != 5 || q.Aggregating != 5 {
		t.Fatalf("queue counters differ from canonical fixture: %+v", q)
	}
	rng := rand.New(rand.NewSource(20261005))
	states := []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStatePending, asynq.TaskStateScheduled, asynq.TaskStateRetry, asynq.TaskStateArchived, asynq.TaskStateCompleted}
	for _, state := range states {
		for n := 0; n < 8; n++ {
			size, page := 1+rng.Intn(4), 1+rng.Intn(4)
			tasksCh := make(chan []*asynq.TaskInfo, 1)
			fetchTasks(f.inspector, f.queues[0], state, size, page, tasksCh, errCh)
			got := fetchReceive(t, tasksCh)
			remaining := 5 - (page-1)*size
			expectedCount := remaining
			if expectedCount < 0 {
				expectedCount = 0
			}
			if expectedCount > size {
				expectedCount = size
			}
			if len(got) != expectedCount {
				t.Fatalf("seeded pagination cardinality: got=%d want=%d", len(got), expectedCount)
			}
			fetchAssertTasks(t, f, state, got, fetchWantTasks(t, f.inspector, f.queues[0], state, size, page))
		}
	}
	groupsCh := make(chan []*asynq.GroupInfo, 1)
	fetchGroups(f.inspector, f.queues[0], groupsCh, errCh)
	groups := fetchReceive(t, groupsCh)
	if len(groups) != 1 || groups[0].Group != f.group || groups[0].Size != 5 {
		t.Fatalf("group contract: %+v", groups)
	}
	tasksCh := make(chan []*asynq.TaskInfo, 1)
	fetchAggregatingTasks(f.inspector, f.queues[0], f.group, 2, 2, tasksCh, errCh)
	want, e := f.inspector.ListAggregatingTasks(f.queues[0], f.group, asynq.PageSize(2), asynq.Page(2))
	if e != nil {
		t.Fatal(e)
	}
	fetchAssertTasks(t, f, asynq.TaskStateAggregating, fetchReceive(t, tasksCh), want)
	taskCh := make(chan *asynq.TaskInfo, 1)
	id := f.ids[asynq.TaskStatePending][0]
	fetchTaskInfo(f.inspector, f.queues[0], id, taskCh, errCh)
	task := fetchReceive(t, taskCh)
	if task.ID != id || task.State != asynq.TaskStatePending || string(task.Payload) != "payload-pending-0" {
		t.Fatalf("modal task identity: %+v", task)
	}
	if len(errCh) != 0 {
		t.Fatalf("successful fetch emitted errors: %v", <-errCh)
	}
}

func TestDashboardFetchDispatchContracts(t *testing.T) {
	f := newFetchContractFixture(t)
	before := f.snapshot(t)
	defer func() {
		if !reflect.DeepEqual(before, f.snapshot(t)) {
			t.Error("fetch changed owned fixture bytes")
		}
	}()
	screen := tcell.NewSimulationScreen("")
	if e := screen.Init(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 17)
	errorCh := make(chan error, 8)
	queueCh := make(chan *asynq.QueueInfo, 8)
	taskCh := make(chan *asynq.TaskInfo, 8)
	queuesCh := make(chan []*asynq.QueueInfo, 8)
	groupsCh := make(chan []*asynq.GroupInfo, 8)
	tasksCh := make(chan []*asynq.TaskInfo, 8)
	fetcher := dataFetcher{inspector: f.inspector, s: screen, errorCh: errorCh, queueCh: queueCh, taskCh: taskCh, queuesCh: queuesCh, groupsCh: groupsCh, tasksCh: tasksCh}
	fetcher.Fetch(&State{view: viewTypeQueues})
	if len(fetchReceive(t, queuesCh)) != 2 {
		t.Fatal("queue overview lost queues")
	}
	queue, e := f.inspector.GetQueueInfo(f.queues[0])
	if e != nil {
		t.Fatal(e)
	}
	for _, state := range []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStatePending, asynq.TaskStateScheduled, asynq.TaskStateRetry, asynq.TaskStateArchived, asynq.TaskStateCompleted} {
		id := f.ids[state][0]
		fetcher.Fetch(&State{view: viewTypeQueueDetails, selectedQueue: queue, taskState: state, pageNum: 2, taskID: id})
		fetchAssertTasks(t, f, state, fetchReceive(t, tasksCh), fetchWantTasks(t, f.inspector, f.queues[0], state, 2, 2))
		if fetchReceive(t, queueCh).Queue != f.queues[0] {
			t.Fatal("wrong queue dispatch")
		}
		if task := fetchReceive(t, taskCh); task.ID != id || task.State != state {
			t.Fatalf("modal identity: %+v", task)
		}
	}
	fetcher.Fetch(&State{view: viewTypeQueueDetails, selectedQueue: queue, taskState: asynq.TaskStateAggregating})
	if groups := fetchReceive(t, groupsCh); len(groups) != 1 || groups[0].Group != f.group {
		t.Fatal("group dispatch")
	}
	_ = fetchReceive(t, queueCh)
	fetcher.Fetch(&State{view: viewTypeQueueDetails, selectedQueue: queue, taskState: asynq.TaskStateAggregating, selectedGroup: &asynq.GroupInfo{Group: f.group}, pageNum: 2})
	want, e := f.inspector.ListAggregatingTasks(f.queues[0], f.group, asynq.PageSize(2), asynq.Page(2))
	if e != nil {
		t.Fatal(e)
	}
	fetchAssertTasks(t, f, asynq.TaskStateAggregating, fetchReceive(t, tasksCh), want)
	_ = fetchReceive(t, queueCh)
	fetcher.Fetch(&State{view: viewTypeHelp})
	if len(errorCh)+len(queueCh)+len(taskCh)+len(queuesCh)+len(groupsCh)+len(tasksCh) != 0 {
		t.Fatal("unexpected or unconsumed dispatch")
	}
}

func TestDashboardFetchClosedTransportContracts(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	inspector := asynq.NewInspectorFromRedisClient(client)
	if e := client.Close(); e != nil {
		t.Fatal(e)
	}
	queuesCh := make(chan []*asynq.QueueInfo, 1)
	queueCh := make(chan *asynq.QueueInfo, 1)
	groupsCh := make(chan []*asynq.GroupInfo, 1)
	tasksCh := make(chan []*asynq.TaskInfo, 1)
	taskCh := make(chan *asynq.TaskInfo, 1)
	errCh := make(chan error, 1)
	type fetchCase struct {
		name     string
		run      func()
		expected func() error
	}
	cases := []fetchCase{
		{"queues", func() { fetchQueues(inspector, queuesCh, errCh, Options{}) }, func() error { _, e := inspector.Queues(); return e }},
		{"queue", func() { fetchQueueInfo(inspector, "owned", queueCh, errCh) }, func() error { _, e := inspector.GetQueueInfo("owned"); return e }},
		{"groups", func() { fetchGroups(inspector, "owned", groupsCh, errCh) }, func() error { _, e := inspector.Groups("owned"); return e }},
		{"aggregating", func() { fetchAggregatingTasks(inspector, "owned", "group", 2, 1, tasksCh, errCh) }, func() error {
			_, e := inspector.ListAggregatingTasks("owned", "group", asynq.PageSize(2), asynq.Page(1))
			return e
		}},
		{"task", func() { fetchTaskInfo(inspector, "owned", "id", taskCh, errCh) }, func() error { _, e := inspector.GetTaskInfo("owned", "id"); return e }},
	}
	for _, state := range []asynq.TaskState{asynq.TaskStateActive, asynq.TaskStatePending, asynq.TaskStateScheduled, asynq.TaskStateRetry, asynq.TaskStateArchived, asynq.TaskStateCompleted} {
		state := state
		cases = append(cases, fetchCase{state.String(), func() { fetchTasks(inspector, "owned", state, 2, 1, tasksCh, errCh) }, func() error {
			opts := []asynq.ListOption{asynq.PageSize(2), asynq.Page(1)}
			switch state {
			case asynq.TaskStateActive:
				_, e := inspector.ListActiveTasks("owned", opts...)
				return e
			case asynq.TaskStatePending:
				_, e := inspector.ListPendingTasks("owned", opts...)
				return e
			case asynq.TaskStateScheduled:
				_, e := inspector.ListScheduledTasks("owned", opts...)
				return e
			case asynq.TaskStateRetry:
				_, e := inspector.ListRetryTasks("owned", opts...)
				return e
			case asynq.TaskStateArchived:
				_, e := inspector.ListArchivedTasks("owned", opts...)
				return e
			default:
				_, e := inspector.ListCompletedTasks("owned", opts...)
				return e
			}
		}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.expected()
			if want == nil || !strings.Contains(want.Error(), redis.ErrClosed.Error()) {
				t.Fatalf("oracle not closed transport: %v", want)
			}
			tc.run()
			got := fetchReceive(t, errCh)
			if got == nil || reflect.TypeOf(got) != reflect.TypeOf(want) || got.Error() != want.Error() || errors.Is(got, redis.ErrClosed) != errors.Is(want, redis.ErrClosed) {
				t.Fatalf("transport changed: got=%v want=%v", got, want)
			}
			if len(queuesCh)+len(queueCh)+len(groupsCh)+len(tasksCh)+len(taskCh) != 0 {
				t.Fatal("failure published success")
			}
		})
	}
}

func (f *fetchContractFixture) snapshot(t *testing.T) map[string]string {
	t.Helper()
	ctx := context.Background()
	keys := []string{base.AllQueues}
	for _, q := range f.queues {
		owned, e := f.client.Keys(ctx, base.QueueKeyPrefix(q)+"*").Result()
		if e != nil {
			t.Fatal(e)
		}
		keys = append(keys, owned...)
	}
	result := map[string]string{}
	for _, key := range keys {
		v, e := f.client.Dump(ctx, key).Result()
		if e != nil {
			t.Fatal(e)
		}
		result[key] = v
	}
	return result
}
