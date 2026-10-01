package dash

import (
	"context"
	"fmt"
	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

func groupNavigationFixture(t *testing.T, height, count int) (*keyEventHandler, *State) {
	t.Helper()
	screen := renderingScreen(t, 100, height)
	state := &State{view: viewTypeQueueDetails, taskState: asynq.TaskStateAggregating, pageNum: 1, selectedQueue: &asynq.QueueInfo{Queue: "owned-groups", Aggregating: count}}
	for i := 0; i < count; i++ {
		state.groups = append(state.groups, &asynq.GroupInfo{Group: fmt.Sprintf("group-%03d", i), Size: 1})
	}
	ticker := time.NewTicker(time.Hour)
	t.Cleanup(ticker.Stop)
	return &keyEventHandler{s: screen, state: state, done: make(chan struct{}), fetcher: &fakeFetcher{}, drawer: &dashDrawer{s: screen}, ticker: ticker, pollInterval: time.Hour}, state
}
func groupNavigationKey(h *keyEventHandler, key tcell.Key, r rune) {
	h.HandleKeyEvent(tcell.NewEventKey(key, r, tcell.ModNone))
}
func groupNavigationNoPanic(t *testing.T) {
	t.Helper()
	if v := recover(); v != nil {
		t.Fatalf("public group navigation/render panicked: %v", v)
	}
}
func TestGroupNavigationRegression(t *testing.T) {
	t.Run("page-two-visible-identity", func(t *testing.T) {
		defer groupNavigationNoPanic(t)
		h, s := groupNavigationFixture(t, 25, 11)
		groupNavigationKey(h, tcell.KeyRune, 'n')
		groupNavigationKey(h, tcell.KeyDown, 0)
		text := renderingText(h.s)
		if !strings.Contains(text, "group-009") || strings.Contains(text, "group-000") {
			t.Fatalf("page-two screen wrong: %s", text)
		}
		groupNavigationKey(h, tcell.KeyEnter, 0)
		if s.selectedGroup != s.groups[9] {
			t.Fatalf("Enter chose%v instead of displayed group-009", s.selectedGroup)
		}
	})
	t.Run("short-page-up-bounds", func(t *testing.T) {
		defer groupNavigationNoPanic(t)
		h, s := groupNavigationFixture(t, 25, 2)
		groupNavigationKey(h, tcell.KeyUp, 0)
		if s.groupTableRowIdx != 2 {
			t.Fatalf("Up from header selected row%d want2", s.groupTableRowIdx)
		}
		groupNavigationKey(h, tcell.KeyEnter, 0)
		if s.selectedGroup != s.groups[1] {
			t.Fatal("Up/Enter did not choose last visible group")
		}
	})
	t.Run("exact-full-no-empty-next", func(t *testing.T) {
		defer groupNavigationNoPanic(t)
		h, s := groupNavigationFixture(t, 25, 9)
		groupNavigationKey(h, tcell.KeyRune, 'n')
		if s.pageNum != 1 {
			t.Fatalf("full final page moved to empty page%d", s.pageNum)
		}
	})
	t.Run("shrunk-groups-render", func(t *testing.T) {
		defer groupNavigationNoPanic(t)
		h, s := groupNavigationFixture(t, 25, 11)
		s.pageNum = 2
		s.groupTableRowIdx = 2
		s.groups = s.groups[:2]
		h.drawer.Draw(s)
		if !strings.Contains(renderingText(h.s), "group-000") {
			t.Fatalf("shrink did not render valid first page: %s", renderingText(h.s))
		}
	})
	t.Run("tiny-height-render", func(t *testing.T) {
		defer groupNavigationNoPanic(t)
		h, s := groupNavigationFixture(t, 10, 2)
		h.drawer.Draw(s)
		groupNavigationKey(h, tcell.KeyUp, 0)
		if s.groupTableRowIdx < 0 || s.groupTableRowIdx > len(s.groups) {
			t.Fatalf("tiny-height row%d out of domain", s.groupTableRowIdx)
		}
	})
}

// This specification model partitions named fixture identities into visible
// pages. It never calls production pagination or row-normalization helpers.
func groupNavigationSequence(t *testing.T, height, count int, input []byte) {
	t.Helper()
	defer groupNavigationNoPanic(t)
	h, s := groupNavigationFixture(t, height, count)
	type expectedGroup struct {
		pointer *asynq.GroupInfo
		value   asynq.GroupInfo
	}
	var expected []expectedGroup
	for _, g := range s.groups {
		expected = append(expected, expectedGroup{g, *g})
	}
	assertGroups := func() {
		t.Helper()
		if len(s.groups) != len(expected) {
			t.Fatalf("production changed group count: got%d want%d", len(s.groups), len(expected))
		}
		for i, want := range expected {
			if s.groups[i] != want.pointer || !reflect.DeepEqual(*s.groups[i], want.value) {
				t.Fatalf("production changed group identity/value at%d", i)
			}
		}
	}
	page, row := 1, 0
	normalize := func() []string {
		capacity := height - 16
		if capacity < 1 {
			capacity = 1
		}
		var pages [][]string
		for _, snapshot := range expected {
			g := snapshot.value
			if len(pages) == 0 || len(pages[len(pages)-1]) == capacity {
				pages = append(pages, []string{})
			}
			pages[len(pages)-1] = append(pages[len(pages)-1], g.Group)
		}
		if len(pages) == 0 {
			pages = append(pages, []string{})
		}
		if page > len(pages) {
			page = len(pages)
		}
		if page < 1 {
			page = 1
		}
		visible := pages[page-1]
		if row > len(visible) {
			row = len(visible)
		}
		if row < 0 {
			row = 0
		}
		return visible
	}
	assertState := func(visible []string) {
		t.Helper()
		assertGroups()
		if s.pageNum != page || s.groupTableRowIdx != row {
			t.Fatalf("state mismatch height%d groups%d gotpage%d row%d wantpage%d row%d", height, len(expected), s.pageNum, s.groupTableRowIdx, page, row)
		}
		if height >= 25 {
			text := renderingText(h.s)
			for _, snapshot := range expected {
				g := snapshot.value
				want := false
				for _, name := range visible {
					if name == g.Group {
						want = true
					}
				}
				if strings.Contains(text, g.Group) != want {
					t.Fatalf("visible identity mismatch group%s expected%t screen%s", g.Group, want, text)
				}
			}
		}
	}
	h.drawer.Draw(s)
	visible := normalize()
	assertState(visible)
	if len(input) > 64 {
		input = input[:64]
	}
	for _, raw := range input {
		visible = normalize()
		switch raw % 7 {
		case 0:
			if row < len(visible) {
				row++
			} else {
				row = 0
			}
			groupNavigationKey(h, tcell.KeyDown, 0)
		case 1:
			if row == 0 {
				row = len(visible)
			} else {
				row--
			}
			groupNavigationKey(h, tcell.KeyUp, 0)
		case 2:
			capacity := height - 16
			if capacity < 1 {
				capacity = 1
			}
			if page*capacity < len(expected) {
				page++
				row = 0
			}
			groupNavigationKey(h, tcell.KeyRune, 'n')
		case 3:
			if page > 1 {
				page--
				row = 0
			}
			groupNavigationKey(h, tcell.KeyRune, 'p')
		case 4:
			var want string
			if row > 0 {
				want = visible[row-1]
			}
			groupNavigationKey(h, tcell.KeyEnter, 0)
			assertGroups()
			if want != "" {
				var wantPointer *asynq.GroupInfo
				for _, g := range expected {
					if g.value.Group == want {
						wantPointer = g.pointer
					}
				}
				if s.selectedGroup != wantPointer {
					t.Fatalf("Enter selected%v want visible%s", s.selectedGroup, want)
				}
				return
			}
			if s.selectedGroup != nil {
				t.Fatal("header Enter selected a group")
			}
		case 5:
			keep := int(raw/7) % (len(expected) + 1)
			s.groups = s.groups[:keep]
			expected = expected[:keep]
			normalize()
			h.drawer.Draw(s)
		case 6:
			height = 1 + int(raw/7)%40
			h.s.(tcell.SimulationScreen).SetSize(100, height)
			normalize()
			h.drawer.Draw(s)
		}
		assertGroups()
		visible = normalize()
		h.drawer.Draw(s)
		assertState(visible)
	}
}
func TestGroupNavigationSeededSequenceContracts(t *testing.T) {
	rng := rand.New(rand.NewSource(20261018))
	for n := 0; n < 128; n++ {
		height := 1 + rng.Intn(45)
		count := rng.Intn(41)
		input := make([]byte, 48)
		rng.Read(input)
		t.Run(fmt.Sprintf("case-%03d", n), func(t *testing.T) { groupNavigationSequence(t, height, count, input) })
	}
}
func FuzzGroupNavigationKeySequences(f *testing.F) {
	f.Add(uint8(25), uint8(11), []byte{2, 0, 4})
	f.Add(uint8(10), uint8(2), []byte{1, 4})
	f.Add(uint8(25), uint8(9), []byte{2, 3, 0, 4})
	f.Add(uint8(25), uint8(20), []byte{2, 5, 6, 1, 4})
	f.Fuzz(func(t *testing.T, height, count uint8, input []byte) {
		groupNavigationSequence(t, 1+int(height)%45, int(count)%41, input)
	})
}

func TestGroupNavigationEventLoopResizeReconcilesWithoutFetch(t *testing.T) {
	x := newLifecycleFixture(t)
	queues := []*asynq.QueueInfo{{Queue: "resize-owned", Aggregating: 34}}
	lifecycleSend(t, x, x.d.queuesCh, queues)
	x.frame(t, hasText("resize-owned"))
	x.key(t, tcell.KeyDown, 0, hasText("queueTableRowIdx=1 "))
	x.key(t, tcell.KeyEnter, 0, hasText("=== Queue Summary ==="))
	x.key(t, tcell.KeyRight, 0, hasText("taskState=pending "))
	x.key(t, tcell.KeyRight, 0, hasText("taskState=aggregating "))
	var groups []*asynq.GroupInfo
	for i := 0; i < 34; i++ {
		groups = append(groups, &asynq.GroupInfo{Group: fmt.Sprintf("resize-group-%03d", i), Size: 1})
	}
	lifecycleSend(t, x, x.d.groupsCh, groups)
	x.frame(t, hasText("resize-group-000"))
	x.key(t, tcell.KeyRune, 'n', func(text string) bool {
		return strings.Contains(text, "pageNum=2") && strings.Contains(text, "resize-group-032")
	})
	x.key(t, tcell.KeyDown, 0, hasText("groupTableRowIdx=1 "))
	x.key(t, tcell.KeyDown, 0, hasText("groupTableRowIdx=2 "))
	beforeFetch := x.f.count.Load()
	after := x.s.serial.Load()
	x.s.SetSize(512, 50)
	x.s.PostEventWait(tcell.NewEventResize(512, 50))
	x.frameAfter(t, after, func(text string) bool {
		return strings.Contains(text, "resize-group-000") && strings.Contains(text, "pageNum=1") && strings.Contains(text, "groupTableRowIdx=2 ")
	})
	if x.f.count.Load() != beforeFetch {
		t.Fatal("resize reconciliation unexpectedly fetched data")
	}
	x.key(t, tcell.KeyEnter, 0, hasText("selectedGroup={Group:resize-group-001} "))
	x.exit(t, tcell.KeyCtrlC, 0)
}

func TestGroupNavigationTinyViewportFetch(t *testing.T) {
	f := newFetchContractFixture(t)
	before := f.snapshot(t)
	t.Cleanup(func() {
		if !reflect.DeepEqual(before, f.snapshot(t)) {
			t.Error("tiny viewport fetch mutated owned/neighbor Redis bytes")
		}
	})
	screen := renderingScreen(t, 100, 10)
	tasks := make(chan []*asynq.TaskInfo, 1)
	failures := make(chan error, 1)
	done := make(chan struct{})
	fetcher := &dataFetcher{inspector: f.inspector, s: screen, tasksCh: tasks, errorCh: failures, done: done, slots: make(chan struct{}, 4)}
	t.Cleanup(func() {
		close(done)
		joined := make(chan struct{})
		go func() { fetcher.wait(); close(joined) }()
		select {
		case <-joined:
		case <-time.After(3 * time.Second):
			t.Error("tiny viewport fetch cleanup did not join")
		}
	})
	expected, e := f.inspector.ListAggregatingTasks(f.queues[0], f.group, asynq.PageSize(1), asynq.Page(1))
	if e != nil {
		t.Fatal(e)
	}
	if len(expected) != 1 || expected[0].ID != f.ids[asynq.TaskStateAggregating][0] {
		t.Fatalf("independent fixture/page1 oracle unexpected: %v", expected)
	}
	state := &State{view: viewTypeQueueDetails, selectedQueue: &asynq.QueueInfo{Queue: f.queues[0], Aggregating: 5}, taskState: asynq.TaskStateAggregating, selectedGroup: &asynq.GroupInfo{Group: f.group, Size: 5}, pageNum: 1}
	fetcher.Fetch(state)
	select {
	case got := <-tasks:
		if len(got) != 1 || got[0].ID != expected[0].ID {
			t.Fatalf("tiny viewport must fetch one ordered task; got%d want1", len(got))
		}
	case e := <-failures:
		t.Fatalf("tiny viewport fetch: %v", e)
	case <-time.After(3 * time.Second):
		t.Fatal("tiny viewport fetch delivery deadline")
	}
	if e := f.client.Ping(context.Background()).Err(); e != nil {
		t.Fatal(e)
	}
}
