package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Table structure editing: a Word-style grid picker for inserting new tables
// (Ctrl-T outside a table), Alt-arrow row/column commands inside a table, and
// column alignment cycling (Ctrl-T inside a table).

type tableGridPick struct {
	rows, cols int // rows includes the header row
}

const tableGridMax = 10

func (e *editor) openTableGrid() {
	e.tableGrid = &tableGridPick{rows: 3, cols: 3}
}

func (e *editor) tableGridKey(ev *tcell.EventKey) {
	g := e.tableGrid
	switch eventKey(ev) {
	case tcell.KeyEsc, tcell.KeyCtrlQ, tcell.KeyCtrlT:
		e.tableGrid = nil
		e.status = "Cancelled"
	case tcell.KeyLeft:
		g.cols = max(1, g.cols-1)
	case tcell.KeyRight:
		g.cols = min(tableGridMax, g.cols+1)
	case tcell.KeyUp:
		g.rows = max(2, g.rows-1)
	case tcell.KeyDown:
		g.rows = min(tableGridMax, g.rows+1)
	case tcell.KeyEnter:
		rows, cols := g.rows, g.cols
		e.tableGrid = nil
		e.checkpoint()
		e.insertSizedTable(rows, cols)
	}
}

func (e *editor) insertSizedTable(rows, cols int) {
	if rows < 2 {
		rows = 2
	}
	if cols < 1 {
		cols = 1
	}
	header := make([]string, cols)
	separator := make([]string, cols)
	empty := make([]string, cols)
	for i := range header {
		header[i] = fmt.Sprintf("Heading %d", i+1)
		width := runeLen(header[i])
		separator[i] = strings.Repeat("-", width)
		empty[i] = strings.Repeat(" ", width)
	}
	r := []rune(e.lines[e.y])
	before, after := string(r[:e.x]), string(r[e.x:])
	table := []string{before + rebuildTableLine(header), rebuildTableLine(separator)}
	for i := 0; i < rows-1; i++ {
		table = append(table, rebuildTableLine(empty))
	}
	table[len(table)-1] += after
	e.lines = append(e.lines[:e.y], append(table, e.lines[e.y+1:]...)...)
	e.x = runeLen(before) + 2
	e.dirty = true
	e.status = fmt.Sprintf("%d×%d table created. Tab moves between cells.", rows, cols)
}

func rebuildTableLine(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |"
}

// tableColumnAt returns the cell index the rune column x falls in, using the
// same pipe rules as splitTable (backslash escapes, backtick code spans).
func tableColumnAt(line string, x int) int {
	runes := []rune(line)
	pipes, inCode, escaped := 0, false, false
	for i := 0; i < len(runes) && i < x; i++ {
		switch {
		case escaped:
			escaped = false
		case runes[i] == '\\':
			escaped = true
		case runes[i] == '`':
			inCode = !inCode
		case runes[i] == '|' && !inCode:
			pipes++
		}
	}
	col := pipes - 1
	if col < 0 {
		col = 0
	}
	cells := splitTable(line)
	if len(cells) > 0 && col >= len(cells) {
		col = len(cells) - 1
	}
	return col
}

func (e *editor) currentTableColumn() int {
	return tableColumnAt(e.lines[e.y], e.x)
}

// moveToTableCell puts the cursor at the start of the given cell on line y.
func (e *editor) moveToTableCell(y, col int) {
	e.y = y
	runes := []rune(e.lines[y])
	pipes, inCode, escaped := 0, false, false
	for i, r := range runes {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '`':
			inCode = !inCode
		case r == '|' && !inCode:
			pipes++
			if pipes == col+1 {
				e.x = min(i+2, len(runes))
				return
			}
		}
	}
	e.x = len(runes)
}

func (e *editor) insertTableColumn() {
	col := e.currentTableColumn()
	start, end := e.tableBounds(e.y)
	for y := start; y <= end; y++ {
		cells := splitTable(e.lines[y])
		value := ""
		if isSeparator(e.lines[y]) {
			value = "---"
		}
		at := min(col+1, len(cells))
		cells = append(cells[:at], append([]string{value}, cells[at:]...)...)
		e.lines[y] = rebuildTableLine(cells)
	}
	e.dirty = true
	e.formatTable()
	e.moveToTableCell(e.y, col+1)
	e.status = "Column added"
}

func (e *editor) deleteTableColumn() {
	col := e.currentTableColumn()
	start, end := e.tableBounds(e.y)
	if len(splitTable(e.lines[start])) <= 1 {
		e.status = "Cannot delete the only column"
		return
	}
	for y := start; y <= end; y++ {
		cells := splitTable(e.lines[y])
		if col < len(cells) {
			cells = append(cells[:col], cells[col+1:]...)
		}
		if len(cells) == 0 {
			cells = []string{""}
		}
		e.lines[y] = rebuildTableLine(cells)
	}
	e.dirty = true
	e.formatTable()
	e.moveToTableCell(e.y, max(0, col-1))
	e.status = "Column deleted"
}

func (e *editor) insertTableRow() {
	start, end := e.tableBounds(e.y)
	cols := len(splitTable(e.lines[start]))
	at := e.y + 1
	// From the header, skip past the separator so the new row is a body row.
	for at <= end && isSeparator(e.lines[at]) {
		at++
	}
	empty := make([]string, cols)
	e.lines = insertLine(e.lines, at, rebuildTableLine(empty))
	e.dirty = true
	col := e.currentTableColumn()
	e.y = at
	e.formatTable()
	e.moveToTableCell(at, col)
	e.status = "Row added"
}

