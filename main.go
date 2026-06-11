package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	themeName   string
	undo, redo  []snapshot
	selecting   bool
	selX, selY  int
	mouseDown   bool
	search      string
	replace     string
	lastAction  time.Time
	focusMode   bool
	modTime     time.Time
	conflict    bool
	showHelp    bool
	showRecent  bool
	recent      []string
	recentIndex int
}

type snapshot struct {
	lines []string
	x, y  int
}

type visualRow struct {
	y, start int
	text     string
}

type theme struct {
	text, heading1, heading2, heading3, table, quote, statusBG, statusFG tcell.Color
}

var themeNames = []string{"calm", "green", "mono", "light"}

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: marko [FILE.md]")
		os.Exit(2)
	}
	path := ""
	if len(os.Args) == 2 {
		path = os.Args[1]
	} else {
		path = uniqueUntitledPath(time.Now(), ".")
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
	var modTime time.Time
	if info, err := os.Stat(path); err == nil {
		modTime = info.ModTime()
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
	s.EnableMouse()
	s.EnablePaste()
	status := "Ctrl-T new table  Ctrl-S save  Ctrl-Q quit"
	themeName := selectedThemeName()
	e := &editor{screen: s, path: path, lines: lines, status: status, themeName: themeName, theme: themeByName(themeName), lastAction: time.Now(), modTime: modTime}
	e.rememberRecent(path)
	return e, nil
}

func datedUntitledPath(now time.Time) string {
	return now.Format("20060102") + "_untitled.md"
}

func uniqueUntitledPath(now time.Time, dir string) string {
	base := now.Format("20060102") + "_untitled"
	path := filepath.Join(dir, base+".md")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(dir, base+"_"+strconv.Itoa(n)+".md")
	}
}

func selectedThemeName() string {
	if name := strings.ToLower(os.Getenv("MARKO_THEME")); validTheme(name) {
		return name
	}
	if data, err := os.ReadFile(themeConfigPath()); err == nil {
		if name := strings.TrimSpace(strings.ToLower(string(data))); validTheme(name) {
			return name
		}
	}
	return "calm"
}

func validTheme(name string) bool {
	for _, candidate := range themeNames {
		if name == candidate {
			return true
		}
	}
	return false
}

func themeByName(name string) theme {
	switch name {
	case "green":
		return theme{tcell.ColorPaleGreen, tcell.ColorLightGreen, tcell.ColorGreen, tcell.ColorDarkSeaGreen, tcell.ColorPaleGreen, tcell.ColorGray, tcell.ColorDarkGreen, tcell.ColorWhite}
	case "mono":
		return theme{tcell.ColorSilver, tcell.ColorWhite, tcell.ColorWhite, tcell.ColorSilver, tcell.ColorSilver, tcell.ColorGray, tcell.ColorGray, tcell.ColorBlack}
	case "light":
		return theme{tcell.ColorBlack, tcell.ColorDarkBlue, tcell.ColorDarkGreen, tcell.ColorDarkGoldenrod, tcell.ColorDarkGreen, tcell.ColorDarkSlateGray, tcell.ColorLightGray, tcell.ColorBlack}
	default:
		return theme{tcell.ColorSilver, tcell.ColorLightSkyBlue, tcell.ColorLightGreen, tcell.ColorLightGoldenrodYellow, tcell.ColorPaleGreen, tcell.ColorGray, tcell.ColorDarkSlateGray, tcell.ColorWhite}
	}
}

func themeConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", ".marko-theme")
	}
	return filepath.Join(dir, "marko", "theme")
}

func recentConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", ".marko-recent")
	}
	return filepath.Join(dir, "marko", "recent")
}

func loadRecent() []string {
	data, err := os.ReadFile(recentConfigPath())
	if err != nil {
		return nil
	}
	var recent []string
	for _, path := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				recent = append(recent, path)
			}
		}
		if len(recent) == 5 {
			break
		}
	}
	return recent
}

