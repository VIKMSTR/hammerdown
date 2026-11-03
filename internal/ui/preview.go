package ui

import (
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	md "mkup/internal/markdown"
)

// previewRenderer turns a parsed document into Fyne canvas objects.
type previewRenderer struct {
	baseDir string
	images  *imageLoader
	// onLink handles taps on links; anchors ("#id") are handled by the caller.
	onLink func(href string)
	// anchors maps heading IDs to their rendered objects for in-page links.
	anchors map[string]fyne.CanvasObject
}

func (r *previewRenderer) render(doc *md.Document) fyne.CanvasObject {
	r.anchors = map[string]fyne.CanvasObject{}
	var objs []fyne.CanvasObject
	if fm := r.frontMatter(doc); fm != nil {
		objs = append(objs, fm)
	}
	objs = append(objs, r.blocks(doc.Blocks)...)
	if len(objs) == 0 {
		hint := widget.NewLabel("Nothing to preview yet — start typing on the left.")
		hint.Importance = widget.LowImportance
		objs = append(objs, hint)
	}
	return container.NewVBox(objs...)
}

func (r *previewRenderer) blocks(bs []md.Block) []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(bs))
	for _, b := range bs {
		if o := r.block(b); o != nil {
			out = append(out, o)
		}
	}
	return out
}

func (r *previewRenderer) block(b md.Block) fyne.CanvasObject {
	switch n := b.(type) {
	case *md.Paragraph:
		return r.richText(r.inlines(n.Inlines, inlineStyle{}))
	case *md.Heading:
		return r.heading(n)
	case *md.List:
		return r.list(n, 0)
	case *md.Blockquote:
		return r.quote(n)
	case *md.Chat:
		return r.chat(n)
	case *md.HorizontalRule:
		return container.NewPadded(widget.NewSeparator())
	case *md.Teaser:
		return r.teaser()
	case *md.CodeBlock:
		return r.codeBlock(n)
	case *md.Table:
		return r.table(n)
	case *md.Image:
		return r.figure(n, 360, canvas.ImageFillContain)
	case *md.Gallery:
		return r.gallery(n)
	case *md.Media:
		return r.media(n)
	case *md.Attachment:
		return r.attachment(n)
	case *md.Container:
		return r.container(n)
	}
	return nil
}

// ---- inline content ----

type inlineStyle struct {
	text  fyne.TextStyle
	size  fyne.ThemeSizeName
	color fyne.ThemeColorName
}

func (r *previewRenderer) inlines(in []md.Inline, st inlineStyle) []widget.RichTextSegment {
	var segs []widget.RichTextSegment
	for _, n := range in {
		switch n := n.(type) {
		case *md.Text:
			segs = append(segs, textSeg(n.Text, st))
		case *md.Strong:
			s := st
			s.text.Bold = true
			segs = append(segs, r.inlines(n.Children, s)...)
		case *md.Emphasis:
			s := st
			s.text.Italic = true
			segs = append(segs, r.inlines(n.Children, s)...)
		case *md.Strike:
			s := st
			s.text.Strikethrough = true
			segs = append(segs, r.inlines(n.Children, s)...)
		case *md.Code:
			s := st
			s.text.Monospace = true
			s.color = colorCode
			segs = append(segs, textSeg(n.Text, s))
		case *md.Link:
			segs = append(segs, r.linkSeg(n, st))
		case *md.LineBreak:
			segs = append(segs, textSeg("\n", st))
		}
	}
	return segs
}

func textSeg(text string, st inlineStyle) *widget.TextSegment {
	return &widget.TextSegment{Text: text, Style: widget.RichTextStyle{
		Inline: true, TextStyle: st.text, SizeName: st.size, ColorName: st.color,
	}}
}

