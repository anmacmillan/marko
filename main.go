package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

type editor struct {
	screen      tcell.Screen
	path        string
	lines       []string
	x, y, top   int
	dirty       bool
	status      string
	confirmQuit bool
	preferredX  int
	prompt      string
	promptValue string
	lastEdit    time.Time
	recovery    string
	theme       theme
}

type theme struct {
	text, heading1, heading2, heading3, table, quote, statusBG, statusFG tcell.Color
}

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: marko [FILE.md]")
		os.Exit(2)
	}
	path := ""
	if len(os.Args) == 2 {
		path = os.Args[1]
	} else {
		path = datedUntitledPath(time.Now())
	}
	e, err := newEditor(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer e.screen.Fini()
	e.run()
}

func newEditor(path string) (*editor, error) {
	data := []byte{}
	if path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := s.Init(); err != nil {
		return nil, err
	}
	status := "Ctrl-T new table  Ctrl-S save  Ctrl-Q quit"
	return &editor{screen: s, path: path, lines: lines, status: status, theme: selectedTheme()}, nil
}

func datedUntitledPath(now time.Time) string {
	return now.Format("20060102") + "_untitled.md"
}

func selectedTheme() theme {
	switch strings.ToLower(os.Getenv("MARKO_THEME")) {
	case "green":
		return theme{tcell.ColorPaleGreen, tcell.ColorLightGreen, tcell.ColorGreen, tcell.ColorDarkSeaGreen, tcell.ColorPaleGreen, tcell.ColorGray, tcell.ColorDarkGreen, tcell.ColorWhite}
	case "mono":
		return theme{tcell.ColorSilver, tcell.ColorWhite, tcell.ColorWhite, tcell.ColorSilver, tcell.ColorSilver, tcell.ColorGray, tcell.ColorGray, tcell.ColorBlack}
	default:
		return theme{tcell.ColorSilver, tcell.ColorLightSkyBlue, tcell.ColorLightGreen, tcell.ColorLightGoldenrodYellow, tcell.ColorPaleGreen, tcell.ColorGray, tcell.ColorDarkSlateGray, tcell.ColorWhite}
	}
}

func (e *editor) run() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			e.screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
	}()
	for {
		e.draw()
		switch ev := e.screen.PollEvent().(type) {
		case *tcell.EventResize:
			e.screen.Sync()
		case *tcell.EventInterrupt:
			e.autosave()
		case *tcell.EventKey:
			if e.key(ev) {
				return
			}
		}
	}
}

func (e *editor) key(ev *tcell.EventKey) bool {
	if e.prompt != "" {
		e.promptKey(ev)
		return false
	}
	switch ev.Key() {
	case tcell.KeyCtrlQ:
		if e.dirty && !e.confirmQuit {
			e.status = "Unsaved changes. Press Ctrl-Q again to quit."
			e.confirmQuit = true
			return false
		}
		return true
	case tcell.KeyCtrlS:
		if e.path == "" {
			e.prompt = "Save as: "
			e.promptValue = "untitled.md"
		} else {
			e.save()
		}
	case tcell.KeyCtrlT:
		e.insertTable()
	case tcell.KeyUp:
		e.moveVertical(-1)
	case tcell.KeyDown:
		e.moveVertical(1)
	case tcell.KeyLeft:
		if e.x > 0 {
			e.x--
		} else if e.y > 0 {
			e.y--
			e.x = runeLen(e.lines[e.y])
		}
	case tcell.KeyRight:
		if e.x < runeLen(e.lines[e.y]) {
			e.x++
		} else if e.y+1 < len(e.lines) {
			e.y++
			e.x = 0
		}
	case tcell.KeyHome:
		e.x = 0
	case tcell.KeyEnd:
		e.x = runeLen(e.lines[e.y])
	case tcell.KeyPgUp:
		e.y = max(0, e.y-10)
		e.clampX()
	case tcell.KeyPgDn:
		e.y = min(len(e.lines)-1, e.y+10)
		e.clampX()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.backspace()
	case tcell.KeyDelete:
		e.delete()
	case tcell.KeyEnter:
		e.enter()
	case tcell.KeyTab:
		if !e.nextTableCell() {
			e.insert("    ")
		}
	case tcell.KeyRune:
		e.insert(string(ev.Rune()))
	}
	e.confirmQuit = false
	e.preferredX = e.x
	if e.dirty {
		e.lastEdit = time.Now()
	}
	return false
}

