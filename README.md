# Hammerdown

A desktop Markdown editor with live preview, written in Go with
[Fyne](https://fyne.io) v2.8.

It implements the Markdown subset from the
[blog.sh cheat sheet](https://github.com/DanielSnor/blog.sh/blob/main/templates/markdown-cheat-sheet.en.md):
paragraphs and hard breaks, headings 1–6, bold / italic / strikethrough /
inline code (nested and adjacent), links with titles and bare URLs, bullet /
numbered / task lists with nesting, blockquotes with attribution, `chat`
dialogues, horizontal rules, the `//--more--//` teaser, fenced code, pipe
tables with alignment (and header-less tables), images with captions,
galleries, video and audio embeds, attachment cards, front matter with a link
card, `::: example` boxes and backslash escaping. The deliberately unsupported
syntax (`_underscores_`, indented code, `===` headings, `>>`, reference links)
renders as written.

## Build & run

Requires Go 1.24+ and a C compiler (Fyne uses cgo; on macOS install the Xcode
command line tools).

```sh
make                   # build ./hammerdown (cgo on, stripped, -trimpath)
make run FILE=notes.md # build and open a file (FILE is optional)
make check             # gofmt check + go vet + tests
make package           # native bundle: hammerdown.app on macOS, .tar.xz on Linux, .exe on Windows
make clean
make help              # list all targets
```

`make package` installs the pinned `fyne` packaging tool into `./bin` on first
use and takes the app name, ID, icon and version from `FyneApp.toml`. Fyne
increments `Build` in that file on every release package.

## Features

- Split view: editor on the left, live preview on the right (toggle with
  View → Show Preview). In-page `#anchor` links scroll the preview; other links
  open in the browser, and links to local `.md` files open in the editor.
- New / Open / Save / Save As, atomic saves, unsaved-changes prompt on close,
  drag-and-drop a file onto the window, Export to standalone HTML.
- Undo / redo covering typing *and* formatting commands (typing is grouped
  into steps).
- Formatting from toolbar, menus, context menu and shortcuts; formatting
  toggles (applying bold to bold text removes it).
- Enter continues lists and quotes; Enter on an empty item ends the list.
- Insert dialogs for links, images, video/audio, attachments and tables.
- Images resolve relative to the document, bare filenames also in `incoming/`;
  remote images and YouTube thumbnails load in the background.
- Help → Markdown Reference shows every supported construct.
- Optional typing speed meter (View → I Show Speed, off by default): an
  animated green-to-red level bar with peak hold in the status bar and the
  current speed in keystrokes ("types") per minute (Backspace and Delete do
  not count), measured over the last five seconds.
- Optional typing sound (View → Typing Sound, off by default): a soft
  vibration-like buzz, a soft mechanical-switch click, bubbles, a clicky
  (Cherry MX Blue) switch, or a typewriter (with a bell on Enter), in
  three volume levels. Space, Enter and Backspace sound slightly different
  from letters; navigation keys and shortcuts are silent. Samples come from
  CC0 recordings on BigSoundBank.com and CC BY / CC BY-SA recordings on
  Wikimedia Commons; see `internal/sound/assets/CREDITS.md`.

## Shortcuts

`Cmd` on macOS, `Ctrl` elsewhere.

| Action | Shortcut | Action | Shortcut |
| --- | --- | --- | --- |
| New / Open / Save | Cmd+N / O / S | Save As | Cmd+Shift+S |
| Export HTML | Cmd+Shift+E | Undo / Redo | Cmd+Z / Cmd+Shift+Z |
| Bold / Italic | Cmd+B / Cmd+I | Strikethrough | Cmd+Shift+X |
| Inline code | Cmd+Shift+C | Code block | Cmd+Shift+K |
| Heading 1–6 / plain | Cmd+1…6 / Cmd+0 | Link | Cmd+K |
| Bulleted / numbered / task list | Cmd+Shift+8 / 7 / 9 | Blockquote | Cmd+' |
| Image | Cmd+Shift+I | Toggle preview | Cmd+Shift+P |

## Layout

```
main.go                  entry point
internal/markdown        parser (AST) + HTML renderer, no GUI deps
internal/editor          undo history and text formatting operations, no GUI deps
internal/sound           typing feedback sounds (embedded CC0 samples, oto audio output)
internal/ui              Fyne app: window, menus, toolbar, preview renderer, dialogs
```
