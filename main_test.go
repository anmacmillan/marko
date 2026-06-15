package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestSplitTable(t *testing.T) {
	got := splitTable("| Feature | Behaviour |")
	want := []string{"Feature", "Behaviour"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTable() = %#v, want %#v", got, want)
	}
}

func TestIsSeparator(t *testing.T) {
	if !isSeparator("| --- | :---: | ---: |") {
		t.Fatal("expected separator row")
	}
	if isSeparator("| words | --- |") {
		t.Fatal("data row incorrectly identified as separator")
	}
}

func TestInsertLine(t *testing.T) {
	got := insertLine([]string{"one", "three"}, 1, "two")
	want := []string{"one", "two", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("insertLine() = %#v, want %#v", got, want)
	}
}

func TestInsertTable(t *testing.T) {
	e := &editor{lines: []string{"before after"}, x: 7}
	e.insertTable()
	want := []string{
		"before | Heading 1 | Heading 2 |",
		"| --------- | --------- |",
		"|           |           |after",
	}
	if !reflect.DeepEqual(e.lines, want) {
		t.Fatalf("insertTable() = %#v, want %#v", e.lines, want)
	}
	if e.x != 9 {
		t.Fatalf("cursor X = %d, want 9", e.x)
	}
}

func TestRenderTableLine(t *testing.T) {
	e := &editor{
		lines: []string{
			"| Name | Role |",
			"| --- | --- |",
			"| Alex | Barrister |",
			"",
		},
		y: 3,
	}
	if got, want := e.renderTableLine(0), "│ Name │ Role      │"; got != want {
		t.Fatalf("header render = %q, want %q", got, want)
	}
	if got, want := e.renderTableLine(1), "├──────┼───────────┤"; got != want {
		t.Fatalf("separator render = %q, want %q", got, want)
	}
}

func TestRenderTableLineFitsWritingArea(t *testing.T) {
	e := &editor{
		lines: []string{
			"| Head | Realistic award |",
			"| --- | --- |",
			"| Loss of statutory rights | £600 |",
			"| Injury to feelings (upper Vento) | £36,400 |",
		},
		y: 10,
	}
	for y := range e.lines {
		got := e.renderTableLine(y, 32)
		if runeLen(got) != 32 {
			t.Fatalf("renderTableLine(%d) width = %d, want 32: %q", y, runeLen(got), got)
		}
	}
	if got := e.renderTableLine(3, 32); !strings.Contains(got, "…") {
		t.Fatalf("expected truncated cell, got %q", got)
	}
}

func TestRenderTableLineMeasuresInlineMarkdownByDisplayWidth(t *testing.T) {
	e := &editor{
		lines: []string{
			"| Head | Award |",
			"| --- | --- |",
			"| **Sub-total** | **£236,304** |",
			"| Plain | £1 |",
		},
		y: 10,
	}
	got := e.renderTableLine(2)
	if want := "│ **Sub-total** │ **£236,304** │"; got != want {
		t.Fatalf("bold table row = %q, want %q", got, want)
	}
	if got, want := inlineDisplayWidth(got), 24; got != want {
		t.Fatalf("bold table row display width = %d, want %d", got, want)
	}
}

func TestRenderedTableAppliesBoldStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 5)
	e := &editor{
		screen: screen,
		lines: []string{
			"| Head | Award |",
			"| --- | --- |",
			"| **Sub-total** | **£236,304** |",
			"",
		},
		y:     3,
		theme: themeByName("calm"),
	}
	e.drawLine(0, 0, 2, e.lines[2], false, 40)
	mainc, _, style, _ := screen.GetContent(2, 0)
	if mainc != 'S' {
		t.Fatalf("first bold table character = %q, want S", mainc)
	}
	_, _, attrs := style.Decompose()
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("bold table cell did not receive bold style")
	}
	if got := simulationLine(screen, 0, 40); got != "│ Sub-total │ £236,304 │                " {
		t.Fatalf("rendered bold table row = %q", got)
	}
}

