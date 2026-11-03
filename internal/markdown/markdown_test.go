package markdown

import (
	"strings"
	"testing"
)

func toHTML(src string) string {
	return strings.TrimSpace(RenderHTML(Parse(src)))
}

func TestInline(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**bold**, *italic*, ~~strikethrough~~ and `code inside a sentence`",
			"<strong>bold</strong>, <em>italic</em>, <del>strikethrough</del> and <code>code inside a sentence</code>"},
		{"**bold text with *italics* inside**", "<strong>bold text with <em>italics</em> inside</strong>"},
		{"**bold***italic*", "<strong>bold</strong><em>italic</em>"},
		{"*italic***bold**", "<em>italic</em><strong>bold</strong>"},
		{"***both***", "<strong><em>both</em></strong>"},
		{"2 * 3 * 4", "2 * 3 * 4"},
		{"snake_case and file_names", "snake_case and file_names"},
		{"_not italic_", "_not italic_"},
		{"unclosed **bold", "unclosed **bold"},
		{`\*not italics\*, the mask \*.mp4, \` + "`backticks\\`" + ` and \[square brackets\]`,
			"*not italics*, the mask *.mp4, `backticks` and [square brackets]"},
		{`the d8-\ emoticon`, `the d8-\ emoticon`},
		{`back\\slash`, `back\slash`},
		{"[Example](https://example.com)", `<a href="https://example.com">Example</a>`},
		{`[Example with a title](https://example.com "Tooltip on hover")`,
			`<a href="https://example.com" title="Tooltip on hover">Example with a title</a>`},
		{"[**bold** link](/x)", `<a href="/x"><strong>bold</strong> link</a>`},
		{"I write about it at https://example.com regularly.",
			`I write about it at <a href="https://example.com">https://example.com</a> regularly.`},
		{"see https://example.com.", `see <a href="https://example.com">https://example.com</a>.`},
		{"(https://en.wikipedia.org/wiki/Go_(game))",
			`(<a href="https://en.wikipedia.org/wiki/Go_(game)">https://en.wikipedia.org/wiki/Go_(game)</a>)`},
		{"`*literal*`", "<code>*literal*</code>"},
		{"[not a link]", "[not a link]"},
		{"<b>", "&lt;b&gt;"},
	}
	for _, c := range cases {
		got := strings.TrimSuffix(strings.TrimPrefix(toHTML(c.in), "<p>"), "</p>")
		if got != c.want {
			t.Errorf("%q\n got: %s\nwant: %s", c.in, got, c.want)
		}
	}
}

func TestPathologicalEmphasisIsFast(t *testing.T) {
	src := strings.Repeat("*a **b ~~c [d ", 400)
	_ = Parse(src) // must terminate quickly
}

func TestParagraphs(t *testing.T) {
	if got, want := toHTML("First paragraph.\n\nSecond paragraph."), "<p>First paragraph.</p>\n<p>Second paragraph.</p>"; got != want {
		t.Errorf("got %s", got)
	}
	if got, want := toHTML("one\ntwo"), "<p>one two</p>"; got != want {
		t.Errorf("got %s", got)
	}
	if got, want := toHTML("First line \\\nsecond line right under it."), "<p>First line<br>\nsecond line right under it.</p>"; got != want {
		t.Errorf("got %s", got)
	}
}

func TestHeadings(t *testing.T) {
	doc := Parse("# One\n###### Six\n#notaheading\n## The link card\n## The link card")
	if len(doc.Blocks) != 5 {
		t.Fatalf("blocks: %d", len(doc.Blocks))
	}
	h := doc.Blocks[0].(*Heading)
	if h.Level != 1 || h.ID != "one" {
		t.Errorf("%+v", h)
	}
	if doc.Blocks[1].(*Heading).Level != 6 {
		t.Error("level 6")
	}
	if _, ok := doc.Blocks[2].(*Paragraph); !ok {
		t.Error("#notaheading should be a paragraph")
	}
	if id := doc.Blocks[3].(*Heading).ID; id != "the-link-card" {
		t.Errorf("id %q", id)
	}
	if id := doc.Blocks[4].(*Heading).ID; id != "the-link-card-1" {
		t.Errorf("dup id %q", id)
	}
}

func TestLists(t *testing.T) {
	got := toHTML("- first\n- second\n* third")
	if want := "<ul>\n<li>first</li>\n<li>second</li>\n<li>third</li>\n</ul>"; got != want {
		t.Errorf("got %s", got)
	}
	got = toHTML("3. a\n7. b")
	if want := "<ol>\n<li>a</li>\n<li>b</li>\n</ol>"; got != want {
		t.Errorf("got %s", got)
	}
	got = toHTML("- [x] write the post\n- [ ] publish it")
	if !strings.Contains(got, `disabled checked> write the post`) || !strings.Contains(got, `disabled> publish it`) {
		t.Errorf("task list: %s", got)
	}
	doc := Parse("- fruit\n  - apple\n  - pear\n- vegetables\n  1. carrot\n  2. parsley")
	l := doc.Blocks[0].(*List)
	if len(l.Items) != 2 || len(l.Items[0].Children) != 1 || len(l.Items[0].Children[0].Items) != 2 {
		t.Fatalf("nesting: %s", toHTML("- fruit\n  - apple\n  - pear"))
	}
	if !l.Items[1].Children[0].Ordered {
		t.Error("nested ordered list")
	}
	doc = Parse("- a long bullet\n  that wraps\n- next")
	if PlainText(doc.Blocks[0].(*List).Items[0].Inlines) != "a long bullet that wraps" {
		t.Error("continuation line")
	}
	doc = Parse("- a\n\n- b")
	if len(doc.Blocks) != 2 {
		t.Error("blank line should end the list")
	}
}

