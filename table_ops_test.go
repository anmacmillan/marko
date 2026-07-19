package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func gridTable() []string {
	return []string{
		"| Name | Role |",
		"| --- | --- |",
		"| Alex | Barrister |",
		"| Sam | Clerk |",
	}
}

func TestTableColumnAt(t *testing.T) {
	line := "| Alex | Barrister |"
	if got := tableColumnAt(line, 3); got != 0 {
		t.Fatalf("tableColumnAt(3) = %d, want 0", got)
	}
	if got := tableColumnAt(line, 10); got != 1 {
		t.Fatalf("tableColumnAt(10) = %d, want 1", got)
	}
	// A pipe inside a code span does not start a new column.
	code := "| `a|b` | second |"
	if got := tableColumnAt(code, 7); got != 0 {
		t.Fatalf("tableColumnAt inside code span = %d, want 0", got)
	}
}

func TestInsertTableColumn(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.insertTableColumn()
	cells := splitTable(e.lines[0])
	if !reflect.DeepEqual(cells, []string{"Name", "", "Role"}) {
		t.Fatalf("header after insert = %#v", cells)
	}
	if !isSeparator(e.lines[1]) {
		t.Fatalf("separator broken after insert: %q", e.lines[1])
	}
	if got := e.currentTableColumn(); got != 1 {
		t.Fatalf("cursor column after insert = %d, want 1", got)
	}
}

func TestDeleteTableColumn(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.deleteTableColumn()
	cells := splitTable(e.lines[0])
	if !reflect.DeepEqual(cells, []string{"Role"}) {
		t.Fatalf("header after delete = %#v", cells)
	}
	e2 := &editor{lines: []string{"| Only |", "| --- |", "| x |"}, y: 2, x: 2}
	e2.deleteTableColumn()
	if len(splitTable(e2.lines[0])) != 1 {
		t.Fatal("deleted the only column")
	}
	if !strings.Contains(e2.status, "only column") {
		t.Fatalf("status = %q, want only-column refusal", e2.status)
	}
}

func TestInsertTableRow(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.insertTableRow()
	if len(e.lines) != 5 {
		t.Fatalf("lines after insert = %d, want 5", len(e.lines))
	}
	if e.y != 3 {
		t.Fatalf("cursor row after insert = %d, want 3", e.y)
	}
	if cells := splitTable(e.lines[3]); len(cells) != 2 || strings.TrimSpace(cells[0]) != "" {
		t.Fatalf("new row cells = %#v, want two empty cells", cells)
	}
	// From the header the new row lands below the separator.
	e2 := &editor{lines: gridTable(), y: 0, x: 2}
	e2.insertTableRow()
	if e2.y != 2 {
		t.Fatalf("header insert row cursor = %d, want 2", e2.y)
	}
	if cells := splitTable(e2.lines[2]); strings.TrimSpace(strings.Join(cells, "")) != "" {
		t.Fatalf("header insert produced non-empty row: %#v", cells)
	}
}

func TestDeleteTableRow(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.deleteTableRow()
	if len(e.lines) != 3 {
		t.Fatalf("lines after delete = %d, want 3", len(e.lines))
	}
	for _, line := range e.lines {
		if strings.Contains(line, "Alex") {
			t.Fatalf("deleted row still present: %q", line)
		}
	}
	e2 := &editor{lines: gridTable(), y: 0, x: 2}
	e2.deleteTableRow()
	if len(e2.lines) != 4 {
		t.Fatal("header row was deleted")
	}
	if !strings.Contains(e2.status, "header") {
		t.Fatalf("status = %q, want header refusal", e2.status)
	}
}

func TestCycleColumnAlignmentPreservedByFormat(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.cycleColumnAlignment() // none -> left
	if got := e.tableAlignments(2); len(got) == 0 || got[0] != 1 {
		t.Fatalf("alignments after first cycle = %#v, want left", got)
	}
	e.cycleColumnAlignment() // left -> center
	e.cycleColumnAlignment() // center -> right
	if got := e.tableAlignments(2); got[0] != 3 {
		t.Fatalf("alignments after third cycle = %#v, want right", got)
	}
	sep := e.lines[1]
	if !strings.Contains(sep, "-:") || strings.Contains(strings.Split(sep, "|")[1], ":-") {
		t.Fatalf("separator does not encode right alignment: %q", sep)
	}
	// formatTable must not destroy the alignment markers.
	e.formatTable()
	if got := e.tableAlignments(2); got[0] != 3 {
		t.Fatalf("formatTable lost alignment: %#v", got)
	}
	// Right-aligned values are padded from the left in the source.
	nameCell := splitTable(e.lines[2])[0]
	if nameCell != "Alex" {
		t.Fatalf("splitTable trimmed cell = %q", nameCell)
	}
	raw := strings.Split(e.lines[2], "|")[1]
	if !strings.HasSuffix(raw, "Alex ") {
		t.Fatalf("right-aligned cell not padded left: %q", raw)
	}
}