func TestRenderedBoldTableKeepsBordersAligned(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(64, 6)
	e := &editor{
		screen: screen,
		lines: []string{
			"| Head | Realistic award |",
			"| --- | --- |",
			"| Basic award | £1,057 |",
			"| **Sub-total** | **£236,304** |",
			"| **Gross** | **£231,304** |",
			"",
		},
		y:     5,
		theme: themeByName("calm"),
	}
	for y := 0; y < 5; y++ {
		e.drawLine(0, y, y, e.lines[y], false, 64)
	}
	wantBorders := tableBorderPositions(simulationLine(screen, 0, 64))
	for _, row := range []int{2, 3, 4} {
		line := simulationLine(screen, row, 64)
		if got := tableBorderPositions(line); !reflect.DeepEqual(got, wantBorders) {
			t.Fatalf("row %d borders = %v, want %v: %q", row, got, wantBorders, line)
		}
	}
}

func tableBorderPositions(line string) []int {
	var positions []int
	for x, r := range []rune(line) {
		if r == '│' {
			positions = append(positions, x)
		}
	}
	return positions
}

func simulationLine(screen tcell.SimulationScreen, y, width int) string {
	runes := make([]rune, width)
	for x := 0; x < width; x++ {
		mainc, _, _, _ := screen.GetContent(x, y)
		runes[x] = mainc
	}
	return string(runes)
}

func TestTruncateInlineCellProducesCleanText(t *testing.T) {
	if got, want := truncateInlineCell("**A long bold value**", 8), "A long …"; got != want {
		t.Fatalf("truncateInlineCell() = %q, want %q", got, want)
	}
	if got, want := truncateInlineCell("**bold**", 4), "**bold**"; got != want {
		t.Fatalf("fitting bold cell = %q, want %q", got, want)
	}
}

func TestRenderedTableRowsDoNotWrapRawMarkdown(t *testing.T) {
	e := &editor{
		lines: []string{
			"| A very long heading that exceeds the viewport | Value |",
			"| --- | --- |",
			"| A very long value that also exceeds the viewport | £1 |",
			"",
		},
		y: 3,
	}
	rows := e.visualRows(24)
	if got, want := len(rows), 4; got != want {
		t.Fatalf("visualRows() count = %d, want %d", got, want)
	}
	for i := 0; i < 3; i++ {
		if got := runeLen(rows[i].text); got != 24 {
			t.Fatalf("table row %d width = %d, want 24", i, got)
		}
	}
}

func TestHeading(t *testing.T) {
	level, text, ok := heading("### Key documents")
	if !ok || level != 3 || text != "Key documents" {
		t.Fatalf("heading() = %d, %q, %v", level, text, ok)
	}
	if _, _, ok := heading("#not a heading"); ok {
		t.Fatal("heading without separating space was accepted")
	}
}

func TestEmphasisAt(t *testing.T) {
	tests := []struct {
		text       string
		markerSize int
		end        int
	}{
		{"**bold**", 2, 6},
		{"*italic*", 1, 7},
		{"~~strike~~", 2, 8},
		{"plain", 0, 0},
	}
	for _, test := range tests {
		size, style, end := emphasisAt([]rune(test.text), 0)
		if size != test.markerSize || end != test.end {
			t.Fatalf("emphasisAt(%q) = %d, %d; want %d, %d", test.text, size, end, test.markerSize, test.end)
		}
		if size > 0 && style == nil {
			t.Fatalf("emphasisAt(%q) returned no style", test.text)
		}
		if style != nil {
			_ = style(tcell.StyleDefault)
		}
	}
}

func TestSavePromptAddsMarkdownExtension(t *testing.T) {
	e := &editor{lines: []string{"hello"}, prompt: "Save as: ", promptValue: "note"}
	e.promptKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if e.path != "note.md" {
		t.Fatalf("path = %q, want note.md", e.path)
	}
	_ = os.Remove("note.md")
}

