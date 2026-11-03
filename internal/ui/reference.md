---
title: Hammerdown Markdown Reference
tags: help, reference
---
Everything this editor understands, in one page. The left side shows the
source, the right side how it renders. Jump to a section:

- [Text](#text)
- [Headings](#headings)
- [Lists](#lists)
- [Quotes and chat](#quotes-and-chat)
- [Code](#code)
- [Tables](#tables)
- [Media](#media)
- [Structure](#structure)
- [Escaping](#escaping)

## Text

Lines next to each other join into one paragraph.
A blank line starts a new one.

End a line with a backslash \
for a hard line break.

**Bold**, *italic*, ~~struck through~~ and `inline code`.
They nest: **bold with *italic* inside**, and touch: **bold***italic*.

A [link](https://example.com), one [with a tooltip](https://example.com "Tooltip"),
and a bare address: https://example.com.

## Headings

### Third level

#### Fourth level

##### Fifth level

###### Sixth level

## Lists

- dash bullet
* star bullet
- a long bullet can wrap onto the next line
  when the continuation is indented by two spaces

1. numbers are
1. renumbered
1. automatically

- [x] a finished task
- [ ] an open task

- nested
  - two spaces deeper
  1. mixed with numbers

## Quotes and chat

> The quote's last line can name its author
> when it starts with an em dash or two hyphens.
> — Somebody Wise

```chat
Ann: Did you try the chat block?
Ben: One line per speaker, name before the colon.
A line without a colon continues the previous one.
```

## Code

```go
func main() {
	fmt.Println("*not* formatted")
}
```

## Tables

| Item | Price | Status |
| :--- | ---: | :---: |
| Coffee | 3.50 | **hot** |
| Tea | 2.80 | ~~gone~~ |

| --- | --- |
| Header-less | tables start |
| with the | separator row |

## Media

An image on its own line, with an optional caption:

![Alt text](photo.jpg "A caption")

Two or more images separated only by blank lines form a gallery.

Video and audio use two exclamation marks; the caption is mandatory:

!![The first video on YouTube](https://www.youtube.com/watch?v=jNQXAC9IVRw)

!![A recording](recording.mp3)

A line that is only a link to a file becomes an attachment card:

[Download the notes](notes.pdf)

## Structure

A line of dashes is a horizontal rule:

---

The teaser marker splits the post; everything above it is the teaser:

//--more--//

Front matter goes at the very top between two `---` lines, with
`key: value` pairs such as `title`, `tags`, `link`, `link_title` and
`link_description`.

## Escaping

\*not italic\*, \`not code\`, \[not a link\] and \~\~not struck\~\~.

The backslash escapes only these seven: `*` `` ` `` `~` `[` `]` `!` `\`.
Before anything else it stays as written: d8-\

Not supported on purpose: _underscore italics_, indented code blocks,
`===` underlined headings, nested `>>` quotes, reference links and footnotes.