func (r *previewRenderer) linkSeg(l *md.Link, st inlineStyle) widget.RichTextSegment {
	text := md.PlainText(l.Children)
	u, err := url.Parse(l.Href)
	if err != nil {
		return textSeg(text, st)
	}
	href := l.Href
	return &widget.HyperlinkSegment{
		Text:      text,
		URL:       u,
		TextStyle: st.text,
		SizeName:  st.size,
		OnTapped:  func() { r.onLink(href) },
	}
}

func (r *previewRenderer) richText(segs []widget.RichTextSegment) *widget.RichText {
	if len(segs) == 0 {
		segs = []widget.RichTextSegment{textSeg(" ", inlineStyle{})}
	}
	rt := widget.NewRichText(segs...)
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// ---- blocks ----

func (r *previewRenderer) heading(h *md.Heading) fyne.CanvasObject {
	st := inlineStyle{text: fyne.TextStyle{Bold: true}, size: headingSizes[h.Level-1]}
	if h.Level == 6 {
		st.color = colorMuted
	}
	// A little air above headings separates sections more clearly than the
	// uniform VBox gap does.
	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(0, max(0, 12-2*float32(h.Level))))
	objs := []fyne.CanvasObject{gap, r.richText(r.inlines(h.Inlines, st))}
	if h.Level <= 2 {
		objs = append(objs, widget.NewSeparator())
	}
	var obj fyne.CanvasObject = container.NewVBox(objs...)
	if h.ID != "" {
		r.anchors[h.ID] = obj
	}
	return obj
}

func (r *previewRenderer) list(l *md.List, depth int) fyne.CanvasObject {
	box := container.NewVBox()
	bullets := []string{"•", "◦", "▪"}
	for i, it := range l.Items {
		var marker fyne.CanvasObject
		switch {
		case it.Task:
			icon := theme.CheckButtonIcon()
			if it.Checked {
				icon = theme.CheckButtonCheckedIcon()
			}
			ic := widget.NewIcon(icon)
			marker = container.NewPadded(ic)
		case l.Ordered:
			marker = widget.NewLabel(fmt.Sprintf("%d.", i+1))
		default:
			marker = widget.NewLabel(bullets[depth%len(bullets)])
		}
		var body fyne.CanvasObject = r.richText(r.inlines(it.Inlines, inlineStyle{}))
		if it.Task && it.Checked {
			body = r.richText(r.inlines(it.Inlines, inlineStyle{color: colorMuted}))
		}
		row := container.NewBorder(nil, nil, topAligned(marker), nil, body)
		box.Add(row)
		for _, c := range it.Children {
			box.Add(indent(r.list(c, depth+1), 24))
		}
	}
	return box
}

func (r *previewRenderer) quote(q *md.Blockquote) fyne.CanvasObject {
	content := container.NewVBox()
	for _, p := range q.Paragraphs {
		content.Add(r.richText(r.inlines(p, inlineStyle{text: fyne.TextStyle{Italic: true}})))
	}
	if q.Attribution != nil {
		segs := append([]widget.RichTextSegment{textSeg("— ", inlineStyle{color: colorMuted})},
			r.inlines(q.Attribution, inlineStyle{color: colorMuted})...)
		attr := r.richText(segs)
		for _, s := range attr.Segments {
			if t, ok := s.(*widget.TextSegment); ok {
				t.Style.Alignment = fyne.TextAlignTrailing
			}
		}
		content.Add(attr)
	}
	bar := canvas.NewRectangle(themeColor(theme.ColorNamePrimary))
	bar.SetMinSize(fyne.NewSize(4, 0))
	bar.CornerRadius = 2
	bg := canvas.NewRectangle(themeColor(colorBlockBg))
	bg.CornerRadius = theme.InputRadiusSize()
	return container.NewStack(bg, container.NewBorder(nil, nil, bar, nil, content))
}

var chatPalette = []fyne.ThemeColorName{
	theme.ColorNamePrimary, theme.ColorNameSuccess, theme.ColorNameWarning, theme.ColorNameError,
}

