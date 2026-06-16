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
	screen       tcell.Screen
	path         string
	lines        []string
	x, y, top    int
	dirty        bool
	status       string
	confirmQuit  bool
	preferredX   int
	prompt       string
	promptValue  string
	lastEdit     time.Time
	recovery     string
	theme        theme
	themeName    string
	undo, redo   []snapshot
	selecting    bool
	selX, selY   int
	mouseDown    bool
	lastClick    time.Time
	clickX       int
	clickY       int
	clickCount   int
	search       string
	replace      string
	lastAction   time.Time
	focusMode    bool
	manualScroll bool
	modTime      time.Time
	conflict     bool
	showHelp        bool
	showRecent      bool
	recent          []string
	recentIndex     int
	renameFrom      string
	waitingForPaste bool
}

type snapshot struct {
	lines []string
	x, y  int
}

type visualRow struct {
	y, start int
	text     string
}

type zopaChart struct {
	claimantTarget, claimantMinimum, respondentMaximum, respondentOffer int
}

type theme struct {
	text, heading1, heading2, heading3, table, quote, background, statusBG, statusFG tcell.Color
}

var themeNames = []string{"calm", "green", "mono", "light"}

const writingWidth = 88

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
		return theme{tcell.ColorPaleGreen, tcell.ColorLightGreen, tcell.ColorGreen, tcell.ColorDarkSeaGreen, tcell.ColorPaleGreen, tcell.ColorGray, tcell.ColorDefault, tcell.ColorDarkGreen, tcell.ColorWhite}
	case "mono":
		return theme{tcell.ColorSilver, tcell.ColorWhite, tcell.ColorWhite, tcell.ColorSilver, tcell.ColorSilver, tcell.ColorGray, tcell.ColorDefault, tcell.ColorGray, tcell.ColorBlack}
	case "light":
		return theme{tcell.ColorBlack, tcell.ColorDarkBlue, tcell.ColorDarkGreen, tcell.ColorDarkGoldenrod, tcell.ColorDarkGreen, tcell.ColorDarkSlateGray, tcell.ColorWhite, tcell.ColorLightGray, tcell.ColorBlack}
	default:
		return theme{tcell.ColorSilver, tcell.ColorLightSkyBlue, tcell.ColorLightGreen, tcell.ColorLightGoldenrodYellow, tcell.ColorPaleGreen, tcell.ColorGray, tcell.ColorDefault, tcell.ColorDarkSlateGray, tcell.ColorWhite}
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
			if e.externalChange() {
				if !e.dirty {
					e.reloadFile()
				} else {
					e.conflict = true
					e.status = "File changed outside Marko. Use Save As or reopen it."
				}
			}
		case *tcell.EventKey:
			e.lastAction, e.focusMode, e.manualScroll = time.Now(), false, false
			if e.key(ev) {
				return
			}
		case *tcell.EventMouse:
			e.lastAction, e.focusMode = time.Now(), false
			e.mouse(ev)
		case *tcell.EventClipboard:
			if e.waitingForPaste {
				e.insertText(string(ev.Data()))
				e.waitingForPaste = false
			}
		}
	}
}

