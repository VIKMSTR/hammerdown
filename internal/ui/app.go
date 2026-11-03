// Package ui is the Fyne front end of the mkup Markdown editor.
package ui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mkup/internal/editor"
	md "mkup/internal/markdown"
)

const (
	appID        = "io.github.mkup"
	appName      = "mkup"
	previewDelay = 150 * time.Millisecond

	prefWrap        = "wordWrap"
	prefShowPreview = "showPreview"
	prefSplit       = "splitOffset"
	prefLastDir     = "lastDir"
)

// App is the editor application state.
type App struct {
	fyneApp fyne.App
	win     fyne.Window

	entry    *mdEntry
	history  *editor.History
	applying bool // true while the app itself changes the entry text

	path      string // current file, "" for an untitled document
	savedText string

	images        *imageLoader
	renderer      *previewRenderer
	previewScroll *container.Scroll
	previewTimer  *time.Timer
	previewGen    int

	body        *fyne.Container // holds either the split or the editor alone
	split       *container.Split
	editorPane  fyne.CanvasObject
	showPreview bool

	posLabel, statsLabel, hintLabel *widget.Label
	status                          *fyne.Container
	wrapItem, previewItem           *fyne.MenuItem
}

// Run starts the application, opening the file given as the first argument.
func Run(args []string) {
	a := newApp(app.NewWithID(appID))
	a.win.CenterOnScreen()
	if len(args) > 0 {
		a.openPath(args[0])
	}
	a.updateTitle()
	a.schedulePreview()
	a.win.ShowAndRun()
}

func newApp(fa fyne.App) *App {
	a := &App{fyneApp: fa}
	fa.Settings().SetTheme(newTheme())
	a.win = fa.NewWindow(appName)
	a.build()
	a.win.Resize(fyne.NewSize(1280, 820))
	a.win.Canvas().Focus(a.entry)
	return a
}

func (a *App) build() {
	prefs := a.fyneApp.Preferences()
	a.history = editor.NewHistory("")
	a.images = newImageLoader()
	a.images.onUpdate = a.schedulePreview
	a.renderer = &previewRenderer{images: a.images, onLink: a.followLink}

	a.posLabel = statusLabel()
	a.statsLabel = statusLabel()
	a.hintLabel = statusLabel()
	a.hintLabel.Truncation = fyne.TextTruncateEllipsis

	a.entry = newMDEntry()
	a.entry.SetPlaceHolder("# Start writing Markdown…")
	if !prefs.BoolWithFallback(prefWrap, true) {
		a.entry.Wrapping = fyne.TextWrapOff
	}
	a.entry.OnChanged = a.onTextChanged
	a.entry.OnCursorChanged = a.updatePosition
	a.entry.onUndo = a.undo
	a.entry.onRedo = a.redo
	a.entry.onReturn = a.continueList
	a.entry.onPaste = func(apply func()) {
		a.history.Break()
		apply()
		a.history.Break()
	}
	a.entry.contextMenu = a.contextMenu

	a.previewScroll = container.NewVScroll(container.NewPadded(widget.NewLabel("")))
	a.editorPane = a.entry
	a.split = container.NewHSplit(a.editorPane, a.previewScroll)
	a.split.Offset = prefs.FloatWithFallback(prefSplit, 0.5)
	a.showPreview = prefs.BoolWithFallback(prefShowPreview, true)
	a.body = container.NewStack()
	a.applyLayout()

	a.status = container.NewBorder(nil, nil, a.posLabel, a.statsLabel, a.hintLabel)

	top := container.NewVBox(a.toolbar(), widget.NewSeparator())
	bottom := container.NewVBox(widget.NewSeparator(), a.status)
	a.win.SetContent(container.NewBorder(top, bottom, nil, nil, a.body))
	a.win.SetMainMenu(a.mainMenu())
	a.win.SetCloseIntercept(func() { a.confirmDiscard(a.quit) })
	a.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 && uris[0].Scheme() == "file" {
			a.confirmDiscard(func() { a.openPath(uris[0].Path()) })
		}
	})
	a.updatePosition()
}

func statusLabel() *widget.Label {
	l := widget.NewLabel("")
	l.SizeName = theme.SizeNameCaptionText
	l.Importance = widget.LowImportance
	return l
}

func (a *App) quit() {
	if a.split != nil {
		a.fyneApp.Preferences().SetFloat(prefSplit, a.split.Offset)
	}
	a.fyneApp.Quit()
}

// ---- text changes, history, preview ----

func (a *App) onTextChanged(text string) {
	if a.applying {
		return
	}
	a.history.Record(text, editor.Typing)
	a.afterChange()
}

func (a *App) afterChange() {
	a.updateTitle()
	a.updatePosition()
	a.schedulePreview()
}

// apply replaces the text with the result of a command as one undo step.
func (a *App) apply(e editor.Edit) {
	a.applying = true
	a.entry.setTextAndCursor(e.Text, e.Cursor)
	a.applying = false
	a.history.Record(e.Text, editor.Command)
	a.afterChange()
	a.win.Canvas().Focus(a.entry)
}

// edit runs a formatting function against the current text and selection.
func (a *App) edit(fn func(text string, sel editor.Selection) editor.Edit) {
	sel := a.entry.selection()
	a.apply(fn(a.entry.Text, sel))
}

