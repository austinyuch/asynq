package dash

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func renderingScreen(t *testing.T, width, height int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(width, height)
	t.Cleanup(s.Fini)
	return s
}
func renderingRow(s tcell.Screen, y int) string {
	width, _ := s.Size()
	var out strings.Builder
	for x := 0; x < width; x++ {
		r, _, _, cellWidth := s.GetContent(x, y)
		if r == 0 {
			r = ' '
		}
		out.WriteRune(r)
		if cellWidth > 1 {
			x += cellWidth - 1
		}
	}
	return strings.TrimRight(out.String(), " ")
}
func renderingText(s tcell.Screen) string {
	_, height := s.Size()
	var rows []string
	for y := 0; y < height; y++ {
		rows = append(rows, renderingRow(s, y))
	}
	return strings.Join(rows, "\n")
}
func renderingContains(t *testing.T, s tcell.Screen, wants ...string) {
	t.Helper()
	text := renderingText(s)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Errorf("screen missing %q:\n%s", want, text)
		}
	}
}
func renderingBackground(t *testing.T, s tcell.Screen, y int, want tcell.Color) {
	t.Helper()
	width, _ := s.Size()
	for x := 0; x < width; x++ {
		_, _, style, _ := s.GetContent(x, y)
		_, background, _ := style.Decompose()
		if background != want {
			t.Fatalf("cell(%d,%d) background=%v want=%v", x, y, background, want)
		}
	}
}

func TestRenderingQueueViewCellsAndStyles(t *testing.T) {
	s := renderingScreen(t, 160, 50)
	state := &State{view: viewTypeQueues, queueTableRowIdx: 2, queues: []*asynq.QueueInfo{
		{Queue: "z-neighbor", Size: 6, Active: 1, Pending: 2, Retry: 3, Processed: 10, Failed: 2, Latency: 2 * time.Second, MemoryUsage: 1200},
		{Queue: "selected", Size: 2, Completed: 2, Paused: true, MemoryUsage: 99},
	}}
	dd := dashDrawer{s: s}
	dd.Draw(state)
	renderingContains(t, s, "=== Queues ===", "z-neighbor", "selected", "RUN", "PAUSED", "1.2 kB", "99 B", "0.20", "<?>: Help", "<Ctrl+C>: Exit")
	text := renderingText(s)
	if strings.Index(text, "z-neighbor") > strings.Index(text, "selected") {
		t.Fatal("rendering reordered neighboring queue inputs")
	}
	foundSelected, foundHeader, foundBlue, foundGreen := false, false, false, false
	for y := 0; y < 49; y++ {
		row := renderingRow(s, y)
		if strings.HasPrefix(row, "selected") && strings.Contains(row, "PAUSED") {
			renderingBackground(t, s, y, tcell.ColorDarkOliveGreen)
			foundSelected = true
		}
		if strings.HasPrefix(row, "Queue") && strings.Contains(row, "MemoryUsage") {
			renderingBackground(t, s, y, tcell.ColorDimGray)
			foundHeader = true
		}
		for x := 0; x < 160; x++ {
			r, _, style, _ := s.GetContent(x, y)
			fg, _, _ := style.Decompose()
			if r == '▇' && fg == tcell.ColorBlue {
				foundBlue = true
			}
			if r == '▇' && fg == tcell.ColorGreen {
				foundGreen = true
			}
		}
	}
	if !foundSelected || !foundHeader || !foundBlue || !foundGreen {
		t.Fatalf("missing selected/header/state styles: %v %v %v %v", foundSelected, foundHeader, foundBlue, foundGreen)
	}
	renderingBackground(t, s, 49, tcell.ColorDarkSlateGray)
}

