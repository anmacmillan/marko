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
	screen          tcell.Screen
	path            string
	lines           []string
	x, y, top       int
	dirty           bool
	status          string
	confirmQuit     bool
	preferredX      int
	prompt          string
	promptValue     string
	promptCursor    int
	lastEdit        time.Time
	recovery        string
	theme           theme
	themeName       string
	undo, redo      []snapshot
	selecting       bool
	selX, selY      int
	mouseDown       bool
	lastClick       time.Time
	clickX          int
	clickY          int
	clickCount      int
	search          string
	replace         string
	lastAction      time.Time
	focusMode       bool
	manualScroll    bool
	modTime         time.Time
	conflict        bool
	showHelp        bool
	showCoach       bool
	coachUntil      time.Time
	showStartMenu   bool
	startMenuIndex  int
	showRecent      bool
	recent          []string
	recentIndex     int
	renameFrom      string
	waitingForPaste bool
	untitled        bool
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
	text, heading1, heading2, heading3, table, quote, background, statusBG, statusFG, muted                tcell.Color
	selectionBG, selectionFG, searchBG, searchFG, codeFG, codeBG, highlightBG, highlightFG, focusBG, dimBG tcell.Color
	accent1, accent2, accent3, accent4                                                                     tcell.Color
}

var themeNames = []string{"calm", "matrix", "midnight", "paper", "ember", "green", "mono", "light", "ia-light", "ia-dark"}

const writingWidth = 88

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: marko [FILE.md]")
		os.Exit(2)
	}
	path := ""
	untitled := false
	if len(os.Args) == 2 {
		path = os.Args[1]
	} else {
		path = uniqueUntitledPath(time.Now(), ".")
		untitled = true
	}
	e, err := newEditor(path, untitled)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer e.screen.Fini()
	e.run()
}

