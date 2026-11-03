package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// textButton is a flat toolbar button showing styled text (Fyne has no
// bold/italic icons) and reporting hover so the app can show a hint.
type textButton struct {
	widget.BaseWidget
	label   string
	style   fyne.TextStyle
	onTap   func()
	onHover func(bool)
	hovered bool
}

func newTextButton(label string, style fyne.TextStyle, tap func(), hover func(bool)) *textButton {
	b := &textButton{label: label, style: style, onTap: tap, onHover: hover}
	b.ExtendBaseWidget(b)
	return b
}

func (b *textButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *textButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
	if b.onHover != nil {
		b.onHover(true)
	}
}

func (b *textButton) MouseMoved(*desktop.MouseEvent) {}

func (b *textButton) MouseOut() {
	b.hovered = false
	b.Refresh()
	if b.onHover != nil {
		b.onHover(false)
	}
}

func (b *textButton) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (b *textButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.InputRadiusSize()
	txt := canvas.NewText(b.label, theme.Color(theme.ColorNameForeground))
	txt.TextStyle = b.style
	txt.Alignment = fyne.TextAlignCenter
	r := &textButtonRenderer{b: b, bg: bg, txt: txt}
	r.Refresh()
	return r
}

type textButtonRenderer struct {
	b   *textButton
	bg  *canvas.Rectangle
	txt *canvas.Text
}

func (r *textButtonRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	ts := r.txt.MinSize()
	r.txt.Move(fyne.NewPos(0, (s.Height-ts.Height)/2))
	r.txt.Resize(fyne.NewSize(s.Width, ts.Height))
}

func (r *textButtonRenderer) MinSize() fyne.Size {
	ts := r.txt.MinSize()
	p := theme.Padding()
	h := max(ts.Height, theme.IconInlineSize()) + p*2
	return fyne.NewSize(max(ts.Width+p*3, h), h)
}

func (r *textButtonRenderer) Refresh() {
	r.txt.Color = theme.Color(theme.ColorNameForeground)
	r.txt.TextSize = theme.TextSize()
	if r.b.hovered {
		r.bg.FillColor = theme.Color(theme.ColorNameHover)
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()
	r.txt.Refresh()
}

func (r *textButtonRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.txt} }
func (r *textButtonRenderer) Destroy()                     {}

// toolbarWidget adapts any canvas object into a widget.ToolbarItem.
type toolbarWidget struct{ obj fyne.CanvasObject }

func (t *toolbarWidget) ToolbarObject() fyne.CanvasObject { return t.obj }

// hoverIcon is a flat icon button for the toolbar that also reports hover.
type hoverIcon struct {
	widget.Button
	onHover func(bool)
}

func newHoverIcon(icon fyne.Resource, tap func(), hover func(bool)) *hoverIcon {
	b := &hoverIcon{onHover: hover}
	b.Icon = icon
	b.OnTapped = tap
	b.Importance = widget.LowImportance
	b.ExtendBaseWidget(b)
	return b
}

func (b *hoverIcon) MouseIn(e *desktop.MouseEvent) {
	b.Button.MouseIn(e)
	if b.onHover != nil {
		b.onHover(true)
	}
}

func (b *hoverIcon) MouseOut() {
	b.Button.MouseOut()
	if b.onHover != nil {
		b.onHover(false)
	}
}

// indentLayout offsets its single child by a fixed left inset.
type indentLayout struct{ inset float32 }

func (l *indentLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(l.inset, 0))
		o.Resize(fyne.NewSize(max(s.Width-l.inset, 0), s.Height))
	}
}

func (l *indentLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var m fyne.Size
	for _, o := range objs {
		m = m.Max(o.MinSize())
	}
	return fyne.NewSize(m.Width+l.inset, m.Height)
}

func indent(obj fyne.CanvasObject, inset float32) fyne.CanvasObject {
	return container.New(&indentLayout{inset: inset}, obj)
}

// card draws content on a rounded, optionally bordered background.
func card(content fyne.CanvasObject, fill, stroke color.Color) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.CornerRadius = theme.InputRadiusSize() * 1.5
	if stroke != nil {
		bg.StrokeColor = stroke
		bg.StrokeWidth = 1
	}
	return container.NewStack(bg, container.NewPadded(content))
}

// topAligned keeps obj at its minimum height at the top of its cell.
func topAligned(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(obj, layout.NewSpacer())
}

// fixedHeight forces a minimum height on obj.
func fixedHeight(obj fyne.CanvasObject, h float32) fyne.CanvasObject {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(1, h))
	return container.NewStack(spacer, obj)
}