func TestRenderingTaskStateColumnsAndSelection(t *testing.T) {
	queue := &asynq.QueueInfo{Queue: "task-queue", Active: 1, Pending: 2, Aggregating: 3, Scheduled: 4, Retry: 5, Archived: 6, Completed: 7, Size: 28, MemoryUsage: 2000}
	for _, tc := range []struct {
		state   asynq.TaskState
		headers []string
	}{
		{asynq.TaskStateActive, []string{"Retried", "Max Retry", "Payload"}},
		{asynq.TaskStatePending, []string{"Retried", "Max Retry", "Payload"}},
		{asynq.TaskStateAggregating, []string{"Group", "Payload"}},
		{asynq.TaskStateScheduled, []string{"Next Process Time", "Payload"}},
		{asynq.TaskStateRetry, []string{"Last Failure", "Next Process Time", "Retry"}},
		{asynq.TaskStateArchived, []string{"Last Failure Time", "Retry"}},
		{asynq.TaskStateCompleted, []string{"Completion Time", "Result"}},
	} {
		t.Run(tc.state.String(), func(t *testing.T) {
			s := renderingScreen(t, 220, 50)
			task := &asynq.TaskInfo{ID: "visible-id", Type: "visible-kind", State: tc.state, Queue: queue.Queue, Payload: []byte("visible-payload"), Result: []byte("visible-result"), Retried: 2, MaxRetry: 9, Group: "visible-group", LastErr: "visible-failure", LastFailedAt: time.Now().Add(-time.Hour), NextProcessAt: time.Now().Add(time.Hour), CompletedAt: time.Now().Add(-time.Hour)}
			state := &State{view: viewTypeQueueDetails, selectedQueue: queue, taskState: tc.state, tasks: []*asynq.TaskInfo{task}, taskTableRowIdx: 1, pageNum: 1, selectedGroup: &asynq.GroupInfo{Group: "visible-group", Size: 1}}
			(&dashDrawer{s: s}).Draw(state)
			renderingContains(t, s, "=== Queue Summary ===", "Name", queue.Queue, "=== Tasks ===", "visible-id", "visible-kind", "visible-payload")
			renderingContains(t, s, tc.headers...)
			selectedState, selectedTask := false, false
			for y := 0; y < 49; y++ {
				if strings.HasPrefix(renderingRow(s, y), "visible-id") {
					renderingBackground(t, s, y, tcell.ColorDarkOliveGreen)
					selectedTask = true
				}
				for x := 0; x < 220; x++ {
					r, _, style, _ := s.GetContent(x, y)
					_, _, attrs := style.Decompose()
					if r != ' ' && attrs&tcell.AttrUnderline != 0 && attrs&tcell.AttrBold != 0 {
						selectedState = true
					}
				}
			}
			if !selectedState || !selectedTask {
				t.Fatal("selected task/state highlight missing")
			}
			if tc.state == asynq.TaskStateRetry || tc.state == asynq.TaskStateArchived {
				renderingContains(t, s, "2/9", "visible-failure")
			}
			if tc.state == asynq.TaskStateCompleted {
				renderingContains(t, s, "visible-result")
			}
		})
	}
}

func TestRenderingTaskAndGroupPaginationNeighbors(t *testing.T) {
	s := renderingScreen(t, 150, 25)
	groups := []*asynq.GroupInfo{}
	for i := 1; i <= 11; i++ {
		groups = append(groups, &asynq.GroupInfo{Group: fmt.Sprintf("group-%02d", i), Size: i})
	}
	state := &State{view: viewTypeQueueDetails, taskState: asynq.TaskStateAggregating, groups: groups, pageNum: 2, groupTableRowIdx: 1}
	drawGroupTable(NewScreenDrawer(s), state)
	renderingContains(t, s, "<<< Select group >>>", "group-10", "group-11", "Showing 10-11 out of 11", "p=PrevPage")
	text := renderingText(s)
	if strings.Contains(text, "group-09") || strings.Contains(text, "n=NextPage") {
		t.Fatal("last group page leaked neighbor/next page")
	}
	s.Clear()
	state = &State{taskState: asynq.TaskStatePending, selectedQueue: &asynq.QueueInfo{Pending: 12}, pageNum: 2, tasks: []*asynq.TaskInfo{{ID: "page-two-a", Type: "a"}, {ID: "page-two-b", Type: "b"}}}
	drawTaskTable(NewScreenDrawer(s), state)
	renderingContains(t, s, "page-two-a", "page-two-b", "Showing 11-12 out of 12", "p=PrevPage")
	if strings.Contains(renderingText(s), "n=NextPage") {
		t.Fatal("last task page exposed extra page")
	}
	s.Clear()
	state.taskState = asynq.TaskStateAggregating
	state.selectedQueue.Aggregating = 999
	state.selectedGroup = &asynq.GroupInfo{Size: 12, Group: "scope"}
	drawTaskTable(NewScreenDrawer(s), state)
	renderingContains(t, s, "out of 12")
	if strings.Contains(renderingText(s), "out of 999") || strings.Contains(renderingText(s), "n=NextPage") {
		t.Fatal("pagination count escaped selected group")
	}
}

