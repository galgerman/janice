package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

const noJSONLinesPreview = "(none)"

// jsonLinesBar provides row navigation and preview-key selection for JSON
// Lines documents.
type jsonLinesBar struct {
	widget.BaseWidget

	currentRow   int
	next         *ttwidget.Button
	previous     *ttwidget.Button
	previewKey   *ttwidget.Select
	rowIndicator *widget.Label
	u            *UI
}

func newJSONLinesBar(u *UI) *jsonLinesBar {
	w := &jsonLinesBar{
		currentRow:   -1,
		rowIndicator: widget.NewLabel(""),
		u:            u,
	}
	w.ExtendBaseWidget(w)
	w.previewKey = ttwidget.NewSelect([]string{noJSONLinesPreview}, func(string) {
		if w.u.tree != nil {
			w.u.tree.Refresh()
		}
	})
	w.previewKey.SetSelected(noJSONLinesPreview)
	w.previewKey.SetToolTip("Choose a top-level key to show beside each row")
	w.previous = ttwidget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		w.goToRow(w.currentRow - 1)
	})
	w.previous.SetToolTip("Previous row")
	w.next = ttwidget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		w.goToRow(w.currentRow + 1)
	})
	w.next.SetToolTip("Next row")
	w.Hide()
	return w
}

func (w *jsonLinesBar) setDocument() {
	if !w.u.document.IsJSONLines() {
		w.reset()
		return
	}
	options := []string{noJSONLinesPreview}
	options = append(options, w.u.document.JSONLinesPreviewKeys()...)
	w.previewKey.Options = options
	w.previewKey.SetSelected(noJSONLinesPreview)
	w.previewKey.Refresh()
	w.currentRow = -1
	w.Show()
	w.updateNavigation()
	if w.u.document.JSONLinesRowCount() > 0 {
		w.goToRow(0)
	}
}

func (w *jsonLinesBar) reset() {
	w.currentRow = -1
	w.rowIndicator.SetText("")
	w.Hide()
}

func (w *jsonLinesBar) selectedPreviewKey() string {
	if w.previewKey.Selected == noJSONLinesPreview {
		return ""
	}
	return w.previewKey.Selected
}

func (w *jsonLinesBar) syncSelection(uid string) {
	row := w.u.document.JSONLinesRowIndex(uid)
	if row < 0 {
		return
	}
	w.currentRow = row
	w.updateNavigation()
}

func (w *jsonLinesBar) goToRow(row int) {
	uid, ok := w.u.document.JSONLinesRowUID(row)
	if !ok {
		return
	}
	if previousUID, found := w.u.document.JSONLinesRowUID(w.currentRow); found && previousUID != uid {
		w.u.tree.CloseBranch(previousUID)
	}
	w.currentRow = row
	w.updateNavigation()
	if w.u.tree.IsBranch(uid) {
		w.u.tree.OpenBranch(uid)
	}
	w.u.tree.ScrollTo(uid)
	w.u.tree.Select(uid)
}

func (w *jsonLinesBar) updateNavigation() {
	count := w.u.document.JSONLinesRowCount()
	if w.currentRow < 0 || count == 0 {
		w.rowIndicator.SetText(fmt.Sprintf("%d rows", count))
		w.previous.Disable()
		if count > 0 {
			w.next.Enable()
		} else {
			w.next.Disable()
		}
		return
	}
	w.rowIndicator.SetText(fmt.Sprintf("Row %d of %d", w.currentRow+1, count))
	if w.currentRow > 0 {
		w.previous.Enable()
	} else {
		w.previous.Disable()
	}
	if w.currentRow < count-1 {
		w.next.Enable()
	} else {
		w.next.Disable()
	}
}

func (w *jsonLinesBar) CreateRenderer() fyne.WidgetRenderer {
	c := container.NewBorder(
		nil,
		nil,
		container.NewHBox(widget.NewLabel("Row preview key"), w.previewKey),
		container.NewHBox(w.previous, w.rowIndicator, w.next),
		layout.NewSpacer(),
	)
	return widget.NewSimpleRenderer(c)
}