func newEditor(path string, untitled bool) (*editor, error) {
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
	status := "F1 shortcuts · F5/F6/F7 headings · Ctrl-A select all · Ctrl-C copy · Ctrl-S save"
	themeName := selectedThemeName()
	now := time.Now()
	e := &editor{screen: s, path: path, untitled: untitled, lines: lines, status: status, themeName: themeName, theme: themeByName(themeName), lastAction: now, focusMode: true, modTime: modTime, showCoach: !untitled, coachUntil: now.Add(5 * time.Second), showStartMenu: untitled}
	if !untitled {
		e.rememberRecent(path)
	} else {
		e.recent = loadRecent()
	}
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
	case "matrix", "green":
		return theme{
			text: tcell.GetColor("#7dff9a"), heading1: tcell.GetColor("#dcff6b"), heading2: tcell.GetColor("#55f6ff"), heading3: tcell.GetColor("#ffcc66"), table: tcell.GetColor("#55f6ff"), quote: tcell.GetColor("#5a8f63"),
			background: tcell.GetColor("#020805"), statusBG: tcell.GetColor("#063d1f"), statusFG: tcell.GetColor("#c6ffd1"), muted: tcell.GetColor("#226b38"),
			selectionBG: tcell.GetColor("#1f6f43"), selectionFG: tcell.GetColor("#f4fff4"), searchBG: tcell.GetColor("#6b5f00"), searchFG: tcell.GetColor("#fffbd1"), codeFG: tcell.GetColor("#f6ff8f"), codeBG: tcell.GetColor("#123019"), highlightBG: tcell.GetColor("#d6ff3f"), highlightFG: tcell.GetColor("#081008"), focusBG: tcell.GetColor("#07150c"), dimBG: tcell.GetColor("#020805"),
			accent1: tcell.GetColor("#45f7b0"), accent2: tcell.GetColor("#ff6b7a"), accent3: tcell.GetColor("#fff06a"), accent4: tcell.GetColor("#708a75"),
		}
	case "midnight":
		return theme{
			text: tcell.GetColor("#d8dee9"), heading1: tcell.GetColor("#8fbcff"), heading2: tcell.GetColor("#88c0d0"), heading3: tcell.GetColor("#b48ead"), table: tcell.GetColor("#8fbcff"), quote: tcell.GetColor("#6f7f95"),
			background: tcell.GetColor("#0b1020"), statusBG: tcell.GetColor("#1b2740"), statusFG: tcell.GetColor("#e6edf7"), muted: tcell.GetColor("#4c566a"),
			selectionBG: tcell.GetColor("#314a6e"), selectionFG: tcell.GetColor("#ffffff"), searchBG: tcell.GetColor("#6f4e1f"), searchFG: tcell.GetColor("#fff2cc"), codeFG: tcell.GetColor("#a3be8c"), codeBG: tcell.GetColor("#152033"), highlightBG: tcell.GetColor("#ebcb8b"), highlightFG: tcell.GetColor("#111827"), focusBG: tcell.GetColor("#111a2e"), dimBG: tcell.GetColor("#080d19"),
			accent1: tcell.GetColor("#8fbcff"), accent2: tcell.GetColor("#bf616a"), accent3: tcell.GetColor("#ebcb8b"), accent4: tcell.GetColor("#7b8798"),
		}
	case "paper":
		return theme{
			text: tcell.GetColor("#1f2933"), heading1: tcell.GetColor("#1d4ed8"), heading2: tcell.GetColor("#0f766e"), heading3: tcell.GetColor("#7c3aed"), table: tcell.GetColor("#0f766e"), quote: tcell.GetColor("#64748b"),
			background: tcell.GetColor("#fbf7ef"), statusBG: tcell.GetColor("#e7dfd0"), statusFG: tcell.GetColor("#1f2933"), muted: tcell.GetColor("#94a3b8"),
			selectionBG: tcell.GetColor("#bfdbfe"), selectionFG: tcell.GetColor("#111827"), searchBG: tcell.GetColor("#fde68a"), searchFG: tcell.GetColor("#111827"), codeFG: tcell.GetColor("#7c2d12"), codeBG: tcell.GetColor("#f1eadc"), highlightBG: tcell.GetColor("#facc15"), highlightFG: tcell.GetColor("#111827"), focusBG: tcell.GetColor("#fffdf7"), dimBG: tcell.GetColor("#f2eadc"),
			accent1: tcell.GetColor("#0f766e"), accent2: tcell.GetColor("#dc2626"), accent3: tcell.GetColor("#b45309"), accent4: tcell.GetColor("#64748b"),
		}
	case "ember":
		return theme{
			text: tcell.GetColor("#f4e7d3"), heading1: tcell.GetColor("#ffb86b"), heading2: tcell.GetColor("#ff7a59"), heading3: tcell.GetColor("#ffd166"), table: tcell.GetColor("#f78c6c"), quote: tcell.GetColor("#9f7a60"),
			background: tcell.GetColor("#120d0b"), statusBG: tcell.GetColor("#3b1f16"), statusFG: tcell.GetColor("#ffe8cc"), muted: tcell.GetColor("#6b4b3b"),
			selectionBG: tcell.GetColor("#7a3f22"), selectionFG: tcell.GetColor("#fff7ed"), searchBG: tcell.GetColor("#8f5e15"), searchFG: tcell.GetColor("#fff3c4"), codeFG: tcell.GetColor("#ffd166"), codeBG: tcell.GetColor("#261611"), highlightBG: tcell.GetColor("#ffb703"), highlightFG: tcell.GetColor("#1f130d"), focusBG: tcell.GetColor("#1c110d"), dimBG: tcell.GetColor("#0d0908"),
			accent1: tcell.GetColor("#ff9f6e"), accent2: tcell.GetColor("#ff5d5d"), accent3: tcell.GetColor("#ffd166"), accent4: tcell.GetColor("#a78b78"),
		}
	case "mono":
		return theme{text: tcell.ColorSilver, heading1: tcell.ColorWhite, heading2: tcell.ColorWhite, heading3: tcell.ColorSilver, table: tcell.ColorSilver, quote: tcell.ColorGray, background: tcell.ColorDefault, statusBG: tcell.ColorGray, statusFG: tcell.ColorBlack, muted: tcell.ColorDarkGray, selectionBG: tcell.ColorGray, selectionFG: tcell.ColorWhite, searchBG: tcell.ColorWhite, searchFG: tcell.ColorBlack, codeFG: tcell.ColorWhite, codeBG: tcell.ColorBlack, highlightBG: tcell.ColorWhite, highlightFG: tcell.ColorBlack, focusBG: tcell.ColorBlack, dimBG: tcell.ColorDefault, accent1: tcell.ColorWhite, accent2: tcell.ColorSilver, accent3: tcell.ColorWhite, accent4: tcell.ColorGray}
	case "light":
		return theme{text: tcell.ColorBlack, heading1: tcell.ColorDarkBlue, heading2: tcell.ColorDarkGreen, heading3: tcell.ColorDarkGoldenrod, table: tcell.ColorDarkGreen, quote: tcell.ColorDarkSlateGray, background: tcell.ColorWhite, statusBG: tcell.ColorLightGray, statusFG: tcell.ColorBlack, muted: tcell.ColorDarkGray, selectionBG: tcell.GetColor("#c7d2fe"), selectionFG: tcell.ColorBlack, searchBG: tcell.GetColor("#fde68a"), searchFG: tcell.ColorBlack, codeFG: tcell.GetColor("#7c2d12"), codeBG: tcell.GetColor("#f3f4f6"), highlightBG: tcell.GetColor("#facc15"), highlightFG: tcell.ColorBlack, focusBG: tcell.ColorWhite, dimBG: tcell.GetColor("#f3f4f6"), accent1: tcell.ColorDarkGreen, accent2: tcell.ColorMaroon, accent3: tcell.ColorDarkGoldenrod, accent4: tcell.ColorDarkGray}
	case "ia-light":
		return theme{text: tcell.GetColor("#1c1c1e"), heading1: tcell.GetColor("#007aff"), heading2: tcell.GetColor("#111111"), heading3: tcell.GetColor("#3a3a3c"), table: tcell.GetColor("#007aff"), quote: tcell.GetColor("#8e8e93"), background: tcell.GetColor("#f5f5f7"), statusBG: tcell.GetColor("#e5e5ea"), statusFG: tcell.GetColor("#1c1c1e"), muted: tcell.GetColor("#aeaeb2"), selectionBG: tcell.GetColor("#bfdbfe"), selectionFG: tcell.GetColor("#1c1c1e"), searchBG: tcell.GetColor("#fde68a"), searchFG: tcell.GetColor("#1c1c1e"), codeFG: tcell.GetColor("#007aff"), codeBG: tcell.GetColor("#e5e5ea"), highlightBG: tcell.GetColor("#ffd60a"), highlightFG: tcell.GetColor("#1c1c1e"), focusBG: tcell.GetColor("#ffffff"), dimBG: tcell.GetColor("#ececf0"), accent1: tcell.GetColor("#007aff"), accent2: tcell.GetColor("#ff3b30"), accent3: tcell.GetColor("#ff9500"), accent4: tcell.GetColor("#8e8e93")}
	case "ia-dark":
		return theme{text: tcell.GetColor("#e5e5ea"), heading1: tcell.GetColor("#0a84ff"), heading2: tcell.GetColor("#ffffff"), heading3: tcell.GetColor("#d1d1d6"), table: tcell.GetColor("#0a84ff"), quote: tcell.GetColor("#636366"), background: tcell.GetColor("#161617"), statusBG: tcell.GetColor("#2c2c2e"), statusFG: tcell.GetColor("#e5e5ea"), muted: tcell.GetColor("#48484a"), selectionBG: tcell.GetColor("#1f4f86"), selectionFG: tcell.GetColor("#ffffff"), searchBG: tcell.GetColor("#6f4e1f"), searchFG: tcell.GetColor("#fff2cc"), codeFG: tcell.GetColor("#64d2ff"), codeBG: tcell.GetColor("#202124"), highlightBG: tcell.GetColor("#ffd60a"), highlightFG: tcell.GetColor("#111111"), focusBG: tcell.GetColor("#1d1d20"), dimBG: tcell.GetColor("#121214"), accent1: tcell.GetColor("#64d2ff"), accent2: tcell.GetColor("#ff453a"), accent3: tcell.GetColor("#ffd60a"), accent4: tcell.GetColor("#636366")}
	default:
		return theme{text: tcell.ColorSilver, heading1: tcell.ColorLightSkyBlue, heading2: tcell.ColorLightGreen, heading3: tcell.ColorLightGoldenrodYellow, table: tcell.ColorPaleGreen, quote: tcell.ColorGray, background: tcell.ColorDefault, statusBG: tcell.ColorDarkSlateGray, statusFG: tcell.ColorWhite, muted: tcell.ColorDarkGray, selectionBG: tcell.ColorDodgerBlue, selectionFG: tcell.ColorWhite, searchBG: tcell.ColorDarkGoldenrod, searchFG: tcell.ColorWhite, codeFG: tcell.ColorLightGoldenrodYellow, codeBG: tcell.ColorDarkSlateGray, highlightBG: tcell.ColorGoldenrod, highlightFG: tcell.ColorBlack, focusBG: tcell.ColorDefault, dimBG: tcell.ColorDefault, accent1: tcell.ColorLightSeaGreen, accent2: tcell.ColorLightCoral, accent3: tcell.ColorLightYellow, accent4: tcell.ColorGray}
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
			recent = append(recent, path)
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
			if e.showStartMenu {
				continue
			}
			e.screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
	}()
	for {
		e.draw()
		switch ev := e.screen.PollEvent().(type) {
		case *tcell.EventResize:
			e.screen.Sync()
		case *tcell.EventInterrupt:
			e.tick()
		case *tcell.EventKey:
			e.lastAction, e.manualScroll = time.Now(), false
			if e.key(ev) {
				return
			}
		case *tcell.EventMouse:
			if ev.Buttons() != tcell.ButtonNone {
				e.lastAction = time.Now()
			}
			e.mouse(ev)
		case *tcell.EventClipboard:
			if e.waitingForPaste {
				e.insertText(string(ev.Data()))
				e.waitingForPaste = false
			}
		}
	}
}