func TestBlockquote(t *testing.T) {
	doc := Parse("> Begin at the beginning,\n> and go on.\n> — Lewis Carroll")
	q := doc.Blocks[0].(*Blockquote)
	if len(q.Paragraphs) != 1 || PlainText(q.Paragraphs[0]) != "Begin at the beginning, and go on." {
		t.Errorf("%+v", q)
	}
	if PlainText(q.Attribution) != "Lewis Carroll" {
		t.Errorf("attribution %q", PlainText(q.Attribution))
	}
	q = Parse("> quote\n> -- Someone").Blocks[0].(*Blockquote)
	if PlainText(q.Attribution) != "Someone" {
		t.Error("-- attribution")
	}
	q = Parse("> plain").Blocks[0].(*Blockquote)
	if q.Attribution != nil {
		t.Error("no attribution expected")
	}
}

func TestChat(t *testing.T) {
	doc := Parse("```chat\nWatson: What does it mean?\nHolmes: Elementary.\nand obvious\n```")
	c := doc.Blocks[0].(*Chat)
	if len(c.Lines) != 2 || c.Lines[0].Speaker != "Watson" || c.Lines[1].Speaker != "Holmes" {
		t.Fatalf("%+v", c)
	}
	if PlainText(c.Lines[1].Inlines) != "Elementary. and obvious" {
		t.Errorf("continuation %q", PlainText(c.Lines[1].Inlines))
	}
	// Indented fence (as in the cheat sheet's example box).
	doc = Parse("::: example\n\n    ```chat\n    A: hi\n    ```\n\n::: end")
	box := doc.Blocks[0].(*Container)
	if _, ok := box.Blocks[0].(*Chat); !ok {
		t.Errorf("indented chat fence: %T", box.Blocks[0])
	}
}

func TestCodeHRTeaser(t *testing.T) {
	doc := Parse("```ruby\ndef greet(name)\n  puts \"*Hello*\"\nend\n```\n---\n//--more--//\n// -- more -- //")
	cb := doc.Blocks[0].(*CodeBlock)
	if cb.Lang != "ruby" || cb.Text != "def greet(name)\n  puts \"*Hello*\"\nend" {
		t.Errorf("%+v", cb)
	}
	if _, ok := doc.Blocks[1].(*HorizontalRule); !ok {
		t.Error("hr")
	}
	if _, ok := doc.Blocks[2].(*Teaser); !ok {
		t.Error("teaser")
	}
	if _, ok := doc.Blocks[3].(*Paragraph); !ok {
		t.Error("non-exact teaser must be text")
	}
}

func TestTables(t *testing.T) {
	doc := Parse("| Column | Right | Center |\n| --- | ---: | :---: |\n| first row | 6228 | 1 |\n| second row | ~435 | **7 to 9** |")
	tb := doc.Blocks[0].(*Table)
	if len(tb.Header) != 3 || len(tb.Rows) != 2 {
		t.Fatalf("%+v", tb)
	}
	if tb.Align[0] != AlignDefault || tb.Align[1] != AlignRight || tb.Align[2] != AlignCenter {
		t.Errorf("align %v", tb.Align)
	}
	if _, ok := tb.Rows[1][2][0].(*Strong); !ok {
		t.Error("formatting in cells")
	}
	doc = Parse("| --- | --- |\n| Ctrl + c | Copy |\n| Ctrl + v | Paste |")
	tb = doc.Blocks[0].(*Table)
	if tb.Header != nil || len(tb.Rows) != 2 {
		t.Errorf("headerless: %+v", tb)
	}
	doc = Parse("| `a|b` | c\\|d |\n|---|---|")
	tb = doc.Blocks[0].(*Table)
	if PlainText(tb.Header[0]) != "a|b" || PlainText(tb.Header[1]) != "c|d" {
		t.Errorf("pipes: %q %q", PlainText(tb.Header[0]), PlainText(tb.Header[1]))
	}
}

func TestImagesAndGallery(t *testing.T) {
	doc := Parse(`![Alt text](/path/to/photo.jpg "Caption under the photo")`)
	img := doc.Blocks[0].(*Image)
	if img.Alt != "Alt text" || img.Src != "/path/to/photo.jpg" || img.Title != "Caption under the photo" {
		t.Errorf("%+v", img)
	}
	doc = Parse("![First](first.jpg)\n\n![Second](second.jpg)\n\ntext\n\n![Third](third.jpg)")
	if g, ok := doc.Blocks[0].(*Gallery); !ok || len(g.Images) != 2 {
		t.Fatalf("gallery: %T", doc.Blocks[0])
	}
	if _, ok := doc.Blocks[2].(*Image); !ok {
		t.Error("image after text stays single")
	}
}