func (e *editor) rememberRecent(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	recent := []string{abs}
	for _, existing := range loadRecent() {
		if existing != abs {
			recent = append(recent, existing)
		}
		if len(recent) == 5 {
			break
		}
	}
	config := recentConfigPath()
	if os.MkdirAll(filepath.Dir(config), 0755) == nil {
		_ = os.WriteFile(config, []byte(strings.Join(recent, "\n")+"\n"), 0644)
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
			e.focusMode = time.Since(e.lastAction) >= 5*time.Second
		case *tcell.EventKey:
			e.lastAction, e.focusMode = time.Now(), false
			if e.key(ev) {
				return
			}
		case *tcell.EventMouse:
			e.lastAction, e.focusMode = time.Now(), false
			e.mouse(ev)
		case *tcell.EventClipboard:
			e.insertText(string(ev.Data()))
		}
	}
}

func (e *editor) mouse(ev *tcell.EventMouse) {
	x, sy := ev.Position()
	w, h := e.screen.Size()
	if sy >= h-1 {
		return
	}
	rows := e.visualRows(max(1, w))
	index := min(len(rows)-1, max(0, e.top+sy))
	y := rows[index].y
	x = min(runeLen(e.lines[y]), max(0, rows[index].start+x))
	if ev.Buttons()&tcell.Button1 != 0 {
		if !e.mouseDown {
			e.selX, e.selY = x, y
			e.mouseDown = true
		}
		e.x, e.y = x, y
		e.selecting = e.selX != e.x || e.selY != e.y
	} else {
		e.mouseDown = false
	}
}

func (e *editor) key(ev *tcell.EventKey) bool {
	if e.showRecent {
		e.recentKey(ev)
		return false
	}
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
		if ev.Modifiers()&tcell.ModShift != 0 {
			e.prompt = "Save as: "
			e.promptValue = e.path
		} else if e.path == "" {
			e.prompt = "Save as: "
			e.promptValue = "untitled.md"
		} else {
			e.save()
		}
	case tcell.KeyCtrlG:
		e.cycleTheme()
	case tcell.KeyCtrlE:
		e.recent = loadRecent()
		e.recentIndex = 0
		e.showRecent = true
	case tcell.KeyCtrlZ:
		e.undoEdit()
	case tcell.KeyCtrlY:
		e.redoEdit()
	case tcell.KeyCtrlF:
		e.prompt = "Find: "
		e.promptValue = e.search
	case tcell.KeyCtrlN:
		e.findNext()
	case tcell.KeyCtrlP:
		e.findPrevious()
	case tcell.KeyCtrlR:
		if e.search == "" {
			e.prompt = "Find: "
			e.promptValue = ""
		} else {
			e.prompt = "Replace with: "
			e.promptValue = e.replace
		}
	case tcell.KeyCtrlC:
		e.copySelection()
	case tcell.KeyCtrlX:
		e.cutSelection()
	case tcell.KeyCtrlV:
		if data, err := clipboardRead(); err == nil {
			e.insertText(string(data))
		} else {
			e.screen.GetClipboard()
		}
	case tcell.KeyCtrlT:
		e.checkpoint()
		e.insertTable()
	case tcell.KeyF1:
		e.showHelp = !e.showHelp
	case tcell.KeyCtrlO:
		e.openLink()
	case tcell.KeyCtrlSpace:
		e.toggleCheckbox()
	case tcell.KeyUp:
		e.beginKeyboardSelection(ev.Modifiers())
		e.moveVertical(-1)
	case tcell.KeyDown:
		e.beginKeyboardSelection(ev.Modifiers())
		e.moveVertical(1)
	case tcell.KeyLeft:
		e.beginKeyboardSelection(ev.Modifiers())
		if e.x > 0 {
			e.x--
		} else if e.y > 0 {
			e.y--
			e.x = runeLen(e.lines[e.y])
		}
	case tcell.KeyRight:
		e.beginKeyboardSelection(ev.Modifiers())
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
		e.checkpoint()
		e.backspace()
	case tcell.KeyDelete:
		e.checkpoint()
		e.delete()
	case tcell.KeyEnter:
		e.checkpoint()
		e.enter()
	case tcell.KeyTab:
		e.checkpoint()
		if !e.nextTableCell() {
			e.insert("    ")
		}
	case tcell.KeyRune:
		e.checkpoint()
		e.insert(string(ev.Rune()))
	}
	e.confirmQuit = false
	e.preferredX = e.x
	if e.dirty {
		e.lastEdit = time.Now()
	}
	return false
}

