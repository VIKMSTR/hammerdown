package ui

import (
	"fmt"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"hammerdown/internal/editor"
	md "hammerdown/internal/markdown"
	"hammerdown/internal/sound"
)

const mod = fyne.KeyModifierShortcutDefault

func key(k fyne.KeyName, extra fyne.KeyModifier) fyne.Shortcut {
	return &desktop.CustomShortcut{KeyName: k, Modifier: mod | extra}
}

func item(label string, sc fyne.Shortcut, action func()) *fyne.MenuItem {
	it := fyne.NewMenuItem(label, action)
	it.Shortcut = sc
	return it
}

// ---- format actions ----

func (a *App) inline(marker string) func() {
	return func() {
		a.edit(func(t string, s editor.Selection) editor.Edit { return editor.ToggleInline(t, s, marker) })
	}
}

func (a *App) heading(level int) func() {
	return func() {
		a.edit(func(t string, s editor.Selection) editor.Edit { return editor.SetHeading(t, s, level) })
	}
}

func (a *App) prefix(kind editor.LinePrefix) func() {
	return func() {
		a.edit(func(t string, s editor.Selection) editor.Edit { return editor.ToggleLinePrefix(t, s, kind) })
	}
}

func (a *App) block(text string, cursorAt int) func() {
	return func() {
		a.edit(func(t string, s editor.Selection) editor.Edit { return editor.InsertBlock(t, s, text, cursorAt) })
	}
}

func (a *App) codeBlock() {
	a.edit(func(t string, s editor.Selection) editor.Edit { return editor.CodeBlock(t, s, "") })
}

func (a *App) chatBlock() {
	a.edit(func(t string, s editor.Selection) editor.Edit {
		return editor.InsertBlock(t, s, "```chat\nAlice: Hello!\nBob: Hi there.\n```", len("```chat\n"))
	})
}

// routeShortcut sends a clipboard/undo shortcut to whatever has focus (a
// dialog entry, a selectable preview label) and falls back to the editor.
func (a *App) routeShortcut(s fyne.Shortcut) {
	if f, ok := a.win.Canvas().Focused().(fyne.Shortcutable); ok && f != fyne.Shortcutable(a.entry) {
		f.TypedShortcut(s)
		return
	}
	a.entry.TypedShortcut(s)
}