func TestMedia(t *testing.T) {
	cases := []struct {
		src      string
		kind     MediaKind
		provider string
	}{
		{"/path/to/video.mp4", MediaVideo, ""},
		{"https://www.youtube.com/watch?v=jNQXAC9IVRw", MediaVideo, "youtube"},
		{"https://vimeo.com/76979871", MediaVideo, "vimeo"},
		{"https://framatube.org/w/kkGMgK9ZtnKfYAgnEtQxbv", MediaVideo, "peertube"},
		{"/path/to/recording.mp3", MediaAudio, ""},
		{"https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT", MediaAudio, "spotify"},
		{"https://soundcloud.com/nasa/sputnik-beep", MediaAudio, "soundcloud"},
	}
	for _, c := range cases {
		m := Parse("!![Caption](" + c.src + ")").Blocks[0].(*Media)
		if m.Kind != c.kind || m.Provider != c.provider || m.Caption != "Caption" {
			t.Errorf("%s: %+v", c.src, m)
		}
	}
	if id := YouTubeID("https://www.youtube.com/watch?v=jNQXAC9IVRw"); id != "jNQXAC9IVRw" {
		t.Errorf("yt id %q", id)
	}
	if id := YouTubeID("https://youtu.be/abc"); id != "abc" {
		t.Errorf("yt short id %q", id)
	}
	// A bare YouTube address is just a link.
	if _, ok := Parse("https://www.youtube.com/watch?v=x").Blocks[0].(*Paragraph); !ok {
		t.Error("bare youtube URL must be a paragraph")
	}
}

func TestAttachments(t *testing.T) {
	if a, ok := Parse("[Reading diary 2025](diary.pdf)").Blocks[0].(*Attachment); !ok || a.Href != "diary.pdf" {
		t.Error("attachment")
	}
	if _, ok := Parse("[Remote](https://example.com/diary.pdf)").Blocks[0].(*Paragraph); !ok {
		t.Error("remote link stays a link")
	}
	if _, ok := Parse("[Page](page.html)").Blocks[0].(*Paragraph); !ok {
		t.Error("non-attachment extension stays a link")
	}
}

func TestFrontMatter(t *testing.T) {
	doc := Parse("---\ntags: release\nlink: https://example.com/x\nlink_title: What this is about\n---\n\nAnd here.")
	if doc.MetaValue("tags") != "release" || doc.MetaValue("link_title") != "What this is about" {
		t.Errorf("%+v", doc.Meta)
	}
	if len(doc.Blocks) != 1 {
		t.Errorf("blocks %d", len(doc.Blocks))
	}
	// A leading rule that is not front matter.
	doc = Parse("---\n\nHello world, not meta.\n\n---")
	if len(doc.Meta) != 0 || len(doc.Blocks) != 3 {
		t.Errorf("hr misread as front matter: %+v", doc)
	}
}

func TestContainer(t *testing.T) {
	doc := Parse("::: example\n\n**x**\n\n::: end\n\nafter")
	c, ok := doc.Blocks[0].(*Container)
	if !ok || c.Name != "example" || len(c.Blocks) != 1 || len(doc.Blocks) != 2 {
		t.Fatalf("%#v", doc.Blocks)
	}
}

func TestNotSupported(t *testing.T) {
	// Setext underline is a paragraph followed by a rule.
	doc := Parse("Title\n===")
	if _, ok := doc.Blocks[0].(*Paragraph); !ok {
		t.Error("=== heading must not be supported")
	}
	// Indented code is not code.
	doc = Parse("    not code")
	if _, ok := doc.Blocks[0].(*Paragraph); !ok {
		t.Error("indented code must not be supported")
	}
	// Nested quote stays literal.
	q := Parse(">> nested").Blocks[0].(*Blockquote)
	if PlainText(q.Paragraphs[0]) != "> nested" {
		t.Errorf("nested quote: %q", PlainText(q.Paragraphs[0]))
	}
	// Reference links render as written.
	if got := toHTML("[text][id]"); got != "<p>[text][id]</p>" {
		t.Errorf("reference link: %s", got)
	}
}

func TestListMarkerChange(t *testing.T) {
	doc := Parse("- nested\n  - deeper\n  1. numbered sibling\n- back")
	l := doc.Blocks[0].(*List)
	if len(l.Items) != 2 {
		t.Fatalf("top items: %d", len(l.Items))
	}
	kids := l.Items[0].Children
	if len(kids) != 2 || kids[0].Ordered || !kids[1].Ordered {
		t.Fatalf("expected bullet and numbered sibling lists, got %s", toHTML("- nested\n  - deeper\n  1. numbered sibling"))
	}
	doc = Parse("- a\n1. b")
	if len(doc.Blocks) != 2 {
		t.Errorf("top-level marker change: %d blocks", len(doc.Blocks))
	}
}