func (e *editor) tick() {
	if e.showCoach && !e.coachUntil.IsZero() && time.Now().After(e.coachUntil) {
		e.showCoach = false
	}
	if e.showStartMenu {
		return
	}
	e.autosave()
	if e.externalChange() {
		if !e.dirty {
			e.reloadFile()
		} else {
			e.conflict = true
			e.status = "File changed outside Marko. Use Save As or reopen it."
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
	x = e.sourceColumnForMouse(rows[index], x-left)
	if buttons&tcell.Button1 != 0 {
		if !e.mouseDown {
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
			e.selX, e.selY = x, y
			e.mouseDown = true
		}
		e.x, e.y = x, y
		e.selecting = e.selX != e.x || e.selY != e.y
	} else {
		e.mouseDown = false
	}
}

func (e *editor) sourceColumnForMouse(vr visualRow, displayColumn int) int {
	displayColumn = max(0, displayColumn)
	line := e.lines[vr.y]
	return sourceColumnForRenderedColumn(line, vr.start, displayColumn)
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
	target := e.nextVisualRowIndex(rows, current, delta*max(1, bodyH-1))
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

func eventKey(ev *tcell.EventKey) tcell.Key {
	if ev.Key() != tcell.KeyRune {
		return ev.Key()
	}
	r := ev.Rune()
	if r >= 1 && r <= 26 {
		return tcell.KeyCtrlA + tcell.Key(r-1)
	}
	if ev.Modifiers()&tcell.ModCtrl == 0 {
		return ev.Key()
	}
	if r >= 'A' && r <= 'Z' {
		return tcell.KeyCtrlA + tcell.Key(r-'A')
	}
	if r >= 'a' && r <= 'z' {
		return tcell.KeyCtrlA + tcell.Key(r-'a')
	}
	return ev.Key()
}

func textInputModifiers(mod tcell.ModMask) bool {
	return mod&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta|tcell.ModHyper) == 0
}

func (e *editor) key(ev *tcell.EventKey) bool {
	if e.showStartMenu {
		return e.startMenuKey(ev)
	}
	if e.showRecent {
		e.recentKey(ev)
		return false
	}
	if e.prompt != "" && eventKey(ev) == tcell.KeyCtrlQ {
		if e.dirty && !e.confirmQuit {
			e.status = "Unsaved changes. Press Ctrl-Q again to quit."
			e.confirmQuit = true
			return false
		}
		return true
	}
	if e.prompt != "" {
		e.promptKey(ev)
		return false
	}
	if e.showCoach {
		e.showCoach = false
		if ev.Key() == tcell.KeyEsc {
			return false
		}
	}
	switch eventKey(ev) {
	case tcell.KeyCtrlQ:
		if e.dirty && !e.confirmQuit {
			e.status = "Unsaved changes. Press Ctrl-Q again to quit."
			e.confirmQuit = true
			return false
		}
		return true
	case tcell.KeyCtrlS:
		if ev.Modifiers()&tcell.ModShift != 0 || e.untitled || e.path == "" {
			e.openSaveAsPrompt()
		} else {
			e.save()
		}
	case tcell.KeyF2:
		e.openSaveAsPrompt()
	case tcell.KeyF3:
		e.openRecentFiles()
	case tcell.KeyF4:
		e.openStartMenu()
	case tcell.KeyF5:
		e.toggleHeading(1)
	case tcell.KeyF6:
		e.toggleHeading(2)
	case tcell.KeyF7:
		e.toggleHeading(3)
	case tcell.KeyCtrlG:
		e.cycleTheme()
	case tcell.KeyCtrlL:
		e.reloadFile()
	case tcell.KeyCtrlD:
		e.checkpoint()
		e.deleteLine()
	case tcell.KeyCtrlE:
		if ev.Modifiers()&tcell.ModShift != 0 {
			e.openRecentFiles()
		} else {
			e.checkpoint()
			e.toggleEmphasis("*", "*")
		}
	case tcell.KeyCtrlB:
		e.checkpoint()
		e.toggleEmphasis("**", "**")
	case tcell.KeyCtrlU:
		e.checkpoint()
		e.toggleEmphasis("<u>", "</u>")
	case tcell.KeyCtrlH:
		e.checkpoint()
		e.toggleEmphasis("==", "==")
	case tcell.KeyCtrlA:
		e.selectAll()
	case tcell.KeyCtrlZ:
		e.undoEdit()
	case tcell.KeyCtrlY:
		e.redoEdit()
	case tcell.KeyCtrlF:
		e.prompt = "Find: "
		e.promptValue = e.search
		e.promptCursor = runeLen(e.promptValue)
	case tcell.KeyCtrlN:
		e.findNext()
	case tcell.KeyCtrlP:
		e.findPrevious()
	case tcell.KeyCtrlR:
		if ev.Modifiers()&tcell.ModShift != 0 {
			e.prompt = "Rename to: "
			e.promptValue = e.path
			e.renameFrom = e.path
			e.promptCursor = runeLen(e.promptValue)
		} else if e.search == "" {
			e.prompt = "Find: "
			e.promptValue = ""
			e.promptCursor = 0
		} else {
			e.prompt = "Replace with: "
			e.promptValue = e.replace
			e.promptCursor = runeLen(e.promptValue)
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
	case tcell.KeyCtrlK:
		e.focusMode = !e.focusMode
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
		if !textInputModifiers(ev.Modifiers()) {
			e.status = "Ignored modified key"
			break
		}
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
	e.untitled = false
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
	switch eventKey(ev) {
	case tcell.KeyEsc:
		e.prompt, e.promptValue, e.promptCursor = "", "", 0
		e.status = "Cancelled"
	case tcell.KeyCtrlK:
		e.focusMode = !e.focusMode
	case tcell.KeyEnter:
		e.submitPrompt()
	case tcell.KeyCtrlS:
		e.submitPrompt()
	case tcell.KeyLeft:
		if e.promptCursor > 0 {
			e.promptCursor--
		}
	case tcell.KeyRight:
		if e.promptCursor < runeLen(e.promptValue) {
			e.promptCursor++
		}
	case tcell.KeyHome:
		e.promptCursor = 0
	case tcell.KeyEnd:
		e.promptCursor = runeLen(e.promptValue)
	case tcell.KeyDelete:
		r := []rune(e.promptValue)
		if e.promptCursor < len(r) {
			e.promptValue = string(r[:e.promptCursor]) + string(r[e.promptCursor+1:])
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		r := []rune(e.promptValue)
		if e.promptCursor > 0 && e.promptCursor <= len(r) {
			e.promptValue = string(r[:e.promptCursor-1]) + string(r[e.promptCursor:])
			e.promptCursor--
		}
		if e.prompt == "Find: " {
			e.search = e.promptValue
		}
	case tcell.KeyCtrlA:
		if e.prompt == "Replace with: " {
			e.replace = e.promptValue
			e.prompt, e.promptValue, e.promptCursor = "", "", 0
			e.replaceAll()
		}
	case tcell.KeyRune:
		r := []rune(e.promptValue)
		insert := []rune{ev.Rune()}
		e.promptValue = string(r[:e.promptCursor]) + string(insert) + string(r[e.promptCursor:])
		e.promptCursor++
		if e.prompt == "Find: " {
			e.search = e.promptValue
			e.findFromStart()
		}
	}
}

func (e *editor) submitPrompt() {
	if e.prompt == "Find: " {
		e.search = e.promptValue
		e.prompt, e.promptValue, e.promptCursor = "", "", 0
		e.findNext()
		return
	}
	if e.prompt == "Replace with: " {
		e.replace = e.promptValue
		e.prompt, e.promptValue, e.promptCursor = "", "", 0
		e.replaceCurrent()
		return
	}
	if e.prompt == "Rename to: " {
		e.renameFile(e.promptValue)
		e.prompt, e.promptValue, e.promptCursor = "", "", 0
		return
	}
	if e.prompt == "Open path: " {
		path := strings.TrimSpace(e.promptValue)
		e.prompt, e.promptValue, e.promptCursor = "", "", 0
		if path == "" {
			e.status = "Enter a filename"
			return
		}
		expanded, err := expandPathInput(path)
		if err != nil {
			e.status = err.Error()
			return
		}
		e.openFile(expanded)
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
	expanded, err := expandPathInput(path)
	if err != nil {
		e.status = err.Error()
		return
	}
	e.path = expanded
	e.untitled = false
	e.conflict = false
	e.modTime = time.Time{}
	e.prompt, e.promptValue, e.promptCursor = "", "", 0
	e.save()
}

func (e *editor) startMenuKey(ev *tcell.EventKey) bool {
	switch eventKey(ev) {
	case tcell.KeyEsc:
		e.showStartMenu = false
	case tcell.KeyCtrlQ:
		return true
	case tcell.KeyF1:
		e.showHelp = !e.showHelp
	case tcell.KeyF3:
		e.activateStartMenuAction("recent")
	case tcell.KeyUp:
		e.moveStartMenuSelection(-1)
	case tcell.KeyDown:
		e.moveStartMenuSelection(1)
	case tcell.KeyEnter:
		return e.activateStartMenuAction(e.startMenuItems()[e.startMenuIndex].action)
	case tcell.KeyRune:
		switch strings.ToLower(string(ev.Rune())) {
		case "n":
			e.activateStartMenuAction("new")
		case "o":
			e.activateStartMenuAction("open")
		case "r":
			e.activateStartMenuAction("recent")
		case "t":
			e.activateStartMenuAction("theme")
		case "q":
			return true
		}
	}
	return false
}

type startMenuItem struct {
	label      string
	action     string
	recentRank int
}

func (e *editor) openStartMenu() {
	if e.dirty && !e.untitled && e.path != "" {
		e.save()
	}
	e.recent = loadRecent()
	e.showStartMenu = true
	e.showRecent = false
	e.showHelp = false
	e.prompt, e.promptValue, e.promptCursor = "", "", 0
	e.startMenuIndex = 0
}

func (e *editor) startMenuItems() []startMenuItem {
	items := []startMenuItem{
		{label: "[N] New document", action: "new"},
		{label: "[O] Open path...", action: "open"},
		{label: "[R] Recent files", action: "recent"},
		{label: "[T] Theme: " + e.displayThemeName(), action: "theme"},
	}
	for i, path := range e.recent {
		items = append(items, startMenuItem{label: "    " + recentDisplayLabel(path, 72), action: "open:" + path, recentRank: i})
	}
	items = append(items, startMenuItem{label: "Return to document", action: "return"})
	items = append(items, startMenuItem{label: "[Q] Quit", action: "quit"})
	return items
}

func (e *editor) displayThemeName() string {
	if e.themeName != "" {
		return e.themeName
	}
	return selectedThemeName()
}

func (e *editor) moveStartMenuSelection(delta int) {
	items := e.startMenuItems()
	if len(items) == 0 {
		e.startMenuIndex = 0
		return
	}
	e.startMenuIndex = (e.startMenuIndex + delta + len(items)) % len(items)
}

func (e *editor) activateStartMenuAction(action string) bool {
	switch {
	case action == "new":
		e.newUntitledDocument()
	case action == "open":
		e.showStartMenu = false
		e.prompt = "Open path: "
		e.promptValue = ""
		e.promptCursor = 0
	case action == "recent":
		e.openRecentFiles()
		e.showStartMenu = false
	case action == "theme":
		e.cycleTheme()
	case strings.HasPrefix(action, "open:"):
		e.showStartMenu = false
		e.openFile(strings.TrimPrefix(action, "open:"))
	case action == "return":
		e.showStartMenu = false
	case action == "quit":
		return true
	}
	return false
}

func (e *editor) newUntitledDocument() {
	e.showStartMenu = false
	e.lines = []string{""}
	e.x, e.y, e.top = 0, 0, 0
	e.path = uniqueUntitledPath(time.Now(), ".")
	e.untitled = true
	e.dirty = false
	e.conflict = false
	e.modTime = time.Time{}
	e.status = "New document"
}

func (e *editor) openSaveAsPrompt() {
	e.prompt = "Save as: "
	if e.untitled || e.path == "" {
		e.promptValue = "untitled.md"
	} else {
		e.promptValue = e.path
	}
	e.promptCursor = runeLen(e.promptValue)
}

func (e *editor) openRecentFiles() {
	e.recent = loadRecent()
	e.recentIndex = 0
	e.showRecent = true
}

func expandUserPath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func expandPathInput(path string) (string, error) {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "z ") {
		return expandZoxidePath(strings.TrimSpace(strings.TrimPrefix(path, "z ")))
	}
	return expandUserPath(path), nil
}

func expandZoxidePath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("Enter a zoxide query")
	}
	query, rest := splitZoxideInput(input)
	if query == "" {
		return "", fmt.Errorf("Enter a zoxide query")
	}
	zoxide, err := exec.LookPath("zoxide")
	if err != nil {
		return "", fmt.Errorf("zoxide not found")
	}
	out, err := exec.Command(zoxide, "query", query).Output()
	if err != nil {
		return "", fmt.Errorf("zoxide match not found: %s", query)
	}
	base := strings.TrimSpace(string(out))
	if base == "" {
		return "", fmt.Errorf("zoxide match not found: %s", query)
	}
	if rest == "" {
		return base, nil
	}
	return filepath.Join(base, rest), nil
}

func splitZoxideInput(input string) (string, string) {
	input = strings.Trim(input, "/")
	parts := strings.Split(input, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], filepath.Join(parts[1:]...)
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
	ax, ay, bx, by, ok := e.selectionBounds()
	if !ok {
		return ""
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
	ax, ay, bx, by, ok := e.selectionBounds()
	if !ok {
		e.selecting = false
		return
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

// toggleEmphasis wraps or unwraps the current selection in the given Markdown
// markers (e.g. "**"/"**" for bold, "<u>"/"</u>" for underline). With no
// selection it inserts an empty marker pair and leaves the cursor between them.
func (e *editor) toggleEmphasis(open, close string) {
	if !e.selecting {
		e.insertEmptyMarkers(open, close)
		e.status = "Insert " + emphasisLabel(open, close)
		return
	}
	ax, ay, bx, by, ok := e.selectionBounds()
	if !ok {
		e.insertEmptyMarkers(open, close)
		e.status = "Insert " + emphasisLabel(open, close)
		return
	}
	if ay == by {
		e.toggleEmphasisSingleLine(ay, ax, bx, open, close)
		return
	}
	// Multi-line selection: wrap each touched line's selected span.
	e.checkpoint()
	for y := ay; y <= by; y++ {
		runes := []rune(e.lines[y])
		startX, endX := 0, len(runes)
		if y == ay {
			startX = ax
		}
		if y == by {
			endX = bx
		}
		if startX == endX {
			continue
		}
		e.lines[y] = string(runes[:startX]) + open + string(runes[startX:endX]) + close + string(runes[endX:])
	}
	e.dirty = true
	// Keep the selection covering the same logical span.
	e.selX, e.selY = ax, ay
	if ay == by {
		e.x = bx + runeLen(open) + runeLen(close)
	} else {
		e.x = bx + runeLen(close)
	}
	e.y = by
	e.selecting = true
	e.status = "Applied " + emphasisLabel(open, close)
}

// toggleEmphasisSingleLine wraps or unwraps [ax, bx) on a single line.
func (e *editor) toggleEmphasisSingleLine(y, ax, bx int, open, close string) {
	runes := []rune(e.lines[y])
	openRunes := []rune(open)
	closeRunes := []rune(close)
	// Unwrap if the selection is already bracketed by these markers.
	if ax >= len(openRunes) && bx+len(closeRunes) <= len(runes) &&
		runesEqualAt(runes, ax-len(openRunes), openRunes) &&
		runesEqualAt(runes, bx, closeRunes) {
		e.checkpoint()
		newRunes := append(append([]rune{}, runes[:ax-len(openRunes)]...), runes[ax:bx]...)
		newRunes = append(newRunes, runes[bx+len(closeRunes):]...)
		e.lines[y] = string(newRunes)
		e.dirty = true
		e.selX, e.selY = ax-len(openRunes), y
		e.x, e.y = bx-len(openRunes), y
		e.selecting = true
		e.status = "Removed " + emphasisLabel(open, close)
		return
	}
	// Otherwise wrap the selection.
	e.checkpoint()
	newRunes := append([]rune{}, runes[:ax]...)
	newRunes = append(newRunes, openRunes...)
	newRunes = append(newRunes, runes[ax:bx]...)
	newRunes = append(newRunes, closeRunes...)
	newRunes = append(newRunes, runes[bx:]...)
	e.lines[y] = string(newRunes)
	e.dirty = true
	e.selX, e.selY = ax+len(openRunes), y
	e.x, e.y = bx+len(openRunes), y
	e.selecting = true
	e.status = "Applied " + emphasisLabel(open, close)
}

// insertEmptyMarkers inserts an open/close pair and places the cursor between
// them so subsequent typing is formatted.
func (e *editor) insertEmptyMarkers(open, close string) {
	e.checkpoint()
	r := []rune(e.lines[e.y])
	before, after := string(r[:e.x]), string(r[e.x:])
	e.lines[e.y] = before + open + close + string(after)
	e.x += runeLen(open)
	e.dirty = true
}

func (e *editor) toggleHeading(level int) {
	if level < 1 || level > 6 || e.y < 0 || e.y >= len(e.lines) {
		return
	}
	e.checkpoint()
	line := e.lines[e.y]
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	oldPrefixLen := runeLen(indent)
	body := strings.TrimSpace(line[len(indent):])
	if existingLevel, text, ok := heading(line); ok {
		oldPrefixLen += existingLevel + 1
		body = text
		if existingLevel == level {
			e.lines[e.y] = indent + body
			e.x = min(max(e.x-(existingLevel+1), 0), runeLen(e.lines[e.y]))
			e.selecting = false
			e.dirty = true
			e.status = "Removed heading"
			return
		}
	}
	prefix := strings.Repeat("#", level) + " "
	e.lines[e.y] = indent + prefix + body
	newPrefixLen := runeLen(indent) + runeLen(prefix)
	if body == "" {
		e.x = newPrefixLen
	} else if e.x >= oldPrefixLen {
		e.x = min(max(e.x-oldPrefixLen+newPrefixLen, 0), runeLen(e.lines[e.y]))
	} else {
		e.x = min(e.x, runeLen(e.lines[e.y]))
	}
	e.selecting = false
	e.dirty = true
	e.status = "Heading " + strconv.Itoa(level)
}

func emphasisLabel(open, close string) string {
	switch open {
	case "**", "__":
		return "bold"
	case "*", "_":
		return "italic"
	case "==":
		return "highlight"
	case "<u>":
		return "underline"
	default:
		return open + close
	}
}

// selectAll selects the entire document.
func (e *editor) selectAll() {
	if len(e.lines) == 0 {
		return
	}
	e.selX, e.selY = 0, 0
	e.y = len(e.lines) - 1
	e.x = runeLen(e.lines[e.y])
	e.selecting = true
	e.status = "Selected all"
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
	target := e.nextVisualRowIndex(rows, current, delta)
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
	e.rememberRecent(e.path)
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
	e.path, e.renameFrom, e.untitled = path, "", false
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
	if e.showStartMenu {
		e.drawStartMenu(w, h)
		if e.showHelp {
			e.drawHelp(w, h)
		}
		e.screen.Show()
		return
	}
	statusRows := 1
	if e.focusMode {
		statusRows = 0
	}
	bodyH := max(1, h-statusRows)
	left, contentWidth := writingArea(w)
	background := tcell.StyleDefault.Background(e.theme.background)
	for row := 0; row < bodyH; row++ {
		e.put(0, row, strings.Repeat(" ", w), background, w)
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
		if vr.start < 0 {
			style := e.focusStyle(tcell.StyleDefault.Background(e.theme.background), e.focusedLine(vr.y))
			e.put(left, row, strings.Repeat(" ", contentWidth), style, left+contentWidth)
			continue
		}
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
		if e.prompt == "Save as: " || e.prompt == "Open path: " {
			status += "  [z query/file.md, or type a path]"
		}
	}
	if !e.focusMode || e.prompt != "" {
		e.put(0, h-1, status, tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG), w)
	}
	if e.showHelp {
		e.drawHelp(w, h)
	}
	if e.showRecent {
		e.drawRecent(w, h)
	}
	if e.showCoach && !e.showHelp && !e.showRecent && e.prompt == "" {
		e.drawCoach(w, h)
	}
	if e.prompt != "" {
		e.screen.ShowCursor(1+runeLen(e.prompt)+e.promptCursor, h-1)
	} else {
		if e.showStartMenu || e.showRecent || e.showHelp {
			e.screen.HideCursor()
			e.screen.Show()
			return
		}
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

func (e *editor) paragraphBounds(y int) (int, int) {
	if y < 0 || y >= len(e.lines) {
		return y, y
	}
	if start, end, ok := e.codeFenceBounds(y); ok {
		return start, end
	}
	if e.inTable(y) {
		return e.tableBounds(y)
	}
	if start, end, ok := e.zopaFenceBounds(y); ok {
		return start, end
	}
	if start, end, ok := e.barChartFenceBounds(y); ok {
		return start, end
	}

	currentLine := e.lines[y]
	if isParagraphBoundary(currentLine) {
		return y, y
	}

	start, end := y, y
	for start > 0 && !isParagraphBoundary(e.lines[start-1]) {
		if _, _, ok := e.codeFenceBounds(start - 1); ok {
			break
		}
		if e.inTable(start - 1) {
			break
		}
		if _, _, ok := e.zopaFenceBounds(start - 1); ok {
			break
		}
		if _, _, ok := e.barChartFenceBounds(start - 1); ok {
			break
		}
		start--
	}
	for end+1 < len(e.lines) && !isParagraphBoundary(e.lines[end+1]) {
		if _, _, ok := e.codeFenceBounds(end + 1); ok {
			break
		}
		if e.inTable(end + 1) {
			break
		}
		if _, _, ok := e.zopaFenceBounds(end + 1); ok {
			break
		}
		if _, _, ok := e.barChartFenceBounds(end + 1); ok {
			break
		}
		end++
	}
	return start, end
}

func (e *editor) focusedLine(y int) bool {
	if !e.focusMode {
		return y == e.y
	}
	start, end := e.paragraphBounds(e.y)
	return y >= start && y <= end
}

func (e *editor) drawHelp(w, h int) {
	lines := []string{
		" Marko help ",
		"Key              Action",
		"F1               Close help",
		"Ctrl-S           Save",
		"F2 / Ctrl-Shift-S Save As",
		"F3 / Ctrl-Shift-E Recent files",
		"F4               Home screen",
		"Ctrl-Q           Quit",
		"Ctrl-A           Select all",
		"Shift-arrows     Select text",
		"Drag             Select text",
		"Ctrl-C/X/V       Copy / cut / paste",
		"Ctrl-B/E/U/H     Bold / italic / underline / highlight",
		"F5 / F6 / F7     Heading 1 / 2 / 3",
		"Ctrl-Space       Toggle checkbox",
		"Ctrl-O           Link selection",
		"Ctrl-T           Create table",
		"Ctrl-F/N/P       Find / next / previous",
		"Ctrl-R           Replace",
		"Ctrl-Shift-R     Rename file",
		"Ctrl-Z/Y         Undo / redo",
		"Ctrl-K           Focus mode",
		"Ctrl-Shift-E     Recent files",
		"Ctrl-G           Cycle theme",
		"Open/Save path   Use z query/file.md, or type a path",
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

func (e *editor) drawCoach(w, h int) {
	lines := []string{
		" MARKO ",
		"Markdown focus",
		"F2 save as   F3 recent   F5/F6/F7 headings",
		"F1 more   Esc dismiss",
	}
	width := 0
	for _, line := range lines {
		width = max(width, runeLen(line))
	}
	x := max(0, (w-width-4)/2)
	y := max(0, h-len(lines)-4)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < len(lines)+2 && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, width+4)), box, w)
	}
	for row, line := range lines {
		e.put(x+2, y+1+row, line, box, w)
	}
}

func (e *editor) drawStartMenu(w, h int) {
	bg := tcell.StyleDefault.Background(e.theme.background).Foreground(e.theme.text)
	for row := 0; row < h; row++ {
		e.put(0, row, strings.Repeat(" ", w), bg, w)
	}
	lines := []string{
		" MARKO ",
		"Markdown focus",
		"",
	}
	items := e.startMenuItems()
	if e.startMenuIndex < 0 || e.startMenuIndex >= len(items) {
		e.startMenuIndex = 0
	}
	for i, item := range items {
		prefix := "  "
		if i == e.startMenuIndex {
			prefix = "> "
		}
		lines = append(lines, prefix+item.label)
	}
	lines = append(lines, "", "Up/Down select   Enter open   Esc return", "F1 help   F3 recent   F4 home")
	width := 0
	for _, line := range lines {
		width = max(width, min(runeLen(line), max(20, w-8)))
	}
	x := max(0, (w-width-4)/2)
	y := max(0, (h-len(lines)-2)/2)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < len(lines)+2 && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, width+4)), box, w)
	}
	for row, line := range lines {
		if runeLen(line) > width {
			line = string([]rune(line)[:width])
		}
		style := box
		if row == 0 {
			style = style.Bold(true)
		} else if strings.HasPrefix(line, "> ") {
			style = e.selectedStyle(style)
		} else if row >= 4 {
			itemIndex := row - 4
			if itemIndex < len(items) {
				item := items[itemIndex]
				if strings.HasPrefix(item.action, "open:") {
					style = e.recentStyle(style, item.recentRank, len(e.recent))
				}
			}
		}
		e.put(x+2, y+1+row, line, style, min(w, x+2+width))
	}
	e.screen.HideCursor()
}

func (e *editor) drawRecent(w, h int) {
	e.recent = loadRecent()
	sections := groupedRecentFiles(e.recent)
	total := 0
	for _, section := range sections {
		total += len(section.entries)
	}
	if total > 0 && e.recentIndex >= total {
		e.recentIndex = 0
	}
	lines, total := e.recentPanelLines(sections, max(24, w-6))
	width := 0
	for _, line := range lines {
		width = max(width, min(runeLen(line.text), max(20, w-6)))
	}
	x, y := max(0, (w-width-4)/2), max(0, (h-len(lines)-2)/2)
	box := tcell.StyleDefault.Background(e.theme.statusBG).Foreground(e.theme.statusFG)
	for row := 0; row < len(lines)+2 && y+row < h; row++ {
		e.put(x, y+row, strings.Repeat(" ", min(w-x, width+4)), box, w)
	}
	for row, line := range lines {
		style := box
		switch line.kind {
		case recentLineSection, recentLineTitle:
			style = style.Bold(true)
		case recentLineEntry:
			style = e.recentStyle(style, line.rank, total)
		}
		e.put(x+2, y+1+row, line.text, style, min(w, x+2+width))
	}
}

func (e *editor) recentPanelLines(sections []recentFileSection, width int) ([]recentPanelLine, int) {
	lines := []recentPanelLine{{text: " Recent Markdown files ", kind: recentLineTitle}}
	total := 0
	for _, section := range sections {
		if len(section.entries) == 0 {
			continue
		}
		lines = append(lines, recentPanelLine{text: " " + section.title + " ", kind: recentLineSection})
		for _, entry := range section.entries {
			prefix := "  "
			if entry.rank == e.recentIndex {
				prefix = "> "
			}
			label := entry.path + " (missing)"
			if !entry.modTime.IsZero() {
				label = recentDisplayLabel(entry.path, width)
			}
			lines = append(lines, recentPanelLine{
				text: prefix + label,
				kind: recentLineEntry,
				rank: entry.rank,
			})
			total++
		}
	}
	if total == 0 {
		lines = append(lines, recentPanelLine{text: "  <No recent files>", kind: recentLineEntry})
	}
	lines = append(lines, recentPanelLine{text: " Up/Down select   Enter open ", kind: recentLineSpacer})
	lines = append(lines, recentPanelLine{text: " Esc cancel ", kind: recentLineSpacer})
	return lines, total
}

type recentPanelLineKind int

const (
	recentLineTitle recentPanelLineKind = iota
	recentLineSection
	recentLineEntry
	recentLineSpacer
)

type recentPanelLine struct {
	text string
	kind recentPanelLineKind
	rank int
}

type recentFileSection struct {
	title   string
	entries []recentFileItem
}

type recentFileItem struct {
	path    string
	modTime time.Time
	rank    int
}

func groupedRecentFiles(paths []string) []recentFileSection {
	sections := []recentFileSection{
		{title: "Past 48 hours"},
		{title: "Past week"},
		{title: "Older"},
		{title: "Missing files"},
	}
	rank := 0
	now := time.Now()
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			sections[3].entries = append(sections[3].entries, recentFileItem{path: path, rank: rank})
			rank++
			continue
		}
		item := recentFileItem{path: path, modTime: info.ModTime(), rank: rank}
		switch {
		case now.Sub(item.modTime) <= 48*time.Hour:
			sections[0].entries = append(sections[0].entries, item)
		case now.Sub(item.modTime) <= 7*24*time.Hour:
			sections[1].entries = append(sections[1].entries, item)
		default:
			sections[2].entries = append(sections[2].entries, item)
		}
		rank++
	}
	return sections
}

