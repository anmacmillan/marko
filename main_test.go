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

func TestLightThemeHasWhiteBackground(t *testing.T) {
	if got := themeByName("light").background; got != tcell.ColorWhite {
		t.Fatalf("light background = %v, want white", got)
	}
}