func TestRenderingModalHelpFooterAndMissingTask(t *testing.T) {
	s := renderingScreen(t, 180, 50)
	state := &State{view: viewTypeHelp}
	dd := &dashDrawer{s: s}
	dd.Draw(state)
	renderingContains(t, s, "=== Help ===", "<Enter>", "to select", "<UpArrow>", "<DownArrow>", "<Ctrl+C>", "to quit", "<Esc>: GoBack")
	r, _, _, _ := s.GetContent(36, 10)
	if r != tcell.RuneULCorner {
		t.Fatalf("help frame missing: %q", r)
	}
	s.Clear()
	state = &State{taskID: "gone"}
	drawTaskModal(NewScreenDrawer(s), state)
	renderingContains(t, s, `Task "gone" no longer exists`)
	s.Clear()
	state.selectedTask = &asynq.TaskInfo{ID: "modal-id", Queue: "modal-queue", Type: "modal-kind", State: asynq.TaskStateCompleted, Retried: 2, MaxRetry: 8, Payload: []byte("line-one line-two"), Result: []byte{0, 255}, LastErr: "modal-error", LastFailedAt: time.Now().Add(-time.Hour), NextProcessAt: time.Now().Add(time.Hour), CompletedAt: time.Now().Add(-time.Hour)}
	drawTaskModal(NewScreenDrawer(s), state)
	renderingContains(t, s, "modal-id", "modal-queue", "modal-kind", "completed", "2/8", "modal-error", "Last Failure Time:", "Next Process Time:", "Completion Time:", "line-one line-two", "<non-printable>")
	s.Clear()
	state = &State{view: viewTypeQueues, err: errors.New("render-error")}
	dd.Draw(state)
	if renderingRow(s, 49) != "render-error" {
		t.Fatalf("error footer missing: %q", renderingRow(s, 49))
	}
	renderingBackground(t, s, 49, tcell.ColorDarkRed)
	s.Clear()
	dd.opts.DebugMode = true
	dd.Draw(state)
	renderingContains(t, s, "len(queues)=0", "err=render-error", "queueTableRowIdx=0")
}

func TestRenderingUnicodeCellsAndClippingContracts(t *testing.T) {
	s := renderingScreen(t, 12, 4)
	d := NewScreenDrawer(s)
	style := baseStyle.Foreground(tcell.ColorYellow)
	d.Print("界a", style)
	r, _, gotStyle, _ := s.GetContent(0, 0)
	if r != '界' || gotStyle != style {
		t.Fatal("wide glyph or style lost")
	}
	r, _, _, _ = s.GetContent(2, 0)
	if r != 'a' {
		t.Fatalf("ASCII neighbor overwrote wide glyph: %q", r)
	}
	d.Goto(10, 1)
	d.Print("abc", style)
	r, _, _, _ = s.GetContent(11, 1)
	if r != 'b' {
		t.Fatal("right-edge clipping lost visible neighbor")
	}
	if renderingRow(s, 2) != "" {
		t.Fatal("clipped line wrapped into unrelated row")
	}
	for _, tc := range []struct {
		data []byte
		want string
	}{{nil, "<nil>"}, {[]byte{0xff}, "<non-printable>"}, {[]byte(" "), "<non-printable>"}, {[]byte("a\nb"), "<non-printable>"}, {[]byte("佇列"), "佇列"}} {
		if got := formatByteSlice(tc.data); got != tc.want {
			t.Errorf("byte format=%q want=%q", got, tc.want)
		}
	}
	for _, tc := range []struct {
		n    int64
		want string
	}{{99, "99 B"}, {1000, "1.0 kB"}, {1000000, "1.0 MB"}, {1000000000, "1.0 GB"}} {
		if got := byteCount(tc.n); got != tc.want {
			t.Errorf("bytes=%d got=%s want=%s", tc.n, got, tc.want)
		}
	}
	if titleCase("佇列") != "佇列" || titleCase("") != "" {
		t.Fatal("title casing lost Unicode or empty value")
	}
}