func (e *editor) mouse(ev *tcell.EventMouse) {
	buttons := ev.Buttons()
	if buttons&(tcell.WheelUp|tcell.WheelDown|tcell.Button4|tcell.Button5) != 0 {
		w, h := e.screen.Size()
		bodyH := max(1, h-1)
		if e.focusMode {
			bodyH = h
		}
		_, contentWidth := writingArea(w)
		rows := e.visualRows(contentWidth)
		switch {
		case buttons&(tcell.WheelUp|tcell.Button4) != 0:
			e.scrollBy(-1, bodyH, len(rows))
		case buttons&(tcell.WheelDown|tcell.Button5) != 0:
			e.scrollBy(1, bodyH, len(rows))
		}
		return
	}
	if buttons&tcell.Button1 != 0 {
		e.manualScroll = false
	}
	x, sy := ev.Position()
	w, h := e.screen.Size()
	statusRows := 1
	if e.focusMode {
		statusRows = 0
	}
	bodyH := max(1, h-statusRows)

	left, contentWidth := writingArea(w)
	rows := e.visualRows(contentWidth)

	targetSy := sy
	if targetSy < 0 {
		targetSy = 0
		if buttons&tcell.Button1 != 0 {
			e.top = max(0, e.top-1)
			e.manualScroll = true
		}
	} else if targetSy >= bodyH {
		targetSy = bodyH - 1
		if buttons&tcell.Button1 != 0 {
			maxTop := manualScrollMaxTop(len(rows))
			e.top = min(e.top+1, maxTop)
			e.manualScroll = true
		}
	}

	index := min(len(rows)-1, max(0, e.top+targetSy))
	y := rows[index].y
	x = min(runeLen(e.lines[y]), max(0, rows[index].start+x-left))
	if buttons&tcell.Button1 != 0 {
		if time.Since(e.lastClick) < 400*time.Millisecond && e.clickX == x && e.clickY == y {
			e.clickCount++
		} else {
			e.clickCount = 1
		}
		e.lastClick, e.clickX, e.clickY = time.Now(), x, y
		if e.clickCount >= 3 {
			e.selectLineAt(y)
			e.mouseDown = false
			e.clickCount = 0
			return
		}
		if e.clickCount == 2 {
			e.selectWordAt(x, y)
			e.mouseDown = false
			return
		}
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

func (e *editor) scrollBy(delta, bodyH, rowCount int) {
	maxTop := manualScrollMaxTop(rowCount)
	e.top = min(max(e.top+delta, 0), maxTop)
	e.manualScroll = true
}

func manualScrollMaxTop(rowCount int) int {
	return max(0, rowCount-1)
}

func (e *editor) pageScroll(delta int) {
	w, h := 80, 1
	if e.screen != nil {
		w, h = e.screen.Size()
	}
	bodyH := max(1, h-1)
	if e.focusMode {
		bodyH = h
	}
	_, contentWidth := writingArea(w)
	rows := e.visualRows(contentWidth)
	step := max(1, bodyH-1)
	e.scrollBy(delta*step, bodyH, len(rows))
}

func (e *editor) movePageVisual(delta int) {
	w, h := 80, 1
	if e.screen != nil {
		w, h = e.screen.Size()
	}
	bodyH := max(1, h-1)
	if e.focusMode {
		bodyH = h
	}
	_, contentWidth := writingArea(w)
	rows := e.visualRows(contentWidth)
	if len(rows) == 0 {
		return
	}
	current := e.cursorVisualRow(rows)
	target := max(0, min(len(rows)-1, current+delta*max(1, bodyH-1)))
	column := e.x - rows[current].start
	if column < 0 {
		column = 0
	}
	vr := rows[target]
	e.y = vr.y
	e.x = min(runeLen(e.lines[e.y]), vr.start+column)
	e.clampX()
}

func (e *editor) moveLineEdge(end bool) {
	w := 80
	if e.screen != nil {
		w, _ = e.screen.Size()
	}
	_, contentWidth := writingArea(w)
	rows := e.visualRows(contentWidth)
	if len(rows) == 0 {
		return
	}
	current := e.cursorVisualRow(rows)
	vr := rows[current]
	if end {
		next := vr.start + contentWidth
		if next > runeLen(e.lines[e.y]) {
			next = runeLen(e.lines[e.y])
		}
		e.x = next
	} else {
		e.x = vr.start
	}
	e.clampX()
}

func (e *editor) selectLineAt(y int) {
	e.selX, e.selY = 0, y
	e.x, e.y = runeLen(e.lines[y]), y
	e.selecting = true
	e.status = "Selected line"
}

func (e *editor) selectWordAt(x, y int) {
	runes := []rune(e.lines[y])
	if len(runes) == 0 {
		return
	}
	x = min(x, len(runes)-1)
	isWord := func(r rune) bool {
		return r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r > 127
	}
	if !isWord(runes[x]) {
		return
	}
	start, end := x, x+1
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	for end < len(runes) && isWord(runes[end]) {
		end++
	}
	e.selX, e.selY, e.x, e.y, e.selecting = start, y, end, y, true
	e.status = "Selected word"
}

func writingArea(screenWidth int) (int, int) {
	contentWidth := min(writingWidth, max(1, screenWidth-4))
	return max(0, (screenWidth-contentWidth)/2), contentWidth
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
	case tcell.KeyCtrlL:
		e.reloadFile()
	case tcell.KeyCtrlD:
		e.checkpoint()
		e.deleteLine()
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
		if ev.Modifiers()&tcell.ModShift != 0 {
			e.prompt = "Rename to: "
			e.promptValue = e.path
			e.renameFrom = e.path
		} else if e.search == "" {
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
			e.waitingForPaste = true
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
		e.moveVisualVertical(-1)
	case tcell.KeyDown:
		e.beginKeyboardSelection(ev.Modifiers())
		e.moveVisualVertical(1)
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
		e.moveLineEdge(false)
	case tcell.KeyEnd:
		e.moveLineEdge(true)
	case tcell.KeyPgUp:
		e.movePageVisual(-1)
	case tcell.KeyPgDn:
		e.movePageVisual(1)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.checkpoint()
		if e.selecting {
			e.deleteSelection()
		} else {
			e.backspace()
		}
	case tcell.KeyDelete:
		e.checkpoint()
		if e.selecting {
			e.deleteSelection()
		} else {
			e.delete()
		}
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
		if e.selecting {
			e.deleteSelection()
		}
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
		if e.prompt == "Rename to: " {
			e.renameFile(e.promptValue)
			e.prompt, e.promptValue = "", ""
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
	if e.selecting && isURL(strings.TrimSpace(text)) && !strings.Contains(e.selectionText(), "\n") {
		label := e.selectionText()
		e.deleteSelection()
		e.insert("[" + label + "](" + strings.TrimSpace(text) + ")")
		e.status = "Created link"
		return
	}
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

func isURL(text string) bool {
	return strings.HasPrefix(text, "https://") || strings.HasPrefix(text, "http://")
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
	if e.expandChartFence() {
		return
	}
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

func (e *editor) expandChartFence() bool {
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(e.lines[e.y]), "```"))
	template, ok := chartTemplates[name]
	if !ok || strings.TrimSpace(e.lines[e.y]) != "```"+name {
		return false
	}
	lines := append([]string(nil), template...)
	e.lines = append(e.lines[:e.y+1], append(lines, e.lines[e.y+1:]...)...)
	e.y++
	if name == "zopa" {
		e.x = runeLen("Claimant target: ")
	} else if name == "chart" {
		e.x = runeLen("Option A: ")
	}
	e.dirty = true
	e.status = "Chart created. Edit the example values, then press Tab."
	return true
}

var chartTemplates = map[string][]string{
	"zopa": {
		"Claimant target: 100000",
		"Claimant minimum: 80000",
		"Respondent maximum: 95000",
		"Respondent offer: 70000",
		"```",
	},
	"chart": {
		"Option A: 80",
		"Option B: 120",
		"Option C: 45",
		"```",
	},
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
	if e.nextChartValue() {
		return true
	}
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

func (e *editor) nextChartValue() bool {
	start, end, ok := e.chartFenceBounds(e.y)
	if !ok {
		return false
	}
	next := e.y + 1
	if e.y == start || next >= end {
		next = start + 1
	}
	e.y = next
	if colon := strings.Index(e.lines[e.y], ":"); colon >= 0 {
		e.x = colon + 2
	} else {
		e.x = runeLen(e.lines[e.y])
	}
	return true
}

func (e *editor) chartFenceBounds(y int) (int, int, bool) {
	start := y
	for start >= 0 && !strings.HasPrefix(strings.TrimSpace(e.lines[start]), "```") {
		start--
	}
	if start < 0 {
		return 0, 0, false
	}
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(e.lines[start]), "```"))
	if _, ok := chartTemplates[name]; !ok {
		return 0, 0, false
	}
	for end := start + 1; end < len(e.lines); end++ {
		if strings.TrimSpace(e.lines[end]) == "```" {
			return start, end, true
		}
	}
	return 0, 0, false
}

func (e *editor) inTable(y int) bool {
	return y >= 0 && y < len(e.lines) && len(splitTable(e.lines[y])) >= 2
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

func (e *editor) moveVisualVertical(delta int) {
	w := 80
	if e.screen != nil {
		w, _ = e.screen.Size()
	}
	_, contentWidth := writingArea(w)
	rows := e.visualRows(contentWidth)
	if len(rows) == 0 {
		return
	}
	current := e.cursorVisualRow(rows)
	target := max(0, min(len(rows)-1, current+delta))
	if target == current {
		return
	}
	column := e.x - rows[current].start
	if column < 0 {
		column = 0
	}
	vr := rows[target]
	e.y = vr.y
	e.x = min(runeLen(e.lines[e.y]), vr.start+column)
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
	_ = os.Remove(journalPath(e.path))
}

func (e *editor) reloadFile() {
	data, err := os.ReadFile(e.path)
	if err != nil {
		e.status = "Reload failed: " + err.Error()
		return
	}
	e.checkpoint()
	e.lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	e.y = min(e.y, len(e.lines)-1)
	e.clampX()
	e.dirty, e.conflict = false, false
	if info, err := os.Stat(e.path); err == nil {
		e.modTime = info.ModTime()
	}
	e.status = "Reloaded " + e.path
}

func (e *editor) deleteLine() {
	if len(e.lines) == 1 {
		e.lines[0], e.x, e.y = "", 0, 0
	} else {
		e.lines = append(e.lines[:e.y], e.lines[e.y+1:]...)
		e.y = min(e.y, len(e.lines)-1)
		e.clampX()
	}
	e.dirty = true
	e.status = "Deleted line"
}

func (e *editor) renameFile(path string) {
	path = strings.TrimSpace(path)
	if filepath.Ext(path) == "" {
		path += ".md"
	}
	if err := os.Rename(e.renameFrom, path); err != nil {
		e.status = "Rename failed: " + err.Error()
		return
	}
	e.path, e.renameFrom = path, ""
	if info, err := os.Stat(e.path); err == nil {
		e.modTime = info.ModTime()
	}
	e.rememberRecent(path)
	e.status = "Renamed to " + path
}

func journalPath(path string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(path)
	return filepath.Join(dir, "marko", "recovery", name+".journal")
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
		journal := journalPath(e.path)
		if os.MkdirAll(filepath.Dir(journal), 0700) == nil {
			_ = os.WriteFile(journal, []byte(strings.Join(e.lines, "\n")), 0600)
		}
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
	left, contentWidth := writingArea(w)
	background := tcell.StyleDefault.Background(e.theme.background)
	for row := 0; row < bodyH; row++ {
		e.put(left, row, strings.Repeat(" ", contentWidth), background, w)
	}
	rows := e.visualRows(contentWidth)
	cursorRow := e.cursorVisualRow(rows)
	if !e.manualScroll {
		if cursorRow < e.top {
			e.top = cursorRow
		}
		if cursorRow >= e.top+bodyH {
			e.top = cursorRow - bodyH + 1
		}
	}
	maxTop := max(0, len(rows)-bodyH)
	if e.manualScroll {
		maxTop = manualScrollMaxTop(len(rows))
	}
	e.top = min(max(e.top, 0), maxTop)
	for row := 0; row < bodyH && e.top+row < len(rows); row++ {
		vr := rows[e.top+row]
		e.drawVisualLine(left, row, vr, e.focusedLine(vr.y), contentWidth)
	}
	name := filepath.Base(e.path)
	if e.path == "" {
		name = "Untitled"
	}
	mark := ""
	if e.dirty {
		mark = " *"
	}
	cwd, _ := os.Getwd()
	status := fmt.Sprintf(" %s%s  Ln %d, Col %d  %s  [%s]  %s", name, mark, e.y+1, e.x+1, cwd, e.stats(), e.status)
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
		if cursorRow >= e.top && cursorRow < e.top+bodyH {
			e.screen.ShowCursor(left+e.x-vr.start, cursorRow-e.top)
		} else {
			e.screen.HideCursor()
		}
	}
	e.screen.Show()
}

func isParagraphBoundary(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	if len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' {
		for idx, r := range trimmed {
			if r == '.' && idx+1 < len(trimmed) && trimmed[idx+1] == ' ' {
				return true
			}
			if r < '0' || r > '9' {
				break
			}
		}
	}
	if strings.HasPrefix(trimmed, "```") {
		return true
	}
	if strings.HasPrefix(trimmed, "|") {
		return true
	}
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	return false
}

func (e *editor) focusedLine(y int) bool {
	if !e.focusMode {
		return y == e.y
	}
	if start, end, ok := e.codeFenceBounds(e.y); ok {
		return y >= start && y <= end
	}
	if e.inTable(e.y) {
		start, end := e.tableBounds(e.y)
		return y >= start && y <= end
	}
	if start, end, ok := e.zopaFenceBounds(e.y); ok {
		return y >= start && y <= end
	}
	if start, end, ok := e.barChartFenceBounds(e.y); ok {
		return y >= start && y <= end
	}

	currentLine := e.lines[e.y]
	if isParagraphBoundary(currentLine) {
		return y == e.y
	}

	start, end := e.y, e.y
	for start > 0 && !isParagraphBoundary(e.lines[start-1]) {
		if _, _, ok := e.codeFenceBounds(start-1); ok {
			break
		}
		if e.inTable(start-1) {
			break
		}
		if _, _, ok := e.zopaFenceBounds(start-1); ok {
			break
		}
		if _, _, ok := e.barChartFenceBounds(start-1); ok {
			break
		}
		start--
	}
	for end+1 < len(e.lines) && !isParagraphBoundary(e.lines[end+1]) {
		if _, _, ok := e.codeFenceBounds(end+1); ok {
			break
		}
		if e.inTable(end+1) {
			break
		}
		if _, _, ok := e.zopaFenceBounds(end+1); ok {
			break
		}
		if _, _, ok := e.barChartFenceBounds(end+1); ok {
			break
		}
		end++
	}
	return y >= start && y <= end
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
	for y := 0; y < len(e.lines); y++ {
		line := e.lines[y]
		if chart, end, ok := e.zopaBlock(y); ok && (e.y < y || e.y > end) {
			for i, text := range renderZOPA(chart, width) {
				rows = append(rows, visualRow{y: y, start: i, text: text})
			}
			y = end
			continue
		}
		if cb, end, ok := e.chartBlock(y); ok && (e.y < y || e.y > end) {
			for i, text := range renderChart(cb, width) {
				rows = append(rows, visualRow{y: y, start: i, text: text})
			}
			y = end
			continue
		}
		if language, end, ok := e.codeFence(y); ok && (e.y < y || e.y > end) {
			rows = append(rows, visualRow{y: y, text: "┌ code · " + language})
			for lineY := y + 1; lineY < end; lineY++ {
				rows = append(rows, visualRow{y: lineY, text: "│ " + e.lines[lineY]})
			}
			rows = append(rows, visualRow{y: end, text: "└"})
			y = end
			continue
		}
		if e.inTable(y) && !e.cursorInSameTable(y) {
			rows = append(rows, visualRow{y: y, text: e.renderTableLine(y, width)})
			continue
		}
		if y != e.y {
			if quote, ok := blockQuote(line); ok {
				line = "│ " + quote
			}
		}
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

func (e *editor) codeFence(start int) (string, int, bool) {
	if start < 0 || start >= len(e.lines) {
		return "", start, false
	}
	open := strings.TrimSpace(e.lines[start])
	if !strings.HasPrefix(open, "```") || open == "```zopa" {
		return "", start, false
	}
	for end := start + 1; end < len(e.lines); end++ {
		if strings.TrimSpace(e.lines[end]) == "```" {
			language := strings.TrimSpace(strings.TrimPrefix(open, "```"))
			if language == "" {
				language = "text"
			}
			return language, end, true
		}
	}
	return "", start, false
}

func (e *editor) zopaBlock(start int) (zopaChart, int, bool) {
	if start < 0 || start >= len(e.lines) || strings.TrimSpace(e.lines[start]) != "```zopa" {
		return zopaChart{}, start, false
	}
	values := map[string]int{}
	for end := start + 1; end < len(e.lines); end++ {
		line := strings.TrimSpace(e.lines[end])
		if line == "```" {
			chart := zopaChart{
				claimantTarget:    values["claimant target"],
				claimantMinimum:   values["claimant minimum"],
				respondentMaximum: values["respondent maximum"],
				respondentOffer:   values["respondent offer"],
			}
			ok := chart.claimantTarget > 0 && chart.claimantMinimum > 0 && chart.respondentMaximum > 0 && chart.respondentOffer > 0
			return chart, end, ok
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		number := strings.NewReplacer("£", "", ",", "", "_", "", " ", "").Replace(value)
		if parsed, err := strconv.Atoi(number); err == nil {
			values[strings.ToLower(strings.TrimSpace(key))] = parsed
		}
	}
	return zopaChart{}, start, false
}

func renderZOPA(chart zopaChart, width int) []string {
	minValue := min(chart.respondentOffer, chart.claimantMinimum)
	maxValue := max(chart.claimantTarget, chart.respondentMaximum)
	barWidth := max(36, min(width-16, 72))
	position := func(value int) int {
		if maxValue == minValue {
			return 0
		}
		return (value - minValue) * (barWidth - 1) / (maxValue - minValue)
	}

	respondentBand := make([]rune, barWidth)
	claimantBand := make([]rune, barWidth)
	axis := make([]rune, barWidth)

	for i := range axis {
		axis[i] = '─'
		respondentBand[i] = ' '
		claimantBand[i] = ' '
	}

	rStart, rEnd := position(chart.respondentOffer), position(chart.respondentMaximum)
	cStart, cEnd := position(chart.claimantMinimum), position(chart.claimantTarget)
	zStart, zEnd := position(chart.claimantMinimum), position(chart.respondentMaximum)

	for x := rStart; x <= rEnd && x < barWidth; x++ {
		respondentBand[x] = '░'
	}
	for x := cStart; x <= cEnd && x < barWidth; x++ {
		claimantBand[x] = '▒'
	}
	if zStart <= zEnd {
		for x := zStart; x <= zEnd && x < barWidth; x++ {
			if x >= rStart && x <= rEnd {
				respondentBand[x] = '▓'
			}
			if x >= cStart && x <= cEnd {
				claimantBand[x] = '▓'
			}
		}
	}

	axisLabels := make([]rune, barWidth)
	for i := range axisLabels {
		axisLabels[i] = ' '
	}

	setMarker := func(value int, marker rune, label string) {
		x := position(value)
		axis[x] = marker
		start := max(0, min(barWidth-len([]rune(label)), x-len([]rune(label))/2))
		for i, r := range []rune(label) {
			axisLabels[start+i] = r
		}
	}

	setMarker(chart.respondentOffer, '●', "R offer")
	setMarker(chart.claimantMinimum, '▲', "C min")
	setMarker(chart.respondentMaximum, '▲', "R max")
	setMarker(chart.claimantTarget, '●', "C tgt")

	overlap := "No ZOPA"
	if chart.claimantMinimum <= chart.respondentMaximum {
		overlap = fmt.Sprintf("ZOPA %s–%s", money(chart.claimantMinimum), money(chart.respondentMaximum))
	}

	return []string{
		"Settlement range · " + overlap,
		"Respondent ░  " + string(respondentBand),
		"Claimant   ▒  " + string(claimantBand),
		"              " + string(axis),
		"              " + string(axisLabels),
	}
}

func money(value int) string {
	return "£" + strconv.Itoa(value/1000) + "k"
}

type chartBlock struct {
	title  string
	items  []chartItem
	maxVal int
}

type chartItem struct {
	label  string
	value  int
	rawVal string
}

func (e *editor) chartBlock(start int) (chartBlock, int, bool) {
	if start < 0 || start >= len(e.lines) || !strings.HasPrefix(strings.TrimSpace(e.lines[start]), "```chart") {
		return chartBlock{}, start, false
	}
	title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(e.lines[start]), "```chart"))
	if title == "" {
		title = "Visual Chart"
	}
	items := []chartItem{}
	maxVal := 0
	for end := start + 1; end < len(e.lines); end++ {
		line := strings.TrimSpace(e.lines[end])
		if line == "```" {
			return chartBlock{title: title, items: items, maxVal: maxVal}, end, len(items) > 0
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		raw := strings.TrimSpace(value)
		number := strings.NewReplacer("£", "", ",", "", "_", "", " ", "").Replace(raw)
		if parsed, err := strconv.Atoi(number); err == nil {
			items = append(items, chartItem{label: strings.TrimSpace(key), value: parsed, rawVal: raw})
			if parsed > maxVal {
				maxVal = parsed
			}
		}
	}
	return chartBlock{}, start, false
}

func (e *editor) barChartFenceBounds(y int) (int, int, bool) {
	for start := y; start >= 0; start-- {
		_, end, ok := e.chartBlock(start)
		if ok && y <= end {
			return start, end, true
		}
	}
	return 0, 0, false
}

func renderChart(cb chartBlock, width int) []string {
	rows := []string{
		"📊 " + cb.title,
	}
	labelWidth := 0
	for _, item := range cb.items {
		if len(item.label) > labelWidth {
			labelWidth = len(item.label)
		}
	}
	if labelWidth > 24 {
		labelWidth = 24
	}
	barMaxWidth := max(16, min(width-labelWidth-16, 48))
	for _, item := range cb.items {
		lbl := item.label
		if len(lbl) > labelWidth {
			lbl = lbl[:labelWidth-3] + "..."
		}
		lbl = lbl + strings.Repeat(" ", labelWidth-len(lbl))
		barW := 0
		if cb.maxVal > 0 {
			barW = item.value * barMaxWidth / cb.maxVal
		}
		if barW < 1 && item.value > 0 {
			barW = 1
		}
		barStr := strings.Repeat("█", barW) + strings.Repeat("░", barMaxWidth-barW)
		valStr := item.rawVal
		if item.value >= 1000 && item.value%1000 == 0 {
			valStr = "£" + strconv.Itoa(item.value/1000) + "k"
		}
		rows = append(rows, fmt.Sprintf("%s  %s  %s", lbl, barStr, valStr))
	}
	return rows
}

func isRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	first := trimmed[0]
	if first != '-' && first != '*' && first != '_' {
		return false
	}
	for _, r := range trimmed {
		if r != rune(first) && r != ' ' {
			return false
		}
	}
	return true
}

func renderRule(width int) string {
	symbol := " ❦ "
	ruleWidth := min(40, width)
	sideWidth := (ruleWidth - len([]rune(symbol))) / 2
	if sideWidth < 1 {
		sideWidth = 1
	}
	side := strings.Repeat("─", sideWidth)
	return side + symbol + side
}

func (e *editor) sectionChecklistProgress(y int, level int) (string, bool) {
	checked, total := 0, 0
	for lineY := y + 1; lineY < len(e.lines); lineY++ {
		if l, _, ok := heading(e.lines[lineY]); ok && l <= level {
			break
		}
		trimmed := strings.TrimSpace(e.lines[lineY])
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "* [ ]") {
			total++
		} else if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "* [x]") || strings.HasPrefix(trimmed, "- [X]") || strings.HasPrefix(trimmed, "* [X]") {
			total++
			checked++
		}
	}
	if total == 0 {
		return "", false
	}
	percent := checked * 100 / total
	barWidth := 6
	filled := percent * barWidth / 100
	bar := make([]rune, barWidth)
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar[i] = '■'
		} else {
			bar[i] = '□'
		}
	}
	return fmt.Sprintf("[%s] %d%% (%d/%d)", string(bar), percent, checked, total), true
}

