package dash

import (
	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
	"testing"
)

func TestTaskPaginationUsesSelectedStateCount(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 25)
	queue := &asynq.QueueInfo{Active: 1, Pending: 11, Scheduled: 2, Retry: 3, Archived: 4, Completed: 5, Aggregating: 6}
	for _, tc := range []struct {
		state asynq.TaskState
		want  int
	}{
		{asynq.TaskStateActive, 1}, {asynq.TaskStatePending, 11}, {asynq.TaskStateScheduled, 2}, {asynq.TaskStateRetry, 3}, {asynq.TaskStateArchived, 4}, {asynq.TaskStateCompleted, 5}, {asynq.TaskStateAggregating, 6},
	} {
		if got := getTaskCount(queue, tc.state); got != tc.want {
			t.Errorf("%s count=%d want=%d", tc.state, got, tc.want)
		}
	}
	state := &State{view: viewTypeQueueDetails, selectedQueue: queue, taskState: asynq.TaskStatePending, pageNum: 1, tasks: make([]*asynq.TaskInfo, 10)}
	if taskPageSize(screen) != 10 || groupPageSize(screen) != 9 {
		t.Fatal("screen rows no longer reserve title/footer space")
	}
	if !isNextTaskPageAvailable(screen, state) {
		t.Fatal("eleventh pending task must expose second page")
	}
	state.pageNum = 2
	state.tasks = make([]*asynq.TaskInfo, 1)
	if isNextTaskPageAvailable(screen, state) {
		t.Fatal("last page exposes a nonexistent page")
	}
	state.pageNum = 1
	state.taskState = asynq.TaskStateActive
	state.tasks = make([]*asynq.TaskInfo, 1)
	if isNextTaskPageAvailable(screen, state) {
		t.Fatal("pagination used pending count for active tasks")
	}
}

func TestPageNavigationBoundaries(t *testing.T) {
	state := &State{view: viewTypeQueueDetails, selectedQueue: &asynq.QueueInfo{Pending: 11}, taskState: asynq.TaskStatePending, pageNum: 1, tasks: make([]*asynq.TaskInfo, 10)}
	handler := makeKeyEventHandler(t, state)
	if err := handler.s.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(handler.s.Fini)
	handler.s.(tcell.SimulationScreen).SetSize(80, 25)
	handler.prevPage()
	if state.pageNum != 1 {
		t.Fatal("first page underflow")
	}
	handler.nextPage()
	if state.pageNum != 2 {
		t.Fatal("next task page not fetched")
	}
	state.tasks = make([]*asynq.TaskInfo, 1)
	handler.nextPage()
	if state.pageNum != 2 {
		t.Fatal("last page overflow")
	}
	handler.prevPage()
	if state.pageNum != 1 {
		t.Fatal("previous task page not fetched")
	}
	state.view = viewTypeQueues
	handler.nextPage()
	handler.prevPage()
	if state.pageNum != 1 {
		t.Fatal("queue view pagination changed task page")
	}
}