func TestRenderingStableScreenSizeProperty(t *testing.T) {
	// A fully contained printable row preserves its prefix across varying screen
	// sizes, while offscreen text never wraps into another row. The oracle reads
	// actual cells and does not reproduce drawer positioning logic.
	property := func(rawWidth, rawHeight uint8, wide bool) bool {
		width, height := 8+int(rawWidth%40), 3+int(rawHeight%12)
		s := tcell.NewSimulationScreen("UTF-8")
		if s.Init() != nil {
			return false
		}
		defer s.Fini()
		s.SetSize(width, height)
		text := "prefix"
		if wide {
			text = "界prefix"
		}
		NewScreenDrawer(s).Print(text, baseStyle)
		if !strings.HasPrefix(renderingRow(s, 0), text) {
			return false
		}
		for y := 1; y < height; y++ {
			if renderingRow(s, y) != "" {
				return false
			}
		}
		cells, w, h := s.GetContents()
		return w == width && h == height && len(cells) == width*height
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 300, Rand: rand.New(rand.NewSource(20261006))}); err != nil {
		t.Fatal(err)
	}
}

func TestRenderingUnicodeTruncationFitsDisplayWidth(t *testing.T) {
	for _, tc := range []struct {
		text  string
		limit int
	}{{"界界", 3}, {"界界界", 5}, {"界a界b", 4}} {
		t.Run(fmt.Sprintf("%s/%d", tc.text, tc.limit), func(t *testing.T) {
			got := truncate(tc.text, tc.limit)
			if runewidth.StringWidth(got) > tc.limit || !strings.HasSuffix(got, "…") {
				t.Fatalf("truncation escaped display budget %d: %q", tc.limit, got)
			}
		})
	}
}

func TestRenderingModalFragmentsStayInsideFrame(t *testing.T) {
	s := renderingScreen(t, 30, 15)
	withModal(NewScreenDrawer(s), []func(*modalRowDrawer){
		func(d *modalRowDrawer) { d.Print("Label: ", labelStyle); d.Print("abcdefghijklmnop", baseStyle) },
		func(d *modalRowDrawer) { d.Print("next", baseStyle) },
	})
	// Width 18 at x=6: frame right edge is x=23. Multiple fragments
	// consume one shared content budget, and the next row gets a new budget.
	r, _, _, _ := s.GetContent(23, 4)
	if r != tcell.RuneVLine {
		t.Errorf("right modal border displaced: %q", r)
	}
	for x := 24; x < 30; x++ {
		r, _, _, _ := s.GetContent(x, 4)
		if r != 0 && r != ' ' {
			t.Errorf("fragment escaped modal at x=%d: %q", x, r)
		}
	}
	if !strings.Contains(renderingRow(s, 5), "next") {
		t.Fatal("content budget was not reset for next row")
	}
}

func TestRenderingTruncationBudgetProperty(t *testing.T) {
	property := func(rawLimit uint8, repeats uint8, wide bool) bool {
		limit := int(rawLimit%32) - 2
		text := strings.Repeat("abc", int(repeats%30))
		if wide {
			text = strings.Repeat("界a", int(repeats%30))
		}
		got := truncate(text, limit)
		if limit <= 0 {
			return got == ""
		}
		if runewidth.StringWidth(got) > limit {
			return false
		}
		if runewidth.StringWidth(text) <= limit {
			return got == text
		}
		return strings.HasSuffix(got, "…")
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 500, Rand: rand.New(rand.NewSource(20261007))}); err != nil {
		t.Fatal(err)
	}
}