func (e *editor) stats() string {
	words := 0
	tasksDone, tasksTotal := 0, 0
	for _, line := range e.lines {
		words += len(strings.Fields(line))
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "* [ ]") {
			tasksTotal++
		} else if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "* [x]") || strings.HasPrefix(trimmed, "- [X]") || strings.HasPrefix(trimmed, "* [X]") {
			tasksTotal++
			tasksDone++
		}
	}
	statsStr := fmt.Sprintf("%d words", words)
	if tasksTotal > 0 {
		statsStr += fmt.Sprintf("  %d/%d tasks (%d%%)", tasksDone, tasksTotal, tasksDone*100/tasksTotal)
	}
	return statsStr
}

func (e *editor) cursorVisualRow(rows []visualRow) int {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].y == e.y && rows[i].start <= e.x {
			return i
		}
	}
	return 0
}

func (e *editor) putCodeLine(left, row int, vr visualRow, style tcell.Style, maxWidth int) {
	runes := []rune(vr.text)
	isFence := strings.HasPrefix(vr.text, "┌") || strings.HasPrefix(vr.text, "└")
	for i := 0; i < len(runes); i++ {
		if left+i >= maxWidth {
			break
		}
		s := style
		if !isFence {
			srcX := i - 2
			if srcX < 0 {
				srcX = 0
			}
			if e.positionSelected(srcX, vr.y) {
				s = s.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
			} else if e.positionMatchesSearch(srcX, vr.y) {
				s = s.Background(tcell.ColorDarkGoldenrod).Foreground(tcell.ColorWhite)
			}
		} else {
			if e.positionSelected(0, vr.y) {
				s = s.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
			}
		}
		e.screen.SetContent(left+i, row, runes[i], nil, s)
	}
}