func TestDatedUntitledPath(t *testing.T) {
	now := time.Date(2026, 12, 30, 9, 0, 0, 0, time.UTC)
	if got, want := datedUntitledPath(now), "20261230_untitled.md"; got != want {
		t.Fatalf("datedUntitledPath() = %q, want %q", got, want)
	}
}

func TestUniqueUntitledPath(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 12, 30, 9, 0, 0, 0, time.UTC)
	first := filepath.Join(dir, "20261230_untitled.md")
	if err := os.WriteFile(first, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := uniqueUntitledPath(now, dir), filepath.Join(dir, "20261230_untitled_2.md"); got != want {
		t.Fatalf("uniqueUntitledPath() = %q, want %q", got, want)
	}
}

func TestThemeNamesAreValid(t *testing.T) {
	for _, name := range themeNames {
		if !validTheme(name) {
			t.Fatalf("built-in theme %q is invalid", name)
		}
		_ = themeByName(name)
	}
	if validTheme("unknown") {
		t.Fatal("unknown theme accepted")
	}
}

func TestVisualRowsWrapAndMap(t *testing.T) {
	e := &editor{lines: []string{"abcdefgh", "xy"}, x: 6, y: 0}
	rows := e.visualRows(4)
	if len(rows) != 3 || rows[1].text != "efgh" || rows[1].start != 4 {
		t.Fatalf("visualRows() = %#v", rows)
	}
	if got := e.cursorVisualRow(rows); got != 1 {
		t.Fatalf("cursorVisualRow() = %d, want 1", got)
	}
}

func TestMoveVisualVerticalFollowsWrappedRows(t *testing.T) {
	e := &editor{lines: []string{strings.Repeat("a", 120), "ij"}, x: 10, y: 0}
	e.moveVisualVertical(1)
	if e.y != 0 {
		t.Fatalf("moveVisualVertical down = (%d,%d), want same source line", e.y, e.x)
	}
	e.moveVisualVertical(1)
	if e.y != 1 {
		t.Fatalf("moveVisualVertical down again = (%d,%d), want next source line after wraps", e.y, e.x)
	}
	e.moveVisualVertical(-1)
	if e.y != 0 {
		t.Fatalf("moveVisualVertical up = (%d,%d), want return to wrapped line", e.y, e.x)
	}
}

func TestMoveLineEdgeUsesVisualRowBoundaries(t *testing.T) {
	e := &editor{lines: []string{strings.Repeat("a", 120)}, x: 37, y: 0}
	e.moveLineEdge(false)
	if e.x != 0 {
		t.Fatalf("moveLineEdge(false) = %d, want 0", e.x)
	}
	e.x = 37
	e.moveLineEdge(true)
	if e.x != 76 {
		t.Fatalf("moveLineEdge(true) = %d, want 76", e.x)
	}
}

func TestFocusedLineUsesCurrentParagraph(t *testing.T) {
	e := &editor{
		lines:     []string{"first", "", "current one", "current two", "", "last"},
		y:         3,
		focusMode: true,
	}
	for y, want := range []bool{false, false, true, true, false, false} {
		if got := e.focusedLine(y); got != want {
			t.Fatalf("focusedLine(%d) = %t, want %t", y, got, want)
		}
	}

	e.y = 1
	if !e.focusedLine(1) || e.focusedLine(0) || e.focusedLine(2) {
		t.Fatal("a blank current line should focus only itself")
	}
}

func TestScrollByClampsTop(t *testing.T) {
	e := &editor{top: 4}
	e.scrollBy(3, 5, 20)
	if e.top != 7 || !e.manualScroll {
		t.Fatalf("scrollBy() = top %d manualScroll %t", e.top, e.manualScroll)
	}
	e.scrollBy(50, 5, 20)
	if e.top != 19 {
		t.Fatalf("scrollBy clamp down = %d, want 19", e.top)
	}
	e.scrollBy(-100, 5, 20)
	if e.top != 0 {
		t.Fatalf("scrollBy clamp up = %d, want 0", e.top)
	}
}