func (e *editor) recentKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlE:
		e.showRecent = false
	case tcell.KeyUp:
		if e.recentIndex > 0 {
			e.recentIndex--
		}
	case tcell.KeyDown:
		if e.recentIndex+1 < len(e.recent) {
			e.recentIndex++
		}
	case tcell.KeyEnter:
		if len(e.recent) == 0 {
			e.showRecent = false
			return
		}
		e.openFile(e.recent[e.recentIndex])
	}
}

func (e *editor) openFile(path string) {
	if e.dirty {
		e.save()
		if e.dirty {
			e.status = "Could not switch: current file was not saved"
			e.showRecent = false
			return
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		e.status = "Could not open: " + err.Error()
		e.showRecent = false
		return
	}
	e.path = path
	e.lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	e.x, e.y, e.top = 0, 0, 0
	e.undo, e.redo = nil, nil
	e.dirty, e.selecting, e.conflict, e.showRecent = false, false, false, false
	if info, err := os.Stat(path); err == nil {
		e.modTime = info.ModTime()
	}
	e.rememberRecent(path)
	e.status = "Opened " + path
}

func (e *editor) cycleTheme() {
	next := 0
	for i, name := range themeNames {
		if name == e.themeName {
			next = (i + 1) % len(themeNames)
			break
		}
	}
	e.themeName = themeNames[next]
	e.theme = themeByName(e.themeName)
	path := themeConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err == nil {
		_ = os.WriteFile(path, []byte(e.themeName+"\n"), 0644)
	}
	e.status = "Theme: " + e.themeName
}

func (e *editor) promptKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEsc:
		e.prompt, e.promptValue = "", ""
		e.status = "Cancelled"
	case tcell.KeyEnter:
		if e.prompt == "Find: " {
			e.search = e.promptValue
			e.prompt, e.promptValue = "", ""
			e.findNext()
			return
		}
		if e.prompt == "Replace with: " {
			e.replace = e.promptValue
			e.prompt, e.promptValue = "", ""
			e.replaceCurrent()
			return
		}
		path := strings.TrimSpace(e.promptValue)
		if path == "" {
			e.status = "Enter a filename"
			return
		}
		if filepath.Ext(path) == "" {
			path += ".md"
		}
		e.path = path
		e.conflict = false
		e.modTime = time.Time{}
		e.prompt, e.promptValue = "", ""
		e.save()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		r := []rune(e.promptValue)
		if len(r) > 0 {
			e.promptValue = string(r[:len(r)-1])
		}
		if e.prompt == "Find: " {
			e.search = e.promptValue
		}
	case tcell.KeyCtrlA:
		if e.prompt == "Replace with: " {
			e.replace = e.promptValue
			e.prompt, e.promptValue = "", ""
			e.replaceAll()
		}
	case tcell.KeyRune:
		e.promptValue += string(ev.Rune())
		if e.prompt == "Find: " {
			e.search = e.promptValue
			e.findFromStart()
		}
	}
}

func (e *editor) checkpoint() {
	e.undo = append(e.undo, snapshot{append([]string(nil), e.lines...), e.x, e.y})
	if len(e.undo) > 200 {
		e.undo = e.undo[1:]
	}
	e.redo = nil
}

