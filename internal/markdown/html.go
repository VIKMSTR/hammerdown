package markdown

import (
	"fmt"
	"html"
	"strings"
)

// RenderHTML renders the document body as an HTML fragment.
func RenderHTML(doc *Document) string {
	var b strings.Builder
	if title := doc.MetaValue("title"); title != "" {
		fmt.Fprintf(&b, "<h1 class=\"title\">%s</h1>\n", esc(title))
	}
	if link := doc.MetaValue("link"); link != "" {
		label := doc.MetaValue("link_title")
		if label == "" {
			label = link
		}
		fmt.Fprintf(&b, "<a class=\"link-card\" href=\"%s\"><strong>%s</strong>", esc(link), esc(label))
		if d := doc.MetaValue("link_description"); d != "" {
			fmt.Fprintf(&b, "<span>%s</span>", esc(d))
		}
		b.WriteString("</a>\n")
	}
	renderBlocks(&b, doc.Blocks)
	return b.String()
}

// RenderHTMLPage renders a complete standalone HTML page with basic styling.
func RenderHTMLPage(doc *Document, title string) string {
	if t := doc.MetaValue("title"); t != "" {
		title = t
	}
	return "<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>" + esc(title) +
		"</title>\n<style>\n" + pageCSS + "</style>\n</head>\n<body>\n<main>\n" +
		RenderHTML(doc) + "</main>\n</body>\n</html>\n"
}

const pageCSS = `body{font-family:system-ui,sans-serif;line-height:1.6;margin:0;padding:2rem;color:#222}
main{max-width:46rem;margin:auto}
pre{background:#f4f4f4;padding:1rem;overflow-x:auto}
code{background:#f4f4f4;padding:0 .2em}
pre code{padding:0;background:none}
blockquote{border-left:4px solid #ccc;margin:1rem 0;padding:.2rem 1rem;color:#444}
blockquote footer{font-style:italic}
.table-wrap{overflow-x:auto}
table{border-collapse:collapse}
td,th{border:1px solid #ccc;padding:.3rem .6rem}
figure{margin:1rem 0}
figure img{max-width:100%}
.gallery{display:grid;grid-template-columns:1fr 1fr;gap:.5rem}
.gallery figure{margin:0}
.gallery figure:last-child:nth-child(odd){grid-column:1/-1}
.gallery img{width:100%;height:14rem;object-fit:cover}
.chat dt{font-weight:bold}
.chat dd{margin:0 0 .5rem 1rem}
.example{border:1px dashed #bbb;padding:0 1rem;margin:1rem 0}
.attachment,.link-card{display:block;border:1px solid #ccc;padding:.6rem 1rem;margin:1rem 0;text-decoration:none}
.link-card span{display:block;color:#555}
.teaser{border:0;border-top:1px dashed #bbb}
li.task{list-style:none}
`

func esc(s string) string { return html.EscapeString(s) }

func renderBlocks(b *strings.Builder, blocks []Block) {
	for _, blk := range blocks {
		renderBlock(b, blk)
	}
}

func renderBlock(b *strings.Builder, blk Block) {
	switch n := blk.(type) {
	case *Paragraph:
		b.WriteString("<p>")
		renderInlines(b, n.Inlines)
		b.WriteString("</p>\n")
	case *Heading:
		fmt.Fprintf(b, "<h%d id=\"%s\">", n.Level, esc(n.ID))
		renderInlines(b, n.Inlines)
		fmt.Fprintf(b, "</h%d>\n", n.Level)
	case *List:
		renderList(b, n)
	case *Blockquote:
		b.WriteString("<blockquote>\n")
		for _, p := range n.Paragraphs {
			b.WriteString("<p>")
			renderInlines(b, p)
			b.WriteString("</p>\n")
		}
		if n.Attribution != nil {
			b.WriteString("<footer>— ")
			renderInlines(b, n.Attribution)
			b.WriteString("</footer>\n")
		}
		b.WriteString("</blockquote>\n")
	case *Chat:
		b.WriteString("<dl class=\"chat\">\n")
		for _, l := range n.Lines {
			fmt.Fprintf(b, "<dt>%s</dt><dd>", esc(l.Speaker))
			renderInlines(b, l.Inlines)
			b.WriteString("</dd>\n")
		}
		b.WriteString("</dl>\n")
	case *HorizontalRule:
		b.WriteString("<hr>\n")
	case *Teaser:
		b.WriteString("<!--more-->\n<hr class=\"teaser\">\n")
	case *CodeBlock:
		if n.Lang != "" {
			fmt.Fprintf(b, "<pre><code class=\"language-%s\">%s</code></pre>\n", esc(n.Lang), esc(n.Text))
		} else {
			fmt.Fprintf(b, "<pre><code>%s</code></pre>\n", esc(n.Text))
		}
	case *Table:
		renderTable(b, n)
	case *Image:
		renderFigure(b, n)
	case *Gallery:
		b.WriteString("<div class=\"gallery\">\n")
		for _, img := range n.Images {
			renderFigure(b, img)
		}
		b.WriteString("</div>\n")
	case *Media:
		renderMedia(b, n)
	case *Attachment:
		fmt.Fprintf(b, "<a class=\"attachment\" href=\"%s\" download>%s</a>\n", esc(n.Href), esc(n.Text))
	case *Container:
		fmt.Fprintf(b, "<div class=\"%s\">\n", esc(n.Name))
		renderBlocks(b, n.Blocks)
		b.WriteString("</div>\n")
	}
}