func (e *editor) drawVisualLine(left, row int, vr visualRow, current bool, width int) {
	if _, _, ok := e.codeFenceBounds(vr.y); ok {
		style := tcell.StyleDefault.Foreground(tcell.ColorLightGoldenrodYellow).Background(tcell.ColorDarkSlateGray)
		e.putCodeLine(left, row, vr, style, left+width)
		return
	}
	if start, end, ok := e.zopaFenceBounds(vr.y); ok && (e.y < start || e.y > end) {
		style := tcell.StyleDefault.Background(e.theme.background)
		if e.positionSelected(0, vr.y) {
			style = style.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
		} else {
			switch vr.start {
			case 0:
				style = style.Foreground(e.theme.heading3).Bold(true)
			case 1:
				style = style.Foreground(tcell.ColorLightSeaGreen)
			case 2:
				style = style.Foreground(tcell.ColorLightCoral)
			case 3:
				style = style.Foreground(tcell.ColorLightYellow)
			case 4:
				style = style.Foreground(tcell.ColorGray)
			}
		}
		e.put(left, row, vr.text, style, left+width)
		return
	}
	if start, end, ok := e.barChartFenceBounds(vr.y); ok && (e.y < start || e.y > end) {
		style := tcell.StyleDefault.Background(e.theme.background)
		if e.positionSelected(0, vr.y) {
			style = style.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
		} else {
			if vr.start == 0 {
				style = style.Foreground(e.theme.heading3).Bold(true)
			} else {
				style = style.Foreground(e.theme.text)
			}
		}
		e.put(left, row, vr.text, style, left+width)
		return
	}
	if isRule(vr.text) && !current {
		style := tcell.StyleDefault.Foreground(e.theme.quote)
		if e.positionSelected(0, vr.y) {
			style = style.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
		}
		ruleText := renderRule(width)
		padding := (width - len([]rune(ruleText))) / 2
		if padding < 0 {
			padding = 0
		}
		e.put(left+padding, row, ruleText, style, left+width)
		return
	}
	if e.inTable(vr.y) && !e.cursorInSameTable(vr.y) {
		e.drawLine(left, row, vr.y, e.lines[vr.y], current, width)
		return
	}
	if vr.start == 0 && runeLen(vr.text) == runeLen(e.lines[vr.y]) {
		e.drawLine(left, row, vr.y, vr.text, current, width)
		return
	}
	style := tcell.StyleDefault.Foreground(e.theme.text).Background(e.theme.background)
	if e.focusMode && !current {
		style = style.Dim(true)
	}
	e.putSelected(left, row, vr.text, style, width, vr.y, vr.start)
}

