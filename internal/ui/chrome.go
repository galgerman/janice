package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// chromeEdge selects the side of a chrome panel that carries the hairline.
type chromeEdge int

const (
	edgeBottom chromeEdge = iota
	edgeTop
)

// chrome draws toolbars and status bars on a subtle gradient with a hairline
// border facing the document.
type chrome struct {
	widget.BaseWidget

	content fyne.CanvasObject
	edge    chromeEdge
}

func newChrome(edge chromeEdge, content fyne.CanvasObject) *chrome {
	w := &chrome{content: content, edge: edge}
	w.ExtendBaseWidget(w)
	return w
}

func (w *chrome) CreateRenderer() fyne.WidgetRenderer {
	r := &chromeRenderer{
		chrome:   w,
		gradient: canvas.NewVerticalGradient(color.Transparent, color.Transparent),
		line:     canvas.NewRectangle(color.Transparent),
	}
	r.objects = []fyne.CanvasObject{r.gradient, w.content, r.line}
	r.applyTheme()
	return r
}

type chromeRenderer struct {
	chrome   *chrome
	gradient *canvas.LinearGradient
	line     *canvas.Rectangle
	objects  []fyne.CanvasObject
}

func (r *chromeRenderer) Destroy() {}

func (r *chromeRenderer) Layout(size fyne.Size) {
	r.gradient.Resize(size)
	r.chrome.content.Resize(size)
	thickness := currentTheme().Size(theme.SizeNameSeparatorThickness)
	r.line.Resize(fyne.NewSize(size.Width, thickness))
	if r.chrome.edge == edgeTop {
		r.line.Move(fyne.NewPos(0, 0))
		return
	}
	r.line.Move(fyne.NewPos(0, size.Height-thickness))
}

func (r *chromeRenderer) MinSize() fyne.Size {
	return r.chrome.content.MinSize()
}

func (r *chromeRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *chromeRenderer) Refresh() {
	r.applyTheme()
	canvas.Refresh(r.chrome)
}

// applyTheme recolors the panel so it tracks light and dark theme changes.
func (r *chromeRenderer) applyTheme() {
	th := currentTheme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	raised := th.Color(theme.ColorNameMenuBackground, variant)
	flat := th.Color(theme.ColorNameBackground, variant)
	if r.chrome.edge == edgeTop {
		r.gradient.StartColor, r.gradient.EndColor = flat, raised
	} else {
		r.gradient.StartColor, r.gradient.EndColor = raised, flat
	}
	r.gradient.Refresh()
	r.line.FillColor = th.Color(theme.ColorNameSeparator, variant)
	r.line.Refresh()
}

func currentTheme() fyne.Theme {
	return fyne.CurrentApp().Settings().Theme()
}