func (e *editor) recentStyle(style tcell.Style, rank, total int) tcell.Style {
	if total <= 0 {
		return style
	}
	color := recentGradientColor(rank, total)
	if rank <= 0 {
		return style.Foreground(color).Bold(true)
	}
	return style.Foreground(color)
}

func recentGradientColor(rank, total int) tcell.Color {
	if total <= 1 {
		return tcell.GetColor("#ff6b5f")
	}
	if rank < 0 {
		rank = 0
	}
	if rank >= total {
		rank = total - 1
	}
	start := [3]int{255, 107, 95}
	end := [3]int{90, 168, 255}
	span := total - 1
	r := start[0] + (end[0]-start[0])*rank/span
	g := start[1] + (end[1]-start[1])*rank/span
	b := start[2] + (end[2]-start[2])*rank/span
	return tcell.GetColor(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

func recentDisplayLabel(path string, width int) string {
	ts := ""
	if info, err := os.Stat(path); err == nil {
		ts = info.ModTime().Format("2006-01-02 15:04")
	}
	if ts == "" {
		return path
	}
	label := path + " [" + ts + "]"
	if runeLen(label) <= width {
		return label
	}
	available := width - runeLen(ts) - 3
	if available < 8 {
		available = 8
	}
	clipped := truncatePathMiddle(path, available)
	return clipped + " [" + ts + "]"
}

func truncatePathMiddle(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	if width == 2 {
		return "…" + string(runes[len(runes)-1:])
	}
	keep := width - 1
	front := keep / 2
	back := keep - front
	return string(runes[:front]) + "…" + string(runes[len(runes)-back:])
}

func (e *editor) visualRows(width int) []visualRow {
	rows := []visualRow{}
	for y := 0; y < len(e.lines); y++ {
		line := e.lines[y]
		if level, _, ok := heading(line); ok {
			before, after := headingSpacing(level)
			for i := 0; i < before; i++ {
				rows = appendSpacerRow(rows, y)
			}
			rows = e.appendVisualLineRows(rows, y, line, width)
			for i := 0; i < after; i++ {
				rows = appendSpacerRow(rows, y)
			}
			continue
		}
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
		rows = e.appendVisualLineRows(rows, y, line, width)
	}
	return rows
}

func (e *editor) appendVisualLineRows(rows []visualRow, y int, line string, width int) []visualRow {
	if y != e.y {
		if quote, ok := blockQuote(line); ok {
			line = "│ " + quote
		}
	}
	runes := []rune(line)
	if len(runes) == 0 {
		return append(rows, visualRow{y: y})
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
	return rows
}

func headingSpacing(level int) (int, int) {
	switch level {
	case 1:
		return 1, 1
	case 2:
		return 1, 0
	default:
		return 0, 0
	}
}

func appendSpacerRow(rows []visualRow, y int) []visualRow {
	if len(rows) > 0 && rows[len(rows)-1].start < 0 {
		return rows
	}
	return append(rows, visualRow{y: y, start: -1})
}

func (e *editor) nextVisualRowIndex(rows []visualRow, current, delta int) int {
	if len(rows) == 0 || delta == 0 {
		return current
	}
	target := current
	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}
	for delta > 0 {
		target += step
		if target < 0 {
			return 0
		}
		if target >= len(rows) {
			return len(rows) - 1
		}
		if rows[target].start >= 0 {
			delta--
		}
	}
	for target >= 0 && target < len(rows) && rows[target].start < 0 {
		target += step
	}
	if target < 0 {
		return 0
	}
	if target >= len(rows) {
		return len(rows) - 1
	}
	return target
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
		if rows[i].start < 0 {
			continue
		}
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
				s = e.selectedStyle(s)
			} else if e.positionMatchesSearch(srcX, vr.y) {
				s = e.searchStyle(s)
			}
		} else {
			if e.positionSelected(0, vr.y) {
				s = e.selectedStyle(s)
			}
		}
		e.screen.SetContent(left+i, row, runes[i], nil, s)
	}
}

