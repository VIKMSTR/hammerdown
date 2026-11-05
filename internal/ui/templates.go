package ui

import (
	"embed"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"

	"hammerdown/internal/editor"
)

//go:embed templates/*.md
var templateFS embed.FS

// docTemplate is a starter document offered under Insert → Template.
type docTemplate struct {
	label string // menu label
	file  string // file name inside templates/
	// cursorAfter places the cursor right after the first occurrence of this
	// text in the inserted template, ready to overwrite the placeholder.
	cursorAfter string
}

var docTemplates = []docTemplate{
	{"AGENTS.md", "agents.md", "## Project overview\n"},
	{"CLAUDE.md", "claude.md", "# CLAUDE.md\n\n"},
	{"Agent Skill (SKILL.md)", "skill.md", "name: "},
}

// text returns the template body without its trailing newline.
func (t docTemplate) text() string {
	b, err := templateFS.ReadFile("templates/" + t.file)
	if err != nil {
		panic(err) // embedded at build time; a miss is a programming error
	}
	return strings.TrimRight(string(b), "\n")
}

// cursor is the rune offset inside text() where the cursor should land.
func (t docTemplate) cursor(text string) int {
	i := strings.Index(text, t.cursorAfter)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(text[:i+len(t.cursorAfter)])
}

// insertTemplate inserts t as a block at the cursor. Templates that start
// with front matter (skills) only work at the very top of a file, so they
// are put there instead, unless the document already has front matter.
func (a *App) insertTemplate(t docTemplate) {
	body := t.text()
	at := t.cursor(body)
	if !strings.HasPrefix(body, "---\n") {
		a.edit(func(s string, sel editor.Selection) editor.Edit { return editor.InsertBlock(s, sel, body, at) })
		return
	}
	if strings.HasPrefix(a.entry.Text, "---\n") {
		a.hint("The document already starts with front matter; " + t.label + " needs it at the top")
		return
	}
	a.apply(editor.InsertBlock(a.entry.Text, editor.Selection{}, body, at))
}

// templateMenuItem builds Insert → Template with one entry per template.
func (a *App) templateMenuItem() *fyne.MenuItem {
	items := make([]*fyne.MenuItem, 0, len(docTemplates))
	for _, t := range docTemplates {
		items = append(items, fyne.NewMenuItem(t.label, func() { a.insertTemplate(t) }))
	}
	it := fyne.NewMenuItem("Template", nil)
	it.ChildMenu = fyne.NewMenu("", items...)
	return it
}
