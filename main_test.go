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
		{"==mark==", 2, 6},
		{"<u>under</u>", 3, 8},
		{"plain", 0, 0},
	}
	for _, test := range tests {
		size, _, _, style, end := emphasisAt([]rune(test.text), 0)
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

func TestInlineCodeUsesThemePalette(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 3)
	e := &editor{screen: screen, lines: []string{"`code`"}, theme: themeByName("midnight")}
	e.putInline(0, 0, "`code`", tcell.StyleDefault, 20, 0, 0)
	_, _, style, _ := screen.GetContent(0, 0)
	fg, bg, _ := style.Decompose()
	if fg != e.theme.codeFG || bg != e.theme.codeBG {
		t.Fatalf("inline code style = fg %v bg %v, want fg %v bg %v", fg, bg, e.theme.codeFG, e.theme.codeBG)
	}
}

func TestCurrentLineRendersInlineMarkdown(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(90, 3)
	e := &editor{
		screen: screen,
		lines:  []string{`**His case:** That possibility defeats the *Dobie* test.`},
		theme:  themeByName("calm"),
	}
	e.drawLine(0, 0, 0, e.lines[0], true, 90)
	got := strings.TrimRight(simulationLine(screen, 0, 90), " ")
	want := "His case: That possibility defeats the Dobie test."
	if got != want {
		t.Fatalf("current inline markdown render = %q, want %q", got, want)
	}
	if strings.ContainsAny(got, "*_") {
		t.Fatalf("current inline markdown exposed marker: %q", got)
	}
	mainc, _, style, _ := screen.GetContent(0, 0)
	if mainc != 'H' {
		t.Fatalf("first rendered character = %q, want H", mainc)
	}
	_, _, attrs := style.Decompose()
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("current bold marker did not apply bold style")
	}
}

func TestHighlightUsesThemePalette(t *testing.T) {
	for _, name := range []string{"matrix", "midnight", "paper", "ember"} {
		t.Run(name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			defer screen.Fini()
			screen.SetSize(40, 3)
			e := &editor{
				screen: screen,
				lines:  []string{"==marked=="},
				theme:  themeByName(name),
			}
			e.drawLine(0, 0, 0, e.lines[0], true, 40)
			_, _, style, _ := screen.GetContent(0, 0)
			fg, bg, _ := style.Decompose()
			if fg != e.theme.highlightFG || bg != e.theme.highlightBG {
				t.Fatalf("highlight style = fg %v bg %v, want fg %v bg %v", fg, bg, e.theme.highlightFG, e.theme.highlightBG)
			}
		})
	}
}

func TestCurrentHeadingHidesMarker(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(90, 3)
	e := &editor{
		screen: screen,
		lines:  []string{`### 4. "Even if misdirection, the outcome would be the same."`},
		theme:  themeByName("calm"),
	}
	e.drawLine(0, 0, 0, e.lines[0], true, 90)
	got := strings.TrimRight(simulationLine(screen, 0, 90), " ")
	want := `4. "Even if misdirection, the outcome would be the same."`
	if got != want {
		t.Fatalf("current heading render = %q, want %q", got, want)
	}
}

func TestHeadingOneUsesBoldWithoutUnderline(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(90, 3)
	e := &editor{
		screen: screen,
		lines:  []string{"# Heading one"},
		theme:  themeByName("ember"),
	}
	e.drawLine(0, 0, 0, e.lines[0], false, 90)
	mainc, _, style, _ := screen.GetContent(0, 0)
	if mainc != 'H' {
		t.Fatalf("heading text = %q, want H", mainc)
	}
	fg, _, attrs := style.Decompose()
	if fg != e.theme.heading1 {
		t.Fatalf("heading foreground = %v, want %v", fg, e.theme.heading1)
	}
	if attrs&tcell.AttrBold == 0 {
		t.Fatal("heading one did not render bold")
	}
	if attrs&tcell.AttrUnderline != 0 {
		t.Fatal("heading one should not be underlined")
	}
}