func (e *editor) promptKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		e.prompt, e.promptValue = "", ""
		e.status = "Save cancelled"
	case tcell.KeyEnter:
		path := strings.TrimSpace(e.promptValue)
		if path == "" {
			e.status = "Enter a filename"
			return
		}
		if filepath.Ext(path) == "" {
			path += ".md"
		}
		e.path = path
		e.prompt, e.promptValue = "", ""
		e.save()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		r := []rune(e.promptValue)
		if len(r) > 0 {
			e.promptValue = string(r[:len(r)-1])
		}
	case tcell.KeyRune:
		e.promptValue += string(ev.Rune())
	}
}

func (e *editor) insertTable() {
	r := []rune(e.lines[e.y])
	before, after := string(r[:e.x]), string(r[e.x:])
	table := []string{
		before + "| Heading 1 | Heading 2 |",
		"| --------- | --------- |",
		"|           |           |" + after,
	}
	e.lines = append(e.lines[:e.y], append(table, e.lines[e.y+1:]...)...)
	e.x = runeLen(before) + 2
	e.dirty = true
	e.status = "Table created. Type a heading, then press Tab."
}

func (e *editor) insert(s string) {
	r := []rune(e.lines[e.y])
	e.lines[e.y] = string(r[:e.x]) + s + string(r[e.x:])
	e.x += runeLen(s)
	e.dirty = true
}

func (e *editor) backspace() {
	if e.x > 0 {
		r := []rune(e.lines[e.y])
		e.lines[e.y] = string(r[:e.x-1]) + string(r[e.x:])
		e.x--
		e.dirty = true
	} else if e.y > 0 {
		x := runeLen(e.lines[e.y-1])
		e.lines[e.y-1] += e.lines[e.y]
		e.lines = append(e.lines[:e.y], e.lines[e.y+1:]...)
		e.y--
		e.x = x
		e.dirty = true
	}
}

func (e *editor) delete() {
	r := []rune(e.lines[e.y])
	if e.x < len(r) {
		e.lines[e.y] = string(r[:e.x]) + string(r[e.x+1:])
		e.dirty = true
	} else if e.y+1 < len(e.lines) {
		e.lines[e.y] += e.lines[e.y+1]
		e.lines = append(e.lines[:e.y+1], e.lines[e.y+2:]...)
		e.dirty = true
	}
}

func (e *editor) enter() {
	if e.inTable(e.y) {
		cells := splitTable(e.lines[e.y])
		row := "| " + strings.Repeat(" | ", max(0, len(cells)-1)) + "|"
		e.lines = insertLine(e.lines, e.y+1, row)
		e.y++
		e.x = 2
		e.formatTable()
		return
	}
	r := []rune(e.lines[e.y])
	left, right := string(r[:e.x]), string(r[e.x:])
	e.lines[e.y] = left
	e.lines = insertLine(e.lines, e.y+1, right)
	e.y++
	e.x = 0
	e.dirty = true
}

func (e *editor) nextTableCell() bool {
	if !e.inTable(e.y) {
		return false
	}
	e.formatTable()
	r := []rune(e.lines[e.y])
	for i := e.x + 1; i < len(r); i++ {
		if r[i] == '|' && i+2 < len(r) {
			e.x = i + 2
			return true
		}
	}
	start, end := e.tableBounds(e.y)
	next := e.y + 1
	if next <= end && isSeparator(e.lines[next]) {
		next++
	}
	if next <= end {
		e.y, e.x = next, 2
		return true
	}
	cells := splitTable(e.lines[start])
	e.lines = insertLine(e.lines, end+1, "| "+strings.Repeat(" | ", max(0, len(cells)-1))+"|")
	e.y, e.x = end+1, 2
	e.formatTable()
	return true
}

func (e *editor) inTable(y int) bool {
	return y >= 0 && y < len(e.lines) && strings.Count(e.lines[y], "|") >= 2
}