func (e *editor) codeFenceBounds(y int) (int, int, bool) {
	for start := y; start >= 0; start-- {
		_, end, ok := e.codeFence(start)
		if ok && y <= end {
			return start, end, true
		}
	}
	return 0, 0, false
}

func (e *editor) zopaFenceBounds(y int) (int, int, bool) {
	for start := y; start >= 0; start-- {
		_, end, ok := e.zopaBlock(start)
		if ok && y <= end {
			return start, end, true
		}
	}
	return 0, 0, false
}

func (e *editor) drawLine(left, row, y int, line string, current bool, width int) {
	if isRule(line) && !current {
		style := tcell.StyleDefault.Foreground(e.theme.quote)
		ruleText := renderRule(width)
		padding := (width - len([]rune(ruleText))) / 2
		if padding < 0 {
			padding = 0
		}
		e.put(left+padding, row, ruleText, style, left+width)
		return
	}
	style := tcell.StyleDefault.Foreground(e.theme.text).Background(e.theme.background)
	if e.focusMode && !current {
		style = style.Dim(true)
	}
	trimmed := strings.TrimSpace(line)
	if e.inTable(y) && !e.cursorInSameTable(y) {
		e.drawTableLine(left, row, y, width, style.Foreground(e.theme.table))
		return
	}
	if y != e.y {
		if quote, ok := blockQuote(line); ok {
			line = "│ " + quote
			style = style.Foreground(e.theme.quote)
			trimmed = strings.TrimSpace(line)
		}
	}
	if level, text, ok := heading(line); ok && !current {
		line = text
		if progressText, hasChecklist := e.sectionChecklistProgress(y, level); hasChecklist {
			line = line + "  " + progressText
		}
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
		e.putSelected(left, row, line, style, width, y, 0)
	} else {
		e.putInline(left, row, line, style, left+width, y, 0)
	}
}

