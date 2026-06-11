package main

import (
	"os"
	"reflect"
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