func (r *previewRenderer) chat(c *md.Chat) fyne.CanvasObject {
	box := container.NewVBox()
	speakers := map[string]int{}
	for _, l := range c.Lines {
		idx, ok := speakers[l.Speaker]
		if !ok {
			idx = len(speakers)
			speakers[l.Speaker] = idx
		}
		name := canvas.NewText(l.Speaker, themeColor(chatPalette[idx%len(chatPalette)]))
		name.TextStyle.Bold = true
		bubble := card(r.richText(r.inlines(l.Inlines, inlineStyle{})), themeColor(colorBlockBg), nil)
		var row fyne.CanvasObject = container.NewVBox(container.NewPadded(name), bubble)
		// Alternate sides for the first two speakers, like a messenger.
		if idx%2 == 1 {
			row = indent(row, 48)
		} else {
			row = container.New(&rightInsetLayout{inset: 48}, row)
		}
		box.Add(row)
	}
	return box
}

// rightInsetLayout leaves a fixed gap on the right of its child.
type rightInsetLayout struct{ inset float32 }

func (l *rightInsetLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(max(s.Width-l.inset, 0), s.Height))
	}
}

func (l *rightInsetLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var m fyne.Size
	for _, o := range objs {
		m = m.Max(o.MinSize())
	}
	return fyne.NewSize(m.Width+l.inset, m.Height)
}

func (r *previewRenderer) teaser() fyne.CanvasObject {
	label := canvas.NewText("  more — teaser ends here  ", themeColor(colorMuted))
	label.TextStyle.Italic = true
	label.TextSize = theme.CaptionTextSize()
	line := func() fyne.CanvasObject {
		return container.NewVBox(layout.NewSpacer(), widget.NewSeparator(), layout.NewSpacer())
	}
	return container.NewPadded(container.NewBorder(nil, nil, nil, nil,
		container.New(&teaserLayout{}, line(), label, line())))
}

// teaserLayout centres the middle object with the outer two filling the sides.
type teaserLayout struct{}

func (teaserLayout) Layout(o []fyne.CanvasObject, s fyne.Size) {
	mid := o[1].MinSize()
	side := (s.Width - mid.Width) / 2
	o[0].Move(fyne.NewPos(0, 0))
	o[0].Resize(fyne.NewSize(side, s.Height))
	o[1].Move(fyne.NewPos(side, (s.Height-mid.Height)/2))
	o[1].Resize(mid)
	o[2].Move(fyne.NewPos(side+mid.Width, 0))
	o[2].Resize(fyne.NewSize(side, s.Height))
}

func (teaserLayout) MinSize(o []fyne.CanvasObject) fyne.Size {
	m := o[1].MinSize()
	return fyne.NewSize(m.Width+20, m.Height)
}

func (r *previewRenderer) codeBlock(c *md.CodeBlock) fyne.CanvasObject {
	code := widget.NewLabel(c.Text)
	code.TextStyle = fyne.TextStyle{Monospace: true}
	code.Wrapping = fyne.TextWrapOff
	code.Selectable = true
	var body fyne.CanvasObject = container.NewHScroll(code)
	if c.Lang != "" {
		lang := canvas.NewText(c.Lang, themeColor(colorMuted))
		lang.TextSize = theme.CaptionTextSize()
		lang.Alignment = fyne.TextAlignTrailing
		body = container.NewBorder(lang, nil, nil, nil, body)
	}
	return card(body, themeColor(colorBlockBg), nil)
}

func (r *previewRenderer) table(t *md.Table) fyne.CanvasObject {
	seg := &widget.TableSegment{}
	cols := t.Columns()
	for c := 0; c < cols; c++ {
		a := fyne.TextAlignLeading
		if c < len(t.Align) {
			switch t.Align[c] {
			case md.AlignCenter:
				a = fyne.TextAlignCenter
			case md.AlignRight:
				a = fyne.TextAlignTrailing
			}
		}
		seg.Alignments = append(seg.Alignments, a)
	}
	cells := func(row [][]md.Inline) [][]widget.RichTextSegment {
		out := make([][]widget.RichTextSegment, len(row))
		for i, c := range row {
			out[i] = r.inlines(c, inlineStyle{})
		}
		return out
	}
	if t.Header != nil {
		seg.Headers = cells(t.Header)
	}
	for _, row := range t.Rows {
		seg.Rows = append(seg.Rows, cells(row))
	}
	return widget.NewRichText(seg)
}