func (e *editor) tableBounds(y int) (int, int) {
	start, end := y, y
	for start > 0 && e.inTable(start-1) {
		start--
	}
	for end+1 < len(e.lines) && e.inTable(end+1) {
		end++
	}
	return start, end
}

func (e *editor) formatTable() {
	start, end := e.tableBounds(e.y)
	rows := make([][]string, end-start+1)
	widths := []int{}
	for y := start; y <= end; y++ {
		rows[y-start] = splitTable(e.lines[y])
		if !isSeparator(e.lines[y]) {
			for i, cell := range rows[y-start] {
				for len(widths) <= i {
					widths = append(widths, 3)
				}
				widths[i] = max(widths[i], runeLen(cell))
			}
		}
	}
	for i, cells := range rows {
		out := make([]string, len(widths))
		for col, width := range widths {
			if isSeparator(e.lines[start+i]) {
				out[col] = strings.Repeat("-", width)
			} else {
				value := ""
				if col < len(cells) {
					value = cells[col]
				}
				out[col] = value + strings.Repeat(" ", width-runeLen(value))
			}
		}
		e.lines[start+i] = "| " + strings.Join(out, " | ") + " |"
	}
	e.dirty = true
}

func (e *editor) moveVertical(delta int) {
	e.y = max(0, min(len(e.lines)-1, e.y+delta))
	e.clampX()
}

func (e *editor) clampX() {
	e.x = min(e.x, runeLen(e.lines[e.y]))
}

func (e *editor) save() {
	text := strings.Join(e.lines, "\n")
	if err := os.MkdirAll(filepath.Dir(e.path), 0755); err != nil {
		e.status = err.Error()
		return
	}
	if err := os.WriteFile(e.path, []byte(text), 0644); err != nil {
		e.status = err.Error()
		return
	}
	e.dirty = false
	e.status = "Saved " + e.path
}

func (e *editor) autosave() {
	if !e.dirty || e.prompt != "" || e.lastEdit.IsZero() {
		return
	}
	if time.Since(e.lastEdit) >= 2*time.Second {
		e.save()
		e.status = "Autosaved " + e.path
	}
}

func (e *editor) draw() {
	e.screen.Clear()
	w, h := e.screen.Size()
	bodyH := max(1, h-1)
	if e.y < e.top {
		e.top = e.y
	}
	if e.y >= e.top+bodyH {
		e.top = e.y - bodyH + 1
	}
	for row := 0; row < bodyH && e.top+row < len(e.lines); row++ {
		y := e.top + row
		e.drawLine(row, e.lines[y], y == e.y, w)
	}
	name := filepath.Base(e.path)
	if e.path == "" {
		name = "Untitled"
	}
	mark := ""
	if e.dirty {
		mark = " *"
	}
	status := fmt.Sprintf(" %s%s  Ln %d, Col %d  %s", name, mark, e.y+1, e.x+1, e.status)
	if e.prompt != "" {
		status = " " + e.prompt + e.promptValue
	}
	e.put(0, h-1, status, tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG), w)
	if e.prompt != "" {
		e.screen.ShowCursor(1+runeLen(e.prompt)+runeLen(e.promptValue), h-1)
	} else {
		e.screen.ShowCursor(e.x, e.y-e.top)
	}
	e.screen.Show()
}

func (e *editor) drawLine(row int, line string, current bool, width int) {
	style := tcell.StyleDefault.Foreground(e.theme.text)
	trimmed := strings.TrimSpace(line)
	y := row + e.top
	if e.inTable(y) && !e.cursorInSameTable(y) {
		line = e.renderTableLine(y)
		style = style.Foreground(e.theme.table)
	}
	if level, text, ok := heading(line); ok && !current {
		line = text
		switch level {
		case 1:
			style = style.Bold(true).Underline(true).Foreground(e.theme.heading1)
		case 2:
			style = style.Bold(true).Foreground(e.theme.heading2)
		default:
			style = style.Bold(true).Foreground(e.theme.heading3)
		}
	}
	if !current {
		switch {
		case strings.HasPrefix(trimmed, ">"):
			style = style.Foreground(e.theme.quote)
		case isSeparator(line):
			style = style.Foreground(tcell.ColorDarkCyan)
		case e.inTable(y):
			style = style.Foreground(e.theme.table)
		}
	}
	if current {
		e.put(0, row, line, style, width)
	} else {
		e.putInline(0, row, line, style, width)
	}
}

