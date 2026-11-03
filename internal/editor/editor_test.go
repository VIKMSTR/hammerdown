package editor

import (
	"testing"
	"time"
)

func TestHistoryMergesTyping(t *testing.T) {
	now := time.Unix(0, 0)
	h := NewHistory("")
	h.now = func() time.Time { return now }

	h.Record("a", Typing)
	now = now.Add(100 * time.Millisecond)
	h.Record("ab", Typing)
	now = now.Add(2 * time.Second) // pause -> new group
	h.Record("abc", Typing)
	h.Record("**abc**", Command)

	for _, want := range []string{"abc", "ab", ""} {
		got, ok := h.Undo()
		if !ok || got != want {
			t.Fatalf("undo: got %q want %q", got, want)
		}
	}
	if _, ok := h.Undo(); ok {
		t.Fatal("undo past start")
	}
	if got, _ := h.Redo(); got != "ab" {
		t.Fatalf("redo got %q", got)
	}
	h.Record("abX", Typing) // new edit truncates redo
	if h.CanRedo() {
		t.Fatal("redo should be cleared")
	}
	if got, _ := h.Undo(); got != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestHistoryUnchangedIsNoop(t *testing.T) {
	h := NewHistory("x")
	h.Record("x", Command)
	if h.CanUndo() {
		t.Fatal("no-op record created a step")
	}
}

func TestChangeEnd(t *testing.T) {
	if got := ChangeEnd("hello world", "hello brave world"); got != 12 {
		t.Errorf("insert: %d", got)
	}
	if got := ChangeEnd("hello brave world", "hello world"); got != 6 {
		t.Errorf("delete: %d", got)
	}
}

func TestToggleInline(t *testing.T) {
	cases := []struct {
		text   string
		sel    Selection
		marker string
		want   string
		cursor int
	}{
		{"make bold here", Selection{5, 9}, "**", "make **bold** here", 13},
		{"make **bold** here", Selection{5, 13}, "**", "make bold here", 9},
		{"make **bold** here", Selection{7, 11}, "**", "make bold here", 9},
		{"word", Selection{2, 2}, "*", "*word*", 6},
		{"a  b", Selection{2, 2}, "~~", "a ~~~~ b", 4},
		{"x  bold  y", Selection{1, 9}, "**", "x  **bold**  y", 11},
		{"**bold**", Selection{0, 8}, "*", "***bold***", 10},
		{"*it*", Selection{0, 4}, "*", "it", 2},
		{"čau světe", Selection{4, 9}, "`", "čau `světe`", 11},
	}
	for _, c := range cases {
		got := ToggleInline(c.text, c.sel, c.marker)
		if got.Text != c.want || got.Cursor != c.cursor {
			t.Errorf("%q %v %q: got %q@%d want %q@%d", c.text, c.sel, c.marker, got.Text, got.Cursor, c.want, c.cursor)
		}
	}
}

func TestSetHeading(t *testing.T) {
	if got := SetHeading("title", Selection{2, 2}, 2).Text; got != "## title" {
		t.Errorf("got %q", got)
	}
	if got := SetHeading("## title", Selection{2, 2}, 2).Text; got != "title" {
		t.Errorf("toggle off: %q", got)
	}
	if got := SetHeading("# title\nx", Selection{0, 0}, 3).Text; got != "### title\nx" {
		t.Errorf("change level: %q", got)
	}
}

func TestToggleLinePrefix(t *testing.T) {
	text := "one\ntwo\n\nthree"
	got := ToggleLinePrefix(text, Selection{0, len(text)}, Numbered).Text
	if got != "1. one\n2. two\n\n3. three" {
		t.Errorf("numbered: %q", got)
	}
	got = ToggleLinePrefix(got, Selection{0, len(got)}, Bullet).Text
	if got != "- one\n- two\n\n- three" {
		t.Errorf("convert to bullets: %q", got)
	}
	got = ToggleLinePrefix(got, Selection{0, len(got)}, Bullet).Text
	if got != "one\ntwo\n\nthree" {
		t.Errorf("remove bullets: %q", got)
	}
	got = ToggleLinePrefix("a\nb", Selection{0, 3}, Task).Text
	if got != "- [ ] a\n- [ ] b" {
		t.Errorf("task: %q", got)
	}
	got = ToggleLinePrefix("a\n\nb", Selection{0, 4}, Quote).Text
	if got != "> a\n>\n> b" {
		t.Errorf("quote: %q", got)
	}
	got = ToggleLinePrefix(got, Selection{0, len(got)}, Quote).Text
	if got != "a\n\nb" {
		t.Errorf("unquote: %q", got)
	}
	// Selection ending at the start of a line does not include that line.
	got = ToggleLinePrefix("a\nb", Selection{0, 2}, Bullet).Text
	if got != "- a\nb" {
		t.Errorf("line range: %q", got)
	}
	// Indentation is kept for nested items.
	got = ToggleLinePrefix("  nested", Selection{0, 0}, Bullet).Text
	if got != "  - nested" {
		t.Errorf("indent: %q", got)
	}
}

func TestInsertBlock(t *testing.T) {
	e := InsertBlock("before", Selection{6, 6}, "---", -1)
	if e.Text != "before\n\n---\n" || e.Cursor != 11 {
		t.Errorf("%q %d", e.Text, e.Cursor)
	}
	e = InsertBlock("a\n\nb", Selection{2, 2}, "---", -1)
	if e.Text != "a\n\n---\n\nb" {
		t.Errorf("%q", e.Text)
	}
	e = CodeBlock("", Selection{0, 0}, "go")
	if e.Text != "```go\n\n```\n" || e.Cursor != 6 {
		t.Errorf("%q %d", e.Text, e.Cursor)
	}
	e = CodeBlock("x := 1", Selection{0, 6}, "")
	if e.Text != "```\nx := 1\n```\n" {
		t.Errorf("%q", e.Text)
	}
}

func TestMarkupHelpers(t *testing.T) {
	if got := LinkMarkup("a [b]", "https://x", `say "hi"`); got != `[a \[b\]](https://x "say 'hi'")` {
		t.Errorf("%s", got)
	}
	if got := ImageMarkup("cap", "v.mp4", "", true); got != "!![cap](v.mp4)" {
		t.Errorf("%s", got)
	}
	if got := Table(2, 1, false); got != "| --- | --- |\n| Cell 1.1 | Cell 1.2 |" {
		t.Errorf("%s", got)
	}
}

func TestContinueList(t *testing.T) {
	cases := []struct {
		text   string
		cursor int
		want   string
		ok     bool
	}{
		{"- item", 6, "- item\n- ", true},
		{"  * nested", 10, "  * nested\n  * ", true},
		{"9. nine", 7, "9. nine\n10. ", true},
		{"- [x] done", 10, "- [x] done\n- [ ] ", true},
		{"> quoted", 8, "> quoted\n> ", true},
		{"a\n- ", 4, "a\n", true},
		{"plain", 5, "", false},
		{"- item", 1, "", false},
	}
	for _, c := range cases {
		e, ok := ContinueList(c.text, c.cursor)
		if ok != c.ok || (ok && e.Text != c.want) {
			t.Errorf("%q@%d: got %q,%v want %q,%v", c.text, c.cursor, e.Text, ok, c.want, c.ok)
		}
	}
}