func (e *editor) putSelected(left, row int, text string, style tcell.Style, width, y, start int) {
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
		e.screen.SetContent(left+x, row, r, nil, s)
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

func blockQuote(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, ">") {
		return line, false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, ">")), true
}

func (e *editor) cursorInSameTable(y int) bool {
	if !e.inTable(e.y) {
		return false
	}
	start, end := e.tableBounds(e.y)
	return y >= start && y <= end
}

func (e *editor) renderTableLine(y int, maxWidths ...int) string {
	widths := e.tableWidths(y)
	if len(maxWidths) > 0 {
		widths = fitTableWidths(widths, maxWidths[0])
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
			value = truncateInlineCell(cells[col], width)
		}
		parts[col] = " " + value + strings.Repeat(" ", width-inlineDisplayWidth(value)+1)
	}
	return "│" + strings.Join(parts, "│") + "│"
}

func (e *editor) drawTableLine(left, row, y, maxWidth int, style tcell.Style) {
	widths := fitTableWidths(e.tableWidths(y), maxWidth)
	if isSeparator(e.lines[y]) {
		e.put(left, row, e.renderTableLine(y, maxWidth), style, left+maxWidth)
		return
	}

	cells := splitTable(e.lines[y])
	x := left
	e.screen.SetContent(x, row, '│', nil, style)
	x++
	for col, width := range widths {
		e.screen.SetContent(x, row, ' ', nil, style)
		x++
		value := ""
		if col < len(cells) {
			value = truncateInlineCell(cells[col], width)
		}
		e.putInline(x, row, value, style, x+width, y, 0)
		x += width
		e.screen.SetContent(x, row, ' ', nil, style)
		x++
		e.screen.SetContent(x, row, '│', nil, style)
		x++
	}
}

