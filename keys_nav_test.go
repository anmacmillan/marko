package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func ctrl(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModCtrl) }

func TestCtrlAAndCtrlEMoveToLineEdges(t *testing.T) {
	e := &editor{lines: []string{"hello world"}, x: 5}
	e.key(ctrl(tcell.KeyCtrlA))
	if e.x != 0 || e.selecting {
		t.Fatalf("Ctrl-A: x=%d selecting=%t, want 0 false", e.x, e.selecting)
	}
	e.key(ctrl(tcell.KeyCtrlE))
	if e.x != 11 {
		t.Fatalf("Ctrl-E: x=%d, want 11", e.x)
	}
	if e.lines[0] != "hello world" {
		t.Fatalf("line nav mutated text: %q", e.lines[0])
	}
}

func TestCtrlShiftASelectsAll(t *testing.T) {
	e := &editor{lines: []string{"one", "two"}, x: 1}
	e.key(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl|tcell.ModShift))
	if got := e.selectionText(); got != "one\ntwo" {
		t.Fatalf("Ctrl-Shift-A selection = %q", got)
	}
}

func TestCtrlUDeletesToLineStart(t *testing.T) {
	e := &editor{lines: []string{"hello world"}, x: 6}
	e.key(ctrl(tcell.KeyCtrlU))
	if got, want := e.lines[0], "world"; got != want {
		t.Fatalf("Ctrl-U: %q, want %q", got, want)
	}
	if e.x != 0 {
		t.Fatalf("Ctrl-U cursor = %d, want 0", e.x)
	}
	e.undoEdit()
	if got := e.lines[0]; got != "hello world" {
		t.Fatalf("undo after Ctrl-U = %q", got)
	}
}

func TestCtrlUAtLineStartJoinsLines(t *testing.T) {
	e := &editor{lines: []string{"one", "two"}, y: 1}
	e.key(ctrl(tcell.KeyCtrlU))
	if len(e.lines) != 1 || e.lines[0] != "onetwo" {
		t.Fatalf("Ctrl-U at line start = %#v", e.lines)
	}
}

func TestOptionArrowsMoveByWord(t *testing.T) {
	e := &editor{lines: []string{"the quick, brown fox", "next"}}
	e.key(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if e.x != 3 {
		t.Fatalf("Alt-Right #1 x=%d, want 3", e.x)
	}
	e.key(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if e.x != 9 {
		t.Fatalf("Alt-Right #2 x=%d, want 9", e.x)
	}
	// Ghostty's default Option-Left sends Esc-b.
	e.key(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt))
	if e.x != 4 {
		t.Fatalf("Esc-b x=%d, want 4", e.x)
	}
	e.x = 20
	e.key(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if e.y != 1 || e.x != 0 {
		t.Fatalf("Esc-f at line end = (%d,%d), want (0,1)", e.x, e.y)
	}
	if e.lines[0] != "the quick, brown fox" {
		t.Fatalf("word nav mutated text: %q", e.lines[0])
	}
}

func TestShiftOptionArrowSelectsWord(t *testing.T) {
	e := &editor{lines: []string{"alpha beta"}}
	e.key(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt|tcell.ModShift))
	if got := e.selectionText(); got != "alpha" {
		t.Fatalf("Shift-Alt-Right selection = %q", got)
	}
}

func TestOptionBackspaceDeletesWord(t *testing.T) {
	e := &editor{lines: []string{"alpha beta"}, x: 10}
	e.key(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModAlt))
	if got, want := e.lines[0], "alpha "; got != want {
		t.Fatalf("Alt-Backspace = %q, want %q", got, want)
	}
}

func TestShiftEndAndShiftHomeExtendSelection(t *testing.T) {
	e := &editor{lines: []string{"hello world"}, x: 6}
	e.key(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModShift))
	if got := e.selectionText(); got != "world" {
		t.Fatalf("Shift-End selection = %q", got)
	}
	e.key(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModShift))
	if got := e.selectionText(); got != "hello " {
		t.Fatalf("Shift-Home selection = %q", got)
	}
	e.key(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if e.selecting {
		t.Fatal("Esc did not clear the selection")
	}
}

