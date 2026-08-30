package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

// detail shows the value of the selected item in the JSON document.
type detail struct {
	widget.BaseWidget

	copyValueClipboard *ttwidget.Button
	applyKey           *ttwidget.Button
	applyValue         *ttwidget.Button
	keyEntry           *widget.Entry
	selectedUID        widget.TreeNodeID
	u                  *UI
	valueEntry         *widget.Entry
}

func newDetail(u *UI) *detail {
	w := &detail{
		keyEntry:   widget.NewEntry(),
		u:          u,
		valueEntry: widget.NewMultiLineEntry(),
	}
	w.ExtendBaseWidget(w)
	w.copyValueClipboard = ttwidget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		u.app.Clipboard().SetContent(w.valueEntry.Text)
	})
	w.copyValueClipboard.SetToolTip("Copy value to clipboard")
	w.applyKey = ttwidget.NewButtonWithIcon("Apply key", theme.ConfirmIcon(), func() {
		u.applyKeyEdit(w.selectedUID, w.keyEntry.Text)
	})
	w.applyValue = ttwidget.NewButtonWithIcon("Apply value", theme.ConfirmIcon(), func() {
		u.applyValueEdit(w.selectedUID, w.valueEntry.Text)
	})
	w.valueEntry.SetMinRowsVisible(2)
	w.reset()
	return w
}

func (w *detail) CreateRenderer() fyne.WidgetRenderer {
	c := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Key"), w.applyKey, w.keyEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Value"), container.NewHBox(w.copyValueClipboard, w.applyValue), w.valueEntry),
	)
	return widget.NewSimpleRenderer(c)
}

func (w *detail) reset() {
	w.selectedUID = ""
	w.keyEntry.SetText("")
	w.valueEntry.SetText("")
	w.keyEntry.Disable()
	w.valueEntry.Disable()
	w.applyKey.Disable()
	w.applyValue.Disable()
	w.copyValueClipboard.Disable()
}

func (w *detail) set(uid widget.TreeNodeID) {
	w.selectedUID = uid
	node := w.u.document.Value(uid)
	w.keyEntry.SetText(node.Key)
	if w.u.document.CanSetKey(uid) {
		w.keyEntry.Enable()
		w.applyKey.Enable()
	} else {
		w.keyEntry.Disable()
		w.applyKey.Disable()
	}
	if value, ok := w.u.document.ScalarText(uid); ok {
		w.valueEntry.SetText(value)
		w.valueEntry.Enable()
		w.applyValue.Enable()
		w.copyValueClipboard.Enable()
	} else {
		w.valueEntry.SetText(node.Type.String())
		w.valueEntry.Disable()
		w.applyValue.Disable()
		w.copyValueClipboard.Disable()
	}
}
