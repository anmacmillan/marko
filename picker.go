package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// The picker is a single overlay reused for opening, saving, and renaming
// files: a text input on top, a fuzzy-filtered list of recents and directory
// entries below. Enter on a directory descends into it; Backspace on an
// empty input goes up one level; "z query" plus Tab jumps via zoxide.

type pickerMode int

const (
	pickerOpen pickerMode = iota
	pickerSaveAs
	pickerRename
	pickerNotes // search the notes directory by name and content
)

type pickerItem struct {
	name    string // basename shown and matched against
	path    string // absolute path
	dir     bool
	recent  bool
	depth   int // indent level in the expanded folder tree
	modTime time.Time
	content string // original file text (notes search only)
	lowered string // lowercased content for case-insensitive matching
	snippet string // matching line shown next to the name
}

// listFirst reports whether the highlighted list row, not the input, is the
// primary target — true for the open and notes-search pickers.
func (p *picker) listFirst() bool {
	return p.mode == pickerOpen || p.mode == pickerNotes
}

type picker struct {
	mode      pickerMode
	dir       string // directory being browsed / saved into
	input     string
	cursor    int
	selectAll bool
	index     int             // -1 = input row focused; >= 0 selects a list row
	expanded  map[string]bool // directories unfolded inline with Right arrow
	items     []pickerItem
	err       string
}

var pickerFileExtensions = map[string]bool{".md": true, ".markdown": true, ".txt": true}

func (e *editor) openFilePicker() {
	dir := e.pickerStartDir()
	e.picker = &picker{mode: pickerOpen, dir: dir, index: 0}
	e.refreshPicker()
}

const (
	maxNotesIndexFiles = 2000
	maxNotesFileBytes  = 128 * 1024
)

// openNotesSearch searches every note in the notes directory by filename and
// content — the "which file was that client call in?" picker.
func (e *editor) openNotesSearch() {
	dir := notesDir()
	e.picker = &picker{mode: pickerNotes, dir: dir, index: 0, items: loadNotesIndex(dir)}
	e.flashStatus()
}

func loadNotesIndex(dir string) []pickerItem {
	var items []pickerItem
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !pickerFileExtensions[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		if len(items) >= maxNotesIndexFiles {
			return filepath.SkipAll
		}
		item := pickerItem{name: d.Name(), path: path}
		if rel, err := filepath.Rel(dir, path); err == nil {
			item.name = rel
		}
		if info, err := d.Info(); err == nil {
			item.modTime = info.ModTime()
			if info.Size() <= maxNotesFileBytes {
				if data, err := os.ReadFile(path); err == nil {
					item.content = string(data)
					item.lowered = strings.ToLower(item.content)
				}
			}
		}
		items = append(items, item)
		return nil
	})
	sort.Slice(items, func(i, j int) bool {
		if !items[i].modTime.Equal(items[j].modTime) {
			return items[i].modTime.After(items[j].modTime)
		}
		return items[i].name < items[j].name
	})
	return items
}