func heading(line string) (int, string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	level := 0
	for level < len(trimmed) && level < 6 && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmed) || trimmed[level] != ' ' {
		return 0, line, false
	}
	return level, strings.TrimSpace(trimmed[level+1:]), true
}

func (e *editor) cursorInSameTable(y int) bool {
	if !e.inTable(e.y) {
		return false
	}
	start, end := e.tableBounds(e.y)
	return y >= start && y <= end
}

func (e *editor) renderTableLine(y int) string {
	start, end := e.tableBounds(y)
	widths := []int{}
	for lineY := start; lineY <= end; lineY++ {
		if isSeparator(e.lines[lineY]) {
			continue
		}
		for col, cell := range splitTable(e.lines[lineY]) {
			for len(widths) <= col {
				widths = append(widths, 3)
			}
			widths[col] = max(widths[col], runeLen(cell))
		}
	}

	cells := splitTable(e.lines[y])
	if isSeparator(e.lines[y]) {
		parts := make([]string, len(widths))
		for col, width := range widths {
			parts[col] = strings.Repeat("─", width+2)
		}
		return "├" + strings.Join(parts, "┼") + "┤"
	}

	parts := make([]string, len(widths))
	for col, width := range widths {
		value := ""
		if col < len(cells) {
			value = cells[col]
		}
		parts[col] = " " + value + strings.Repeat(" ", width-runeLen(value)+1)
	}
	return "│" + strings.Join(parts, "│") + "│"
}

func (e *editor) put(x, y int, text string, style tcell.Style, maxWidth int) {
	for _, r := range text {
		if x >= maxWidth {
			break
		}
		e.screen.SetContent(x, y, r, nil, style)
		x++
	}
}

func (e *editor) putInline(x, y int, text string, base tcell.Style, maxWidth int) {
	runes := []rune(text)
	for i := 0; i < len(runes) && x < maxWidth; {
		marker, styled, end := emphasisAt(runes, i)
		if marker > 0 {
			for _, r := range runes[i+marker : end] {
				if x >= maxWidth {
					break
				}
				e.screen.SetContent(x, y, r, nil, styled(base))
				x++
			}
			i = end + marker
			continue
		}
		e.screen.SetContent(x, y, runes[i], nil, base)
		x++
		i++
	}
}

func emphasisAt(runes []rune, start int) (int, func(tcell.Style) tcell.Style, int) {
	type markerStyle struct {
		marker string
		style  func(tcell.Style) tcell.Style
	}
	markers := []markerStyle{
		{"**", func(s tcell.Style) tcell.Style { return s.Bold(true) }},
		{"__", func(s tcell.Style) tcell.Style { return s.Bold(true) }},
		{"~~", func(s tcell.Style) tcell.Style { return s.StrikeThrough(true) }},
		{"*", func(s tcell.Style) tcell.Style { return s.Italic(true) }},
		{"_", func(s tcell.Style) tcell.Style { return s.Italic(true) }},
	}
	for _, candidate := range markers {
		marker := []rune(candidate.marker)
		if !runesEqualAt(runes, start, marker) {
			continue
		}
		for end := start + len(marker) + 1; end+len(marker) <= len(runes); end++ {
			if runesEqualAt(runes, end, marker) {
				return len(marker), candidate.style, end
			}
		}
	}
	return 0, nil, 0
}

func runesEqualAt(haystack []rune, start int, needle []rune) bool {
	if start < 0 || start+len(needle) > len(haystack) {
		return false
	}
	for i := range needle {
		if haystack[start+i] != needle[i] {
			return false
		}
	}
	return true
}

func splitTable(line string) []string {
	line = strings.TrimSpace(strings.Trim(line, "|"))
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparator(line string) bool {
	cells := splitTable(line)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.Trim(c, ":")
		if len(c) < 3 || strings.Trim(c, "-") != "" {
			return false
		}
	}
	return true
}

func insertLine(lines []string, at int, value string) []string {
	lines = append(lines, "")
	copy(lines[at+1:], lines[at:])
	lines[at] = value
	return lines
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}
