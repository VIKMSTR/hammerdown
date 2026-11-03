package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Custom theme names used by the preview and the window chrome.
const (
	sizeH1 fyne.ThemeSizeName = "hammerdown-h1"
	sizeH2 fyne.ThemeSizeName = "hammerdown-h2"
	sizeH3 fyne.ThemeSizeName = "hammerdown-h3"
	sizeH4 fyne.ThemeSizeName = "hammerdown-h4"
	sizeH5 fyne.ThemeSizeName = "hammerdown-h5"
	sizeH6 fyne.ThemeSizeName = "hammerdown-h6"

	colorCode     fyne.ThemeColorName = "hammerdown-code"
	colorBlockBg  fyne.ThemeColorName = "hammerdown-block-bg"
	colorMuted    fyne.ThemeColorName = "hammerdown-muted"
	colorChrome   fyne.ThemeColorName = "hammerdown-chrome"    // toolbar and status bar
	colorEditorBg fyne.ThemeColorName = "hammerdown-editor-bg" // the source pane
	colorCaret    fyne.ThemeColorName = "hammerdown-caret"     // the editor's text cursor
)

var headingSizes = [...]fyne.ThemeSizeName{sizeH1, sizeH2, sizeH3, sizeH4, sizeH5, sizeH6}

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

func rgba(hex uint32, a uint8) color.NRGBA {
	c := rgb(hex)
	c.A = a
	return c
}

// palette pairs a light and a dark value for every colour the theme defines.
// Light is a warm "paper" look, dark a soft slate; both share an indigo accent.
var palette = map[fyne.ThemeColorName][2]color.Color{
	theme.ColorNameBackground:          {rgb(0xfdfcfa), rgb(0x23252b)},
	theme.ColorNameForeground:          {rgb(0x2b2c31), rgb(0xe2e3e8)},
	theme.ColorNamePrimary:             {rgb(0x4f5bd5), rgb(0x8f9cf8)},
	theme.ColorNameForegroundOnPrimary: {rgb(0xffffff), rgb(0x16171c)},
	theme.ColorNameHyperlink:           {rgb(0x3f4bc4), rgb(0x9daaff)},
	theme.ColorNameFocus:               {rgba(0x4f5bd5, 0x66), rgba(0x8f9cf8, 0x66)},
	theme.ColorNameSelection:           {rgba(0x4f5bd5, 0x38), rgba(0x8f9cf8, 0x45)},
	theme.ColorNameHover:               {rgba(0x2b2c31, 0x10), rgba(0xffffff, 0x12)},
	theme.ColorNamePressed:             {rgba(0x2b2c31, 0x1c), rgba(0xffffff, 0x20)},
	theme.ColorNameButton:              {rgb(0xeeece6), rgb(0x30323a)},
	theme.ColorNameDisabledButton:      {rgb(0xeae8e2), rgb(0x2a2c33)},
	theme.ColorNameDisabled:            {rgb(0x8a8b92), rgb(0x80838f)},
	theme.ColorNamePlaceHolder:         {rgb(0xa3a3a8), rgb(0x6c6f7a)},
	theme.ColorNameInputBackground:     {rgb(0xffffff), rgb(0x1b1c21)},
	theme.ColorNameInputBorder:         {rgb(0xdcd9d1), rgb(0x373a44)},
	theme.ColorNameSeparator:           {rgb(0xe4e1d9), rgb(0x30323a)},
	theme.ColorNameShadow:              {rgba(0x000000, 0x22), rgba(0x000000, 0x66)},
	theme.ColorNameScrollBar:           {rgba(0x2b2c31, 0x40), rgba(0xffffff, 0x38)},
	theme.ColorNameScrollBarBackground: {color.Transparent, color.Transparent},
	theme.ColorNameMenuBackground:      {rgb(0xfdfcfa), rgb(0x2a2c33)},
	theme.ColorNameOverlayBackground:   {rgb(0xfdfcfa), rgb(0x2a2c33)},
	theme.ColorNameHeaderBackground:    {rgb(0xf1efe9), rgb(0x2a2c33)},
	theme.ColorNameSuccess:             {rgb(0x2f8f5b), rgb(0x6fcf97)},
	theme.ColorNameWarning:             {rgb(0xb7791f), rgb(0xf2c46d)},
	theme.ColorNameError:               {rgb(0xc8413a), rgb(0xf08079)},

	colorChrome:   {rgb(0xf1efe9), rgb(0x1a1b20)},
	colorEditorBg: {rgb(0xf7f6f2), rgb(0x1e2025)},
	colorCaret:    {rgb(0x3341d8), rgb(0xb3bcff)},
	colorCode:     {rgb(0xb4462f), rgb(0xeaa97f)},
	colorBlockBg:  {rgb(0xf3f1ec), rgb(0x2b2e36)},
	colorMuted:    {rgb(0x7a7b82), rgb(0x8d909c)},
}

// hammerdownTheme extends the default theme with its own palette, heading sizes and
// slightly rounder, airier geometry.
type hammerdownTheme struct{ fyne.Theme }

func newTheme() fyne.Theme { return &hammerdownTheme{theme.DefaultTheme()} }

func (t *hammerdownTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case sizeH1:
		return 30
	case sizeH2:
		return 24
	case sizeH3:
		return 19.5
	case sizeH4:
		return 17
	case sizeH5:
		return 15
	case sizeH6:
		return 14
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 6
	case theme.SizeNameSelectionRadius:
		return 4
	case theme.SizeNameScrollBarRadius:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return t.Theme.Size(n)
}

func (t *hammerdownTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if c, ok := palette[n]; ok {
		if v == theme.VariantDark {
			return c[1]
		}
		return c[0]
	}
	return t.Theme.Color(n, v)
}

// editorTheme drops the input background so the source pane reads as a
// writing surface rather than a form field. Fyne draws the text cursor as
// wide as the input border and in the primary colour, so the border keeps a
// visible width (bleedLayout hides it off-pane) and primary becomes the
// high-contrast caret colour.
// It delegates to the current app theme so it follows theme changes.
type editorTheme struct{ appTheme }

func (t editorTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameInputBackground:
		return color.Transparent
	case theme.ColorNamePrimary:
		return t.base().Color(colorCaret, v)
	}
	return t.base().Color(n, v)
}

// editorBorder is the editor's border width, and so the width of its cursor.
const editorBorder = 2

func (t editorTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputBorder:
		return editorBorder
	case theme.SizeNameInnerPadding:
		return 14 + 2*editorBorder // make up for the bleed
	}
	return t.base().Size(n)
}

// splitTheme draws the split handle as a hairline instead of a shadow bar.
type splitTheme struct{ appTheme }

func (t splitTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameShadow {
		return t.base().Color(theme.ColorNameSeparator, v)
	}
	return t.base().Color(n, v)
}

// appTheme forwards to whatever theme the app currently uses; embed it to
// override a few values on top of that.
type appTheme struct{}

func (appTheme) base() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (t appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.base().Color(n, v)
}
func (t appTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.base().Font(s) }
func (t appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base().Icon(n) }
func (t appTheme) Size(n fyne.ThemeSizeName) float32       { return t.base().Size(n) }

// themeColor resolves a colour name against the current app theme.
func themeColor(n fyne.ThemeColorName) color.Color {
	app := fyne.CurrentApp()
	return app.Settings().Theme().Color(n, app.Settings().ThemeVariant())
}