func (e *editor) drawVisualLine(left, row int, vr visualRow, current bool, width int) {
	if _, _, ok := e.codeFenceBounds(vr.y); ok {
		style := tcell.StyleDefault.Foreground(e.theme.codeFG).Background(e.theme.codeBG)
		style = e.focusStyle(style, current)
		e.putCodeLine(left, row, vr, style, left+width)
		return
	}
	if start, end, ok := e.zopaFenceBounds(vr.y); ok && (e.y < start || e.y > end) {
		style := tcell.StyleDefault.Background(e.theme.background)
		if e.positionSelected(0, vr.y) {
			style = e.selectedStyle(style)
		} else {
			if e.focusMode {
				style = e.focusStyle(style, current)
			} else {
				switch vr.start {
				case 0:
					style = style.Foreground(e.theme.heading3).Bold(true)
				case 1:
					style = style.Foreground(e.theme.accent1)
				case 2:
					style = style.Foreground(e.theme.accent2)
				case 3:
					style = style.Foreground(e.theme.accent3)
				case 4:
					style = style.Foreground(e.theme.accent4)
				}
			}
		}
		e.put(left, row, vr.text, style, left+width)
		return
	}
	if start, end, ok := e.barChartFenceBounds(vr.y); ok && (e.y < start || e.y > end) {
		style := tcell.StyleDefault.Background(e.theme.background)
		if e.positionSelected(0, vr.y) {
			style = e.selectedStyle(style)
		} else {
			if e.focusMode {
				style = e.focusStyle(style, current)
			} else {
				if vr.start == 0 {
					style = style.Foreground(e.theme.heading3).Bold(true)
				} else {
					style = style.Foreground(e.theme.text)
				}
			}
		}
		e.put(left, row, vr.text, style, left+width)
		return
	}
	if isRule(vr.text) && !current {
		style := tcell.StyleDefault.Foreground(e.theme.quote)
		style = e.focusStyle(style, current)
		if e.positionSelected(0, vr.y) {
			style = e.selectedStyle(style)
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
	style = e.focusStyle(style, current)
	// Reconstruct the rendered line exactly as visualRows wrapped it, so that
	// vr.start aligns with the rune offsets emphasis markers use. For
	// non-current block quotes visualRows prepends "│ " to the quote body.
	line := e.lines[vr.y]
	if vr.y != e.y {
		if quote, ok := blockQuote(line); ok {
			line = "│ " + quote
		}
	}
	visStart := vr.start
	visEnd := vr.start + runeLen(vr.text)
	e.putInlineWindow(left, row, line, style, left+width, vr.y, visStart, visEnd)
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
		style = e.focusStyle(style, current)
		ruleText := renderRule(width)
		padding := (width - len([]rune(ruleText))) / 2
		if padding < 0 {
			padding = 0
		}
		e.put(left+padding, row, ruleText, style, left+width)
		return
	}
	style := tcell.StyleDefault.Foreground(e.theme.text).Background(e.theme.background)
	style = e.focusStyle(style, current)
	if line == "" && e.positionSelected(0, y) {
		e.put(left, row, strings.Repeat(" ", width), e.selectedStyle(style), left+width)
		return
	}
	trimmed := strings.TrimSpace(line)
	if e.inTable(y) && !e.cursorInSameTable(y) {
		tableStyle := style
		if !(e.focusMode && !current) {
			tableStyle = tableStyle.Foreground(e.theme.table)
		}
		e.drawTableLine(left, row, y, width, tableStyle)
		return
	}
	if y != e.y {
		if quote, ok := blockQuote(line); ok {
			line = "│ " + quote
			if e.focusMode && !current {
				style = e.focusStyle(style, current)
			} else {
				style = style.Foreground(e.theme.quote)
			}
			trimmed = strings.TrimSpace(line)
		}
	}
	if level, text, ok := heading(line); ok {
		line = text
		if progressText, hasChecklist := e.sectionChecklistProgress(y, level); hasChecklist {
			line = line + "  " + progressText
		}
		if !e.focusMode {
			switch level {
			case 1:
				style = style.Bold(true).Foreground(e.theme.heading1)
			case 2:
				style = style.Bold(true).Foreground(e.theme.heading2)
			default:
				style = style.Bold(true).Foreground(e.theme.heading3)
			}
		}
	}
	if !current && !e.focusMode {
		switch {
		case strings.HasPrefix(trimmed, ">"):
			style = style.Foreground(e.theme.quote)
		case isSeparator(line):
			style = style.Foreground(e.theme.accent1)
		case e.inTable(y):
			style = style.Foreground(e.theme.table)
		}
	}
	e.putInline(left, row, line, style, left+width, y, 0)
}

func (e *editor) putSelected(left, row int, text string, style tcell.Style, width, y, start int) {
	for x, r := range []rune(text) {
		if x >= width {
			break
		}
		s := style
		if e.positionSelected(start+x, y) {
			s = e.selectedStyle(s)
		} else if e.positionMatchesSearch(start+x, y) {
			s = e.searchStyle(s)
		}
		e.screen.SetContent(left+x, row, r, nil, s)
	}
}

func (e *editor) selectedStyle(style tcell.Style) tcell.Style {
	th := e.activeTheme()
	return style.Background(th.selectionBG).Foreground(th.selectionFG).Bold(true)
}

func (e *editor) searchStyle(style tcell.Style) tcell.Style {
	th := e.activeTheme()
	return style.Background(th.searchBG).Foreground(th.searchFG)
}

func (e *editor) inlineCodeStyle(style tcell.Style) tcell.Style {
	th := e.activeTheme()
	return style.Foreground(th.codeFG).Background(th.codeBG)
}

func (e *editor) focusStyle(style tcell.Style, current bool) tcell.Style {
	if !e.focusMode {
		return style
	}
	if current {
		return style.Background(e.theme.focusBG)
	}
	return style.Foreground(e.theme.muted).Background(e.theme.dimBG)
}

func (e *editor) highlightStyle(style tcell.Style) tcell.Style {
	th := e.activeTheme()
	return style.Background(th.highlightBG).Foreground(th.highlightFG)
}

func (e *editor) emphasisStyle(marker string, styled func(tcell.Style) tcell.Style, style tcell.Style) tcell.Style {
	if marker == "==" {
		return e.highlightStyle(style)
	}
	return styled(style)
}

func (e *editor) activeTheme() theme {
	if e.theme.codeFG == tcell.ColorDefault && e.theme.selectionBG == tcell.ColorDefault && e.theme.searchBG == tcell.ColorDefault {
		return themeByName("calm")
	}
	return e.theme
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

func (e *editor) selectionBounds() (int, int, int, int, bool) {
	if !e.selecting || len(e.lines) == 0 {
		return 0, 0, 0, 0, false
	}
	ax, ay, bx, by := e.selX, e.selY, e.x, e.y
	ay = max(0, min(len(e.lines)-1, ay))
	by = max(0, min(len(e.lines)-1, by))
	ax = max(0, min(runeLen(e.lines[ay]), ax))
	bx = max(0, min(runeLen(e.lines[by]), bx))
	if ay > by || (ay == by && ax > bx) {
		ax, ay, bx, by = bx, by, ax, ay
	}
	if ay == by && ax == bx {
		return ax, ay, bx, by, false
	}
	return ax, ay, bx, by, true
}

func (e *editor) positionSelected(x, y int) bool {
	ax, ay, bx, by, ok := e.selectionBounds()
	if !ok {
		return false
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
		openLen, closeLen, _, _, end := emphasisAt(runes, i)
		if openLen > 0 {
			plain = append(plain, runes[i+openLen:end]...)
			i = end + closeLen
			continue
		}
		plain = append(plain, runes[i])
		i++
	}
	return string(plain)
}

func sourceColumnForRenderedColumn(line string, start, renderedColumn int) int {
	runes := []rune(line)
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	renderedColumn = max(0, renderedColumn)
	sourceAtBoundary := start
	displayed := 0
	for i := start; i < len(runes); {
		if runes[i] == '`' {
			if end := closingRune(runes, i+1, '`'); end > i+1 {
				sourceAtBoundary = max(sourceAtBoundary, i+1)
				for idx := i + 1; idx < end; idx++ {
					if displayed == renderedColumn {
						return sourceAtBoundary
					}
					displayed++
					sourceAtBoundary = idx + 1
				}
				i = end + 1
				continue
			}
		}
		openLen, closeLen, _, _, end := emphasisAt(runes, i)
		if openLen > 0 {
			sourceAtBoundary = max(sourceAtBoundary, i+openLen)
			for idx := i + openLen; idx < end; idx++ {
				if displayed == renderedColumn {
					return sourceAtBoundary
				}
				displayed++
				sourceAtBoundary = idx + 1
			}
			i = end + closeLen
			continue
		}
		if displayed == renderedColumn {
			return sourceAtBoundary
		}
		displayed++
		sourceAtBoundary = i + 1
		i++
	}
	return sourceAtBoundary
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
			s = e.selectedStyle(s)
		} else if e.positionMatchesSearch(srcX, y) {
			s = e.searchStyle(s)
		}

		if runes[i] == '`' {
			if end := closingRune(runes, i+1, '`'); end > i+1 {
				codeStyle := e.inlineCodeStyle(s)
				if e.focusMode && y != e.y {
					codeStyle = s
				}
				for idx, r := range runes[i+1 : end] {
					if x >= maxWidth {
						break
					}
					rStyle := codeStyle
					rSrcX := start + i + 1 + idx
					if e.positionSelected(rSrcX, y) {
						rStyle = e.selectedStyle(rStyle)
					} else if e.positionMatchesSearch(rSrcX, y) {
						rStyle = e.searchStyle(rStyle)
					}
					e.screen.SetContent(x, screenY, r, nil, rStyle)
					x++
				}
				i = end + 1
				continue
			}
		}
		marker, closeLen, markerText, styled, end := emphasisAt(runes, i)
		if marker > 0 {
			for idx, r := range runes[i+marker : end] {
				if x >= maxWidth {
					break
				}
				rStyle := e.emphasisStyle(markerText, styled, s)
				if e.focusMode && y != e.y {
					rStyle = s
				}
				rSrcX := start + i + marker + idx
				if e.positionSelected(rSrcX, y) {
					rStyle = e.selectedStyle(rStyle)
				} else if e.positionMatchesSearch(rSrcX, y) {
					rStyle = e.searchStyle(rStyle)
				}
				e.screen.SetContent(x, screenY, r, nil, rStyle)
				x++
			}
			i = end + closeLen
			continue
		}
		e.screen.SetContent(x, screenY, runes[i], nil, s)
		x++
		i++
	}
}