// visibleNotes filters the notes index: every space-separated term must
// appear in the filename or the content, filename hits ranking higher.
func (p *picker) visibleNotes() []pickerItem {
	terms := strings.Fields(strings.ToLower(p.input))
	if len(terms) == 0 {
		return p.items
	}
	type scored struct {
		item  pickerItem
		score int
	}
	var matches []scored
	for _, item := range p.items {
		nameLower := strings.ToLower(item.name)
		score, ok, snippetTerm := 0, true, ""
		for _, term := range terms {
			inName := strings.Contains(nameLower, term)
			inContent := strings.Contains(item.lowered, term)
			if !inName && !inContent {
				ok = false
				break
			}
			if inName {
				score += 8
			}
			if inContent {
				score += 2
				if snippetTerm == "" {
					snippetTerm = term
				}
			}
		}
		if !ok {
			continue
		}
		if s, matched := fuzzyScore(p.input, item.name); matched {
			score += s
		}
		item.snippet = noteSnippet(item.content, item.lowered, snippetTerm)
		matches = append(matches, scored{item, score})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	out := make([]pickerItem, len(matches))
	for i, m := range matches {
		out[i] = m.item
	}
	return out
}

// noteSnippet returns the original-case line holding the first match of term.
// Line lookup goes via the lowered copy so Unicode case folding that changes
// byte offsets cannot point the snippet at the wrong text.
func noteSnippet(original, lowered, term string) string {
	if term == "" {
		return ""
	}
	idx := strings.Index(lowered, term)
	if idx < 0 {
		return ""
	}
	lineIdx := strings.Count(lowered[:idx], "\n")
	lines := strings.SplitN(original, "\n", lineIdx+2)
	if lineIdx < len(lines) {
		return strings.Join(strings.Fields(lines[lineIdx]), " ")
	}
	return ""
}

func (e *editor) openSaveAsPicker() {
	dir := e.pickerStartDir()
	name := e.suggestedSaveName(time.Now())
	if !e.untitled && e.path != "" {
		name = filepath.Base(e.path)
	}
	e.picker = &picker{mode: pickerSaveAs, dir: dir, input: name, cursor: runeLen(name), selectAll: true, index: -1}
	e.refreshPicker()
	e.flashStatus()
}

func (e *editor) openRenamePicker() {
	if e.path == "" {
		return
	}
	// Anchor renames to the file's own directory so a bare name never moves
	// the file into whatever the process working directory happens to be.
	dir := absDir(filepath.Dir(e.path))
	name := filepath.Base(e.path)
	e.renameFrom = e.path
	e.picker = &picker{mode: pickerRename, dir: dir, input: name, cursor: runeLen(name), selectAll: true, index: -1}
	e.refreshPicker()
}

// pickerStartDir anchors the picker to the current file's own directory —
// for untitled notes that is the notes directory, so quick captures never
// depend on whatever the process working directory happens to be.
func (e *editor) pickerStartDir() string {
	if e.path != "" {
		return absDir(filepath.Dir(e.path))
	}
	return absDir(".")
}

func absDir(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func (e *editor) refreshPicker() {
	p := e.picker
	if p == nil || p.mode == pickerNotes {
		return
	}
	p.items = p.items[:0]
	if p.mode == pickerOpen {
		for _, path := range loadRecent() {
			if filepath.Dir(path) == p.dir {
				continue // shown as a directory entry below
			}
			item := pickerItem{name: filepath.Base(path), path: path, recent: true}
			if info, err := os.Stat(path); err == nil {
				item.modTime = info.ModTime()
			}
			p.items = append(p.items, item)
		}
	}
	if p.mode == pickerSaveAs {
		for _, dir := range loadRecentDirs() {
			if dir == p.dir {
				continue
			}
			p.items = append(p.items, pickerItem{name: filepath.Base(dir), path: dir, dir: true, recent: true})
		}
	}
	if parent := filepath.Dir(p.dir); p.mode != pickerOpen && parent != p.dir {
		p.items = append(p.items, pickerItem{name: "..", path: parent, dir: true})
	}
	p.items = append(p.items, listPickerTree(p.dir, 0, p.expanded)...)
}

// listPickerTree lists dir and splices the contents of any unfolded
// subdirectory in after its row, indented one level deeper.
func listPickerTree(dir string, depth int, expanded map[string]bool) []pickerItem {
	if depth > 8 {
		return nil // symlink-cycle guard
	}
	var out []pickerItem
	for _, item := range listPickerDir(dir) {
		item.depth = depth
		out = append(out, item)
		if item.dir && expanded[item.path] {
			out = append(out, listPickerTree(item.path, depth+1, expanded)...)
		}
	}
	return out
}

func listPickerDir(dir string) []pickerItem {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs, files []pickerItem
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		if entry.IsDir() {
			dirs = append(dirs, pickerItem{name: name, path: path, dir: true})
			continue
		}
		if !pickerFileExtensions[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		item := pickerItem{name: name, path: path}
		if info, err := entry.Info(); err == nil {
			item.modTime = info.ModTime()
		}
		files = append(files, item)
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool {
		if !files[i].modTime.Equal(files[j].modTime) {
			return files[i].modTime.After(files[j].modTime)
		}
		return files[i].name < files[j].name
	})
	return append(dirs, files...)
}

// fuzzyScore reports whether pattern is a case-insensitive subsequence of
// text, and how good the match is: consecutive runs and word starts score
// higher, and matches beginning earlier win ties.
func fuzzyScore(pattern, text string) (int, bool) {
	if pattern == "" {
		return 0, true
	}
	p := []rune(strings.ToLower(pattern))
	t := []rune(strings.ToLower(text))
	score, pi, streak := 0, 0, 0
	for ti := 0; ti < len(t) && pi < len(p); ti++ {
		if t[ti] != p[pi] {
			streak = 0
			continue
		}
		streak++
		score += 1 + streak
		if ti == 0 || t[ti-1] == '/' || t[ti-1] == ' ' || t[ti-1] == '_' || t[ti-1] == '-' || t[ti-1] == '.' {
			score += 4
		}
		if pi == 0 {
			score -= ti / 4 // earlier first matches rank higher
		}
		pi++
	}
	if pi < len(p) {
		return 0, false
	}
	return score, true
}

func (p *picker) matchText(item pickerItem) string {
	if item.recent {
		return shortenHomePath(item.path)
	}
	return item.name
}

func (p *picker) visibleItems() []pickerItem {
	if p.mode == pickerNotes {
		return p.visibleNotes()
	}
	if strings.TrimSpace(p.input) == "" || strings.HasPrefix(p.input, "z ") {
		return p.items
	}
	type scored struct {
		item  pickerItem
		score int
	}
	var matches []scored
	for _, item := range p.items {
		if score, ok := fuzzyScore(p.input, p.matchText(item)); ok {
			matches = append(matches, scored{item, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	out := make([]pickerItem, len(matches))
	for i, m := range matches {
		out[i] = m.item
	}
	return out
}

func (p *picker) clampIndex(visible int) {
	minIndex := 0
	if !p.listFirst() {
		minIndex = -1
	}
	if p.index < minIndex {
		p.index = minIndex
	}
	if p.index >= visible {
		p.index = visible - 1
	}
	if p.index < 0 && p.listFirst() {
		p.index = 0
	}
}

func (e *editor) pickerMove(delta int) {
	p := e.picker
	visible := len(p.visibleItems())
	p.index += delta
	p.clampIndex(visible)
	p.selectAll = false
}

func (e *editor) pickerKey(ev *tcell.EventKey) {
	p := e.picker
	switch eventKey(ev) {
	case tcell.KeyEsc, tcell.KeyCtrlQ:
		e.picker = nil
		e.renameFrom = ""
		e.status = "Cancelled"
	case tcell.KeyUp:
		e.pickerMove(-1)
	case tcell.KeyDown:
		e.pickerMove(1)
	case tcell.KeyPgUp:
		e.pickerMove(-10)
	case tcell.KeyPgDn:
		e.pickerMove(10)
	case tcell.KeyEnter:
		e.pickerSubmit()
	case tcell.KeyTab:
		e.pickerTab()
	case tcell.KeyLeft:
		if e.pickerCollapse() {
			return
		}
		p.selectAll = false
		if p.cursor > 0 {
			p.cursor--
		}
	case tcell.KeyRight:
		if e.pickerExpand() {
			return
		}
		p.selectAll = false
		if p.cursor < runeLen(p.input) {
			p.cursor++
		}
	case tcell.KeyHome:
		p.selectAll, p.cursor = false, 0
	case tcell.KeyEnd:
		p.selectAll, p.cursor = false, runeLen(p.input)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if p.selectAll {
			p.input, p.cursor, p.selectAll = "", 0, false
			e.pickerInputChanged()
			return
		}
		if p.input == "" {
			e.pickerAscend()
			return
		}
		r := []rune(p.input)
		if p.cursor > 0 && p.cursor <= len(r) {
			p.input = string(r[:p.cursor-1]) + string(r[p.cursor:])
			p.cursor--
			e.pickerInputChanged()
		}
	case tcell.KeyDelete:
		if p.selectAll {
			p.input, p.cursor, p.selectAll = "", 0, false
			e.pickerInputChanged()
			return
		}
		r := []rune(p.input)
		if p.cursor < len(r) {
			p.input = string(r[:p.cursor]) + string(r[p.cursor+1:])
			e.pickerInputChanged()
		}
	case tcell.KeyRune:
		if !textInputModifiers(ev.Modifiers()) {
			return
		}
		if p.selectAll {
			p.input, p.cursor, p.selectAll = string(ev.Rune()), 1, false
			e.pickerInputChanged()
			return
		}
		r := []rune(p.input)
		p.input = string(r[:p.cursor]) + string(ev.Rune()) + string(r[p.cursor:])
		p.cursor++
		e.pickerInputChanged()
	}
}

func (e *editor) pickerInputChanged() {
	p := e.picker
	p.err = ""
	if p.listFirst() {
		p.index = 0
	} else {
		p.index = -1
	}
}

// treeRow reports whether the highlighted row is a real directory inside the
// browsed tree (not "..", not a recent-folder shortcut) and returns it.
func (p *picker) treeRow() (pickerItem, bool) {
	if p.mode == pickerNotes || p.index < 0 {
		return pickerItem{}, false
	}
	visible := p.visibleItems()
	if p.index >= len(visible) {
		return pickerItem{}, false
	}
	item := visible[p.index]
	if item.recent || item.name == ".." {
		return pickerItem{}, false
	}
	return item, true
}

// pickerExpand unfolds the highlighted directory inline (Right arrow).
func (e *editor) pickerExpand() bool {
	p := e.picker
	item, ok := p.treeRow()
	if !ok || !item.dir || p.expanded[item.path] {
		return false
	}
	if p.expanded == nil {
		p.expanded = map[string]bool{}
	}
	p.expanded[item.path] = true
	e.refreshPicker()
	return true
}

// pickerCollapse folds the highlighted directory, or jumps to the parent
// row of a nested entry (Left arrow).
func (e *editor) pickerCollapse() bool {
	p := e.picker
	item, ok := p.treeRow()
	if !ok {
		return false
	}
	if item.dir && p.expanded[item.path] {
		delete(p.expanded, item.path)
		e.refreshPicker()
		return true
	}
	if item.depth > 0 {
		visible := p.visibleItems()
		for i := p.index - 1; i >= 0; i-- {
			if visible[i].dir && !visible[i].recent && visible[i].name != ".." && visible[i].depth == item.depth-1 {
				p.index = i
				return true
			}
		}
	}
	return false
}

func (e *editor) pickerAscend() {
	p := e.picker
	if p.mode == pickerNotes {
		return // flat search over the notes tree; nothing to ascend to
	}
	parent := filepath.Dir(p.dir)
	if parent == p.dir {
		return
	}
	p.dir = parent
	p.index = 0
	if p.mode != pickerOpen {
		p.index = -1
	}
	e.refreshPicker()
}

func (e *editor) pickerDescend(dir string) {
	p := e.picker
	p.dir = dir
	p.input, p.cursor, p.selectAll = "", 0, false
	p.index = 0
	if p.mode != pickerOpen {
		p.index = -1
	}
	e.refreshPicker()
}

func (e *editor) pickerTab() {
	p := e.picker
	if strings.HasPrefix(strings.TrimSpace(p.input), "z ") {
		value, err := completeZoxidePromptValue(p.input)
		if err != nil {
			p.err = err.Error()
			return
		}
		if value == "" {
			p.err = "Type z query, then Tab"
			return
		}
		if strings.HasSuffix(value, string(os.PathSeparator)) {
			e.pickerDescend(filepath.Clean(value))
			return
		}
		p.input = value
		p.cursor = runeLen(value)
		p.selectAll = false
		return
	}
	visible := p.visibleItems()
	if p.index >= 0 && p.index < len(visible) && visible[p.index].dir {
		e.pickerDescend(visible[p.index].path)
	}
}

func (e *editor) pickerSubmit() {
	p := e.picker
	visible := p.visibleItems()
	if p.index >= 0 && p.index < len(visible) {
		item := visible[p.index]
		if item.dir {
			e.pickerDescend(item.path)
			return
		}
		if p.listFirst() {
			e.picker = nil
			e.openFile(item.path)
			return
		}
		// Save As / Rename: adopt the highlighted file's name into the input.
		p.input = item.name
		p.cursor = runeLen(item.name)
		p.selectAll = false
		p.index = -1
		return
	}
	input := strings.TrimSpace(p.input)
	if input == "" {
		p.err = "Enter a filename"
		return
	}
	switch p.mode {
	case pickerOpen, pickerNotes:
		expanded, err := expandPathInput(input)
		if err != nil {
			p.err = err.Error()
			return
		}
		if !filepath.IsAbs(expanded) {
			expanded = filepath.Join(p.dir, expanded)
		}
		e.picker = nil
		e.openPath(expanded)
	case pickerSaveAs:
		expanded, err := e.expandSavePathInput(input)
		if err != nil {
			p.err = err.Error()
			return
		}
		if !filepath.IsAbs(expanded) {
			expanded = filepath.Join(p.dir, expanded)
		}
		e.picker = nil
		e.path = expanded
		e.untitled = false
		e.conflict = false
		e.modTime = time.Time{}
		e.save()
	case pickerRename:
		if filepath.Ext(input) == "" {
			input += ".md"
		}
		target := input
		if !filepath.IsAbs(target) {
			target = filepath.Join(p.dir, target)
		}
		if target != e.renameFrom {
			if _, err := os.Stat(target); err == nil {
				p.err = filepath.Base(target) + " already exists"
				return
			}
		}
		e.picker = nil
		e.renameFile(target)
	}
}

func shortenHomePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

func (p *picker) title() string {
	switch p.mode {
	case pickerSaveAs:
		return "Save as"
	case pickerRename:
		return "Rename"
	case pickerNotes:
		return "Search notes"
	}
	return "Open"
}

func (p *picker) footerHint() string {
	switch p.mode {
	case pickerOpen:
		return "Enter open · → unfold folder · Backspace up · z query Tab · Esc cancel"
	case pickerNotes:
		return "Type words to match names & content · Enter open · Esc cancel"
	}
	return "Enter save · →/← unfold/fold folders · Backspace up · Esc cancel"
}

// pickerCollision reports a warning when committing the current input would
// overwrite an existing file.
func (p *picker) collision(renameFrom string) string {
	if p.listFirst() {
		return ""
	}
	input := strings.TrimSpace(p.input)
	if input == "" || strings.HasPrefix(input, "z ") {
		return ""
	}
	if filepath.Ext(input) == "" {
		input += ".md"
	}
	target := input
	if !filepath.IsAbs(target) {
		target = filepath.Join(p.dir, target)
	}
	if target == renameFrom {
		return ""
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		if p.mode == pickerRename {
			return filepath.Base(target) + " already exists"
		}
		return "overwrites " + filepath.Base(target)
	}
	return ""
}

type pickerRow struct {
	text    string
	header  bool
	itemIdx int // index into visibleItems, -1 for headers
}

func (e *editor) pickerRows(visible []pickerItem, width int) []pickerRow {
	p := e.picker
	rows := []pickerRow{}
	showHeaders := (p.mode == pickerOpen || p.mode == pickerSaveAs) && strings.TrimSpace(p.input) == ""
	recentHeader := "Recent"
	if p.mode == pickerSaveAs {
		recentHeader = "Recent folders"
	}
	lastRecent := false
	for i, item := range visible {
		if showHeaders {
			if i == 0 && item.recent {
				rows = append(rows, pickerRow{text: recentHeader, header: true, itemIdx: -1})
			}
			if (i == 0 && !item.recent) || (lastRecent && !item.recent) {
				rows = append(rows, pickerRow{text: "This folder", header: true, itemIdx: -1})
			}
			lastRecent = item.recent
		}
		label := item.name
		if item.dir {
			label += string(os.PathSeparator)
		}
		if item.recent {
			label = truncatePathMiddle(shortenHomePath(item.path), width-20)
			if item.dir {
				label += string(os.PathSeparator)
			}
		}
		if item.depth > 0 {
			label = strings.Repeat("  ", item.depth) + label
		}
		if item.snippet != "" {
			label += "  · " + item.snippet
		} else if !item.dir && !item.modTime.IsZero() {
			label += "  · " + item.modTime.Format("2006-01-02 15:04")
		}
		if runeLen(label) > width {
			label = truncatePathMiddle(label, width)
		}
		rows = append(rows, pickerRow{text: label, itemIdx: i})
	}
	return rows
}

func (e *editor) drawPicker(w, h int) {
	p := e.picker
	boxWidth := min(w-4, 92)
	if boxWidth < 24 {
		boxWidth = min(24, w)
	}
	inner := boxWidth - 4
	visible := p.visibleItems()
	rows := e.pickerRows(visible, inner-2)

	maxList := max(3, h-9)
	// Keep the selected row inside the window.
	selectedRow := 0
	for i, row := range rows {
		if row.itemIdx == p.index {
			selectedRow = i
			break
		}
	}
	start := 0
	if len(rows) > maxList {
		start = min(max(0, selectedRow-maxList/2), len(rows)-maxList)
	}
	listRows := rows[start:min(len(rows), start+maxList)]

	boxHeight := len(listRows) + 6
	x := max(0, (w-boxWidth)/2)
	y := max(0, (h-boxHeight)/2)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < boxHeight && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, boxWidth)), box, w)
	}

	title := " " + p.title() + " — " + truncatePathMiddle(shortenHomePath(p.dir), inner-runeLen(p.title())-4) + " "
	e.put(x+1, y, title, box.Bold(true), x+boxWidth-1)

	inputStyle := box
	if p.selectAll {
		inputStyle = e.selectedStyle(box)
	}
	prompt := "> "
	e.put(x+2, y+2, prompt, box, x+boxWidth-2)
	inputText := p.input
	if runeLen(inputText) > inner-2 {
		r := []rune(inputText)
		inputText = string(r[len(r)-(inner-2):])
	}
	e.put(x+2+runeLen(prompt), y+2, inputText+strings.Repeat(" ", max(0, inner-2-runeLen(inputText))), inputStyle, x+boxWidth-2)

	listTop := y + 4
	total := recentItemCount(groupedRecentFiles(loadRecent()))
	for i, row := range listRows {
		style := box
		prefix := "  "
		switch {
		case row.header:
			style = style.Bold(true)
		case row.itemIdx == p.index:
			prefix = "> "
			style = e.selectedStyle(style)
		case visible[row.itemIdx].dir:
			style = style.Foreground(e.theme.accent1)
		case visible[row.itemIdx].recent:
			style = e.recentStyle(style, row.itemIdx, max(total, len(visible)))
		}
		e.put(x+2, listTop+i, prefix+row.text, style, x+boxWidth-2)
	}
	if len(listRows) == 0 {
		empty := "<empty folder>"
		if p.mode == pickerNotes {
			empty = "<no matching notes>"
			if strings.TrimSpace(p.input) != "" {
				empty = "<no matching notes — Enter creates \"" + strings.TrimSpace(p.input) + "\">"
			}
		} else if p.mode == pickerOpen && strings.TrimSpace(p.input) != "" {
			empty = "<no matches — Enter creates \"" + strings.TrimSpace(p.input) + "\">"
		}
		e.put(x+2, listTop, empty, box, x+boxWidth-2)
	}

	footer := p.footerHint()
	if warning := p.collision(e.renameFrom); warning != "" {
		footer = warning
	}
	if p.err != "" {
		footer = p.err
	}
	e.put(x+2, y+boxHeight-1, truncatePathMiddle(footer, inner), box, x+boxWidth-2)

	if p.index < 0 || p.mode == pickerOpen {
		e.screen.ShowCursor(x+2+runeLen(prompt)+min(p.cursor, runeLen(inputText)), y+2)
	} else {
		e.screen.HideCursor()
	}
}