func (a *App) mainMenu() *fyne.MainMenu {
	clip := a.fyneApp.Clipboard()

	file := fyne.NewMenu("File",
		item("New", key(fyne.KeyN, 0), func() { a.confirmDiscard(a.newDocument) }),
		item("Open…", key(fyne.KeyO, 0), func() { a.confirmDiscard(a.openDialog) }),
		fyne.NewMenuItemSeparator(),
		item("Save", key(fyne.KeyS, 0), func() { a.save(nil) }),
		item("Save As…", key(fyne.KeyS, fyne.KeyModifierShift), func() { a.saveAs(nil) }),
		item("Export HTML…", key(fyne.KeyE, fyne.KeyModifierShift), a.exportHTML),
		fyne.NewMenuItemSeparator(),
		&fyne.MenuItem{Label: "Quit", IsQuit: true, Action: func() { a.confirmDiscard(a.quit) }},
	)

	edit := fyne.NewMenu("Edit",
		item("Undo", &fyne.ShortcutUndo{}, func() { a.routeShortcut(&fyne.ShortcutUndo{}) }),
		item("Redo", &fyne.ShortcutRedo{}, func() { a.routeShortcut(&fyne.ShortcutRedo{}) }),
		fyne.NewMenuItemSeparator(),
		item("Cut", &fyne.ShortcutCut{Clipboard: clip}, func() { a.routeShortcut(&fyne.ShortcutCut{Clipboard: clip}) }),
		item("Copy", &fyne.ShortcutCopy{Clipboard: clip}, func() { a.routeShortcut(&fyne.ShortcutCopy{Clipboard: clip}) }),
		item("Paste", &fyne.ShortcutPaste{Clipboard: clip}, func() { a.routeShortcut(&fyne.ShortcutPaste{Clipboard: clip}) }),
		item("Select All", &fyne.ShortcutSelectAll{}, func() { a.routeShortcut(&fyne.ShortcutSelectAll{}) }),
	)

	headings := make([]*fyne.MenuItem, 0, 7)
	for l := 1; l <= 6; l++ {
		headings = append(headings, item(fmt.Sprintf("Heading %d", l), key(fyne.KeyName(fmt.Sprint(l)), 0), a.heading(l)))
	}
	headings = append(headings, item("Plain Paragraph", key(fyne.Key0, 0), a.heading(0)))
	headingItem := fyne.NewMenuItem("Heading", nil)
	headingItem.ChildMenu = fyne.NewMenu("", headings...)

	format := fyne.NewMenu("Format",
		item("Bold", key(fyne.KeyB, 0), a.inline("**")),
		item("Italic", key(fyne.KeyI, 0), a.inline("*")),
		item("Strikethrough", key(fyne.KeyX, fyne.KeyModifierShift), a.inline("~~")),
		item("Inline Code", key(fyne.KeyC, fyne.KeyModifierShift), a.inline("`")),
		fyne.NewMenuItemSeparator(),
		headingItem,
		fyne.NewMenuItemSeparator(),
		item("Bulleted List", key(fyne.Key8, fyne.KeyModifierShift), a.prefix(editor.Bullet)),
		item("Numbered List", key(fyne.Key7, fyne.KeyModifierShift), a.prefix(editor.Numbered)),
		item("Task List", key(fyne.Key9, fyne.KeyModifierShift), a.prefix(editor.Task)),
		item("Blockquote", key(fyne.KeyApostrophe, 0), a.prefix(editor.Quote)),
		item("Code Block", key(fyne.KeyK, fyne.KeyModifierShift), a.codeBlock),
	)

	insert := fyne.NewMenu("Insert",
		item("Link…", key(fyne.KeyK, 0), a.linkDialog),
		item("Image…", key(fyne.KeyI, fyne.KeyModifierShift), a.imageDialog),
		fyne.NewMenuItem("Video / Audio…", a.mediaDialog),
		fyne.NewMenuItem("Attachment…", a.attachmentDialog),
		fyne.NewMenuItem("Table…", a.tableDialog),
		fyne.NewMenuItem("Chat", a.chatBlock),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Horizontal Rule", a.block("---", -1)),
		fyne.NewMenuItem("Teaser Break (//--more--//)", a.block(md.TeaserMarker, -1)),
		fyne.NewMenuItem("Front Matter", a.frontMatter),
	)

	a.previewItem = item("Show Preview", key(fyne.KeyP, fyne.KeyModifierShift), a.togglePreview)
	a.previewItem.Checked = a.showPreview
	a.wrapItem = fyne.NewMenuItem("Word Wrap", a.toggleWrap)
	a.wrapItem.Checked = a.entry.Wrapping != fyne.TextWrapOff
	a.speedItem = fyne.NewMenuItem("I Show Speed", a.toggleSpeed)
	a.speedItem.Checked = a.speedMeter.Visible()
	view := fyne.NewMenu("View", a.previewItem, a.wrapItem, fyne.NewMenuItemSeparator(),
		a.speedItem, a.soundMenuItem())

	help := fyne.NewMenu("Help",
		fyne.NewMenuItem("Markdown Reference", a.showReference),
		fyne.NewMenuItem("Open Reference as Document", func() { a.confirmDiscard(a.openReferenceDoc) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("About Hammerdown", a.about),
	)
	return fyne.NewMainMenu(file, edit, format, insert, view, help)
}

// soundVolumes are the volume presets offered in the Typing Sound menu.
var soundVolumes = []struct {
	label string
	value float64
}{{"Quiet", 0.3}, {"Normal", 0.6}, {"Loud", 1}}

// soundMenuItem builds View → Typing Sound: a profile choice (off by
// default) and a volume preset. Picking either plays a preview.
func (a *App) soundMenuItem() *fyne.MenuItem {
	a.soundMenu = a.soundMenu[:0]
	for _, p := range sound.Profiles {
		a.soundMenu = append(a.soundMenu, fyne.NewMenuItem(p.Label(), func() { a.setSoundProfile(p) }))
	}
	a.volumeMenu = a.volumeMenu[:0]
	for _, v := range soundVolumes {
		a.volumeMenu = append(a.volumeMenu, fyne.NewMenuItem("Volume: "+v.label, func() { a.setSoundVolume(v.value) }))
	}
	items := append(append([]*fyne.MenuItem{}, a.soundMenu...), fyne.NewMenuItemSeparator())
	items = append(items, a.volumeMenu...)
	it := fyne.NewMenuItem("Typing Sound", nil)
	it.ChildMenu = fyne.NewMenu("", items...)
	a.syncSoundMenu()
	return it
}

func (a *App) setSoundProfile(p sound.Profile) {
	a.sounds.SetProfile(p)
	a.fyneApp.Preferences().SetString(prefSound, string(p))
	a.syncSoundMenu()
	a.sounds.Preview()
}

func (a *App) setSoundVolume(v float64) {
	a.sounds.SetVolume(v)
	a.fyneApp.Preferences().SetFloat(prefSoundVolume, v)
	a.syncSoundMenu()
	a.sounds.Preview()
}

// syncSoundMenu ticks the current profile and the nearest volume preset;
// the volume presets are disabled while the sound is off.
func (a *App) syncSoundMenu() {
	cur := a.sounds.Profile()
	for i, it := range a.soundMenu {
		it.Checked = sound.Profiles[i] == cur
	}
	vol, best := a.sounds.Volume(), 0
	for i, v := range soundVolumes {
		if math.Abs(v.value-vol) < math.Abs(soundVolumes[best].value-vol) {
			best = i
		}
	}
	for i, it := range a.volumeMenu {
		it.Checked = i == best
		it.Disabled = cur == sound.Off
	}
	if m := a.win.MainMenu(); m != nil {
		m.Refresh()
	}
}

func (a *App) contextMenu() *fyne.Menu {
	clip := a.fyneApp.Clipboard()
	return fyne.NewMenu("",
		fyne.NewMenuItem("Undo", a.undo),
		fyne.NewMenuItem("Redo", a.redo),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Cut", func() { a.entry.TypedShortcut(&fyne.ShortcutCut{Clipboard: clip}) }),
		fyne.NewMenuItem("Copy", func() { a.entry.TypedShortcut(&fyne.ShortcutCopy{Clipboard: clip}) }),
		fyne.NewMenuItem("Paste", func() { a.entry.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clip}) }),
		fyne.NewMenuItem("Select All", func() { a.entry.TypedShortcut(&fyne.ShortcutSelectAll{}) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Bold", a.inline("**")),
		fyne.NewMenuItem("Italic", a.inline("*")),
		fyne.NewMenuItem("Strikethrough", a.inline("~~")),
		fyne.NewMenuItem("Inline Code", a.inline("`")),
		fyne.NewMenuItem("Link…", a.linkDialog),
	)
}

func (a *App) toolbar() fyne.CanvasObject {
	icon := func(res fyne.Resource, hint string, f func()) widget.ToolbarItem {
		return &toolbarWidget{newHoverIcon(res, f, a.hover(hint))}
	}
	text := func(label string, style fyne.TextStyle, hint string, f func()) widget.ToolbarItem {
		return &toolbarWidget{newTextButton(label, style, f, a.hover(hint))}
	}
	sep := widget.NewToolbarSeparator
	mono := fyne.TextStyle{Monospace: true}
	return widget.NewToolbar(
		icon(theme.DocumentCreateIcon(), "New document ("+shortcutText("N", false)+")", func() { a.confirmDiscard(a.newDocument) }),
		icon(theme.FolderOpenIcon(), "Open… ("+shortcutText("O", false)+")", func() { a.confirmDiscard(a.openDialog) }),
		icon(theme.DocumentSaveIcon(), "Save ("+shortcutText("S", false)+")", func() { a.save(nil) }),
		sep(),
		icon(theme.ContentUndoIcon(), "Undo ("+shortcutText("Z", false)+")", a.undo),
		icon(theme.ContentRedoIcon(), "Redo ("+shortcutText("Z", true)+")", a.redo),
		sep(),
		text("B", fyne.TextStyle{Bold: true}, "Bold **text** ("+shortcutText("B", false)+")", a.inline("**")),
		text("I", fyne.TextStyle{Italic: true}, "Italic *text* ("+shortcutText("I", false)+")", a.inline("*")),
		text("S", fyne.TextStyle{Strikethrough: true}, "Strikethrough ~~text~~ ("+shortcutText("X", true)+")", a.inline("~~")),
		text("`c`", mono, "Inline code `text` ("+shortcutText("C", true)+")", a.inline("`")),
		sep(),
		text("H1", fyne.TextStyle{Bold: true}, "Heading 1 ("+shortcutText("1", false)+")", a.heading(1)),
		text("H2", fyne.TextStyle{Bold: true}, "Heading 2 ("+shortcutText("2", false)+")", a.heading(2)),
		text("H3", fyne.TextStyle{Bold: true}, "Heading 3 ("+shortcutText("3", false)+")", a.heading(3)),
		sep(),
		text("•", fyne.TextStyle{Bold: true}, "Bulleted list ("+shortcutText("8", true)+")", a.prefix(editor.Bullet)),
		text("1.", fyne.TextStyle{}, "Numbered list ("+shortcutText("7", true)+")", a.prefix(editor.Numbered)),
		text("[x]", mono, "Task list ("+shortcutText("9", true)+")", a.prefix(editor.Task)),
		text("“ ”", fyne.TextStyle{Bold: true}, "Blockquote ("+shortcutText("'", false)+")", a.prefix(editor.Quote)),
		text("{ }", mono, "Code block ("+shortcutText("K", true)+")", a.codeBlock),
		sep(),
		text("Link", fyne.TextStyle{Underline: true}, "Insert link… ("+shortcutText("K", false)+")", a.linkDialog),
		icon(theme.MediaPhotoIcon(), "Insert image… ("+shortcutText("I", true)+")", a.imageDialog),
		icon(theme.MediaVideoIcon(), "Insert video or audio…", a.mediaDialog),
		icon(theme.GridIcon(), "Insert table…", a.tableDialog),
		text("Chat", fyne.TextStyle{}, "Insert chat dialogue", a.chatBlock),
		text("—", fyne.TextStyle{Bold: true}, "Horizontal rule", a.block("---", -1)),
		text("more", fyne.TextStyle{Italic: true}, "Teaser break //--more--//", a.block(md.TeaserMarker, -1)),
		widget.NewToolbarSpacer(),
		icon(theme.VisibilityIcon(), "Toggle preview ("+shortcutText("P", true)+")", a.togglePreview),
		icon(theme.HelpIcon(), "Markdown reference", a.showReference),
	)
}
