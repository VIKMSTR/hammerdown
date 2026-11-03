package ui

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"hammerdown/internal/editor"
	md "hammerdown/internal/markdown"
	"hammerdown/internal/sound"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	a := newApp(test.NewTempApp(t))
	a.showPreview = true
	return a
}

// TestRenderReference renders the built-in reference in the preview and,
// with HAMMERDOWN_SHOT=path, saves a screenshot for visual inspection.
func TestRenderReference(t *testing.T) {
	a := newTestApp(t)
	switch os.Getenv("HAMMERDOWN_VARIANT") {
	case "light":
		a.fyneApp.Settings().SetTheme(fixedVariant{newTheme(), theme.VariantLight})
	case "dark":
		a.fyneApp.Settings().SetTheme(fixedVariant{newTheme(), theme.VariantDark})
	}
	src, path := referenceDoc, ""
	if p := os.Getenv("HAMMERDOWN_DOC"); p != "" {
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
	if p := os.Getenv("HAMMERDOWN_SHOT"); p != "" {
		if off := os.Getenv("HAMMERDOWN_SCROLL"); off != "" {
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

// TestTypingSoundHooks checks typing sound is off by default (no audio device
// opened, volume presets disabled) and that keys map to the right sound kind.
func TestTypingSoundHooks(t *testing.T) {
	a := newTestApp(t)
	if a.sounds.Profile() != sound.Off {
		t.Fatalf("default profile %q, want off", a.sounds.Profile())
	}
	if !a.soundMenu[0].Checked || !a.volumeMenu[0].Disabled {
		t.Fatal("menu should show Off ticked and volume disabled")
	}

	var got []sound.Kind
	a.entry.onKeystroke = func(k sound.Kind) { got = append(got, k) }
	test.Type(a.entry, "a ")
	a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
	a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft}) // navigation is silent
	want := []sound.Kind{sound.Key, sound.Space, sound.Return, sound.Delete}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("kinds %v, want %v", got, want)
	}
}

// TestSpeedMeter checks the typing-speed meter is hidden by default, counts
// keystrokes only when shown, animates towards the reading, and goes idle
// after a pause. With HAMMERDOWN_METER_SHOT=path it saves a screenshot.
func TestSpeedMeter(t *testing.T) {
	a := newTestApp(t)
	m := a.speedMeter
	if m.Visible() || a.speedItem.Checked {
		t.Fatal("meter must be hidden by default")
	}
	test.Type(a.entry, "abc")
	if m.speed.PerMinute(time.Now()) != 0 {
		t.Fatal("keystrokes counted while hidden")
	}

	a.toggleSpeed()
	if !m.Visible() || !a.speedItem.Checked {
		t.Fatal("toggle did not show the meter")
	}
	a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
	a.entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDelete})
	if m.speed.PerMinute(time.Now()) != 0 {
		t.Fatal("Backspace/Delete counted as typing speed")
	}
	test.Type(a.entry, "x")
	if m.speed.PerMinute(time.Now()) == 0 {
		t.Fatal("typed character not counted")
	}
	m.speed.Reset()
	m.halt() // drive frames by hand with a fake clock
	clock := time.Unix(1000, 0)
	m.now = func() time.Time { return clock }
	for range 20 { // 6 keys per second → 360 TPM
		m.speed.Record(clock)
		clock = clock.Add(166 * time.Millisecond)
	}
	for range 30 {
		m.tick()
	}
	if m.display < 330 || m.display > 390 || m.peak < m.display {
		t.Fatalf("display %.0f peak %.0f, want ~360", m.display, m.peak)
	}
	if p := os.Getenv("HAMMERDOWN_METER_SHOT"); p != "" {
		a.win.Resize(fyne.NewSize(1000, 300))
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

	clock = clock.Add(10 * time.Second)
	for range 200 {
		m.tick()
	}
	if m.display != 0 || m.peak != 0 || m.running {
		t.Fatalf("after pause: display %.0f peak %.0f running %v", m.display, m.peak, m.running)
	}

	a.toggleSpeed()
	if m.Visible() || a.fyneApp.Preferences().Bool(prefShowSpeed) {
		t.Fatal("toggle did not hide the meter")
	}
}
