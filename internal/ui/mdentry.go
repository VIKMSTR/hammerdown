package ui

import (
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"mkup/internal/editor"
)

// mdEntry is a multi-line Entry that routes undo/redo to the app's own
// history (so formatting commands are undoable too), auto-continues lists on
// Enter and offers a context menu wired to the same actions.
type mdEntry struct {
	widget.Entry
	onUndo, onRedo func()
	onReturn       func() bool // returns true if it handled the key
	onPaste        func(apply func())
	contextMenu    func() *fyne.Menu
}

func newMDEntry() *mdEntry {
	e := &mdEntry{}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.ExtendBaseWidget(e)
	return e
}

func (e *mdEntry) TypedShortcut(s fyne.Shortcut) {
	switch s.(type) {
	case *fyne.ShortcutUndo:
		if e.onUndo != nil {
			e.onUndo()
		}
		return
	case *fyne.ShortcutRedo:
		if e.onRedo != nil {
			e.onRedo()
		}
		return
	case *fyne.ShortcutPaste, *fyne.ShortcutCut:
		if e.onPaste != nil {
			e.onPaste(func() { e.Entry.TypedShortcut(s) })
			return
		}
	}
	e.Entry.TypedShortcut(s)
}

func (e *mdEntry) TypedKey(k *fyne.KeyEvent) {
	if (k.Name == fyne.KeyReturn || k.Name == fyne.KeyEnter) && e.SelectedText() == "" &&
		e.onReturn != nil && e.onReturn() {
		return
	}
	e.Entry.TypedKey(k)
}

func (e *mdEntry) TappedSecondary(pe *fyne.PointEvent) {
	if e.contextMenu == nil {
		e.Entry.TappedSecondary(pe)
		return
	}
	c := fyne.CurrentApp().Driver().CanvasForObject(e)
	if c == nil {
		return
	}
	widget.ShowPopUpMenuAtPosition(e.contextMenu(), c, pe.AbsolutePosition)
}

// selection returns the current selection as rune offsets. Fyne does not
// expose the selection anchor, so a non-empty selection is collapsed to its
// start (a Left key press does exactly that) to learn where it begins.
func (e *mdEntry) selection() editor.Selection {
	sel := e.SelectedText()
	if sel == "" {
		off := e.CursorTextOffset()
		return editor.Selection{Start: off, End: off}
	}
	e.Entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	start := e.CursorTextOffset()
	return editor.Selection{Start: start, End: start + utf8.RuneCountInString(sel)}
}

// setTextAndCursor replaces the whole text and moves the cursor to the given
// rune offset.
func (e *mdEntry) setTextAndCursor(text string, offset int) {
	e.SetText(text)
	e.placeCursor(offset)
	// SetText syncs the internal selection state with CursorRow/CursorColumn;
	// with unchanged text it is cheap and fires no change callback.
	e.SetText(text)
}

// placeCursor sets CursorRow/CursorColumn for a rune offset. With word wrap
// on, rows are visual rows, so it walks forward from the logical line (a
// lower bound) using CursorTextOffset to find the visual row.
func (e *mdEntry) placeCursor(offset int) {
	line, col := 0, 0
	i := 0
	for _, r := range e.Text {
		if i == offset {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		i++
	}
	if e.Wrapping == fyne.TextWrapOff {
		e.CursorRow, e.CursorColumn = line, col
		return
	}
	row := line
	e.CursorRow, e.CursorColumn = row, 0
	base := e.CursorTextOffset()
	for {
		e.CursorRow, e.CursorColumn = row+1, 0
		next := e.CursorTextOffset()
		if next <= base || next > offset {
			break
		}
		row, base = row+1, next
	}
	e.CursorRow, e.CursorColumn = row, offset-base
}

// lineCol returns the 1-based logical line and column of the cursor.
func (e *mdEntry) lineCol() (int, int) {
	offset := e.CursorTextOffset()
	line, col, i := 1, 1, 0
	for _, r := range e.Text {
		if i == offset {
			break
		}
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
		i++
	}
	return line, col
}
