package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type readableDisabledTheme struct {
	fyne.Theme
	textScale float32
}

func (t readableDisabledTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameDisabled {
		background := t.Theme.Color(theme.ColorNameMenuBackground, variant)
		red, green, blue, _ := background.RGBA()
		if red+green+blue < 3*0x8000 {
			return color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}
		}
		return color.NRGBA{R: 0x68, G: 0x68, B: 0x68, A: 0xff}
	}
	return t.Theme.Color(name, variant)
}

func (t readableDisabledTheme) Size(name fyne.ThemeSizeName) float32 {
	size := t.Theme.Size(name)
	scale := t.textScale
	if scale <= 0 {
		scale = 1
	}
	switch name {
	case theme.SizeNameText, theme.SizeNameCaptionText, theme.SizeNameHeadingText, theme.SizeNameSubHeadingText:
		return size * scale
	default:
		return size
	}
}