type renderingCountingFetcher struct{ calls int }

func (f *renderingCountingFetcher) Fetch(*State) { f.calls++ }

func TestRenderingAggregatingLastPageHandlerStaysInGroup(t *testing.T) {
	state := &State{view: viewTypeQueueDetails, taskState: asynq.TaskStateAggregating, selectedQueue: &asynq.QueueInfo{Aggregating: 999}, selectedGroup: &asynq.GroupInfo{Group: "chosen", Size: 12}, pageNum: 2, tasks: []*asynq.TaskInfo{{ID: "eleven"}, {ID: "twelve"}}}
	h := makeKeyEventHandler(t, state)
	h.s = renderingScreen(t, 150, 25)
	f := &renderingCountingFetcher{}
	h.fetcher = f
	h.HandleKeyEvent(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if state.pageNum != 2 || f.calls != 0 {
		t.Fatalf("last selected-group page escaped: page=%d fetches=%d", state.pageNum, f.calls)
	}
	if state.tasks[0].ID != "eleven" || state.tasks[1].ID != "twelve" {
		t.Fatal("last-page neighbor tasks changed")
	}
}

func TestRenderingGraphemeCellsAndModalBoundary(t *testing.T) {
	for _, text := range []string{"e\u0301", "👩‍💻", "🇹🇼"} {
		t.Run(text, func(t *testing.T) {
			s := renderingScreen(t, 30, 15)
			style := baseStyle.Foreground(tcell.ColorYellow)
			NewScreenDrawer(s).Print(text+"X", style)
			main, comb, gotStyle, _ := s.GetContent(0, 0)
			got := string(append([]rune{main}, comb...))
			if got != text || gotStyle != style {
				t.Fatalf("grapheme/style lost: got=%q want=%q", got, text)
			}
			main, _, _, _ = s.GetContent(runewidth.StringWidth(text), 0)
			if main != 'X' {
				t.Fatalf("grapheme neighbor lost: %q", main)
			}
			s.Clear()
			withModal(NewScreenDrawer(s), []func(*modalRowDrawer){func(d *modalRowDrawer) { d.Print(strings.Repeat("👩‍💻", 7), style) }})
			main, _, _, _ = s.GetContent(23, 4)
			if main != tcell.RuneVLine {
				t.Fatalf("ZWJ displaced modal frame: %q", main)
			}
			for x := 24; x < 30; x++ {
				main, _, _, _ = s.GetContent(x, 4)
				if main != 0 && main != ' ' {
					t.Errorf("ZWJ escaped modal at x=%d: %q", x, main)
				}
			}
		})
	}
}

func TestRenderingNarrowHelpAndModalFrames(t *testing.T) {
	for width := 1; width <= 12; width++ {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			s := renderingScreen(t, width, 15)
			(&dashDrawer{s: s}).Draw(&State{view: viewTypeHelp})
			// The help footer remains visible even when no modal fits.
			if !strings.HasPrefix(renderingRow(s, 14), "<") {
				t.Fatal("narrow help footer disappeared")
			}
			s.Clear()
			withModal(NewScreenDrawer(s), []func(*modalRowDrawer){func(d *modalRowDrawer) { d.Print("payload", baseStyle) }})
			if width < 7 {
				for row := 0; row < 15; row++ {
					if renderingRow(s, row) != "" {
						t.Fatalf("unrenderable modal left cells on row %d", row)
					}
				}
				return
			}
			right := int(float64(width)*0.2) + int(float64(width)*0.6) - 1
			r, _, _, _ := s.GetContent(right, 4)
			if r != tcell.RuneVLine {
				t.Fatalf("narrow modal right border lost: %q", r)
			}
			for x := right + 1; x < width; x++ {
				r, _, _, _ = s.GetContent(x, 4)
				if r != 0 && r != ' ' {
					t.Fatalf("narrow modal escaped at x=%d: %q", x, r)
				}
			}
		})
	}
}