func (e *editor) restore(s snapshot) {
	e.lines, e.x, e.y = append([]string(nil), s.lines...), s.x, s.y
	e.dirty = true
	e.selecting = false
}

func (e *editor) undoEdit() {
	if len(e.undo) == 0 {
		e.status = "Nothing to undo"
		return
	}
	e.redo = append(e.redo, snapshot{append([]string(nil), e.lines...), e.x, e.y})
	s := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.restore(s)
	e.status = "Undo"
}

func (e *editor) redoEdit() {
	if len(e.redo) == 0 {
		e.status = "Nothing to redo"
		return
	}
	e.undo = append(e.undo, snapshot{append([]string(nil), e.lines...), e.x, e.y})
	s := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	e.restore(s)
	e.status = "Redo"
}

func (e *editor) findNext() {
	if e.search == "" {
		return
	}
	for offset := 0; offset < len(e.lines); offset++ {
		y := (e.y + offset) % len(e.lines)
		start := 0
		if offset == 0 {
			start = min(e.x+1, len([]rune(e.lines[y])))
		}
		if index := strings.Index(string([]rune(e.lines[y])[start:]), e.search); index >= 0 {
			e.y, e.x = y, start+runeLen(string([]rune(e.lines[y])[start:start+index]))
			e.status = "Found: " + e.search
			return
		}
	}
	e.status = "Not found: " + e.search
}

func (e *editor) findPrevious() {
	if e.search == "" {
		return
	}
	for offset := 0; offset < len(e.lines); offset++ {
		y := (e.y - offset + len(e.lines)) % len(e.lines)
		limit := runeLen(e.lines[y])
		if offset == 0 {
			limit = e.x
		}
		runes := []rune(e.lines[y])
		index := strings.LastIndex(string(runes[:limit]), e.search)
		if index >= 0 {
			e.y, e.x = y, runeLen(string(runes[:index]))
			e.status = "Found previous: " + e.search
			return
		}
	}
	e.status = "Not found: " + e.search
}

func (e *editor) findFromStart() {
	if e.search == "" {
		return
	}
	for y, line := range e.lines {
		if index := strings.Index(line, e.search); index >= 0 {
			e.y, e.x = y, runeLen(line[:index])
			return
		}
	}
}

func (e *editor) currentMatch() (int, int, bool) {
	if e.search == "" {
		return 0, 0, false
	}
	runes := []rune(e.lines[e.y])
	end := e.x + runeLen(e.search)
	return e.x, end, end <= len(runes) && string(runes[e.x:end]) == e.search
}

func (e *editor) replaceCurrent() {
	start, end, ok := e.currentMatch()
	if !ok {
		e.findNext()
		start, end, ok = e.currentMatch()
	}
	if !ok {
		return
	}
	e.checkpoint()
	runes := []rune(e.lines[e.y])
	e.lines[e.y] = string(runes[:start]) + e.replace + string(runes[end:])
	e.x = start + runeLen(e.replace)
	e.dirty = true
	e.status = "Replaced current match"
}

func (e *editor) replaceAll() {
	if e.search == "" {
		return
	}
	count := 0
	e.checkpoint()
	for y, line := range e.lines {
		count += strings.Count(line, e.search)
		e.lines[y] = strings.ReplaceAll(line, e.search, e.replace)
	}
	if count == 0 {
		e.undo = e.undo[:len(e.undo)-1]
		e.status = "No matches to replace"
		return
	}
	e.dirty = true
	e.status = fmt.Sprintf("Replaced %d match(es)", count)
}

func (e *editor) beginKeyboardSelection(mod tcell.ModMask) {
	if mod&tcell.ModShift != 0 {
		if !e.selecting {
			e.selX, e.selY, e.selecting = e.x, e.y, true
		}
	} else {
		e.selecting = false
	}
}