// putInlineWindow renders a visible window [visStart, visEnd) of the full line
// while searching for emphasis markers across the entire line. This lets
// emphasis that straddles a soft-wrap boundary still render on every wrapped
// row instead of leaking its markers as literal text.
func (e *editor) putInlineWindow(x, screenY int, line string, base tcell.Style, maxWidth, y, visStart, visEnd int) {
	runes := []rune(line)
	if visStart < 0 {
		visStart = 0
	}
	if visEnd > len(runes) {
		visEnd = len(runes)
	}
	for i := 0; i < len(runes) && x < maxWidth; {
		// Skip ahead to the visible window.
		if i >= visEnd {
			break
		}

		// Inline code span.
		if runes[i] == '`' {
			if end := closingRune(runes, i+1, '`'); end > i+1 {
				i = end + 1
				continue
			}
		}
		openLen, closeLen, markerText, styled, end := emphasisAt(runes, i)
		if openLen > 0 {
			innerStart := i + openLen
			innerEnd := end
			if innerEnd > visEnd {
				innerEnd = visEnd
			}
			if innerStart < visStart {
				innerStart = visStart
			}
			for idx := innerStart; idx < innerEnd; idx++ {
				if x >= maxWidth {
					break
				}
				s := base
				rStyle := e.emphasisStyle(markerText, styled, s)
				if e.focusMode && y != e.y {
					rStyle = s
				}
				if e.positionSelected(idx, y) {
					rStyle = e.selectedStyle(rStyle)
				} else if e.positionMatchesSearch(idx, y) {
					rStyle = e.searchStyle(rStyle)
				}
				e.screen.SetContent(x, screenY, runes[idx], nil, rStyle)
				x++
			}
			i = end + closeLen
			continue
		}
		// Plain rune inside the visible window only.
		if i >= visStart {
			s := base
			if e.positionSelected(i, y) {
				s = e.selectedStyle(s)
			} else if e.positionMatchesSearch(i, y) {
				s = e.searchStyle(s)
			}
			e.screen.SetContent(x, screenY, runes[i], nil, s)
			x++
		}
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

func emphasisAt(runes []rune, start int) (int, int, string, func(tcell.Style) tcell.Style, int) {
	type markerStyle struct {
		marker string
		style  func(tcell.Style) tcell.Style
	}
	markers := []markerStyle{
		{"**", func(s tcell.Style) tcell.Style { return s.Bold(true) }},
		{"__", func(s tcell.Style) tcell.Style { return s.Bold(true) }},
		{"~~", func(s tcell.Style) tcell.Style { return s.StrikeThrough(true) }},
		{"==", func(s tcell.Style) tcell.Style { return s.Reverse(true) }},
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
				return len(marker), len(marker), candidate.marker, candidate.style, end
			}
		}
	}
	// Asymmetric HTML markers, e.g. <u>…</u>. The open and close tags may
	// differ in length, so they are matched as a distinct prefix/suffix pair.
	type htmlMarker struct {
		open, close string
		style       func(tcell.Style) tcell.Style
	}
	htmlMarkers := []htmlMarker{
		{"<u>", "</u>", func(s tcell.Style) tcell.Style { return s.Underline(true) }},
	}
	for _, candidate := range htmlMarkers {
		open := []rune(candidate.open)
		close := []rune(candidate.close)
		if !runesEqualAt(runes, start, open) {
			continue
		}
		for end := start + len(open); end+len(close) <= len(runes); end++ {
			if runesEqualAt(runes, end, close) {
				return len(open), len(close), candidate.open, candidate.style, end
			}
		}
	}
	return 0, 0, "", nil, 0
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