func TestManualScrollCanPutFinalRowAtTop(t *testing.T) {
	if got, want := manualScrollMaxTop(20), 19; got != want {
		t.Fatalf("manualScrollMaxTop() = %d, want %d", got, want)
	}
	if got := manualScrollMaxTop(0); got != 0 {
		t.Fatalf("manualScrollMaxTop(0) = %d, want 0", got)
	}
}

func TestMouseReleaseDoesNotCancelManualScroll(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)
	e := &editor{screen: screen, lines: []string{"one", "two"}, manualScroll: true}
	e.mouse(tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone))
	if !e.manualScroll {
		t.Fatal("mouse release canceled manual scroll")
	}
}

func TestUndoRedo(t *testing.T) {
	e := &editor{lines: []string{"a"}, x: 1}
	e.checkpoint()
	e.insert("b")
	e.undoEdit()
	if e.lines[0] != "a" {
		t.Fatalf("undo = %q", e.lines[0])
	}
	e.redoEdit()
	if e.lines[0] != "ab" {
		t.Fatalf("redo = %q", e.lines[0])
	}
}

func TestSelectionText(t *testing.T) {
	e := &editor{lines: []string{"abc", "def"}, selX: 1, selY: 0, x: 2, y: 1, selecting: true}
	if got, want := e.selectionText(), "bc\nde"; got != want {
		t.Fatalf("selectionText() = %q, want %q", got, want)
	}
}

func TestFindPrevious(t *testing.T) {
	e := &editor{lines: []string{"one two one"}, search: "one", x: 11}
	e.findPrevious()
	if e.x != 8 {
		t.Fatalf("findPrevious x = %d, want 8", e.x)
	}
}

func TestReplaceAll(t *testing.T) {
	e := &editor{lines: []string{"old old", "old"}, search: "old", replace: "new"}
	e.replaceAll()
	want := []string{"new new", "new"}
	if !reflect.DeepEqual(e.lines, want) {
		t.Fatalf("replaceAll = %#v, want %#v", e.lines, want)
	}
}

func TestSearchHighlightPosition(t *testing.T) {
	e := &editor{lines: []string{"find this and this"}, search: "this"}
	if !e.positionMatchesSearch(6, 0) || !e.positionMatchesSearch(15, 0) {
		t.Fatal("expected both search matches to highlight")
	}
	if e.positionMatchesSearch(0, 0) {
		t.Fatal("non-match highlighted")
	}
}