// imageView builds an image (or a placeholder while loading / on error).
func (r *previewRenderer) imageView(src string, maxH float32, fill canvas.ImageFill, alt string) fyne.CanvasObject {
	e := r.images.get(src, r.baseDir)
	if e == nil {
		return fixedHeight(r.placeholder(theme.FileImageIcon(), "Loading "+src+" …"), min(maxH, 120))
	}
	if e.err != nil || e.res == nil {
		msg := "Image not found: " + src
		if e.err != nil && isRemote(src) {
			msg = "Could not load " + src + ": " + e.err.Error()
		}
		if alt != "" {
			msg = alt + " — " + msg
		}
		return r.placeholder(theme.BrokenImageIcon(), msg)
	}
	img := canvas.NewImageFromResource(e.res)
	img.FillMode = fill
	h := maxH
	if e.h > 0 && float32(e.h) < h && fill == canvas.ImageFillContain {
		h = float32(e.h)
	}
	img.SetMinSize(fyne.NewSize(min(float32(max(e.w, 0)), 64), h))
	return img
}

func (r *previewRenderer) placeholder(icon fyne.Resource, msg string) fyne.CanvasObject {
	l := widget.NewLabel(msg)
	l.Wrapping = fyne.TextWrapWord
	l.Importance = widget.LowImportance
	return card(container.NewBorder(nil, nil, widget.NewIcon(icon), nil, l),
		themeColor(colorBlockBg), themeColor(theme.ColorNameInputBorder))
}

func (r *previewRenderer) caption(text string) fyne.CanvasObject {
	l := widget.NewLabel(text)
	l.Alignment = fyne.TextAlignCenter
	l.TextStyle.Italic = true
	l.Wrapping = fyne.TextWrapWord
	l.Importance = widget.LowImportance
	l.SizeName = theme.SizeNameCaptionText
	return l
}

func (r *previewRenderer) figure(img *md.Image, maxH float32, fill canvas.ImageFill) fyne.CanvasObject {
	view := r.imageView(img.Src, maxH, fill, img.Alt)
	if img.Title == "" {
		return view
	}
	return container.NewVBox(view, r.caption(img.Title))
}

func (r *previewRenderer) gallery(g *md.Gallery) fyne.CanvasObject {
	box := container.NewVBox()
	for i := 0; i < len(g.Images); i += 2 {
		if i+1 < len(g.Images) {
			box.Add(container.NewGridWithColumns(2,
				r.figure(g.Images[i], 220, canvas.ImageFillCover),
				r.figure(g.Images[i+1], 220, canvas.ImageFillCover)))
		} else {
			box.Add(r.figure(g.Images[i], 300, canvas.ImageFillCover))
		}
	}
	return box
}

func (r *previewRenderer) media(m *md.Media) fyne.CanvasObject {
	icon := theme.MediaVideoIcon()
	kind := "Video"
	if m.Kind == md.MediaAudio {
		icon, kind = theme.MediaMusicIcon(), "Audio"
	}
	if m.Provider != "" {
		kind += " · " + m.Provider
	}
	title := widget.NewLabel(m.Caption)
	title.TextStyle.Bold = true
	title.Wrapping = fyne.TextWrapWord
	info := container.NewVBox(title)
	if m.Caption == "" {
		warn := widget.NewLabel("A caption is mandatory for " + strings.ToLower(kind[:5]) + ".")
		warn.Importance = widget.DangerImportance
		info.Objects[0] = warn
	}
	src := m.Src
	link := widget.NewHyperlink(kind+": "+src, nil)
	link.OnTapped = func() { r.onLink(src) }
	link.Wrapping = fyne.TextWrapBreak
	info.Add(link)

	var thumb fyne.CanvasObject
	if id := md.YouTubeID(m.Src); id != "" {
		thumb = r.imageView("https://img.youtube.com/vi/"+url.PathEscape(id)+"/hqdefault.jpg", 240, canvas.ImageFillContain, "")
	}
	body := container.NewBorder(nil, nil, container.NewPadded(widget.NewIcon(icon)), nil, info)
	if thumb != nil {
		return card(container.NewVBox(thumb, body), themeColor(colorBlockBg), themeColor(theme.ColorNameInputBorder))
	}
	return card(body, themeColor(colorBlockBg), themeColor(theme.ColorNameInputBorder))
}

