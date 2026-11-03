package ui

import (
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"mkup/internal/editor"
	md "mkup/internal/markdown"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	a := newApp(test.NewTempApp(t))
	a.showPreview = true
	return a
}

// TestRenderReference renders the built-in reference in the preview and,
// with MKUP_SHOT=path, saves a screenshot for visual inspection.
func TestRenderReference(t *testing.T) {
	a := newTestApp(t)
	switch os.Getenv("MKUP_VARIANT") {
	case "light":
		a.fyneApp.Settings().SetTheme(fixedVariant{newTheme(), theme.VariantLight})
	case "dark":
		a.fyneApp.Settings().SetTheme(fixedVariant{newTheme(), theme.VariantDark})
	}
	src, path := referenceDoc, ""
	if p := os.Getenv("MKUP_DOC"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		src, path = string(b), p
	}
	a.setDocument(path, src)
	a.renderPreview(md.Parse(src))
	if len(a.renderer.anchors) == 0 {
		t.Fatal("no heading anchors rendered")
	}
	a.followLink("#tables") // must not panic, scrolls preview
	if p := os.Getenv("MKUP_SHOT"); p != "" {
		if off := os.Getenv("MKUP_SCROLL"); off != "" {
			a.followLink("#" + off)
		}
		img := a.win.Canvas().Capture()
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
}

// fixedVariant renders a theme in one variant regardless of the system setting.
type fixedVariant struct {
	fyne.Theme
	v fyne.ThemeVariant
}

func (f fixedVariant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return f.Theme.Color(n, f.v)
}

func TestFormattingUndoRedo(t *testing.T) {
	a := newTestApp(t)
	a.setDocument("", "hello world")
	test.Type(a.entry, "!") // cursor at 0 after setDocument
	if a.entry.Text != "!hello world" {
		t.Fatalf("typed: %q", a.entry.Text)
	}

	// Select "hello" with shift+right and make it bold.
	a.entry.setTextAndCursor(a.entry.Text, 1)
	a.entry.KeyDown(&fyne.KeyEvent{Name: "LeftShift"})
	for i := 0; i < 5; i++ {
		a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	}
	a.entry.KeyUp(&fyne.KeyEvent{Name: "LeftShift"})
	if got := a.entry.SelectedText(); got != "hello" {
		t.Fatalf("selection %q", got)
	}
	a.inline("**")()
	if a.entry.Text != "!**hello** world" {
		t.Fatalf("bold: %q", a.entry.Text)
	}
	if a.entry.CursorTextOffset() != 10 {
		t.Errorf("cursor after bold: %d", a.entry.CursorTextOffset())
	}
	if a.entry.SelectedText() != "" {
		t.Errorf("selection should be cleared, got %q", a.entry.SelectedText())
	}

	a.heading(2)()
	if a.entry.Text != "## !**hello** world" {
		t.Fatalf("heading: %q", a.entry.Text)
	}
	a.undo()
	if a.entry.Text != "!**hello** world" {
		t.Fatalf("undo heading: %q", a.entry.Text)
	}
	a.undo()
	if a.entry.Text != "!hello world" {
		t.Fatalf("undo bold: %q", a.entry.Text)
	}
	a.redo()
	if a.entry.Text != "!**hello** world" {
		t.Fatalf("redo: %q", a.entry.Text)
	}
	if !a.dirty() {
		t.Error("document should be dirty")
	}
}

func TestListContinuationOnEnter(t *testing.T) {
	a := newTestApp(t)
	a.setDocument("", "- one")
	a.entry.setTextAndCursor(a.entry.Text, 5)
	a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	test.Type(a.entry, "two")
	if a.entry.Text != "- one\n- two" {
		t.Fatalf("got %q", a.entry.Text)
	}
}

func TestPlaceCursorWithWrap(t *testing.T) {
	a := newTestApp(t)
	long := strings.Repeat("word ", 400) + "\nend"
	a.setDocument("", long)
	a.entry.Wrapping = fyne.TextWrapWord
	a.win.Resize(fyne.NewSize(600, 600))
	for _, off := range []int{0, 7, 1000, 2000, len([]rune(long))} {
		a.entry.setTextAndCursor(long, off)
		if got := a.entry.CursorTextOffset(); got != off {
			t.Errorf("offset %d -> %d", off, got)
		}
	}
}

func TestSaveAndOpen(t *testing.T) {
	a := newTestApp(t)
	path := t.TempDir() + "/doc.md"
	a.setDocument(path, "")
	a.apply(editor.Edit{Text: "# Title\n", Cursor: 8})
	a.save(nil)
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "# Title\n" {
		t.Fatalf("saved %q, %v", b, err)
	}
	if a.dirty() {
		t.Error("clean after save")
	}
	a.openPath(path)
	if a.entry.Text != "# Title\n" || a.history.CanUndo() {
		t.Error("open should load text and reset history")
	}
}
