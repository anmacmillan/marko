package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func renderRows(t *testing.T, e *editor, width, height int) []string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, height)
	e.screen = screen
	rows := e.visualRows(width)
	var out []string
	for i, vr := range rows {
		if i >= height {
			break
		}
		if vr.start >= 0 {
			e.drawVisualLine(0, i, vr, false, width)
		}
		out = append(out, strings.TrimRight(simulationLine(screen, i, width), " "))
	}
	return out
}

func TestListsAndTasksRenderWithGlyphs(t *testing.T) {
	e := &editor{lines: []string{"", "- [ ] open task", "- [x] done task", "- bullet", "  - nested"}, theme: themeByName("calm")}
	got := renderRows(t, e, 40, 6)
	want := []string{"", taskGlyphs[0] + " open task", taskGlyphs[1] + " done task", "• bullet", "  ◦ nested"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d = %q, want %q (all rows %q)", i, got[i], want[i], got)
		}
	}
}

func TestCursorLineShowsRawListMarkdown(t *testing.T) {
	e := &editor{lines: []string{"- [ ] open task"}, theme: themeByName("calm")}
	if got := renderRows(t, e, 40, 2)[0]; got != "- [ ] open task" {
		t.Fatalf("cursor line = %q, want raw markdown", got)
	}
}

func TestWrappedQuoteKeepsBarOnEveryRow(t *testing.T) {
	e := &editor{lines: []string{"", "> one two three four five six"}, theme: themeByName("calm")}
	got := renderRows(t, e, 14, 6)
	if len(got) < 4 {
		t.Fatalf("expected the quote to wrap, got %q", got)
	}
	for _, row := range got[1:] {
		if row != "" && !strings.HasPrefix(row, "▎ ") {
			t.Fatalf("quote row without bar: %q (all %q)", row, got)
		}
	}
}

func TestWrappedListItemHangsUnderText(t *testing.T) {
	e := &editor{lines: []string{"", "- alpha beta gamma delta"}, theme: themeByName("calm")}
	got := renderRows(t, e, 14, 6)
	if !strings.HasPrefix(got[1], "• ") || !strings.HasPrefix(got[2], "  ") || strings.HasPrefix(got[2], "   ") {
		t.Fatalf("list wrap = %q, want bullet then two-space hang", got)
	}
}

func TestMouseOnDecoratedRowMapsToSource(t *testing.T) {
	e := &editor{lines: []string{"", "- [ ] open task"}, theme: themeByName("calm")}
	rows := e.visualRows(40)
	vr := rows[1]
	// Display column 2 is the 'o' after "☐ "; in source that is column 6.
	if got := e.sourceColumnForMouse(vr, 2); got != 6 {
		t.Fatalf("source column = %d, want 6", got)
	}
}

func TestListInsideCodeBlockBeingEditedStaysRaw(t *testing.T) {
	e := &editor{lines: []string{"```", "- not a list", "```"}, theme: themeByName("calm")}
	for _, vr := range e.visualRows(40) {
		if vr.kind != rowPlain {
			t.Fatalf("row %+v decorated inside a code block under edit", vr)
		}
	}
}

func TestChecklistMeterOnlyOnNearestHeading(t *testing.T) {
	e := &editor{lines: []string{"# Top", "## Tasks", "- [x] one", "- [ ] two"}}
	if _, ok := e.sectionChecklistProgress(0, 1); ok {
		t.Fatal("H1 showed a meter for tasks under its H2")
	}
	got, ok := e.sectionChecklistProgress(1, 2)
	if !ok || !strings.HasSuffix(got, "1/2") {
		t.Fatalf("H2 meter = %q %t", got, ok)
	}
}

func TestTableHeaderBoldAndBordersFaint(t *testing.T) {
	e := &editor{lines: []string{"", "| Head | Low |", "|---|---:|", "| Basic | 4 |"}, theme: themeByName("midnight")}
	base := tcell.StyleDefault.Foreground(e.theme.table).Background(e.theme.background)
	borders, header := e.tableStyles(1, base)
	_, body := e.tableStyles(3, base)
	if _, _, attr := header.Decompose(); attr&tcell.AttrBold == 0 {
		t.Fatal("table header is not bold")
	}
	if _, _, attr := body.Decompose(); attr&tcell.AttrBold != 0 {
		t.Fatal("table body is bold")
	}
	if bfg, _, _ := borders.Decompose(); bfg == e.theme.table {
		t.Fatal("table borders not faded")
	}
}

func TestWhisperStatusInFocusMode(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(60, 6)
	e := &editor{screen: screen, path: "/tmp/note.md", lines: []string{"one two three"}, theme: themeByName("midnight"), focusMode: true}
	e.draw()
	if got := strings.TrimSpace(simulationLine(screen, 5, 60)); !strings.HasSuffix(got, "note.md · 3 words") {
		t.Fatalf("whisper status = %q", got)
	}
}

func TestCodePanelSurvivesFocusDimming(t *testing.T) {
	e := &editor{lines: []string{"", "```go", "x := 1", "```"}, theme: themeByName("midnight"), focusMode: true}
	renderRows(t, e, 30, 5)
	mainc, _, style, _ := e.screen.GetContent(5, 2)
	_, cellBG, _ := style.Decompose()
	if cellBG == e.theme.dimBG {
		t.Fatalf("code row background is plain dimBG (rune %q): panel lost in focus mode", mainc)
	}
}