func (e *editor) selectionText() string {
	if !e.selecting || (e.selX == e.x && e.selY == e.y) {
		return ""
	}
	ax, ay, bx, by := e.selX, e.selY, e.x, e.y
	if ay > by || (ay == by && ax > bx) {
		ax, ay, bx, by = bx, by, ax, ay
	}
	if ay == by {
		return string([]rune(e.lines[ay])[ax:bx])
	}
	parts := []string{string([]rune(e.lines[ay])[ax:])}
	parts = append(parts, e.lines[ay+1:by]...)
	parts = append(parts, string([]rune(e.lines[by])[:bx]))
	return strings.Join(parts, "\n")
}

func (e *editor) copySelection() {
	text := e.selectionText()
	if text == "" {
		e.status = "Nothing selected"
		return
	}
	_ = clipboardWrite(text)
	e.screen.SetClipboard([]byte(text))
	e.status = "Copied selection"
}

func clipboardWrite(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func clipboardRead() ([]byte, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pbpaste").Output()
	case "windows":
		return exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
	default:
		if _, err := exec.LookPath("wl-paste"); err == nil {
			return exec.Command("wl-paste", "--no-newline").Output()
		}
		return exec.Command("xclip", "-selection", "clipboard", "-o").Output()
	}
}

func (e *editor) cutSelection() {
	if e.selectionText() == "" {
		return
	}
	e.copySelection()
	e.checkpoint()
	e.deleteSelection()
}

func (e *editor) deleteSelection() {
	ax, ay, bx, by := e.selX, e.selY, e.x, e.y
	if ay > by || (ay == by && ax > bx) {
		ax, ay, bx, by = bx, by, ax, ay
	}
	left, right := []rune(e.lines[ay])[:ax], []rune(e.lines[by])[bx:]
	e.lines[ay] = string(left) + string(right)
	e.lines = append(e.lines[:ay+1], e.lines[by+1:]...)
	e.x, e.y, e.selecting, e.dirty = ax, ay, false, true
}

func (e *editor) insertText(text string) {
	e.checkpoint()
	if e.selecting {
		e.deleteSelection()
	}
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, part := range parts {
		if i > 0 {
			e.enter()
		}
		e.insert(part)
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
		_, end := e.tableBounds(e.y)
		if e.y == end && tableRowEmpty(e.lines[e.y]) {
			e.lines = insertLine(e.lines, e.y+1, "")
			e.y++
			e.x = 0
			e.dirty = true
			e.status = "Left table"
			return
		}
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
	prefix := listPrefix(left)
	e.lines[e.y] = left
	e.lines = insertLine(e.lines, e.y+1, prefix+right)
	e.y++
	e.x = runeLen(prefix)
	e.dirty = true
}

func tableRowEmpty(line string) bool {
	if isSeparator(line) {
		return false
	}
	for _, cell := range splitTable(line) {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func listPrefix(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	trimmed := strings.TrimSpace(line)
	for _, marker := range []string{"- [ ] ", "- [x] ", "- [X] ", "- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, marker) {
			return indent + marker
		}
	}
	digits := 0
	for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
		digits++
	}
	if digits > 0 && strings.HasPrefix(trimmed[digits:], ". ") {
		n, _ := strconv.Atoi(trimmed[:digits])
		return indent + strconv.Itoa(n+1) + ". "
	}
	return ""
}

func (e *editor) toggleCheckbox() {
	line := e.lines[e.y]
	for _, pair := range [][2]string{{"- [ ]", "- [x]"}, {"- [x]", "- [ ]"}, {"- [X]", "- [ ]"}} {
		if index := strings.Index(line, pair[0]); index >= 0 {
			e.checkpoint()
			e.lines[e.y] = line[:index] + pair[1] + line[index+len(pair[0]):]
			e.dirty = true
			e.status = "Toggled checkbox"
			return
		}
	}
	e.status = "No checkbox on this line"
}

func (e *editor) openLink() {
	line := e.lines[e.y]
	url := ""
	for _, word := range strings.Fields(line) {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			url = strings.TrimRight(word, ".,;)")
			break
		}
	}
	if url == "" {
		if open := strings.Index(line, "]("); open >= 0 {
			if close := strings.Index(line[open+2:], ")"); close >= 0 {
				url = line[open+2 : open+2+close]
			}
		}
	}
	if url == "" {
		e.status = "No link on this line"
		return
	}
	command, args := "xdg-open", []string{url}
	if runtime.GOOS == "darwin" {
		command = "open"
	} else if runtime.GOOS == "windows" {
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	}
	if err := exec.Command(command, args...).Start(); err != nil {
		e.status = "Could not open link: " + err.Error()
		return
	}
	e.status = "Opened " + url
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
	if e.externalChange() {
		e.conflict = true
		e.status = "File changed outside Marko. Use Save As or reopen it."
		return
	}
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
	if info, err := os.Stat(e.path); err == nil {
		e.modTime = info.ModTime()
	}
	e.conflict = false
	e.status = "Saved " + e.path
}

