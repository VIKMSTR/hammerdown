package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Custom theme names used by the preview.
const (
	sizeH1 fyne.ThemeSizeName = "mkup-h1"
	sizeH2 fyne.ThemeSizeName = "mkup-h2"
	sizeH3 fyne.ThemeSizeName = "mkup-h3"
	sizeH4 fyne.ThemeSizeName = "mkup-h4"
	sizeH5 fyne.ThemeSizeName = "mkup-h5"
	sizeH6 fyne.ThemeSizeName = "mkup-h6"

	colorCode    fyne.ThemeColorName = "mkup-code"
	colorBlockBg fyne.ThemeColorName = "mkup-block-bg"
	colorMuted   fyne.ThemeColorName = "mkup-muted"
)

var headingSizes = [...]fyne.ThemeSizeName{sizeH1, sizeH2, sizeH3, sizeH4, sizeH5, sizeH6}

// mkupTheme extends the default theme with heading sizes and preview colours.
type mkupTheme struct{ fyne.Theme }

func newTheme() fyne.Theme { return &mkupTheme{theme.DefaultTheme()} }

func (t *mkupTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case sizeH1:
		return 28
	case sizeH2:
		return 23
	case sizeH3:
		return 19
	case sizeH4:
		return 16.5
	case sizeH5:
		return 15
	case sizeH6:
		return 14
	}
	return t.Theme.Size(n)
}

func (t *mkupTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	dark := v == theme.VariantDark
	switch n {
	case colorCode:
		if dark {
			return color.NRGBA{R: 0xf0, G: 0x9a, B: 0x6a, A: 0xff}
		}
		return color.NRGBA{R: 0xb3, G: 0x2d, B: 0x2e, A: 0xff}
	case colorBlockBg:
		if dark {
			return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x10}
		}
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0b}
	case colorMuted:
		if dark {
			return color.NRGBA{R: 0xa0, G: 0xa0, B: 0xa8, A: 0xff}
		}
		return color.NRGBA{R: 0x6a, G: 0x6a, B: 0x72, A: 0xff}
	}
	return t.Theme.Color(n, v)
}

// themeColor resolves a colour name against the current app theme.
func themeColor(n fyne.ThemeColorName) color.Color {
	app := fyne.CurrentApp()
	return app.Settings().Theme().Color(n, app.Settings().ThemeVariant())
}