func TestAlignmentCellRoundTrip(t *testing.T) {
	for align := 0; align <= 3; align++ {
		cell := alignmentCell(align, 7)
		if got := alignmentOf(cell); got != align {
			t.Fatalf("alignmentOf(alignmentCell(%d)) = %d", align, got)
		}
		if runeLen(cell) != 7 {
			t.Fatalf("alignmentCell(%d, 7) width = %d", align, runeLen(cell))
		}
	}
}

func TestPadCellAligned(t *testing.T) {
	if got := padCellAligned("ab", 6, 0); got != "ab    " {
		t.Fatalf("left pad = %q", got)
	}
	if got := padCellAligned("ab", 6, 3); got != "    ab" {
		t.Fatalf("right pad = %q", got)
	}
	if got := padCellAligned("ab", 6, 2); got != "  ab  " {
		t.Fatalf("center pad = %q", got)
	}
}

func TestCtrlTOpensGridPickerOutsideTable(t *testing.T) {
	e := &editor{lines: []string{"prose"}}
	e.key(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if e.tableGrid == nil {
		t.Fatal("Ctrl-T outside a table did not open the grid picker")
	}
	e.key(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	e.key(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if e.tableGrid.cols != 4 || e.tableGrid.rows != 4 {
		t.Fatalf("grid size = %dx%d, want 4x4", e.tableGrid.cols, e.tableGrid.rows)
	}
	e.key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if e.tableGrid != nil {
		t.Fatal("grid picker stayed open after Enter")
	}
	if len(e.lines) != 5 { // header + separator + 3 body rows
		t.Fatalf("inserted table lines = %d, want 5", len(e.lines))
	}
	if got := len(splitTable(e.lines[0])); got != 4 {
		t.Fatalf("inserted table columns = %d, want 4", got)
	}
}

func TestCtrlTInsideTableCyclesAlignment(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.key(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if e.tableGrid != nil {
		t.Fatal("Ctrl-T inside a table opened the grid picker")
	}
	if got := e.tableAlignments(2); len(got) == 0 || got[0] != 1 {
		t.Fatalf("Ctrl-T inside table did not cycle alignment: %#v", got)
	}
}

func TestAltArrowsEditTableStructure(t *testing.T) {
	e := &editor{lines: gridTable(), y: 2, x: 2}
	e.key(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	if len(e.lines) != 5 {
		t.Fatalf("Alt-Down lines = %d, want 5", len(e.lines))
	}
	e.key(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt))
	if len(e.lines) != 4 {
		t.Fatalf("Alt-Up lines = %d, want 4", len(e.lines))
	}
	e.key(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if got := len(splitTable(e.lines[0])); got != 3 {
		t.Fatalf("Alt-Right columns = %d, want 3", got)
	}
	e.key(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt))
	if got := len(splitTable(e.lines[0])); got != 2 {
		t.Fatalf("Alt-Left columns = %d, want 2", got)
	}
	// Outside a table Alt-arrows fall through to normal movement.
	e2 := &editor{lines: []string{"one", "two"}, y: 1}
	e2.key(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt))
	if len(e2.lines) != 2 {
		t.Fatal("Alt-Up outside table mutated the document")
	}
}

func TestFuzzyScore(t *testing.T) {
	if _, ok := fuzzyScore("xyz", "meeting.md"); ok {
		t.Fatal("fuzzyScore matched non-subsequence")
	}
	if _, ok := fuzzyScore("mtng", "meeting-notes.md"); !ok {
		t.Fatal("fuzzyScore rejected valid subsequence")
	}
	exact, _ := fuzzyScore("meet", "meeting.md")
	scattered, _ := fuzzyScore("meet", "my-elegant-etchings-thing.md")
	if exact <= scattered {
		t.Fatalf("consecutive match %d not ranked above scattered %d", exact, scattered)
	}
	if _, ok := fuzzyScore("MEET", "meeting.md"); !ok {
		t.Fatal("fuzzyScore is case sensitive")
	}
}

func TestBlendColor(t *testing.T) {
	got := blendColor(tcell.NewRGBColor(200, 100, 0), tcell.NewRGBColor(0, 100, 200), 0.5)
	r, g, b := got.RGB()
	if r != 100 || g != 100 || b != 100 {
		t.Fatalf("blendColor midpoint = %d,%d,%d, want 100,100,100", r, g, b)
	}
	if got := blendColor(tcell.ColorDefault, tcell.NewRGBColor(0, 0, 0), 0.5); got != tcell.ColorDefault {
		t.Fatalf("blendColor with invalid input = %v, want unchanged", got)
	}
}

func TestFocusStyleDimsForegroundTowardBackground(t *testing.T) {
	e := &editor{focusMode: true, theme: themeByName("calm")}
	base := tcell.StyleDefault.Foreground(e.theme.text).Background(e.theme.background)
	dimmed := e.focusStyle(base, false)
	fg, _, _ := dimmed.Decompose()
	if fg == e.theme.text {
		t.Fatal("focusStyle did not dim the foreground")
	}
	if fg == e.theme.muted {
		t.Fatal("focusStyle fell back to flat muted colour for an RGB theme")
	}
	// Soft edge: adjacent rows dim less than distant rows.
	e.dimStrength = 0.5
	nearFg, _, _ := e.focusStyle(base, false).Decompose()
	e.dimStrength = 1
	farFg, _, _ := e.focusStyle(base, false).Decompose()
	tr, tg, tb := e.theme.text.RGB()
	nr, ng, nb := nearFg.RGB()
	fr, fgc, fb := farFg.RGB()
	nearDist := abs32(tr-nr) + abs32(tg-ng) + abs32(tb-nb)
	farDist := abs32(tr-fr) + abs32(tg-fgc) + abs32(tb-fb)
	if nearDist >= farDist {
		t.Fatalf("adjacent dim (%d) not softer than distant dim (%d)", nearDist, farDist)
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func TestSectionBoundsAndScopeCycle(t *testing.T) {
	e := &editor{lines: []string{"# One", "a", "b", "", "## Two", "c"}, y: 2}
	start, end := e.sectionBounds(2)
	if start != 0 || end != 3 {
		t.Fatalf("sectionBounds = %d,%d, want 0,3", start, end)
	}
	e.focusMode = false
	e.key(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl|tcell.ModShift))
	if !e.focusMode || e.focusScope != 1 {
		t.Fatalf("Ctrl-Shift-K: focusMode=%v scope=%d, want section scope on", e.focusMode, e.focusScope)
	}
	if !e.focusedLine(0) || !e.focusedLine(3) || e.focusedLine(4) {
		t.Fatal("section scope focusedLine mismatch")
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl|tcell.ModShift))
	if e.focusScope != 0 {
		t.Fatalf("second Ctrl-Shift-K scope = %d, want paragraph", e.focusScope)
	}
}

func TestDimStrengthForSoftEdge(t *testing.T) {
	if got := dimStrengthFor(5, 4, 6); got != 0 {
		t.Fatalf("inside focus dim = %v, want 0", got)
	}
	if got := dimStrengthFor(3, 4, 6); got != 0.5 {
		t.Fatalf("adjacent dim = %v, want 0.5", got)
	}
	if got := dimStrengthFor(0, 4, 6); got != 1 {
		t.Fatalf("distant dim = %v, want 1", got)
	}
}

func TestTypewriterScrollCentresCursorInFocusMode(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	e := &editor{screen: screen, lines: lines, theme: themeByName("ia-dark"), focusMode: true, y: 40}
	e.draw()
	// Cursor row 40 should sit near the vertical centre, not at the bottom.
	centre := 40 - e.top
	if centre < 8 || centre > 16 {
		t.Fatalf("cursor drawn at screen row %d, want near centre of 24-row screen", centre)
	}
	// Without focus mode the cursor is only kept on screen, near the bottom.
	e2 := &editor{screen: screen, lines: lines, theme: themeByName("ia-dark"), y: 40}
	e2.draw()
	if bottom := 40 - e2.top; bottom < 17 {
		t.Fatalf("non-focus scroll unexpectedly centred cursor (row %d)", bottom)
	}
}