func (e *editor) externalChange() bool {
	if e.modTime.IsZero() {
		return false
	}
	info, err := os.Stat(e.path)
	return err == nil && !info.ModTime().Equal(e.modTime)
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
	statusRows := 1
	if e.focusMode {
		statusRows = 0
	}
	bodyH := max(1, h-statusRows)
	rows := e.visualRows(max(1, w))
	cursorRow := e.cursorVisualRow(rows)
	if cursorRow < e.top {
		e.top = cursorRow
	}
	if cursorRow >= e.top+bodyH {
		e.top = cursorRow - bodyH + 1
	}
	for row := 0; row < bodyH && e.top+row < len(rows); row++ {
		vr := rows[e.top+row]
		e.drawVisualLine(row, vr, vr.y == e.y, w)
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
	if !e.focusMode {
		e.put(0, h-1, status, tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG), w)
	}
	if e.showHelp {
		e.drawHelp(w, h)
	}
	if e.showRecent {
		e.drawRecent(w, h)
	}
	if e.prompt != "" {
		e.screen.ShowCursor(1+runeLen(e.prompt)+runeLen(e.promptValue), h-1)
	} else {
		vr := rows[cursorRow]
		e.screen.ShowCursor(e.x-vr.start, cursorRow-e.top)
	}
	e.screen.Show()
}

func (e *editor) drawHelp(w, h int) {
	lines := []string{
		" Marko help ",
		"F1 close   Ctrl-S save   Ctrl-Shift-S save as   Ctrl-Q quit",
		"Ctrl-F find   Ctrl-N/P next/previous   Ctrl-R replace",
		"Ctrl-Z/Y undo/redo   Ctrl-C/X/V clipboard",
		"Ctrl-T table   Ctrl-Space checkbox   Ctrl-O open link",
		"Ctrl-E recent files   Ctrl-G theme   Shift-arrows or mouse drag select",
	}
	width := 0
	for _, line := range lines {
		width = max(width, runeLen(line))
	}
	x, y := max(0, (w-width-4)/2), max(0, (h-len(lines)-2)/2)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < len(lines)+2 && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, width+4)), box, w)
	}
	for row, line := range lines {
		e.put(x+2, y+1+row, line, box, w)
	}
}

func (e *editor) drawRecent(w, h int) {
	lines := []string{" Recent Markdown files "}
	for i, path := range e.recent {
		prefix := "  "
		if i == e.recentIndex {
			prefix = "> "
		}
		lines = append(lines, prefix+path)
	}
	if len(e.recent) == 0 {
		lines = append(lines, "  No recent files")
	}
	lines = append(lines, " Up/Down select   Enter open   Esc cancel ")
	width := 0
	for _, line := range lines {
		width = max(width, min(runeLen(line), max(20, w-6)))
	}
	x, y := max(0, (w-width-4)/2), max(0, (h-len(lines)-2)/2)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < len(lines)+2 && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, width+4)), box, w)
	}
	for row, line := range lines {
		e.put(x+2, y+1+row, line, box, min(w, x+2+width))
	}
}

