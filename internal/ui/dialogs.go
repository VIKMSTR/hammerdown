package ui

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mkup/internal/editor"
)

func required(what string) fyne.StringValidator {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New(what + " is required")
		}
		return nil
	}
}

func (a *App) showForm(title string, items []*widget.FormItem, onOK func()) {
	d := dialog.NewForm(title, "Insert", "Cancel", items, func(ok bool) {
		if ok {
			onOK()
		} else {
			a.win.Canvas().Focus(a.entry)
		}
	}, a.win)
	d.Resize(fyne.NewSize(520, d.MinSize().Height))
	d.Show()
	if len(items) > 0 {
		if f, ok := items[0].Widget.(fyne.Focusable); ok {
			a.win.Canvas().Focus(f)
		}
	}
}

// selectedText returns the current selection and its text.
func (a *App) selectedText() (editor.Selection, string) {
	sel := a.entry.selection()
	r := []rune(a.entry.Text)
	return sel, string(r[sel.Start:sel.End])
}

func (a *App) linkDialog() {
	sel, selected := a.selectedText()
	text := widget.NewEntry()
	text.SetText(selected)
	text.SetPlaceHolder("Link text")
	href := widget.NewEntry()
	href.SetPlaceHolder("https://example.com or #heading-anchor")
	href.Validator = required("Address")
	if isRemote(selected) {
		href.SetText(selected)
	}
	title := widget.NewEntry()
	title.SetPlaceHolder("Optional tooltip")
	a.showForm("Insert Link", []*widget.FormItem{
		widget.NewFormItem("Text", text),
		widget.NewFormItem("Address", href),
		widget.NewFormItem("Title", title),
	}, func() {
		s := editor.LinkMarkup(text.Text, strings.TrimSpace(href.Text), title.Text)
		a.apply(editor.InsertText(a.entry.Text, sel, s, -1))
	})
}

// pathEntry is an entry with a "Browse…" button that picks a file and stores
// its path relative to the document when possible.
func (a *App) pathEntry(placeholder string, exts []string) (*widget.Entry, fyne.CanvasObject) {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			r.Close()
			e.SetText(a.relPath(r.URI().Path()))
		}, a.win)
		if len(exts) > 0 {
			d.SetFilter(storage.NewExtensionFileFilter(exts))
		}
		if l := a.startDir(); l != nil {
			d.SetLocation(l)
		}
		d.Resize(fyne.NewSize(820, 560))
		d.Show()
	})
	return e, container.NewBorder(nil, nil, nil, browse, e)
}

func (a *App) relPath(path string) string {
	base := a.baseDir()
	if base == "" {
		return path
	}
	if rel, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

func (a *App) imageDialog() {
	sel, selected := a.selectedText()
	src, srcRow := a.pathEntry("photo.jpg, /path/to/photo.png or https://…",
		[]string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"})
	src.Validator = required("Path")
	alt := widget.NewEntry()
	alt.SetText(selected)
	alt.SetPlaceHolder("Description for screen readers")
	caption := widget.NewEntry()
	caption.SetPlaceHolder("Optional caption under the photo")
	a.showForm("Insert Image", []*widget.FormItem{
		widget.NewFormItem("Image", srcRow),
		widget.NewFormItem("Alt text", alt),
		widget.NewFormItem("Caption", caption),
	}, func() {
		s := editor.ImageMarkup(alt.Text, strings.TrimSpace(src.Text), caption.Text, false)
		a.apply(editor.InsertBlock(a.entry.Text, sel, s, -1))
	})
}

func (a *App) mediaDialog() {
	sel, selected := a.selectedText()
	src, srcRow := a.pathEntry("video.mp4, song.mp3, or a YouTube / Vimeo / Spotify … address",
		[]string{".mp4", ".mov", ".m4v", ".mp3", ".m4a", ".ogg", ".opus", ".aac", ".flac", ".wav"})
	src.Validator = required("Source")
	if isRemote(selected) {
		src.SetText(selected)
		selected = ""
	}
	caption := widget.NewEntry()
	caption.SetText(selected)
	caption.SetPlaceHolder("Caption (mandatory)")
	caption.Validator = required("Caption")
	a.showForm("Insert Video / Audio", []*widget.FormItem{
		widget.NewFormItem("Source", srcRow),
		widget.NewFormItem("Caption", caption),
	}, func() {
		s := editor.ImageMarkup(caption.Text, strings.TrimSpace(src.Text), "", true)
		a.apply(editor.InsertBlock(a.entry.Text, sel, s, -1))
	})
}

func (a *App) attachmentDialog() {
	sel, selected := a.selectedText()
	src, srcRow := a.pathEntry("diary.pdf",
		[]string{".pdf", ".zip", ".tgz", ".epub", ".txt", ".md", ".ics", ".gpx", ".csv"})
	src.Validator = required("File")
	text := widget.NewEntry()
	text.SetText(selected)
	text.SetPlaceHolder("Label of the download card")
	a.showForm("Insert Attachment", []*widget.FormItem{
		widget.NewFormItem("File", srcRow),
		widget.NewFormItem("Label", text),
	}, func() {
		label := text.Text
		if label == "" {
			label = filepath.Base(src.Text)
		}
		a.apply(editor.InsertBlock(a.entry.Text, sel, editor.LinkMarkup(label, strings.TrimSpace(src.Text), ""), -1))
	})
}

func (a *App) tableDialog() {
	sel := a.entry.selection()
	sizes := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	cols := widget.NewSelect(sizes, nil)
	cols.SetSelected("3")
	rows := widget.NewSelect(sizes, nil)
	rows.SetSelected("2")
	header := widget.NewCheck("First row is a header", nil)
	header.SetChecked(true)
	a.showForm("Insert Table", []*widget.FormItem{
		widget.NewFormItem("Columns", cols),
		widget.NewFormItem("Rows", rows),
		widget.NewFormItem("", header),
	}, func() {
		c, _ := strconv.Atoi(cols.Selected)
		r, _ := strconv.Atoi(rows.Selected)
		tbl := editor.Table(c, r, header.Checked)
		// Put the cursor on the first cell so the user can start typing over it.
		first := strings.Index(tbl, "| ") + 2
		a.apply(editor.InsertBlock(a.entry.Text, sel, tbl, utf8.RuneCountInString(tbl[:first])))
	})
}