func TestHeadingSpacingAddsBlankRows(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(40, 6)
	e := &editor{
		screen: screen,
		lines:  []string{"# Alpha", "bravo"},
		theme:  themeByName("calm"),
	}
	rows := e.visualRows(40)
	if got, want := len(rows), 4; got != want {
		t.Fatalf("visualRows() = %d, want %d", got, want)
	}
	if rows[0].start != -1 || rows[2].start != -1 {
		t.Fatalf("heading spacing rows not inserted correctly: %#v", rows)
	}
	e.draw()
	if got := strings.TrimRight(simulationLine(screen, 0, 40), " "); got != "" {
		t.Fatalf("top heading spacer rendered as %q, want blank", got)
	}
	if got := strings.TrimSpace(simulationLine(screen, 1, 40)); got != "Alpha" {
		t.Fatalf("heading row rendered as %q, want Alpha", got)
	}
	if got := strings.TrimRight(simulationLine(screen, 2, 40), " "); got != "" {
		t.Fatalf("bottom heading spacer rendered as %q, want blank", got)
	}
}

func TestHeadingSpacingSkipsVisualNavigationRows(t *testing.T) {
	e := &editor{
		lines: []string{"# Alpha", "bravo"},
		x:     0,
		y:     0,
	}
	e.moveVisualVertical(1)
	if e.y != 1 || e.x != 0 {
		t.Fatalf("moveVisualVertical down = (%d,%d), want (1,0)", e.y, e.x)
	}
	e.moveVisualVertical(-1)
	if e.y != 0 || e.x != 0 {
		t.Fatalf("moveVisualVertical up = (%d,%d), want (0,0)", e.y, e.x)
	}
}

