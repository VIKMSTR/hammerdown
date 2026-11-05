package ui

import (
	"strings"
	"testing"

	md "hammerdown/internal/markdown"
)

func templateByFile(t *testing.T, file string) docTemplate {
	t.Helper()
	for _, tp := range docTemplates {
		if tp.file == file {
			return tp
		}
	}
	t.Fatalf("no template %q", file)
	return docTemplate{}
}

// Every template is embedded, places the cursor and parses into a document
// that starts with a heading (after any front matter).
func TestTemplatesParse(t *testing.T) {
	for _, tp := range docTemplates {
		body := tp.text()
		if body == "" {
			t.Fatalf("%s: empty template", tp.label)
		}
		if tp.cursor(body) < 0 {
			t.Errorf("%s: cursor anchor %q not found", tp.label, tp.cursorAfter)
		}
		doc := md.Parse(body)
		if len(doc.Blocks) == 0 {
			t.Fatalf("%s: no blocks", tp.label)
		}
		if _, ok := doc.Blocks[0].(*md.Heading); !ok {
			t.Errorf("%s: first block is not a heading: %+v", tp.label, doc.Blocks)
		}
	}
	skill := md.Parse(templateByFile(t, "skill.md").text())
	if skill.MetaValue("name") == "" || skill.MetaValue("description") == "" {
		t.Errorf("skill front matter needs name and description: %+v", skill.Meta)
	}
}

func TestInsertTemplateAtCursor(t *testing.T) {
	a := newTestApp(t)
	a.setDocument("", "intro")
	a.entry.setTextAndCursor("intro", len("intro"))
	a.insertTemplate(templateByFile(t, "agents.md"))

	want := "intro\n\n" + templateByFile(t, "agents.md").text() + "\n"
	if a.entry.Text != want {
		t.Fatalf("text:\n%q\nwant:\n%q", a.entry.Text, want)
	}
	cur := a.entry.CursorTextOffset()
	if got := []rune(a.entry.Text)[cur-len("## Project overview\n") : cur]; string(got) != "## Project overview\n" {
		t.Errorf("cursor at %d, preceded by %q", cur, string(got))
	}
	a.undo()
	if a.entry.Text != "intro" {
		t.Errorf("undo: %q", a.entry.Text)
	}
}

// The skill template carries front matter, so it always goes to the top.
func TestInsertSkillTemplateGoesToTop(t *testing.T) {
	a := newTestApp(t)
	a.setDocument("", "existing text")
	a.entry.setTextAndCursor("existing text", len("existing"))
	skill := templateByFile(t, "skill.md")
	a.insertTemplate(skill)

	if !strings.HasPrefix(a.entry.Text, skill.text()+"\n\nexisting text") {
		t.Fatalf("skill not at top: %q", a.entry.Text)
	}
	if md.Parse(a.entry.Text).MetaValue("name") == "" {
		t.Error("front matter not recognised after insert")
	}

	// A second front matter block is refused, leaving the text untouched.
	before := a.entry.Text
	a.insertTemplate(skill)
	if a.entry.Text != before {
		t.Error("second skill template should not be inserted")
	}
}
