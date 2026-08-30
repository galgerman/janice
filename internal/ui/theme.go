package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Modern accent used for primary actions, focus and links.
var (
	accentDark  = color.NRGBA{R: 0x7f, G: 0x93, B: 0xff, A: 0xff}
	accentLight = color.NRGBA{R: 0x43, G: 0x51, B: 0xd0, A: 0xff}
)

type readableDisabledTheme struct {
	fyne.Theme
	textScale float32
}

func (t readableDisabledTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := isDarkTheme(t.Theme, variant)
	if name == theme.ColorNameDisabled {
		if dark {
			return color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}
		}
		return color.NRGBA{R: 0x68, G: 0x68, B: 0x68, A: 0xff}
	}
	if c, ok := paletteColor(name, dark); ok {
		return c
	}
	return t.Theme.Color(name, variant)
}

// isDarkTheme reports the effective brightness of the wrapped theme, which may
// pin a variant that differs from the one requested by the system.
func isDarkTheme(base fyne.Theme, variant fyne.ThemeVariant) bool {
	red, green, blue, _ := base.Color(theme.ColorNameMenuBackground, variant).RGBA()
	return red+green+blue < 3*0x8000
}

// paletteColor returns the sleek surface palette for the given color role.
func paletteColor(name fyne.ThemeColorName, dark bool) (color.Color, bool) {
	accent := accentLight
	if dark {
		accent = accentDark
	}
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		return accent, true
	case theme.ColorNameFocus:
		return withAlpha(accent, 0x8c), true
	case theme.ColorNameSelection:
		return withAlpha(accent, 0x59), true
	case theme.ColorNameHover:
		return withAlpha(accent, 0x2b), true
	case theme.ColorNameBackground:
		if dark {
			return color.NRGBA{R: 0x15, G: 0x17, B: 0x1d, A: 0xff}, true
		}
		return color.NRGBA{R: 0xf6, G: 0xf7, B: 0xfa, A: 0xff}, true
	case theme.ColorNameMenuBackground, theme.ColorNameHeaderBackground, theme.ColorNameOverlayBackground:
		if dark {
			return color.NRGBA{R: 0x1e, G: 0x21, B: 0x29, A: 0xff}, true
		}
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, true
	case theme.ColorNameInputBackground:
		if dark {
			return color.NRGBA{R: 0x23, G: 0x27, B: 0x30, A: 0xff}, true
		}
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, true
	case theme.ColorNameButton:
		if dark {
			return color.NRGBA{R: 0x28, G: 0x2d, B: 0x37, A: 0xff}, true
		}
		return color.NRGBA{R: 0xed, G: 0xef, B: 0xf4, A: 0xff}, true
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		if dark {
			return color.NRGBA{R: 0x33, G: 0x39, B: 0x45, A: 0xff}, true
		}
		return color.NRGBA{R: 0xdd, G: 0xe1, B: 0xe8, A: 0xff}, true
	}
	return nil, false
}

func withAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = alpha
	return c
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
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameScrollBarRadius:
		return 4
	default:
		return size
	}
}