func TestModifiedRuneDoesNotReplaceSelection(t *testing.T) {
	for _, mod := range []tcell.ModMask{tcell.ModAlt, tcell.ModMeta, tcell.ModHyper} {
		e := &editor{
			lines:     []string{"alpha", "bravo"},
			selX:      0,
			selY:      0,
			x:         5,
			y:         1,
			selecting: true,
		}
		before := append([]string(nil), e.lines...)
		e.key(tcell.NewEventKey(tcell.KeyRune, 'c', mod))
		if !reflect.DeepEqual(e.lines, before) {
			t.Fatalf("modified rune with mod %v mutated lines: %#v, want %#v", mod, e.lines, before)
		}
		if !e.selecting {
			t.Fatalf("modified rune with mod %v cleared selection", mod)
		}
		if len(e.undo) != 0 {
			t.Fatalf("modified rune with mod %v created undo checkpoint", mod)
		}
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

func TestSavePromptExpandsHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	e := &editor{lines: []string{"hello"}, prompt: "Save as: ", promptValue: "~/drafts/note"}
	e.promptKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	want := filepath.Join(home, "drafts", "note.md")
	if e.path != want {
		t.Fatalf("path = %q, want %q", e.path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestPromptSupportsMiddleEditing(t *testing.T) {
	e := &editor{prompt: "Save as: ", promptValue: "untitled.md", promptCursor: runeLen("untitled.md")}
	e.promptKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if e.promptCursor != 0 {
		t.Fatalf("home cursor = %d, want 0", e.promptCursor)
	}
	e.promptKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if got, want := e.promptValue, "ntitled.md"; got != want {
		t.Fatalf("delete at home = %q, want %q", got, want)
	}
	e.promptKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	e.promptKey(tcell.NewEventKey(tcell.KeyRune, 'z', 0))
	if got, want := e.promptValue, "nztitled.md"; got != want {
		t.Fatalf("insert in middle = %q, want %q", got, want)
	}
	if e.promptCursor != 2 {
		t.Fatalf("cursor after insert = %d, want 2", e.promptCursor)
	}
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

func TestNewPolishedThemesAreAvailable(t *testing.T) {
	for _, name := range []string{"matrix", "midnight", "paper", "ember"} {
		if !validTheme(name) {
			t.Fatalf("polished theme %q is not valid", name)
		}
		th := themeByName(name)
		if th.text == tcell.ColorDefault || th.statusBG == tcell.ColorDefault || th.selectionBG == tcell.ColorDefault || th.focusBG == tcell.ColorDefault || th.dimBG == tcell.ColorDefault {
			t.Fatalf("theme %q has incomplete palette: %#v", name, th)
		}
	}
}

func TestSelectionUsesThemePalette(t *testing.T) {
	e := &editor{lines: []string{"x"}, theme: themeByName("matrix")}
	style := e.selectedStyle(tcell.StyleDefault)
	_, bg, _ := style.Decompose()
	if bg != e.theme.selectionBG {
		t.Fatalf("selection background = %v, want %v", bg, e.theme.selectionBG)
	}
}

func TestSearchUsesThemePalette(t *testing.T) {
	e := &editor{lines: []string{"match"}, search: "match", theme: themeByName("ember")}
	style := e.searchStyle(tcell.StyleDefault)
	_, bg, _ := style.Decompose()
	if bg != e.theme.searchBG {
		t.Fatalf("search background = %v, want %v", bg, e.theme.searchBG)
	}
}

func TestFocusModeUsesFocusAndDimBackgrounds(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 6)
	e := &editor{
		screen:    screen,
		lines:     []string{"outside", "", "current one", "current two", "", "after"},
		y:         2,
		theme:     themeByName("midnight"),
		focusMode: true,
	}
	e.draw()
	left, _ := writingArea(80)
	_, _, focusedStyle, _ := screen.GetContent(left, 2)
	_, focusedBG, _ := focusedStyle.Decompose()
	if focusedBG != e.theme.focusBG {
		t.Fatalf("focused background = %v, want %v", focusedBG, e.theme.focusBG)
	}
	_, _, dimStyle, _ := screen.GetContent(left, 0)
	_, dimBG, _ := dimStyle.Decompose()
	if dimBG != e.theme.dimBG {
		t.Fatalf("dim background = %v, want %v", dimBG, e.theme.dimBG)
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

func TestDrawFillsFullTerminalWithThemeBackground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 8)
	e := &editor{
		screen: screen,
		lines:  []string{"paper"},
		theme:  themeByName("paper"),
	}
	e.draw()
	for _, x := range []int{0, 119} {
		_, _, style, _ := screen.GetContent(x, 0)
		_, bg, _ := style.Decompose()
		if bg != e.theme.background {
			t.Fatalf("background at x=%d = %v, want %v", x, bg, e.theme.background)
		}
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

func TestMouseDragSelectsRenderedInlineMarkdownText(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 5)
	e := &editor{
		screen: screen,
		lines:  []string{"Read **this** now"},
		theme:  themeByName("calm"),
	}
	left, _ := writingArea(80)
	e.draw()
	e.mouse(tcell.NewEventMouse(left+5, 0, tcell.Button1, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left+9, 0, tcell.Button1, tcell.ModNone))
	e.mouse(tcell.NewEventMouse(left+9, 0, tcell.ButtonNone, tcell.ModNone))
	if got, want := e.selectionText(), "this"; got != want {
		t.Fatalf("mouse inline markdown selection = %q, want %q", got, want)
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

func TestLoadRecentKeepsMissingAndLimitsFive(t *testing.T) {
	oldConfig := os.Getenv("XDG_CONFIG_HOME")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
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
	if len(got) != 5 || got[0] != paths[0] || got[1] != paths[1] {
		t.Fatalf("loadRecent() = %#v", got)
	}
}

func TestRememberRecentMovesFileToFront(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
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
	t.Setenv("HOME", dir)
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

func TestSaveMovesFileIntoRecentList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	path := filepath.Join(dir, "new-note.md")
	e := &editor{path: path, lines: []string{"hello"}, dirty: true}
	e.save()
	got := loadRecent()
	if len(got) != 1 || got[0] != path {
		t.Fatalf("recent after save = %#v, want [%q]", got, path)
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

func TestToggleEmphasisBoldWrapsSelection(t *testing.T) {
	e := &editor{
		lines: []string{"hello world"},
		x:     5, y: 0,
		selX: 0, selY: 0,
		selecting: true,
	}
	e.toggleEmphasis("**", "**")
	if got, want := e.lines[0], "**hello** world"; got != want {
		t.Fatalf("wrap: line = %q, want %q", got, want)
	}
	if e.x != 7 || e.y != 0 {
		t.Fatalf("wrap: cursor = (%d,%d), want (7,0)", e.x, e.y)
	}
}

func TestToggleEmphasisBoldUnwraps(t *testing.T) {
	e := &editor{
		lines: []string{"**hello** world"},
		x:     7, y: 0,
		selX: 2, selY: 0,
		selecting: true,
	}
	e.toggleEmphasis("**", "**")
	if got, want := e.lines[0], "hello world"; got != want {
		t.Fatalf("unwrap: line = %q, want %q", got, want)
	}
	if e.x != 5 {
		t.Fatalf("unwrap: cursor = %d, want 5", e.x)
	}
}

func TestToggleEmphasisUnderlineWrapUnwrap(t *testing.T) {
	e := &editor{
		lines: []string{"a note here"},
		x:     6, y: 0,
		selX: 2, selY: 0,
		selecting: true,
	}
	e.toggleEmphasis("<u>", "</u>")
	if got, want := e.lines[0], "a <u>note</u> here"; got != want {
		t.Fatalf("underline wrap: line = %q, want %q", got, want)
	}
	// Selection now covers "note" inside the markers; toggle again unwraps.
	e.selX, e.x = 5, 9 // inside <u>...</u> wrapping "note"
	e.toggleEmphasis("<u>", "</u>")
	if got, want := e.lines[0], "a note here"; got != want {
		t.Fatalf("underline unwrap: line = %q, want %q", got, want)
	}
}

func TestToggleEmphasisHighlightWrapUnwrap(t *testing.T) {
	e := &editor{
		lines: []string{"key point"},
		x:     9, y: 0,
		selX: 4, selY: 0,
		selecting: true,
	}
	e.toggleEmphasis("==", "==")
	if got, want := e.lines[0], "key ==point=="; got != want {
		t.Fatalf("highlight wrap: line = %q, want %q", got, want)
	}
	// Selection now covers "point" inside the markers; toggle again unwraps.
	e.selX, e.x = 6, 11 // inside ==...== wrapping "point"
	e.toggleEmphasis("==", "==")
	if got, want := e.lines[0], "key point"; got != want {
		t.Fatalf("highlight unwrap: line = %q, want %q", got, want)
	}
}

func TestToggleEmphasisNoSelectionInsertsMarkers(t *testing.T) {
	e := &editor{lines: []string{"word"}, x: 0, y: 0}
	e.toggleEmphasis("**", "**")
	if got, want := e.lines[0], "****word"; got != want {
		t.Fatalf("empty bold: line = %q, want %q", got, want)
	}
	if e.x != 2 {
		t.Fatalf("empty bold: cursor = %d, want 2 (inside markers)", e.x)
	}
}

func TestCtrlBBoldKeybind(t *testing.T) {
	e := &editor{
		lines: []string{"plain"},
		x:     5, y: 0,
		selX: 0, selY: 0,
		selecting: true,
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl))
	if got, want := e.lines[0], "**plain**"; got != want {
		t.Fatalf("Ctrl-B: line = %q, want %q", got, want)
	}
}

func TestCtrlHHighlightKeybind(t *testing.T) {
	e := &editor{
		lines: []string{"plain"},
		x:     5, y: 0,
		selX: 0, selY: 0,
		selecting: true,
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl))
	if got, want := e.lines[0], "==plain=="; got != want {
		t.Fatalf("Ctrl-H: line = %q, want %q", got, want)
	}
}

func TestCtrlEItalicKeybindAndRecentOnShift(t *testing.T) {
	// Plain Ctrl-E toggles italic.
	e := &editor{
		lines: []string{"plain"},
		x:     5, y: 0,
		selX: 0, selY: 0,
		selecting: true,
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlE, 0, tcell.ModCtrl))
	if got, want := e.lines[0], "*plain*"; got != want {
		t.Fatalf("Ctrl-E italic: line = %q, want %q", got, want)
	}
	// Ctrl-Shift-E opens the recent panel rather than toggling italic.
	e2 := &editor{lines: []string{"plain"}}
	e2.key(tcell.NewEventKey(tcell.KeyCtrlE, 0, tcell.ModCtrl|tcell.ModShift))
	if !e2.showRecent {
		t.Fatal("Ctrl-Shift-E did not open recent files")
	}
	if got := e2.lines[0]; got != "plain" {
		t.Fatalf("Ctrl-Shift-E mutated text: %q", got)
	}
}

func TestCtrlUUnderlineKeybind(t *testing.T) {
	e := &editor{
		lines: []string{"plain"},
		x:     5, y: 0,
		selX: 0, selY: 0,
		selecting: true,
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModCtrl))
	if got, want := e.lines[0], "<u>plain</u>"; got != want {
		t.Fatalf("Ctrl-U: line = %q, want %q", got, want)
	}
}

func TestF5TogglesHeadingOne(t *testing.T) {
	e := &editor{lines: []string{"Heading"}}
	e.key(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if got, want := e.lines[0], "# Heading"; got != want {
		t.Fatalf("F5 heading: line = %q, want %q", got, want)
	}
	e.key(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if got, want := e.lines[0], "Heading"; got != want {
		t.Fatalf("F5 unheading: line = %q, want %q", got, want)
	}
}

func TestF6ConvertsExistingHeadingToHeadingTwo(t *testing.T) {
	e := &editor{lines: []string{"# Heading"}, x: 3}
	e.key(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
	if got, want := e.lines[0], "## Heading"; got != want {
		t.Fatalf("F6 heading: line = %q, want %q", got, want)
	}
}

func TestF7InsertsHeadingThreeOnBlankLine(t *testing.T) {
	e := &editor{lines: []string{""}}
	e.key(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if got, want := e.lines[0], "### "; got != want {
		t.Fatalf("F7 blank heading: line = %q, want %q", got, want)
	}
	if e.x != len("### ") {
		t.Fatalf("F7 cursor = %d, want %d", e.x, len("### "))
	}
}

func TestCtrlASelectAll(t *testing.T) {
	e := &editor{lines: []string{"one", "two", "three"}, x: 1, y: 1}
	e.key(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if !e.selecting {
		t.Fatal("Ctrl-A did not enable selection")
	}
	if e.selX != 0 || e.selY != 0 {
		t.Fatalf("Ctrl-A anchor = (%d,%d), want (0,0)", e.selX, e.selY)
	}
	if e.y != 2 || e.x != 5 {
		t.Fatalf("Ctrl-A cursor = (%d,%d), want (5,2)", e.x, e.y)
	}
	_, _, bx, by, ok := e.selectionBounds()
	if !ok || by != 2 || bx != 5 {
		t.Fatalf("Ctrl-A selection bounds = (%d,%d) ok=%t, want (5,2)", bx, by, ok)
	}
}

func TestF2OpensSaveAsPrompt(t *testing.T) {
	e := &editor{lines: []string{"x"}, path: "named.md", untitled: false}
	e.key(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if e.prompt != "Save as: " {
		t.Fatalf("F2 prompt = %q, want Save as: ", e.prompt)
	}
	if e.promptValue != "named.md" {
		t.Fatalf("F2 promptValue = %q, want named.md", e.promptValue)
	}
}

func TestF3OpensRecentFiles(t *testing.T) {
	e := &editor{lines: []string{"x"}}
	e.key(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if !e.showRecent {
		t.Fatal("F3 did not open recent files")
	}
}

func TestHelpRendersAlignedShortcutColumns(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 20)
	e := &editor{screen: screen, lines: []string{""}, theme: themeByName("calm")}
	e.drawHelp(100, 20)
	var rendered strings.Builder
	for row := 0; row < 20; row++ {
		rendered.WriteString(simulationLine(screen, row, 100))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	for _, want := range []string{"F2", "Save As", "F3", "Recent files", "F5", "Heading 1", "Ctrl-Shift-S", "Select all"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help did not render %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Ctrl-B bold   Ctrl-E italic") {
		t.Fatalf("help still rendered dense unaligned shortcut block:\n%s", text)
	}
}

func TestShortcutCoachRendersStartupHint(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(72, 12)
	e := &editor{
		screen:    screen,
		path:      "note.md",
		lines:     []string{"hello"},
		theme:     themeByName("calm"),
		showCoach: true,
		status:    "ready",
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 12; row++ {
		rendered.WriteString(simulationLine(screen, row, 72))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	for _, want := range []string{"MARKO", "F2 save as", "F3 recent", "F5/F6/F7 headings", "F1 more"} {
		if !strings.Contains(text, want) {
			t.Fatalf("shortcut coach did not render %q:\n%s", want, text)
		}
	}
}

func TestShortcutCoachRendersMarkoTextArt(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(72, 14)
	e := &editor{
		screen:    screen,
		path:      "note.md",
		lines:     []string{"hello"},
		theme:     themeByName("calm"),
		showCoach: true,
		status:    "ready",
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 14; row++ {
		rendered.WriteString(simulationLine(screen, row, 72))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	for _, want := range []string{"MARKO", "Markdown focus", "F2 save as", "F3 recent"} {
		if !strings.Contains(text, want) {
			t.Fatalf("shortcut coach did not render %q:\n%s", want, text)
		}
	}
}

func TestShortcutCoachDismissesOnKeyAndF1OpensHelp(t *testing.T) {
	e := &editor{lines: []string{""}, theme: themeByName("calm"), showCoach: true}
	e.key(tcell.NewEventKey(tcell.KeyRune, 'x', 0))
	if e.showCoach {
		t.Fatal("shortcut coach stayed visible after text input")
	}

	e.showCoach = true
	e.key(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	if e.showCoach {
		t.Fatal("shortcut coach stayed visible after F1")
	}
	if !e.showHelp {
		t.Fatal("F1 did not open full help")
	}
}

func TestShortcutCoachExpiresAfterFiveSeconds(t *testing.T) {
	e := &editor{lines: []string{""}, theme: themeByName("calm"), showCoach: true, coachUntil: time.Now().Add(-time.Second)}
	e.tick()
	if e.showCoach {
		t.Fatal("shortcut coach stayed visible after expiry")
	}
}

func TestStartMenuRendersNewRecentAndOpenPath(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 18)
	e := &editor{
		screen:        screen,
		lines:         []string{""},
		theme:         themeByName("calm"),
		showStartMenu: true,
		recent:        []string{"/tmp/one.md"},
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 18; row++ {
		rendered.WriteString(simulationLine(screen, row, 80))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	for _, want := range []string{"MARKO", "New document", "/tmp/one.md", "Open path", "F1 help"} {
		if !strings.Contains(text, want) {
			t.Fatalf("start menu did not render %q:\n%s", want, text)
		}
	}
}

func TestStartMenuKeyActions(t *testing.T) {
	e := &editor{lines: []string{""}, theme: themeByName("calm"), showStartMenu: true, recent: []string{"/tmp/one.md"}}

	e.key(tcell.NewEventKey(tcell.KeyRune, 'o', 0))
	if e.prompt != "Open path: " {
		t.Fatalf("open path prompt = %q, want Open path: ", e.prompt)
	}
	if e.showStartMenu {
		t.Fatal("start menu stayed visible after Open path")
	}

	e = &editor{lines: []string{""}, theme: themeByName("calm"), showStartMenu: true}
	e.key(tcell.NewEventKey(tcell.KeyRune, 'n', 0))
	if e.showStartMenu {
		t.Fatal("start menu stayed visible after New document")
	}
	if !e.untitled {
		t.Fatal("new document did not mark editor untitled")
	}
}

func TestF4OpensStartMenuFromDocument(t *testing.T) {
	e := &editor{lines: []string{"x"}, path: "note.md", theme: themeByName("calm")}
	e.key(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
	if !e.showStartMenu {
		t.Fatal("F4 did not open start menu")
	}
	if e.startMenuIndex != 0 {
		t.Fatalf("startMenuIndex = %d, want 0", e.startMenuIndex)
	}
}

func TestStartMenuArrowSelectionAndEnter(t *testing.T) {
	e := &editor{lines: []string{"x"}, path: "note.md", theme: themeByName("calm"), showStartMenu: true}
	e.key(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if e.startMenuIndex != 1 {
		t.Fatalf("startMenuIndex after down = %d, want 1", e.startMenuIndex)
	}
	e.key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if e.prompt != "Open path: " {
		t.Fatalf("selected Open path prompt = %q, want Open path: ", e.prompt)
	}
}

func TestStartMenuEscapeReturnsToDocument(t *testing.T) {
	e := &editor{lines: []string{"x"}, path: "note.md", theme: themeByName("calm"), showStartMenu: true}
	e.key(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if e.showStartMenu {
		t.Fatal("Esc did not close start menu")
	}
	if e.lines[0] != "x" {
		t.Fatalf("document changed after Esc: %#v", e.lines)
	}
}

func TestStartMenuThemeActionCyclesTheme(t *testing.T) {
	e := &editor{lines: []string{"x"}, themeName: "matrix", theme: themeByName("matrix"), showStartMenu: true}
	e.startMenuIndex = 3
	e.key(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if e.themeName != "midnight" {
		t.Fatalf("themeName = %q, want midnight", e.themeName)
	}
	if !e.showStartMenu {
		t.Fatal("theme action should keep start menu open")
	}
}

func TestZoxidePathExpansion(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "zoxide")
	script := "#!/bin/sh\nif [ \"$1\" = query ] && [ \"$2\" = briefs ]; then printf '%s\\n' '" + dir + "'; exit 0; fi\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := expandPathInput("z briefs/draft")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "draft")
	if got != want {
		t.Fatalf("expandPathInput = %q, want %q", got, want)
	}
}

func TestStartMenuCanShowHelpOverlay(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 24)
	e := &editor{
		screen:        screen,
		lines:         []string{""},
		theme:         themeByName("calm"),
		showStartMenu: true,
	}
	e.key(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 24; row++ {
		rendered.WriteString(simulationLine(screen, row, 100))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	if !strings.Contains(text, "Marko help") {
		t.Fatalf("start menu help overlay missing:\n%s", text)
	}
}

func TestRecentPanelUsesBracketedEmptyStateAndSplitFooter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)
	e := &editor{
		screen:     screen,
		lines:      []string{"x"},
		theme:      themeByName("calm"),
		showRecent: true,
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 16; row++ {
		rendered.WriteString(simulationLine(screen, row, 80))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	if !strings.Contains(text, "<No recent files>") {
		t.Fatalf("recent panel empty state missing:\n%s", text)
	}
	if !strings.Contains(text, "Up/Down select   Enter open") || !strings.Contains(text, "Esc cancel") {
		t.Fatalf("recent panel footer not split across lines:\n%s", text)
	}
}

func TestRecentPanelRefreshesFromDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(recentConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recentConfigPath(), []byte(path+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)
	e := &editor{
		screen:     screen,
		lines:      []string{"x"},
		theme:      themeByName("calm"),
		showRecent: true,
		recent:     nil,
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 16; row++ {
		rendered.WriteString(simulationLine(screen, row, 80))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	if !strings.Contains(text, "note.md [") {
		t.Fatalf("recent panel did not refresh from disk:\n%s", rendered.String())
	}
}

func TestRecentPanelGroupsByAge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	now := time.Now()
	fresh := filepath.Join(dir, "fresh.md")
	week := filepath.Join(dir, "week.md")
	old := filepath.Join(dir, "old.md")
	for _, item := range []struct {
		path string
		when time.Time
	}{
		{fresh, now.Add(-2 * time.Hour)},
		{week, now.Add(-3 * 24 * time.Hour)},
		{old, now.Add(-12 * 24 * time.Hour)},
	} {
		if err := os.WriteFile(item.path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(item.path, item.when, item.when); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(recentConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recentConfigPath(), []byte(strings.Join([]string{fresh, week, old}, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(110, 20)
	e := &editor{
		screen:     screen,
		lines:      []string{"x"},
		theme:      themeByName("calm"),
		showRecent: true,
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 20; row++ {
		rendered.WriteString(simulationLine(screen, row, 110))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	for _, want := range []string{"Past 48 hours", "Past week", "Older", "fresh.md", "week.md", "old.md"} {
		if !strings.Contains(text, want) {
			t.Fatalf("grouped recent panel missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "Past 48 hours") > strings.Index(text, "Past week") || strings.Index(text, "Past week") > strings.Index(text, "Older") {
		t.Fatalf("recent groups out of order:\n%s", text)
	}
}

func TestRecentPanelShowsMissingFilesSection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	missing := "missing.md"
	if err := os.MkdirAll(filepath.Dir(recentConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recentConfigPath(), []byte(missing+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 16)
	e := &editor{
		screen:     screen,
		lines:      []string{"x"},
		theme:      themeByName("calm"),
		showRecent: true,
	}
	e.draw()
	var rendered strings.Builder
	for row := 0; row < 16; row++ {
		rendered.WriteString(simulationLine(screen, row, 100))
		rendered.WriteByte('\n')
	}
	text := rendered.String()
	if !strings.Contains(text, "Missing files") || !strings.Contains(text, "missing.md (missing)") {
		t.Fatalf("recent panel did not surface missing history:\n%s", text)
	}
}

func TestRecentPanelShowsRecencyGradient(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	oldPath := filepath.Join(dir, "old.md")
	newPath := filepath.Join(dir, "new.md")
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(recentConfigPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recentConfigPath(), []byte(newPath+"\n"+oldPath+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 18)
	e := &editor{
		screen:     screen,
		lines:      []string{"x"},
		theme:      themeByName("calm"),
		showRecent: true,
	}
	newColor := recentGradientColor(0, 2)
	oldColor := recentGradientColor(1, 2)
	if newColor != tcell.GetColor("#ff6b5f") {
		t.Fatalf("newest recent color = %v, want %v", newColor, tcell.GetColor("#ff6b5f"))
	}
	if oldColor != tcell.GetColor("#5aa8ff") {
		t.Fatalf("oldest recent color = %v, want %v", oldColor, tcell.GetColor("#5aa8ff"))
	}
	style := e.recentStyle(tcell.StyleDefault, 0, 2)
	fg, _, attrs := style.Decompose()
	if fg != newColor || attrs&tcell.AttrBold == 0 {
		t.Fatalf("recent style for newest item = fg:%v attrs:%v, want fg:%v bold", fg, attrs, newColor)
	}
}

func TestStartMenuTickDoesNotAutosave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	e := &editor{
		lines:         []string{"draft"},
		path:          path,
		dirty:         true,
		lastEdit:      time.Now().Add(-3 * time.Second),
		theme:         themeByName("calm"),
		showStartMenu: true,
	}
	e.tick()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("start menu tick wrote file or unexpected stat error: %v", err)
	}
}

func TestInlinePlainTextStripsUnderlineAndHighlight(t *testing.T) {
	tests := []struct{ in, want string }{
		{"<u>under</u>", "under"},
		{"==mark==", "mark"},
		{"**bold**", "bold"},
		{"plain", "plain"},
	}
	for _, tc := range tests {
		if got := inlinePlainText(tc.in); got != tc.want {
			t.Fatalf("inlinePlainText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWrappedLineEmphasisRenders(t *testing.T) {
	// A long paragraph that must wrap across two rows, with bold spanning the
	// wrap. The continuation row must not leak the ** markers.
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	width := 20
	screen.SetSize(width, 3)
	body := "leading words then **bold bit trailing** text tail"
	e := &editor{
		screen: screen,
		lines:  []string{body},
		theme:  themeByName("calm"),
		y:      5, // cursor elsewhere: line is not "current"
	}
	rows := e.visualRows(width)
	if len(rows) < 2 {
		t.Fatalf("visualRows() = %d rows, want >=2", len(rows))
	}
	for row, vr := range rows {
		e.drawVisualLine(0, row, vr, false, width)
	}
	for row := 0; row < len(rows); row++ {
		got := simulationLine(screen, row, width)
		if strings.Contains(got, "**") {
			t.Fatalf("row %d leaked ** markers: %q", row, got)
		}
	}
}

func TestUntitledSavePrompts(t *testing.T) {
	e := &editor{lines: []string{"x"}, path: "20260618_untitled.md", untitled: true}
	e.key(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl))
	if e.prompt != "Save as: " {
		t.Fatalf("untitled Ctrl-S prompt = %q, want %q", e.prompt, "Save as: ")
	}
	if e.promptValue != "untitled.md" {
		t.Fatalf("untitled Ctrl-S promptValue = %q, want untitled.md", e.promptValue)
	}
}

func TestSavePromptShowsZoxideHint(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 3)
	e := &editor{
		screen:      screen,
		lines:       []string{"x"},
		prompt:      "Save as: ",
		promptValue: "untitled.md",
		theme:       themeByName("calm"),
	}
	e.draw()
	got := simulationLine(screen, 2, 80)
	if !strings.Contains(got, "[z query/file.md, or type a path]") {
		t.Fatalf("save prompt hint missing: %q", got)
	}
}

func TestPromptRendersInFocusMode(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 4)
	e := &editor{
		screen:       screen,
		lines:        []string{"hello"},
		theme:        themeByName("calm"),
		focusMode:    true,
		prompt:       "Save as: ",
		promptValue:  "untitled.md",
		promptCursor: runeLen("untitled.md"),
	}
	e.draw()
	got := strings.TrimRight(simulationLine(screen, 3, 80), " ")
	if !strings.Contains(got, "Save as: untitled.md") {
		t.Fatalf("prompt not visible in focus mode: %q", got)
	}
}

func TestCtrlSSubmitsSavePrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	e := &editor{
		lines:        []string{"hello"},
		prompt:       "Save as: ",
		promptValue:  path,
		promptCursor: runeLen(path),
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl))
	if e.prompt != "" {
		t.Fatalf("Ctrl-S left prompt open: %q", e.prompt)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "hello"; got != want {
		t.Fatalf("Ctrl-S save wrote %q, want %q", got, want)
	}
}

func TestCtrlKTogglesFocusWhilePromptOpen(t *testing.T) {
	e := &editor{
		lines:        []string{"hello"},
		focusMode:    true,
		prompt:       "Save as: ",
		promptValue:  "untitled.md",
		promptCursor: runeLen("untitled.md"),
	}
	e.key(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModCtrl))
	if e.focusMode {
		t.Fatal("Ctrl-K did not toggle focus mode while prompt was open")
	}
}

func TestCtrlQQuitsWhilePromptOpen(t *testing.T) {
	e := &editor{
		lines:        []string{"hello"},
		prompt:       "Save as: ",
		promptValue:  "untitled.md",
		promptCursor: runeLen("untitled.md"),
	}
	if !e.key(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModCtrl)) {
		t.Fatal("Ctrl-Q did not quit while prompt was open")
	}
}

func TestNamedSaveDoesNotPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "named.md")
	e := &editor{lines: []string{"x"}, path: path, untitled: false}
	e.key(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl))
	if e.prompt != "" {
		t.Fatalf("named Ctrl-S prompted %q, wanted direct save", e.prompt)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "x" {
		t.Fatalf("named Ctrl-S wrote %q, want %q", data, "x")
	}
}