func (e *editor) visualRows(width int) []visualRow {
	rows := []visualRow{}
	for y, line := range e.lines {
		runes := []rune(line)
		if len(runes) == 0 {
			rows = append(rows, visualRow{y: y})
			continue
		}
		for start := 0; start < len(runes); {
			end := min(len(runes), start+width)
			if end < len(runes) {
				for candidate := end; candidate > start+width/2; candidate-- {
					if runes[candidate-1] == ' ' {
						end = candidate
						break
					}
				}
			}
			rows = append(rows, visualRow{y: y, start: start, text: string(runes[start:end])})
			start = end
		}
	}
	return rows
}

func (e *editor) cursorVisualRow(rows []visualRow) int {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].y == e.y && rows[i].start <= e.x {
			return i
		}
	}
	return 0
}

func (e *editor) drawVisualLine(row int, vr visualRow, current bool, width int) {
	if vr.start == 0 && runeLen(vr.text) == runeLen(e.lines[vr.y]) {
		e.drawLine(row, vr.y, vr.text, current, width)
		return
	}
	style := tcell.StyleDefault.Foreground(e.theme.text)
	if e.focusMode && !current {
		style = style.Foreground(tcell.ColorGray)
	}
	e.putSelected(row, vr.text, style, width, vr.y, vr.start)
}

func (e *editor) drawLine(row, y int, line string, current bool, width int) {
	style := tcell.StyleDefault.Foreground(e.theme.text)
	if e.focusMode && !current {
		style = style.Foreground(tcell.ColorGray)
	}
	trimmed := strings.TrimSpace(line)
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
		e.putSelected(row, line, style, width, y, 0)
	} else {
		e.putInline(0, row, line, style, width)
	}
}

func (e *editor) putSelected(row int, text string, style tcell.Style, width, y, start int) {
	for x, r := range []rune(text) {
		if x >= width {
			break
		}
		s := style
		if e.positionSelected(start+x, y) {
			s = s.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
		} else if e.positionMatchesSearch(start+x, y) {
			s = s.Background(tcell.ColorDarkGoldenrod).Foreground(tcell.ColorWhite)
		}
		e.screen.SetContent(x, row, r, nil, s)
	}
}

func (e *editor) positionMatchesSearch(x, y int) bool {
	if e.search == "" {
		return false
	}
	line := []rune(e.lines[y])
	needle := []rune(e.search)
	for start := 0; start+len(needle) <= len(line); start++ {
		if runesEqualAt(line, start, needle) && x >= start && x < start+len(needle) {
			return true
		}
	}
	return false
}

func (e *editor) positionSelected(x, y int) bool {
	if !e.selecting {
		return false
	}
	ax, ay, bx, by := e.selX, e.selY, e.x, e.y
	if ay > by || (ay == by && ax > bx) {
		ax, ay, bx, by = bx, by, ax, ay
	}
	return (y > ay || (y == ay && x >= ax)) && (y < by || (y == by && x < bx))
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

func (e *editor) putInline(x, screenY int, text string, base tcell.Style, maxWidth int) {
	runes := []rune(text)
	for i := 0; i < len(runes) && x < maxWidth; {
		marker, styled, end := emphasisAt(runes, i)
		if marker > 0 {
			for _, r := range runes[i+marker : end] {
				if x >= maxWidth {
					break
				}
				e.screen.SetContent(x, screenY, r, nil, styled(base))
				x++
			}
			i = end + marker
			continue
		}
		e.screen.SetContent(x, screenY, runes[i], nil, base)
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
