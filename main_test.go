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

func TestSplitTablePreservesEscapedAndCodePipes(t *testing.T) {
	got := splitTable("| literal \\| pipe | `a | b` | final |")
	want := []string{"literal \\| pipe", "`a | b`", "final"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTable() = %#v, want %#v", got, want)
	}
}

func TestEscapedPipeDoesNotCreateTable(t *testing.T) {
	e := &editor{lines: []string{"Use A \\| B in prose"}}
	if e.inTable(0) {
		t.Fatal("escaped prose pipe detected as table")
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

func TestJamesDeanAwardTableRendersStyledAndAligned(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(88, 20)
	e := &editor{
		screen: screen,
		lines: []string{
			"| Head | Realistic award |",
			"| --- | --- |",
			"| Basic award | £1,057 |",
			"| Past loss (net of UC mitigation) | £48,839 |",
			"| Future loss (3 years net) | £51,067 |",
			"| Pension loss (past + 3yr future) | £22,968 |",
			"| BUPA + benefits | £27,373 |",
			"| Loss of statutory rights | £600 |",
			"| PTSD PSLA (moderately severe, 18th ed. JCG) | £44,000 |",
			"| Injury to feelings (upper Vento) | £36,400 |",
			"| Aggravated damages | £4,000 |",
			"| **Sub-total** | **£236,304** |",
			"| Less Polkey (30% on UD-attributable loss only) | (£10,500) |",
			"| Plus ACAS uplift (10%) | £14,500 |",
			"| **Gross** | **£231,304** |",
			"| Plus interest (~3yr at 8%) | £20,000 |",
			"| **Total gross** | **~£251,000** |",
			"",
		},
		y:     17,
		theme: themeByName("calm"),
	}
	for y := 0; y < 17; y++ {
		e.drawLine(0, y, y, e.lines[y], false, 88)
	}
	wantBorders := tableBorderPositions(simulationLine(screen, 0, 88))
	for y := 2; y < 17; y++ {
		if got := tableBorderPositions(simulationLine(screen, y, 88)); !reflect.DeepEqual(got, wantBorders) {
			t.Fatalf("row %d borders = %v, want %v: %q", y, got, wantBorders, simulationLine(screen, y, 88))
		}
	}
	for _, y := range []int{11, 14, 16} {
		line := simulationLine(screen, y, 88)
		if strings.Contains(line, "**") {
			t.Fatalf("row %d exposed bold markers: %q", y, line)
		}
	}
}

func TestRenderedZOPAUsesColor(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 8)
	e := &editor{
		screen: screen,
		lines: []string{
			"```zopa",
			"Claimant target: 100000",
			"Claimant minimum: 80000",
			"Respondent maximum: 95000",
			"Respondent offer: 70000",
			"```",
			"",
		},
		y:     6,
		theme: themeByName("calm"),
	}
	rows := e.visualRows(80)
	for row, vr := range rows[:5] {
		e.drawVisualLine(0, row, vr, false, 80)
	}
	_, _, headerStyle, _ := screen.GetContent(0, 0)
	fg, _, attrs := headerStyle.Decompose()
	if fg != tcell.ColorLightGoldenrodYellow || attrs&tcell.AttrBold == 0 {
		t.Fatalf("zopa header style = fg %v attrs %v", fg, attrs)
	}
	_, _, respondentStyle, _ := screen.GetContent(0, 1)
	fg, _, _ = respondentStyle.Decompose()
	if fg != tcell.ColorLightSeaGreen {
		t.Fatalf("zopa respondent style = fg %v", fg)
	}
	_, _, claimantStyle, _ := screen.GetContent(0, 2)
	fg, _, _ = claimantStyle.Decompose()
	if fg != tcell.ColorLightCoral {
		t.Fatalf("zopa claimant style = fg %v", fg)
	}
}

func TestFullDrawPipelineRendersBoldTableRows(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(88, 10)
	e := &editor{
		screen: screen,
		lines: []string{
			"| Head | Realistic award |",
			"| --- | --- |",
			"| Basic award | £1,057 |",
			"| **Sub-total** | **£236,304** |",
			"| **Gross** | **£231,304** |",
			"| **Total gross** | **~£251,000** |",
			"",
		},
		y:     6,
		theme: themeByName("calm"),
	}
	rows := e.visualRows(88)
	for row, vr := range rows {
		e.drawVisualLine(0, row, vr, false, 88)
	}
	wantBorders := tableBorderPositions(simulationLine(screen, 0, 88))
	for y := 2; y < 6; y++ {
		line := simulationLine(screen, y, 88)
		if strings.Contains(line, "**") {
			t.Fatalf("full draw row %d exposed bold markers: %q", y, line)
		}
		if got := tableBorderPositions(line); !reflect.DeepEqual(got, wantBorders) {
			t.Fatalf("full draw row %d borders = %v, want %v: %q", y, got, wantBorders, line)
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

func TestBlockQuote(t *testing.T) {
	if got, ok := blockQuote("  > Important point"); !ok || got != "Important point" {
		t.Fatalf("blockQuote() = %q, %t", got, ok)
	}
	if _, ok := blockQuote("ordinary prose"); ok {
		t.Fatal("ordinary prose detected as block quote")
	}
}

func TestRenderedBlockQuoteHidesMarker(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 4)
	e := &editor{
		screen: screen,
		lines:  []string{"> Important point", ""},
		y:      1,
		theme:  themeByName("calm"),
	}
	rows := e.visualRows(40)
	e.drawVisualLine(0, 0, rows[0], false, 40)
	if got, want := strings.TrimRight(simulationLine(screen, 0, 40), " "), "│ Important point"; got != want {
		t.Fatalf("rendered quote = %q, want %q", got, want)
	}
}

func TestZOPABlockRendersOutsideFence(t *testing.T) {
	e := &editor{
		lines: []string{
			"```zopa",
			"Claimant target: 100000",
			"Claimant minimum: 80000",
			"Respondent maximum: 95000",
			"Respondent offer: 70000",
			"```",
			"",
		},
		y: 6,
	}
	rows := e.visualRows(70)
	if got, want := len(rows), 6; got != want {
		t.Fatalf("visualRows() = %d rows, want %d", got, want)
	}
	if got, want := rows[0].text, "Settlement range · ZOPA £80k–£95k"; got != want {
		t.Fatalf("ZOPA heading = %q, want %q", got, want)
	}
	if !strings.Contains(rows[1].text, "Respondent") {
		t.Fatalf("ZOPA has no Respondent row: %q", rows[1].text)
	}
	if !strings.Contains(rows[2].text, "Claimant") {
		t.Fatalf("ZOPA has no Claimant row: %q", rows[2].text)
	}
}

func TestChartBlockRendering(t *testing.T) {
	e := &editor{
		lines: []string{
			"```chart Project Progress",
			"Backend API: 80",
			"Frontend UI: 40",
			"Testing: 20",
			"```",
			"",
		},
		y: 5,
	}
	rows := e.visualRows(70)
	if got, want := len(rows), 5; got != want {
		t.Fatalf("visualRows() = %d rows, want %d", got, want)
	}
	if got, want := rows[0].text, "📊 Project Progress"; got != want {
		t.Fatalf("chart block title = %q, want %q", got, want)
	}
	if !strings.Contains(rows[1].text, "Backend API") || !strings.Contains(rows[1].text, "█") {
		t.Fatalf("chart block row 1 is incorrect: %q", rows[1].text)
	}
}

func TestIsRule(t *testing.T) {
	if !isRule("---") || !isRule("  ***  ") || !isRule("________") {
		t.Fatal("isRule failed to detect valid rules")
	}
	if isRule("--- text") || isRule("abc") || isRule("-") {
		t.Fatal("isRule detected invalid line as rule")
	}
}

func TestZOPABlockShowsSourceWhileEditing(t *testing.T) {
	e := &editor{
		lines: []string{
			"```zopa",
			"Claimant target: 100000",
			"Claimant minimum: 80000",
			"Respondent maximum: 95000",
			"Respondent offer: 70000",
			"```",
		},
		y: 2,
	}
	rows := e.visualRows(70)
	if got, want := len(rows), 6; got != want {
		t.Fatalf("editing visualRows() = %d rows, want %d", got, want)
	}
	if rows[0].text != "```zopa" {
		t.Fatalf("editing first row = %q", rows[0].text)
	}
}

func TestZOPANoOverlap(t *testing.T) {
	lines := renderZOPA(zopaChart{claimantTarget: 120000, claimantMinimum: 100000, respondentMaximum: 90000, respondentOffer: 70000}, 70)
	if got, want := lines[0], "Settlement range · No ZOPA"; got != want {
		t.Fatalf("no-overlap heading = %q, want %q", got, want)
	}
}

func TestOrdinaryCodeFenceRendersCalmBlock(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 6)
	e := &editor{
		screen: screen,
		lines:  []string{"```go", `fmt.Println("hello")`, "```", ""},
		y:      3,
		theme:  themeByName("calm"),
	}
	rows := e.visualRows(40)
	if got, want := len(rows), 4; got != want {
		t.Fatalf("code visualRows() = %d, want %d", got, want)
	}
	for row, vr := range rows[:3] {
		e.drawVisualLine(0, row, vr, false, 40)
	}
	if got := strings.TrimRight(simulationLine(screen, 0, 40), " "); got != "┌ code · go" {
		t.Fatalf("code header = %q", got)
	}
	if got := strings.TrimRight(simulationLine(screen, 1, 40), " "); got != `│ fmt.Println("hello")` {
		t.Fatalf("code body = %q", got)
	}
}

func TestIncompleteCodeFenceRemainsRaw(t *testing.T) {
	e := &editor{lines: []string{"```go", "fmt.Println()"}, y: 1}
	rows := e.visualRows(40)
	if got, want := rows[0].text, "```go"; got != want {
		t.Fatalf("incomplete fence = %q, want %q", got, want)
	}
}

func TestCodeFenceShowsSourceWhileEditing(t *testing.T) {
	e := &editor{lines: []string{"```go", "fmt.Println()", "```"}, y: 1}
	rows := e.visualRows(40)
	if got, want := rows[0].text, "```go"; got != want {
		t.Fatalf("editing code fence = %q, want %q", got, want)
	}
}

func TestEnterExpandsZOPAChartWithExampleValues(t *testing.T) {
	e := &editor{lines: []string{"```zopa", ""}, x: 7}
	e.enter()
	want := []string{
		"```zopa",
		"Claimant target: 100000",
		"Claimant minimum: 80000",
		"Respondent maximum: 95000",
		"Respondent offer: 70000",
		"```",
		"",
	}
	if !reflect.DeepEqual(e.lines, want) {
		t.Fatalf("expanded ZOPA = %#v, want %#v", e.lines, want)
	}
	if e.y != 1 || e.x != runeLen("Claimant target: ") {
		t.Fatalf("cursor = (%d,%d)", e.x, e.y)
	}
}

func TestEnterDoesNotExpandUnsupportedFence(t *testing.T) {
	e := &editor{lines: []string{"```mermaid"}, x: 10}
	e.enter()
	if got, want := e.lines, []string{"```mermaid", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unsupported fence = %#v, want %#v", got, want)
	}
}

func TestTabMovesThroughZOPAValues(t *testing.T) {
	e := &editor{
		lines: []string{
			"```zopa",
			"Claimant target: 100000",
			"Claimant minimum: 80000",
			"Respondent maximum: 95000",
			"Respondent offer: 70000",
			"```",
		},
		y: 1,
		x: 17,
	}
	for _, wantY := range []int{2, 3, 4, 1} {
		if !e.nextTableCell() {
			t.Fatal("Tab did not handle ZOPA chart")
		}
		if e.y != wantY {
			t.Fatalf("Tab y = %d, want %d", e.y, wantY)
		}
		if e.x != strings.Index(e.lines[e.y], ":")+2 {
			t.Fatalf("Tab x = %d on line %q", e.x, e.lines[e.y])
		}
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

func TestInlineCodeRendering(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 3)
	e := &editor{screen: screen}
	e.putInline(0, 0, "Use `marko note.md` now", tcell.StyleDefault, 40, 0, 0)
	if got, want := strings.TrimRight(simulationLine(screen, 0, 40), " "), "Use marko note.md now"; got != want {
		t.Fatalf("inline code rendering = %q, want %q", got, want)
	}
	_, _, style, _ := screen.GetContent(4, 0)
	fg, bg, _ := style.Decompose()
	if fg != tcell.ColorLightGoldenrodYellow || bg != tcell.ColorDarkSlateGray {
		t.Fatalf("inline code style = fg %v bg %v", fg, bg)
	}
}

func TestClosingRune(t *testing.T) {
	if got, want := closingRune([]rune("`code`"), 1, '`'), 5; got != want {
		t.Fatalf("closingRune() = %d, want %d", got, want)
	}
	if got := closingRune([]rune("`open"), 1, '`'); got != -1 {
		t.Fatalf("closingRune() = %d, want -1", got)
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

func TestSelectionTextAcrossParagraphs(t *testing.T) {
	e := &editor{
		lines:     []string{"one", "", "two", "", "three"},
		selX:      0,
		selY:      0,
		x:         5,
		y:         4,
		selecting: true,
	}
	if got, want := e.selectionText(), "one\n\ntwo\n\nthree"; got != want {
		t.Fatalf("multi-paragraph selectionText() = %q, want %q", got, want)
	}
}

func TestSelectionBoundsClampBeforeSlicing(t *testing.T) {
	e := &editor{lines: []string{"short", "end"}, selX: 50, selY: 0, x: 20, y: 1, selecting: true}
	if got, want := e.selectionText(), "\nend"; got != want {
		t.Fatalf("clamped selectionText() = %q, want %q", got, want)
	}
	e.deleteSelection()
	if got, want := e.lines, []string{"short"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clamped deleteSelection() = %#v, want %#v", got, want)
	}
}

func TestCtrlCCopyDoesNotMutateSelection(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	e := &editor{
		screen:    screen,
		lines:     []string{"alpha", "", "bravo"},
		selX:      0,
		selY:      0,
		x:         5,
		y:         2,
		selecting: true,
	}
	before := append([]string(nil), e.lines...)
	e.key(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))
	if !reflect.DeepEqual(e.lines, before) {
		t.Fatalf("Ctrl-C mutated lines: %#v, want %#v", e.lines, before)
	}
	if !e.selecting {
		t.Fatal("Ctrl-C cleared selection")
	}
}

func TestSelectedBlankLineIsHighlighted(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 3)
	e := &editor{
		screen:    screen,
		lines:     []string{"one", "", "two"},
		x:         3,
		y:         2,
		selX:      0,
		selY:      0,
		selecting: true,
		theme:     themeByName("calm"),
	}
	e.drawLine(0, 0, 1, e.lines[1], false, 20)
	_, _, style, _ := screen.GetContent(0, 0)
	_, bg, _ := style.Decompose()
	if bg != tcell.ColorDodgerBlue {
		t.Fatalf("selected blank line background = %v, want %v", bg, tcell.ColorDodgerBlue)
	}
}

func TestMouseDragSelectsAcrossParagraphs(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 10)
	e := &editor{
		screen: screen,
		lines:  []string{"one", "", "two", "", "three"},
		theme:  themeByName("calm"),
	}
	left, _ := writingArea(80)
	e.mouse(tcell.NewEventMouse(left, 0, tcell.Button1, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left+5, 4, tcell.Button1, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left+5, 4, tcell.ButtonNone, tcell.ModNone))
	if got, want := e.selectionText(), "one\n\ntwo\n\nthree"; got != want {
		t.Fatalf("mouse multi-paragraph selection = %q, want %q", got, want)
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

func TestPastingURLOverSelectionCreatesLink(t *testing.T) {
	e := &editor{lines: []string{"Read the judgment"}, selX: 9, selY: 0, x: 17, y: 0, selecting: true}
	e.insertText("https://example.com/judgment")
	if got, want := e.lines[0], "Read the [judgment](https://example.com/judgment)"; got != want {
		t.Fatalf("smart URL paste = %q, want %q", got, want)
	}
	if e.status != "Created link" {
		t.Fatalf("smart URL paste status = %q", e.status)
	}
}

func TestPastingNonURLStillReplacesSelection(t *testing.T) {
	e := &editor{lines: []string{"hello world"}, selX: 6, selY: 0, x: 11, y: 0, selecting: true}
	e.insertText("Marko")
	if got, want := e.lines[0], "hello Marko"; got != want {
		t.Fatalf("ordinary paste = %q, want %q", got, want)
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

func TestAutosaveWritesThenRemovesRecoveryJournal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "doc.md")
	e := &editor{path: path, lines: []string{"recovered £ text"}, dirty: true, lastEdit: time.Now().Add(-3 * time.Second)}
	e.autosave()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "recovered £ text"; got != want {
		t.Fatalf("autosaved content = %q, want %q", got, want)
	}
	if _, err := os.Stat(journalPath(path)); !os.IsNotExist(err) {
		t.Fatalf("recovery journal remains after save: %v", err)
	}
}

func TestSaveProtectsExternalChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	e := &editor{path: path, lines: []string{"local"}, dirty: true, modTime: old}
	e.save()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "external" || !e.conflict {
		t.Fatalf("external change overwritten: data=%q conflict=%t", data, e.conflict)
	}
}