func TestListPrefix(t *testing.T) {
	tests := map[string]string{
		"- item":       "- ",
		"  - [ ] task": "  - [ ] ",
		"9. item":      "10. ",
		"plain":        "",
	}
	for input, want := range tests {
		if got := listPrefix(input); got != want {
			t.Fatalf("listPrefix(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToggleCheckbox(t *testing.T) {
	e := &editor{lines: []string{"- [ ] task"}}
	e.toggleCheckbox()
	if e.lines[0] != "- [x] task" {
		t.Fatalf("toggleCheckbox = %q", e.lines[0])
	}
}

func TestTableRowEmpty(t *testing.T) {
	if !tableRowEmpty("|   |   |") {
		t.Fatal("empty table row was not detected")
	}
	if tableRowEmpty("| value |   |") {
		t.Fatal("non-empty table row was detected as empty")
	}
	if tableRowEmpty("| --- | --- |") {
		t.Fatal("separator row was detected as empty")
	}
}

func TestEnterLeavesEmptyFinalTableRow(t *testing.T) {
	e := &editor{lines: []string{"| A | B |", "| --- | --- |", "|   |   |"}, y: 2, x: 2}
	e.enter()
	if e.y != 3 || len(e.lines) != 4 || e.lines[3] != "" {
		t.Fatalf("enter() did not leave table: y=%d lines=%#v", e.y, e.lines)
	}
}

func TestExternalChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	e := &editor{path: path, modTime: info.ModTime()}
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	if !e.externalChange() {
		t.Fatal("external change was not detected")
	}
}

func TestLoadRecentRemovesMissingAndLimitsFive(t *testing.T) {
	oldConfig := os.Getenv("XDG_CONFIG_HOME")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	defer os.Setenv("XDG_CONFIG_HOME", oldConfig)

	var paths []string
	for i := 0; i < 6; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%d.md", i))
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	paths = append([]string{filepath.Join(dir, "missing.md")}, paths...)
	if err := os.MkdirAll(filepath.Dir(recentConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recentConfigPath(), []byte(strings.Join(paths, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	got := loadRecent()
	if len(got) != 5 || got[0] != paths[1] {
		t.Fatalf("loadRecent() = %#v", got)
	}
}

func TestRememberRecentMovesFileToFront(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	first := filepath.Join(dir, "first.md")
	second := filepath.Join(dir, "second.md")
	_ = os.WriteFile(first, nil, 0644)
	_ = os.WriteFile(second, nil, 0644)
	e := &editor{}
	e.rememberRecent(first)
	e.rememberRecent(second)
	got := loadRecent()
	if len(got) != 2 || got[0] != second || got[1] != first {
		t.Fatalf("recent ordering = %#v", got)
	}
}

func TestWritingArea(t *testing.T) {
	left, width := writingArea(120)
	if left != 16 || width != 88 {
		t.Fatalf("writingArea(120) = %d, %d; want 16, 88", left, width)
	}
	left, width = writingArea(60)
	if left != 2 || width != 56 {
		t.Fatalf("writingArea(60) = %d, %d; want 2, 56", left, width)
	}
}

func TestTypingReplacesSelection(t *testing.T) {
	e := &editor{lines: []string{"hello world"}, selX: 6, selY: 0, x: 11, y: 0, selecting: true}
	e.deleteSelection()
	e.insert("Marko")
	if got, want := e.lines[0], "hello Marko"; got != want {
		t.Fatalf("selection replacement = %q, want %q", got, want)
	}
}

func TestSelectWordAt(t *testing.T) {
	e := &editor{lines: []string{"hello Marko world"}}
	e.selectWordAt(8, 0)
	if got, want := e.selectionText(), "Marko"; got != want {
		t.Fatalf("selected word = %q, want %q", got, want)
	}
}

func TestSelectLineAt(t *testing.T) {
	e := &editor{lines: []string{"one", "select this", "three"}}
	e.selectLineAt(1)
	if got, want := e.selectionText(), "select this"; got != want {
		t.Fatalf("selected line = %q, want %q", got, want)
	}
}

func TestLightThemeHasWhiteBackground(t *testing.T) {
	if got := themeByName("light").background; got != tcell.ColorWhite {
		t.Fatalf("light background = %v, want white", got)
	}
}

func TestDeleteLine(t *testing.T) {
	e := &editor{lines: []string{"one", "two", "three"}, y: 1}
	e.deleteLine()
	want := []string{"one", "three"}
	if !reflect.DeepEqual(e.lines, want) {
		t.Fatalf("deleteLine = %#v, want %#v", e.lines, want)
	}
}

func TestReloadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	_ = os.WriteFile(path, []byte("new content"), 0644)
	e := &editor{path: path, lines: []string{"old content"}, dirty: true}
	e.reloadFile()
	if e.lines[0] != "new content" || e.dirty {
		t.Fatalf("reloadFile = %#v dirty=%v", e.lines, e.dirty)
	}
}

func TestRenameFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.md")
	newPath := filepath.Join(dir, "new.md")
	_ = os.WriteFile(oldPath, []byte("content"), 0644)
	e := &editor{path: oldPath, renameFrom: oldPath}
	e.renameFile(newPath)
	if e.path != newPath {
		t.Fatalf("rename path = %q", e.path)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatal(err)
	}
}

func TestJournalPathIsInsideConfig(t *testing.T) {
	got := journalPath("/tmp/example.md")
	if !strings.Contains(got, filepath.Join("marko", "recovery")) || !strings.HasSuffix(got, ".journal") {
		t.Fatalf("journalPath = %q", got)
	}
}