func (r *previewRenderer) attachment(a *md.Attachment) fyne.CanvasObject {
	path := resolvePath(a.Href, r.baseDir)
	detail := filepath.Base(a.Href)
	if st, err := os.Stat(path); err == nil {
		detail += " · " + humanSize(st.Size())
	} else {
		detail += " · file not found"
	}
	title := widget.NewHyperlink(a.Text, nil)
	title.OnTapped = func() { r.onLink(a.Href) }
	title.TextStyle.Bold = true
	sub := widget.NewLabel(detail)
	sub.Importance = widget.LowImportance
	sub.SizeName = theme.SizeNameCaptionText
	body := container.NewBorder(nil, nil, container.NewPadded(widget.NewIcon(theme.FileIcon())), nil,
		container.NewVBox(title, sub))
	return card(body, themeColor(colorBlockBg), themeColor(theme.ColorNameInputBorder))
}

func (r *previewRenderer) container(c *md.Container) fyne.CanvasObject {
	label := canvas.NewText(strings.ToUpper(c.Name), themeColor(colorMuted))
	label.TextSize = theme.CaptionTextSize()
	label.TextStyle.Bold = true
	content := container.NewVBox(append([]fyne.CanvasObject{label}, r.blocks(c.Blocks)...)...)
	return card(content, color.Transparent, themeColor(theme.ColorNameInputBorder))
}

func (r *previewRenderer) frontMatter(doc *md.Document) fyne.CanvasObject {
	if len(doc.Meta) == 0 {
		return nil
	}
	box := container.NewVBox()
	if title := doc.MetaValue("title"); title != "" {
		box.Add(r.heading(&md.Heading{Level: 1, Inlines: []md.Inline{&md.Text{Text: title}}}))
	}
	var extra []string
	for _, f := range doc.Meta {
		switch f.Key {
		case "title", "link", "link_title", "link_description":
		default:
			extra = append(extra, f.Key+": "+f.Value)
		}
	}
	if len(extra) > 0 {
		l := widget.NewLabel(strings.Join(extra, "   ·   "))
		l.Importance = widget.LowImportance
		l.Wrapping = fyne.TextWrapWord
		l.SizeName = theme.SizeNameCaptionText
		box.Add(l)
	}
	if link := doc.MetaValue("link"); link != "" {
		label := doc.MetaValue("link_title")
		if label == "" {
			label = link
		}
		t := widget.NewHyperlink(label, nil)
		t.TextStyle.Bold = true
		t.Wrapping = fyne.TextWrapWord
		t.OnTapped = func() { r.onLink(link) }
		inner := container.NewVBox(t)
		if d := doc.MetaValue("link_description"); d != "" {
			dl := widget.NewLabel(d)
			dl.Wrapping = fyne.TextWrapWord
			inner.Add(dl)
		}
		u := widget.NewLabel(link)
		u.Importance = widget.LowImportance
		u.SizeName = theme.SizeNameCaptionText
		u.Truncation = fyne.TextTruncateEllipsis
		inner.Add(u)
		box.Add(card(inner, themeColor(colorBlockBg), themeColor(theme.ColorNameInputBorder)))
	}
	if len(box.Objects) == 0 {
		return nil
	}
	return box
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
