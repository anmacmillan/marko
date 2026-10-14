package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type rowKind uint8

const (
	rowPlain rowKind = iota
	rowQuote
	rowBullet
	rowTask
	rowTaskDone
	rowCodeTop
	rowCode
	rowCodeBottom
)

var bulletGlyphs = []string{"•", "◦", "▪"}

// taskGlyphs are Nerd Font checkbox icons, drawn at full cell size by the
// terminal font. MARKO_GLYPHS=plain falls back to Unicode ballot boxes for
// fonts without Nerd Font icons.
var taskGlyphs = func() [2]string {
	if os.Getenv("MARKO_GLYPHS") == "plain" {
		return [2]string{"☐", "☑"}
	}
	return [2]string{"\U000F0131", "\U000F0135"}
}()

// lineDecoration recognises block quotes, bullets and task items on a line
// the cursor is not on. It returns the decoration to draw in place of the
// Markdown prefix and how many source runes that prefix covers. The source
// text after the prefix is drawn unchanged, so column mapping stays exact.
func lineDecoration(line string) (rowKind, string, int, bool) {
	runes := []rune(line)
	indent := 0
	for indent < len(runes) && runes[indent] == ' ' {
		indent++
	}
	rest := runes[indent:]
	if len(rest) > 0 && rest[0] == '>' {
		skip := indent + 1
		if skip < len(runes) && runes[skip] == ' ' {
			skip++
		}
		return rowQuote, "▎ ", skip, true
	}
	if len(rest) < 2 || (rest[0] != '-' && rest[0] != '*' && rest[0] != '+') || rest[1] != ' ' {
		return rowPlain, "", 0, false
	}
	pad := strings.Repeat(" ", indent)
	if len(rest) >= 6 && rest[2] == '[' && rest[4] == ']' && rest[5] == ' ' {
		switch rest[3] {
		case ' ':
			return rowTask, pad + taskGlyphs[0] + " ", indent + 6, true
		case 'x', 'X':
			return rowTaskDone, pad + taskGlyphs[1] + " ", indent + 6, true
		}
	}
	glyph := bulletGlyphs[min(indent/2, len(bulletGlyphs)-1)]
	return rowBullet, pad + glyph + " ", indent + 2, true
}

// appendWrappedRows word-wraps line[skip:] into rows modelled on proto.
// Continuation rows of a decorated line hang under the text: quotes repeat
// their bar, list items indent by the width of the bullet.
func appendWrappedRows(rows []visualRow, proto visualRow, line string, skip, width int) []visualRow {
	runes := []rune(line)
	first := proto.lead
	hang := proto.lead
	if proto.kind != rowQuote {
		hang = strings.Repeat(" ", runeLen(proto.lead))
	}
	textWidth := max(1, width-runeLen(proto.lead))
	if skip >= len(runes) {
		proto.start, proto.text = skip, ""
		return append(rows, proto)
	}
	for start := skip; start < len(runes); {
		end := min(len(runes), start+textWidth)
		if end < len(runes) {
			for candidate := end; candidate > start+textWidth/2; candidate-- {
				if runes[candidate-1] == ' ' {
					end = candidate
					break
				}
			}
		}
		vr := proto
		vr.start, vr.text = start, string(runes[start:end])
		if start == skip {
			vr.lead = first
		} else {
			vr.lead = hang
		}
		rows = append(rows, vr)
		start = end
	}
	return rows
}

func (e *editor) drawDecoratedRow(left, row int, vr visualRow, current bool, width int) {
	base := tcell.StyleDefault.Foreground(e.theme.text).Background(e.theme.background)
	lead := base.Foreground(e.theme.accent1)
	switch vr.kind {
	case rowQuote:
		base = base.Foreground(e.theme.quote).Italic(true)
		lead = lead.Foreground(blendColor(e.theme.accent1, e.theme.background, 0.35))
	case rowTaskDone:
		base = base.Foreground(blendColor(e.theme.text, e.theme.background, 0.45)).StrikeThrough(true)
		lead = lead.Foreground(blendColor(e.theme.accent1, e.theme.background, 0.3))
	}
	base = e.focusStyle(base, current)
	lead = e.focusStyle(lead, current)
	if e.positionSelected(vr.start, vr.y) && e.positionSelected(max(0, vr.start-1), vr.y) {
		lead = e.selectedStyle(lead)
	}
	e.put(left, row, vr.lead, lead, left+width)
	x := left + runeLen(vr.lead)
	e.putInlineWindow(x, row, e.lines[vr.y], base, left+width, vr.y, vr.start, vr.start+runeLen(vr.text))
}

// dimmedEmphasis keeps bold, italic, underline and strikethrough on lines
// receded by focus mode; only the highlight fill is dropped, since a bright
// block would pull the eye away from the lit paragraph.
func dimmedEmphasis(marker string, styled func(tcell.Style) tcell.Style, s tcell.Style) tcell.Style {
	if marker == "==" {
		return s
	}
	return styled(s)
}

// dimmedFG returns colour c as focus mode would draw it on line y.
func (e *editor) dimmedFG(c tcell.Color, y int) tcell.Color {
	fg, _, _ := e.focusStyle(tcell.StyleDefault.Foreground(c), e.focusedLine(y)).Decompose()
	return fg
}

// codePanelBG tints the page toward the accent colour so code reads as a
// panel in every theme; themes on the terminal's own background keep codeBG.
func (e *editor) codePanelBG() tcell.Color {
	if !e.theme.background.Valid() || e.theme.background == tcell.ColorDefault {
		return e.theme.codeBG
	}
	return blendColor(e.theme.background, e.theme.accent1, 0.12)
}

// tableStyles returns faint borders and cell text for a table row: body
// cells in the text colour, the header row bold in the accent colour.
func (e *editor) tableStyles(y int, style tcell.Style) (tcell.Style, tcell.Style) {
	_, bg, _ := style.Decompose()
	borders := style.Foreground(e.dimmedFG(blendColor(e.theme.table, bg, 0.5), y))
	cells := style.Foreground(e.dimmedFG(e.theme.text, y))
	if start, _ := e.tableBounds(y); start == y {
		cells = style.Foreground(e.dimmedFG(e.theme.accent1, y)).Bold(true)
	}
	return borders, cells
}

// drawWhisperStatus is focus mode's status line: file name, unsaved mark and
// word count, faint in the bottom-right corner.
func (e *editor) drawWhisperStatus(w, bodyH int) {
	words := 0
	for _, line := range e.lines {
		words += len(strings.Fields(line))
	}
	name := "Untitled"
	if e.path != "" {
		name = filepath.Base(e.path)
	}
	if e.dirty {
		name += " •"
	}
	text := fmt.Sprintf("%s · %d words", name, words)
	x := w - runeLen(text) - 2
	if x < 0 {
		return
	}
	page := e.theme.dimBG
	if !page.Valid() {
		page = e.theme.background
	}
	style := tcell.StyleDefault.Background(page).Foreground(blendColor(e.theme.muted, page, 0.35))
	e.put(x, bodyH-1, text, style, w)
}