func (e *editor) deleteTableRow() {
	start, _ := e.tableBounds(e.y)
	if e.y == start {
		e.status = "Cannot delete the header row"
		return
	}
	if isSeparator(e.lines[e.y]) {
		e.status = "Cannot delete the separator row"
		return
	}
	col := e.currentTableColumn()
	e.lines = append(e.lines[:e.y], e.lines[e.y+1:]...)
	e.y = max(0, e.y-1)
	e.dirty = true
	if e.inTable(e.y) {
		e.formatTable()
		if isSeparator(e.lines[e.y]) && e.y > 0 {
			e.y--
		}
		e.moveToTableCell(e.y, col)
	} else {
		e.clampX()
	}
	e.status = "Row deleted"
}

// Column alignment: 0 none, 1 left (:---), 2 center (:---:), 3 right (---:).

func alignmentOf(cell string) int {
	cell = strings.TrimSpace(cell)
	left := strings.HasPrefix(cell, ":")
	right := strings.HasSuffix(cell, ":")
	switch {
	case left && right:
		return 2
	case right:
		return 3
	case left:
		return 1
	}
	return 0
}

// separatorMinWidth keeps at least three dashes in a separator cell so
// isSeparator still recognises it once colons are added.
func separatorMinWidth(align int) int {
	switch align {
	case 1, 3:
		return 4
	case 2:
		return 5
	}
	return 3
}

func alignmentCell(align, width int) string {
	if width < separatorMinWidth(align) {
		width = separatorMinWidth(align)
	}
	switch align {
	case 1:
		return ":" + strings.Repeat("-", width-1)
	case 2:
		return ":" + strings.Repeat("-", width-2) + ":"
	case 3:
		return strings.Repeat("-", width-1) + ":"
	}
	return strings.Repeat("-", width)
}

func alignmentName(align int) string {
	switch align {
	case 1:
		return "left"
	case 2:
		return "center"
	case 3:
		return "right"
	}
	return "default"
}

// tableSeparatorLine finds the separator row of the table containing y.
func (e *editor) tableSeparatorLine(y int) (int, bool) {
	start, end := e.tableBounds(y)
	for line := start; line <= end; line++ {
		if isSeparator(e.lines[line]) {
			return line, true
		}
	}
	return 0, false
}

func (e *editor) tableAlignments(y int) []int {
	sep, ok := e.tableSeparatorLine(y)
	if !ok {
		return nil
	}
	cells := splitTable(e.lines[sep])
	aligns := make([]int, len(cells))
	for i, cell := range cells {
		aligns[i] = alignmentOf(cell)
	}
	return aligns
}

func (e *editor) cycleColumnAlignment() {
	sep, ok := e.tableSeparatorLine(e.y)
	if !ok {
		e.status = "No separator row in this table"
		return
	}
	col := e.currentTableColumn()
	cells := splitTable(e.lines[sep])
	for len(cells) <= col {
		cells = append(cells, "---")
	}
	next := (alignmentOf(cells[col]) + 1) % 4
	cells[col] = alignmentCell(next, 3)
	e.lines[sep] = rebuildTableLine(cells)
	e.dirty = true
	e.formatTable()
	e.status = fmt.Sprintf("Column %d: %s aligned", col+1, alignmentName(next))
}

// padCellAligned pads value to width according to the column alignment.
func padCellAligned(value string, width, align int) string {
	gap := width - runeLen(value)
	if gap <= 0 {
		return value
	}
	switch align {
	case 2:
		left := gap / 2
		return strings.Repeat(" ", left) + value + strings.Repeat(" ", gap-left)
	case 3:
		return strings.Repeat(" ", gap) + value
	}
	return value + strings.Repeat(" ", gap)
}

// padInlineAligned pads using the inline display width (emphasis markers
// hidden), for rendered table cells.
func inlinePadLeft(value string, width, align int) int {
	gap := width - inlineDisplayWidth(value)
	if gap <= 0 {
		return 0
	}
	switch align {
	case 2:
		return gap / 2
	case 3:
		return gap
	}
	return 0
}

func (e *editor) drawTableGrid(w, h int) {
	g := e.tableGrid
	lines := []string{}
	for r := 0; r < tableGridMax; r++ {
		var b strings.Builder
		for c := 0; c < tableGridMax; c++ {
			if r < g.rows && c < g.cols {
				if r == 0 {
					b.WriteString("▣ ")
				} else {
					b.WriteString("◼ ")
				}
			} else {
				b.WriteString("· ")
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	caption := fmt.Sprintf("%d columns × %d rows", g.cols, g.rows)
	lines = append(lines, "", caption, "Arrows size · Enter insert · Esc cancel")
	width := 0
	for _, line := range lines {
		width = max(width, runeLen(line))
	}
	x := max(0, (w-width-4)/2)
	y := max(0, (h-len(lines)-3)/2)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < len(lines)+3 && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, width+4)), box, w)
	}
	e.put(x+1, y, " Insert table ", box.Bold(true), x+width+4)
	for row, line := range lines {
		style := box
		if row < tableGridMax {
			style = style.Foreground(e.theme.accent1)
		}
		if line == caption {
			style = style.Bold(true)
		}
		e.put(x+2, y+2+row, line, style, min(w, x+2+width))
	}
	e.screen.HideCursor()
}
