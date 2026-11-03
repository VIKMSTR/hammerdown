package ui

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"mkup/internal/editor"
	md "mkup/internal/markdown"
)

//go:embed reference.md
var referenceDoc string

var markdownFilter = storage.NewExtensionFileFilter([]string{".md", ".markdown", ".mdown", ".txt"})

// confirmDiscard runs then() right away if there are no unsaved changes,
// otherwise asks whether to save, discard or cancel.
func (a *App) confirmDiscard(then func()) {
	if !a.dirty() {
		then()
		return
	}
	msg := widget.NewLabel(fmt.Sprintf("Do you want to save the changes made to %q?\nYour changes will be lost if you don't save them.", a.displayName()))
	var d *dialog.CustomDialog
	save := widget.NewButton("Save", func() { d.Hide(); a.save(then) })
	save.Importance = widget.HighImportance
	discard := widget.NewButton("Don't Save", func() { d.Hide(); then() })
	cancel := widget.NewButton("Cancel", func() { d.Hide() })
	d = dialog.NewCustomWithoutButtons("Unsaved changes", msg, a.win)
	d.SetButtons([]fyne.CanvasObject{cancel, discard, save})
	d.Show()
}

func (a *App) setDocument(path, text string) {
	a.path = path
	a.savedText = text
	a.history.Reset(text)
	a.applying = true
	a.entry.setTextAndCursor(text, 0)
	a.applying = false
	a.afterChange()
	a.win.Canvas().Focus(a.entry)
}

func (a *App) newDocument() { a.setDocument("", "") }

func (a *App) startDir() fyne.ListableURI {
	dir := a.baseDir()
	if dir == "" {
		dir = a.fyneApp.Preferences().String(prefLastDir)
	}
	if dir == "" {
		return nil
	}
	l, err := storage.ListerForURI(storage.NewFileURI(dir))
	if err != nil {
		return nil
	}
	return l
}

func (a *App) rememberDir(path string) {
	a.fyneApp.Preferences().SetString(prefLastDir, filepath.Dir(path))
}

func (a *App) openDialog() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if r == nil {
			return
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.rememberDir(r.URI().Path())
		a.setDocument(r.URI().Path(), string(data))
	}, a.win)
	d.SetFilter(markdownFilter)
	if l := a.startDir(); l != nil {
		d.SetLocation(l)
	}
	d.Resize(fyne.NewSize(860, 600))
	d.Show()
}

func (a *App) openPath(path string) {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// Opening a new file name from the command line: start it empty.
		a.setDocument(path, "")
		a.savedText = "\x00" // mark dirty until first save creates the file
		a.updateTitle()
		return
	}
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.rememberDir(path)
	a.setDocument(path, string(data))
}

// save writes to the current path (asking for one if untitled) and calls
// then() on success.
func (a *App) save(then func()) {
	if a.path == "" {
		a.saveAs(then)
		return
	}
	if err := writeFileAtomic(a.path, a.entry.Text); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.savedText = a.entry.Text
	a.updateTitle()
	a.hint("Saved " + a.path)
	if then != nil {
		then()
	}
}

func (a *App) saveAs(then func()) {
	a.chooseSavePath("untitled.md", markdownFilter, ".md", func(path string) {
		a.path = path
		a.rememberDir(path)
		a.save(then)
		a.schedulePreview() // relative images may resolve differently now
	})
}

func (a *App) exportHTML() {
	name := strings.TrimSuffix(a.displayName(), filepath.Ext(a.displayName())) + ".html"
	if a.path == "" {
		name = "untitled.html"
	}
	filter := storage.NewExtensionFileFilter([]string{".html", ".htm"})
	a.chooseSavePath(name, filter, ".html", func(path string) {
		doc := md.Parse(a.entry.Text)
		page := md.RenderHTMLPage(doc, strings.TrimSuffix(a.displayName(), filepath.Ext(a.displayName())))
		if err := writeFileAtomic(path, page); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.hint("Exported " + path)
	})
}

func (a *App) chooseSavePath(name string, filter storage.FileFilter, ext string, done func(path string)) {
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if w == nil {
			return
		}
		path := w.URI().Path()
		_ = w.Close()
		if filepath.Ext(path) == "" {
			// The dialog created the bare name; we write to name+ext instead.
			if st, err := os.Stat(path); err == nil && st.Size() == 0 {
				_ = os.Remove(path)
			}
			path += ext
		}
		done(path)
	}, a.win)
	d.SetFileName(name)
	d.SetFilter(filter)
	if l := a.startDir(); l != nil {
		d.SetLocation(l)
	}
	d.Resize(fyne.NewSize(860, 600))
	d.Show()
}

// writeFileAtomic writes via a temp file and rename so a crash never leaves a
// half-written document; it keeps the original file's permissions.
func writeFileAtomic(path, content string) error {
	mode := os.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// openExternal opens a link: web addresses in the browser, local Markdown
// files in the editor and other local files with the system handler.
func (a *App) openExternal(href string) {
	if strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:") {
		u, err := url.Parse(href)
		if err == nil {
			_ = a.fyneApp.OpenURL(u)
		}
		return
	}
	path := resolvePath(strings.SplitN(href, "#", 2)[0], a.baseDir())
	if _, err := os.Stat(path); err != nil {
		a.hint("File not found: " + path)
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".md" || ext == ".markdown" {
		a.confirmDiscard(func() { a.openPath(path) })
		return
	}
	_ = a.fyneApp.OpenURL(&url.URL{Scheme: "file", Path: path})
}

// showReference opens a window with the syntax reference, source and render
// side by side.
func (a *App) showReference() {
	w := a.fyneApp.NewWindow("Markdown Reference — " + appName)
	src := widget.NewMultiLineEntry()
	src.SetText(referenceDoc)
	src.TextStyle = fyne.TextStyle{Monospace: true}
	src.Wrapping = fyne.TextWrapWord
	r := &previewRenderer{images: a.images, onLink: func(href string) {
		if !strings.HasPrefix(href, "#") {
			a.openExternal(href)
		}
	}}
	preview := container.NewVScroll(container.NewPadded(r.render(md.Parse(referenceDoc))))
	r.onLink = func(href string) {
		if strings.HasPrefix(href, "#") {
			if t := r.anchors[href[1:]]; t != nil {
				d := a.fyneApp.Driver()
				preview.ScrollToOffset(fyne.NewPos(0, d.AbsolutePositionForObject(t).Y-d.AbsolutePositionForObject(preview.Content).Y))
			}
			return
		}
		a.openExternal(href)
	}
	split := container.NewHSplit(src, preview)
	w.SetContent(split)
	w.Resize(fyne.NewSize(1100, 760))
	w.Show()
}

func (a *App) openReferenceDoc() {
	a.setDocument("", referenceDoc)
	a.savedText = referenceDoc
	a.updateTitle()
}

func (a *App) frontMatter() {
	if strings.HasPrefix(a.entry.Text, "---\n") {
		a.hint("The document already starts with front matter")
		return
	}
	fm := "---\ntitle: \ntags: \n---\n\n"
	a.apply(editor.Edit{Text: fm + a.entry.Text, Cursor: len("---\ntitle: ")})
}

func (a *App) about() {
	dialog.ShowInformation("About "+appName,
		appName+" — a Markdown editor with live preview.\n\n"+
			"Supports the blog.sh Markdown subset: headings, emphasis, links,\n"+
			"lists, task lists, quotes, chat, tables, code, images, galleries,\n"+
			"video, audio, attachments, teaser and front matter.", a.win)
}