func (a *App) restore(text string) {
	cursor := editor.ChangeEnd(a.entry.Text, text)
	a.applying = true
	a.entry.setTextAndCursor(text, cursor)
	a.applying = false
	a.afterChange()
	a.win.Canvas().Focus(a.entry)
}

func (a *App) undo() {
	if text, ok := a.history.Undo(); ok {
		a.restore(text)
	}
}

func (a *App) redo() {
	if text, ok := a.history.Redo(); ok {
		a.restore(text)
	}
}

func (a *App) continueList() bool {
	e, ok := editor.ContinueList(a.entry.Text, a.entry.CursorTextOffset())
	if !ok {
		return false
	}
	a.history.Break()
	a.applying = true
	a.entry.setTextAndCursor(e.Text, e.Cursor)
	a.applying = false
	a.history.Record(e.Text, editor.Typing)
	a.afterChange()
	return true
}

func (a *App) schedulePreview() {
	if !a.showPreview {
		return
	}
	a.previewGen++
	gen, text := a.previewGen, a.entry.Text
	if a.previewTimer != nil {
		a.previewTimer.Stop()
	}
	a.previewTimer = time.AfterFunc(previewDelay, func() {
		doc := md.Parse(text)
		fyne.Do(func() {
			if gen == a.previewGen {
				a.renderPreview(doc)
			}
		})
	})
}

func (a *App) renderPreview(doc *md.Document) {
	offset := a.previewScroll.Offset
	a.renderer.baseDir = a.baseDir()
	a.previewScroll.Content = container.NewPadded(a.renderer.render(doc))
	a.previewScroll.Refresh()
	a.previewScroll.ScrollToOffset(offset)
}

func (a *App) baseDir() string {
	if a.path == "" {
		return ""
	}
	return filepath.Dir(a.path)
}

// followLink scrolls to in-page anchors and opens everything else externally.
func (a *App) followLink(href string) {
	if strings.HasPrefix(href, "#") {
		target := a.renderer.anchors[strings.TrimPrefix(href, "#")]
		if target == nil {
			a.hint("No heading with anchor " + href)
			return
		}
		d := a.fyneApp.Driver()
		y := d.AbsolutePositionForObject(target).Y - d.AbsolutePositionForObject(a.previewScroll.Content).Y
		a.previewScroll.ScrollToOffset(fyne.NewPos(0, y))
		return
	}
	a.openExternal(href)
}

// ---- layout & status ----

func (a *App) applyLayout() {
	if a.showPreview {
		a.split.Leading = a.editorPane
		a.body.Objects = []fyne.CanvasObject{a.split}
	} else {
		a.body.Objects = []fyne.CanvasObject{a.editorPane}
	}
	a.split.Refresh()
	a.body.Refresh()
}

func (a *App) togglePreview() {
	a.showPreview = !a.showPreview
	a.fyneApp.Preferences().SetBool(prefShowPreview, a.showPreview)
	if a.previewItem != nil {
		a.previewItem.Checked = a.showPreview
		a.win.MainMenu().Refresh()
	}
	a.applyLayout()
	a.schedulePreview()
}

func (a *App) toggleWrap() {
	off := a.entry.CursorTextOffset()
	if a.entry.Wrapping == fyne.TextWrapOff {
		a.entry.Wrapping = fyne.TextWrapWord
	} else {
		a.entry.Wrapping = fyne.TextWrapOff
	}
	wrap := a.entry.Wrapping != fyne.TextWrapOff
	a.fyneApp.Preferences().SetBool(prefWrap, wrap)
	if a.wrapItem != nil {
		a.wrapItem.Checked = wrap
		a.win.MainMenu().Refresh()
	}
	a.entry.Refresh()
	a.applying = true
	a.entry.setTextAndCursor(a.entry.Text, off)
	a.applying = false
}

func (a *App) dirty() bool { return a.entry.Text != a.savedText }

func (a *App) displayName() string {
	if a.path == "" {
		return "Untitled"
	}
	return filepath.Base(a.path)
}

func (a *App) updateTitle() {
	t := a.displayName() + " — " + appName
	if a.dirty() {
		t = "● " + t
	}
	a.win.SetTitle(t)
	a.hintLabel.SetText(a.fileHint())
}

func (a *App) updatePosition() {
	line, col := a.entry.lineCol()
	a.posLabel.SetText(fmt.Sprintf("Ln %d, Col %d", line, col))
	words, chars := editor.Stats(a.entry.Text)
	a.statsLabel.SetText(fmt.Sprintf("%d words · %d characters", words, chars))
	if a.status != nil {
		a.status.Refresh() // labels changed width; re-run the border layout
	}
}

func (a *App) fileHint() string {
	if a.path == "" {
		return "Untitled document"
	}
	return a.path
}

// hint shows a transient message in the status bar.
func (a *App) hint(msg string) {
	a.hintLabel.SetText(msg)
}

// hover returns a callback for toolbar hover hints.
func (a *App) hover(msg string) func(bool) {
	return func(in bool) {
		if in {
			a.hintLabel.SetText(msg)
		} else {
			a.hintLabel.SetText(a.fileHint())
		}
	}
}

// shortcutText renders a shortcut for hints in the platform's convention.
func shortcutText(key string, shift bool) string {
	mod := "Ctrl+"
	if runtime.GOOS == "darwin" {
		mod = "Cmd+"
	}
	if shift {
		mod += "Shift+"
	}
	return mod + key
}
