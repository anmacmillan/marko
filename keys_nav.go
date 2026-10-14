package main

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// dragTick is posted while a mouse drag is held at the top or bottom edge, so
// the selection keeps scrolling even when the pointer stops moving.
type dragTick struct{}

// cmdKey handles Cmd (Super) shortcuts. Ghostty reports Cmd as ModMeta via
// the kitty keyboard protocol for any Cmd combination it has not bound
// itself. It returns true when the event was consumed.
func (e *editor) cmdKey(ev *tcell.EventKey) bool {
	mod := ev.Modifiers()
	if mod&tcell.ModMeta == 0 || mod&(tcell.ModCtrl|tcell.ModAlt) != 0 {
		return false
	}
	switch ev.Key() {
	case tcell.KeyLeft, tcell.KeyRight:
		e.beginKeyboardSelection(mod)
		e.moveLineEdge(ev.Key() == tcell.KeyRight)
		return true
	case tcell.KeyUp, tcell.KeyDown:
		e.beginKeyboardSelection(mod)
		e.moveDocEdge(ev.Key() == tcell.KeyDown)
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.checkpoint()
		e.deleteToLineStart()
		return true
	case tcell.KeyRune:
	default:
		return false
	}
	shift := mod&tcell.ModShift != 0 || unicode.IsUpper(ev.Rune())
	switch unicode.ToLower(ev.Rune()) {
	case 'c':
		e.copySelection()
	case 'x':
		e.cutSelection()
	case 'v':
		e.paste()
	case 'a':
		e.selectAll()
	case 'z':
		if shift {
			e.redoEdit()
		} else {
			e.undoEdit()
		}
	case 'y':
		e.redoEdit()
	case 's':
		if shift || e.untitled || e.path == "" {
			e.openSaveAsPicker()
		} else {
			e.save()
		}
	case 'b':
		e.checkpoint()
		e.toggleEmphasis("**", "**")
	case 'i':
		e.checkpoint()
		e.toggleEmphasis("*", "*")
	case 'u':
		e.checkpoint()
		e.toggleEmphasis("<u>", "</u>")
	default:
		return false
	}
	return true
}

func (e *editor) paste() {
	if data, err := clipboardRead(); err == nil {
		e.insertText(string(data))
	} else {
		e.waitingForPaste = true
		e.screen.GetClipboard()
	}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '\''
}

// moveWord jumps to the start of the previous word or the end of the next
// one, crossing line boundaries the way Option-arrow does in a Mac editor.
func (e *editor) moveWord(delta int) {
	runes := []rune(e.lines[e.y])
	if delta < 0 {
		if e.x == 0 {
			if e.y > 0 {
				e.y--
				e.x = runeLen(e.lines[e.y])
			}
			return
		}
		x := min(e.x, len(runes))
		for x > 0 && !isWordRune(runes[x-1]) {
			x--
		}
		for x > 0 && isWordRune(runes[x-1]) {
			x--
		}
		e.x = x
		return
	}
	if e.x >= len(runes) {
		if e.y+1 < len(e.lines) {
			e.y++
			e.x = 0
		}
		return
	}
	x := e.x
	for x < len(runes) && !isWordRune(runes[x]) {
		x++
	}
	for x < len(runes) && isWordRune(runes[x]) {
		x++
	}
	e.x = x
}

func (e *editor) moveDocEdge(end bool) {
	if end {
		e.y = len(e.lines) - 1
		e.x = runeLen(e.lines[e.y])
	} else {
		e.x, e.y = 0, 0
	}
}

// deleteBackTo deletes from wherever move() leaves the cursor up to the
// current position, reusing the selection machinery.
func (e *editor) deleteBackTo(move func()) {
	if e.selecting {
		e.deleteSelection()
		return
	}
	x, y := e.x, e.y
	move()
	if e.x == x && e.y == y {
		e.backspace()
		return
	}
	e.selX, e.selY = x, y
	e.selecting = true
	e.deleteSelection()
}

func (e *editor) deleteWordBack() {
	e.deleteBackTo(func() { e.moveWord(-1) })
}

func (e *editor) deleteToLineStart() {
	e.deleteBackTo(func() { e.moveLineEdge(false) })
}

// dragScroll continues an edge drag by replaying the held button just past
// the edge, which mouse() already turns into a one-row scroll.
func (e *editor) dragScroll() {
	edge := e.dragEdge.Load()
	if edge == 0 || !e.mouseDown {
		e.dragEdge.Store(0)
		return
	}
	_, h := e.screen.Size()
	y := -1
	if edge > 0 {
		y = h
	}
	e.mouse(tcell.NewEventMouse(e.dragX, y, tcell.Button1, tcell.ModNone))
}