func renderList(b *strings.Builder, l *List) {
	tag := "ul"
	if l.Ordered {
		tag = "ol"
	}
	fmt.Fprintf(b, "<%s>\n", tag)
	for _, it := range l.Items {
		if it.Task {
			checked := ""
			if it.Checked {
				checked = " checked"
			}
			fmt.Fprintf(b, "<li class=\"task\"><input type=\"checkbox\" disabled%s> ", checked)
		} else {
			b.WriteString("<li>")
		}
		renderInlines(b, it.Inlines)
		for _, c := range it.Children {
			b.WriteString("\n")
			renderList(b, c)
		}
		b.WriteString("</li>\n")
	}
	fmt.Fprintf(b, "</%s>\n", tag)
}

func renderTable(b *strings.Builder, t *Table) {
	cols := t.Columns()
	style := func(c int) string {
		if c >= len(t.Align) {
			return ""
		}
		switch t.Align[c] {
		case AlignLeft:
			return " style=\"text-align:left\""
		case AlignCenter:
			return " style=\"text-align:center\""
		case AlignRight:
			return " style=\"text-align:right\""
		}
		return ""
	}
	row := func(cells [][]Inline, tag string) {
		b.WriteString("<tr>")
		for c := 0; c < cols; c++ {
			fmt.Fprintf(b, "<%s%s>", tag, style(c))
			if c < len(cells) {
				renderInlines(b, cells[c])
			}
			fmt.Fprintf(b, "</%s>", tag)
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("<div class=\"table-wrap\"><table>\n")
	if t.Header != nil {
		b.WriteString("<thead>\n")
		row(t.Header, "th")
		b.WriteString("</thead>\n")
	}
	b.WriteString("<tbody>\n")
	for _, r := range t.Rows {
		row(r, "td")
	}
	b.WriteString("</tbody>\n</table></div>\n")
}

func renderFigure(b *strings.Builder, img *Image) {
	fmt.Fprintf(b, "<figure><img src=\"%s\" alt=\"%s\">", esc(img.Src), esc(img.Alt))
	if img.Title != "" {
		fmt.Fprintf(b, "<figcaption>%s</figcaption>", esc(img.Title))
	}
	b.WriteString("</figure>\n")
}

func renderMedia(b *strings.Builder, m *Media) {
	b.WriteString("<figure class=\"media\">")
	switch {
	case m.Provider == "youtube" && YouTubeID(m.Src) != "":
		fmt.Fprintf(b, "<iframe width=\"560\" height=\"315\" src=\"https://www.youtube-nocookie.com/embed/%s\" title=\"%s\" allowfullscreen></iframe>",
			esc(YouTubeID(m.Src)), esc(m.Caption))
	case m.Provider != "":
		fmt.Fprintf(b, "<a href=\"%s\">%s</a>", esc(m.Src), esc(m.Caption))
	case m.Kind == MediaAudio:
		fmt.Fprintf(b, "<audio controls src=\"%s\"></audio>", esc(m.Src))
	default:
		fmt.Fprintf(b, "<video controls src=\"%s\"></video>", esc(m.Src))
	}
	if m.Caption != "" {
		fmt.Fprintf(b, "<figcaption>%s</figcaption>", esc(m.Caption))
	}
	b.WriteString("</figure>\n")
}

func renderInlines(b *strings.Builder, in []Inline) {
	for _, n := range in {
		switch n := n.(type) {
		case *Text:
			b.WriteString(esc(n.Text))
		case *Strong:
			b.WriteString("<strong>")
			renderInlines(b, n.Children)
			b.WriteString("</strong>")
		case *Emphasis:
			b.WriteString("<em>")
			renderInlines(b, n.Children)
			b.WriteString("</em>")
		case *Strike:
			b.WriteString("<del>")
			renderInlines(b, n.Children)
			b.WriteString("</del>")
		case *Code:
			fmt.Fprintf(b, "<code>%s</code>", esc(n.Text))
		case *Link:
			fmt.Fprintf(b, "<a href=\"%s\"", esc(n.Href))
			if n.Title != "" {
				fmt.Fprintf(b, " title=\"%s\"", esc(n.Title))
			}
			b.WriteString(">")
			renderInlines(b, n.Children)
			b.WriteString("</a>")
		case *LineBreak:
			b.WriteString("<br>\n")
		}
	}
}
