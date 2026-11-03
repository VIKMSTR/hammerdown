// Package markdown implements the Markdown subset described by the blog.sh
// cheat sheet: paragraphs, headings, emphasis, links, lists, blockquotes, chat,
// horizontal rules, teaser, code blocks, tables, images, video, audio,
// attachments, front matter and escaping.
//
// Parsing produces a small AST (Document -> Blocks -> Inlines) that the GUI
// preview and the HTML exporter both render from.
package markdown

// Document is the root of a parsed Markdown text.
type Document struct {
	// Meta holds the key/value pairs from a leading front matter block, in order.
	Meta []MetaField
	// Blocks are the top-level block elements.
	Blocks []Block
}

// MetaField is a single "key: value" line from the front matter.
type MetaField struct {
	Key, Value string
}

// MetaValue returns the value of the first front matter field named key.
func (d *Document) MetaValue(key string) string {
	for _, f := range d.Meta {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// Block is any block-level element.
type Block interface{ isBlock() }

// Paragraph is a run of text lines joined by spaces.
type Paragraph struct{ Inlines []Inline }

// Heading is an ATX heading, level 1-6.
type Heading struct {
	Level   int
	Inlines []Inline
	// ID is the anchor slug, e.g. "the-link-card".
	ID string
}

// List is a bullet or ordered list. Ordered lists are always renumbered from 1.
type List struct {
	Ordered bool
	Items   []*ListItem
}

// ListItem is one entry of a List.
type ListItem struct {
	Task    bool // rendered as a checkbox
	Checked bool
	Inlines []Inline
	// Children holds nested lists.
	Children []*List
}

// Blockquote is a quotation, optionally with an attribution line.
type Blockquote struct {
	Paragraphs  [][]Inline
	Attribution []Inline // nil when there is none
}

// Chat is a dialogue written in a ```chat fence.
type Chat struct{ Lines []ChatLine }

// ChatLine is one statement of a Chat.
type ChatLine struct {
	Speaker string
	Inlines []Inline
}

// HorizontalRule is a line of three or more dashes.
type HorizontalRule struct{}

// Teaser marks the //--more--// split point.
type Teaser struct{}

// CodeBlock is a fenced code block. Lang is cosmetic only.
type CodeBlock struct {
	Lang string
	Text string
}

// Align is a table column alignment.
type Align int

// Column alignments.
const (
	AlignDefault Align = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// Table is a pipe table. Header is nil for a header-less table.
type Table struct {
	Header [][]Inline
	Align  []Align
	Rows   [][][]Inline
}

// Columns returns the number of columns of the widest row.
func (t *Table) Columns() int {
	n := len(t.Header)
	if len(t.Align) > n {
		n = len(t.Align)
	}
	for _, r := range t.Rows {
		if len(r) > n {
			n = len(r)
		}
	}
	return n
}

// Image is a block-level picture.
type Image struct {
	Alt, Src, Title string
}

// Gallery is two or more images separated only by blank lines.
type Gallery struct{ Images []*Image }

// MediaKind distinguishes video from audio embeds.
type MediaKind int

// Media kinds.
const (
	MediaVideo MediaKind = iota
	MediaAudio
)

// Media is a video or audio embed written as !![caption](src).
type Media struct {
	Kind    MediaKind
	Caption string
	Src     string
	// Provider is a known hosting service ("youtube", "vimeo", "spotify", ...)
	// or "" for a plain file.
	Provider string
}

// Attachment is a line that is only a link to a downloadable file.
type Attachment struct {
	Text, Href string
}

// Container is a fenced "::: name" ... "::: end" section (e.g. "::: example").
type Container struct {
	Name   string
	Blocks []Block
}

func (*Paragraph) isBlock()      {}
func (*Heading) isBlock()        {}
func (*List) isBlock()           {}
func (*Blockquote) isBlock()     {}
func (*Chat) isBlock()           {}
func (*HorizontalRule) isBlock() {}
func (*Teaser) isBlock()         {}
func (*CodeBlock) isBlock()      {}
func (*Table) isBlock()          {}
func (*Image) isBlock()          {}
func (*Gallery) isBlock()        {}
func (*Media) isBlock()          {}
func (*Attachment) isBlock()     {}
func (*Container) isBlock()      {}

// Inline is any inline element.
type Inline interface{ isInline() }

// Text is literal text.
type Text struct{ Text string }

// Strong is **bold** text.
type Strong struct{ Children []Inline }

// Emphasis is *italic* text.
type Emphasis struct{ Children []Inline }

// Strike is ~~strikethrough~~ text.
type Strike struct{ Children []Inline }

// Code is `inline code`.
type Code struct{ Text string }

// Link is [text](href "title") or a bare URL.
type Link struct {
	Href, Title string
	Children    []Inline
}

// LineBreak is a hard line break (a line ending with a backslash).
type LineBreak struct{}

func (*Text) isInline()      {}
func (*Strong) isInline()    {}
func (*Emphasis) isInline()  {}
func (*Strike) isInline()    {}
func (*Code) isInline()      {}
func (*Link) isInline()      {}
func (*LineBreak) isInline() {}

// PlainText flattens inlines into their visible text.
func PlainText(in []Inline) string {
	var b []byte
	var walk func([]Inline)
	walk = func(in []Inline) {
		for _, n := range in {
			switch n := n.(type) {
			case *Text:
				b = append(b, n.Text...)
			case *Code:
				b = append(b, n.Text...)
			case *Strong:
				walk(n.Children)
			case *Emphasis:
				walk(n.Children)
			case *Strike:
				walk(n.Children)
			case *Link:
				walk(n.Children)
			case *LineBreak:
				b = append(b, ' ')
			}
		}
	}
	walk(in)
	return string(b)
}
