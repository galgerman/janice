package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

type previewKeySelect struct {
	widget.BaseWidget

	Options   []string
	Selected  string
	button    *ttwidget.Button
	onChanged func(string)
	onRemove  func(string)
	popup     *widget.PopUp
	window    fyne.Window
}

func newPreviewKeySelect(window fyne.Window, changed func(string), remove func(string)) *previewKeySelect {
	w := &previewKeySelect{
		Options:   []string{noJSONLinesPreview},
		Selected:  noJSONLinesPreview,
		onChanged: changed,
		onRemove:  remove,
		window:    window,
	}
	w.button = ttwidget.NewButtonWithIcon(noJSONLinesPreview, theme.MenuDropDownIcon(), w.showOptions)
	w.button.SetToolTip("Choose a key to show beside each row")
	w.ExtendBaseWidget(w)
	return w
}

func (w *previewKeySelect) SetSelected(selected string) {
	w.Selected = selected
	w.button.SetText(selected)
	if w.onChanged != nil {
		w.onChanged(selected)
	}
}

func (w *previewKeySelect) showOptions() {
	if w.popup != nil {
		w.popup.Hide()
	}
	rows := make([]fyne.CanvasObject, 0, len(w.Options))
	for _, option := range w.Options {
		option := option
		selectButton := widget.NewButton(option, func() {
			w.SetSelected(option)
			w.popup.Hide()
		})
		selectButton.Alignment = widget.ButtonAlignLeading
		if option == noJSONLinesPreview {
			rows = append(rows, selectButton)
			continue
		}
		removeButton := ttwidget.NewButtonWithIcon("", theme.CancelIcon(), func() {
			w.removeOption(option)
		})
		removeButton.SetToolTip(fmt.Sprintf("Remove %s", option))
		rows = append(rows, container.NewBorder(nil, nil, nil, removeButton, selectButton))
	}
	content := container.NewVScroll(container.NewVBox(rows...))
	width := fyne.Max(w.Size().Width, 260)
	height := fyne.Min(float32(len(rows))*44, 320)
	content.SetMinSize(fyne.NewSize(width, height))
	w.popup = widget.NewPopUp(content, w.window.Canvas())
	w.popup.ShowAtRelativePosition(fyne.NewPos(0, w.Size().Height), w)
}

func (w *previewKeySelect) removeOption(option string) {
	if w.popup != nil {
		w.popup.Hide()
	}
	if w.onRemove != nil {
		w.onRemove(option)
	}
}

func (w *previewKeySelect) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.button)
}