func (e *editor) tableWidths(y int) []int {
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
			widths[col] = max(widths[col], inlineDisplayWidth(cell))
		}
	}
	return widths
}

func fitTableWidths(widths []int, maxWidth int) []int {
	fitted := append([]int(nil), widths...)
	available := maxWidth - 3*len(fitted) - 1
	for sumInts(fitted) > available {
		widest := -1
		for i, width := range fitted {
			if width > 1 && (widest == -1 || width > fitted[widest]) {
				widest = i
			}
		}
		if widest == -1 {
			break
		}
		fitted[widest]--
	}
	return fitted
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func truncateCell(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func truncateInlineCell(value string, width int) string {
	if inlineDisplayWidth(value) <= width {
		return value
	}
	return truncateCell(inlinePlainText(value), width)
}

func inlineDisplayWidth(value string) int {
	return runeLen(inlinePlainText(value))
}

func inlinePlainText(value string) string {
	runes := []rune(value)
	var plain []rune
	for i := 0; i < len(runes); {
		marker, _, end := emphasisAt(runes, i)
		if marker > 0 {
			plain = append(plain, runes[i+marker:end]...)
			i = end + marker
			continue
		}
		plain = append(plain, runes[i])
		i++
	}
	return string(plain)
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

func (e *editor) putInline(x, screenY int, text string, base tcell.Style, maxWidth int, y int, start int) {
	runes := []rune(text)
	for i := 0; i < len(runes) && x < maxWidth; {
		s := base
		srcX := start + i
		if e.positionSelected(srcX, y) {
			s = s.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
		} else if e.positionMatchesSearch(srcX, y) {
			s = s.Background(tcell.ColorDarkGoldenrod).Foreground(tcell.ColorWhite)
		}

		if runes[i] == '`' {
			if end := closingRune(runes, i+1, '`'); end > i+1 {
				codeStyle := s.Foreground(tcell.ColorLightGoldenrodYellow).Background(tcell.ColorDarkSlateGray)
				for idx, r := range runes[i+1 : end] {
					if x >= maxWidth {
						break
					}
					rStyle := codeStyle
					rSrcX := start + i + 1 + idx
					if e.positionSelected(rSrcX, y) {
						rStyle = rStyle.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
					} else if e.positionMatchesSearch(rSrcX, y) {
						rStyle = rStyle.Background(tcell.ColorDarkGoldenrod).Foreground(tcell.ColorWhite)
					}
					e.screen.SetContent(x, screenY, r, nil, rStyle)
					x++
				}
				i = end + 1
				continue
			}
		}
		marker, styled, end := emphasisAt(runes, i)
		if marker > 0 {
			for idx, r := range runes[i+marker : end] {
				if x >= maxWidth {
					break
				}
				rStyle := styled(s)
				rSrcX := start + i + marker + idx
				if e.positionSelected(rSrcX, y) {
					rStyle = rStyle.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite).Bold(true)
				} else if e.positionMatchesSearch(rSrcX, y) {
					rStyle = rStyle.Background(tcell.ColorDarkGoldenrod).Foreground(tcell.ColorWhite)
				}
				e.screen.SetContent(x, screenY, r, nil, rStyle)
				x++
			}
			i = end + marker
			continue
		}
		e.screen.SetContent(x, screenY, runes[i], nil, s)
		x++
		i++
	}
}

func closingRune(runes []rune, start int, target rune) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
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
	runes := []rune(strings.TrimSpace(line))
	var parts []string
	var cell []rune
	inCode, escaped := false, false
	for _, r := range runes {
		switch {
		case escaped:
			cell = append(cell, r)
			escaped = false
		case r == '\\':
			cell = append(cell, r)
			escaped = true
		case r == '`':
			cell = append(cell, r)
			inCode = !inCode
		case r == '|' && !inCode:
			parts = append(parts, string(cell))
			cell = nil
		default:
			cell = append(cell, r)
		}
	}
	parts = append(parts, string(cell))
	if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
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