func TestCtrlEndJumpsToDocumentEnd(t *testing.T) {
	e := &editor{lines: []string{"one", "two", "three"}}
	e.key(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if e.y != 2 || e.x != 5 {
		t.Fatalf("Ctrl-End = (%d,%d), want (5,2)", e.x, e.y)
	}
}

func TestCmdShortcuts(t *testing.T) {
	cmd := func(r rune, mod tcell.ModMask) *tcell.EventKey {
		return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModMeta|mod)
	}
	e := &editor{lines: []string{"plain"}, x: 5, selecting: true}
	e.key(cmd('b', 0))
	if got := e.lines[0]; got != "**plain**" {
		t.Fatalf("Cmd-B = %q", got)
	}
	e.key(cmd('z', 0))
	if got := e.lines[0]; got != "plain" {
		t.Fatalf("Cmd-Z = %q", got)
	}
	e.key(cmd('z', tcell.ModShift))
	if got := e.lines[0]; got != "**plain**" {
		t.Fatalf("Cmd-Shift-Z = %q", got)
	}
	// Cmd never types its letter into the document.
	e.key(cmd('k', 0))
	if got := e.lines[0]; got != "**plain**" {
		t.Fatalf("unbound Cmd-K typed into document: %q", got)
	}
}

func TestCmdArrowsAndBackspace(t *testing.T) {
	e := &editor{lines: []string{"one", "hello world"}, y: 1, x: 6}
	e.key(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModMeta))
	if e.x != 0 || e.y != 1 {
		t.Fatalf("Cmd-Left = (%d,%d)", e.x, e.y)
	}
	e.key(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModMeta))
	if e.y != 1 || e.x != 11 {
		t.Fatalf("Cmd-Down = (%d,%d)", e.x, e.y)
	}
	e.key(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModMeta))
	if got := e.lines[1]; got != "" {
		t.Fatalf("Cmd-Backspace = %q", got)
	}
}

func newMouseEditor(t *testing.T, lines []string) *editor {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 10)
	return &editor{screen: screen, lines: lines, theme: themeByName("default")}
}

func TestShiftClickExtendsSelection(t *testing.T) {
	e := newMouseEditor(t, []string{"alpha beta gamma"})
	left, _ := writingArea(80)
	e.mouse(tcell.NewEventMouse(left+2, 0, tcell.Button1, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left+2, 0, tcell.ButtonNone, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left+10, 0, tcell.Button1, tcell.ModShift))
	e.mouse(tcell.NewEventMouse(left+10, 0, tcell.ButtonNone, tcell.ModNone))
	if got := e.selectionText(); got != "pha beta" {
		t.Fatalf("shift-click selection = %q", got)
	}
}

func TestDragHeldAtBottomEdgeKeepsScrolling(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = "line"
	}
	e := newMouseEditor(t, lines)
	left, _ := writingArea(80)
	e.mouse(tcell.NewEventMouse(left, 2, tcell.Button1, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left, 8, tcell.Button1, tcell.ModNone))
	if e.dragEdge.Load() != 1 {
		t.Fatalf("dragEdge = %d at bottom row, want 1", e.dragEdge.Load())
	}
	top := e.top
	for range 5 {
		e.dragScroll()
	}
	if e.top != top+5 {
		t.Fatalf("top = %d after 5 drag ticks, want %d", e.top, top+5)
	}
	if !e.selecting || e.selY != 2 || e.y <= 8 {
		t.Fatalf("selection did not follow the scroll: selY=%d y=%d", e.selY, e.y)
	}
	e.mouse(tcell.NewEventMouse(left, 8, tcell.ButtonNone, tcell.ModNone))
	if e.dragEdge.Load() != 0 {
		t.Fatal("release did not stop edge scrolling")
	}
}
