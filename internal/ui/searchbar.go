package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"github.com/ErikKalkoken/janice/internal/jsondocument"
)

const (
	searchTypeKey     = "key"
	searchTypeString  = "string"
	searchTypeNumber  = "number"
	searchTypeKeyword = "keyword"
)

var type2importance = map[jsondocument.JSONType]widget.Importance{
	jsondocument.Array:   widget.HighImportance,
	jsondocument.Object:  widget.HighImportance,
	jsondocument.String:  widget.WarningImportance,
	jsondocument.Number:  widget.SuccessImportance,
	jsondocument.Boolean: widget.DangerImportance,
	jsondocument.Null:    widget.DangerImportance,
}

// searchBar represents a search bar for searching in the JSON document.
type searchBar struct {
	widget.BaseWidget

	collapseAll  *ttwidget.Button
	previous     *ttwidget.Button
	replace      *widget.Accordion
	replaceAll   *widget.Button
	replaceEntry *searchEntry
	replaceNext  *widget.Button
	replacePrev  *widget.Button
	result       *widget.Label
	scrollBottom *ttwidget.Button
	scrollTop    *ttwidget.Button
	searchButton *ttwidget.Button
	searchEntry  *searchEntry
	searchType   *ttwidget.Select
	cancelSearch context.CancelFunc
	u            *UI
}

type findKeyHandler struct {
	onFind    func(jsondocument.SearchDirection)
	shiftDown bool
}

func (h *findKeyHandler) keyDown(event *fyne.KeyEvent) {
	switch event.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		h.shiftDown = true
	case fyne.KeyF3:
		direction := jsondocument.SearchForward
		if h.shiftDown {
			direction = jsondocument.SearchBackward
		}
		h.onFind(direction)
	}
}

func (h *findKeyHandler) keyUp(event *fyne.KeyEvent) {
	if event.Name == desktop.KeyShiftLeft || event.Name == desktop.KeyShiftRight {
		h.shiftDown = false
	}
}

type searchEntry struct {
	widget.Entry
	findKeys findKeyHandler
}

func newSearchEntry(onFind func(jsondocument.SearchDirection)) *searchEntry {
	e := &searchEntry{findKeys: findKeyHandler{onFind: onFind}}
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *searchEntry) KeyDown(event *fyne.KeyEvent) {
	e.Entry.KeyDown(event)
	e.findKeys.keyDown(event)
}

func (e *searchEntry) KeyUp(event *fyne.KeyEvent) {
	e.Entry.KeyUp(event)
	e.findKeys.keyUp(event)
}

func newSearchBar(u *UI) *searchBar {
	w := &searchBar{
		u: u,
	}
	w.ExtendBaseWidget(w)
	w.searchEntry = newSearchEntry(w.doSearchDirection)
	w.searchType = ttwidget.NewSelect(
		[]string{
			searchTypeKey,
			searchTypeKeyword,
			searchTypeNumber,
			searchTypeString,
		},
		nil,
	)
	w.searchType.SetSelected(searchTypeKey)
	w.searchType.SetToolTip("Select what to search")
	w.searchType.Disable()
	w.searchEntry.SetPlaceHolder(
		"Enter pattern to search for...")
	w.searchEntry.OnSubmitted = func(s string) {
		w.doSearchDirection(jsondocument.SearchForward)
	}
	w.searchButton = ttwidget.NewButtonWithIcon("", theme.SearchIcon(), func() {
		w.doSearchDirection(jsondocument.SearchForward)
	})
	w.searchButton.SetToolTip("Find next (F3)")
	w.previous = ttwidget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		w.doSearchDirection(jsondocument.SearchBackward)
	})
	w.previous.SetToolTip("Find previous (Shift+F3)")
	w.replaceEntry = newSearchEntry(w.doSearchDirection)
	w.replaceEntry.SetPlaceHolder("Replace with...")
	w.replaceEntry.OnSubmitted = func(string) {
		w.doReplace(jsondocument.SearchForward)
	}
	w.replacePrev = widget.NewButton("Replace previous", func() {
		w.doReplace(jsondocument.SearchBackward)
	})
	w.replaceNext = widget.NewButton("Replace next", func() {
		w.doReplace(jsondocument.SearchForward)
	})
	w.replaceAll = widget.NewButton("Replace all", w.doReplaceAll)
	replaceRow := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(w.replacePrev, w.replaceNext, w.replaceAll),
		w.replaceEntry,
	)
	w.replace = widget.NewAccordion(widget.NewAccordionItem("Replace", replaceRow))
	w.result = widget.NewLabel("")
	w.result.Importance = widget.LowImportance
	w.scrollBottom = ttwidget.NewButtonWithIcon("", theme.NewThemedResource(resourceVerticalalignbottomSvg), func() {
		w.u.tree.ScrollToBottom()
	})
	w.scrollBottom.SetToolTip("Scroll to bottom")
	w.scrollTop = ttwidget.NewButtonWithIcon("", theme.NewThemedResource(resourceVerticalaligntopSvg), func() {
		w.u.tree.ScrollToTop()
	})
	w.scrollTop.SetToolTip("Scroll to top")
	w.collapseAll = ttwidget.NewButtonWithIcon("", theme.NewThemedResource(resourceUnfoldlessSvg), func() {
		w.u.tree.CloseAllBranches()
	})
	w.collapseAll.SetToolTip("Collapse all")
	return w
}

func (w *searchBar) enable() {
	w.searchButton.Enable()
	w.previous.Enable()
	w.searchType.Enable()
	w.searchEntry.Enable()
	w.replaceEntry.Enable()
	w.replacePrev.Enable()
	w.replaceNext.Enable()
	w.replaceAll.Enable()
	w.scrollBottom.Enable()
	w.scrollTop.Enable()
	w.collapseAll.Enable()
}

func (w *searchBar) disable() {
	w.searchButton.Disable()
	w.previous.Disable()
	w.searchType.Disable()
	w.searchEntry.Disable()
	w.replaceEntry.Disable()
	w.replacePrev.Disable()
	w.replaceNext.Disable()
	w.replaceAll.Disable()
	w.scrollBottom.Disable()
	w.scrollTop.Disable()
	w.collapseAll.Disable()
}

func (w *searchBar) selectedSearch() (string, jsondocument.SearchType, bool) {
	search := w.searchEntry.Text
	if search == "" {
		w.result.SetText("Enter a search pattern")
		return "", 0, false
	}
	searchType := w.searchType.Selected
	var typ jsondocument.SearchType
	switch searchType {
	case searchTypeKey:
		typ = jsondocument.SearchKey
	case searchTypeKeyword:
		typ = jsondocument.SearchKeyword
		search = strings.ToLower(search)
		if search != "true" && search != "false" && search != "null" {
			w.u.showErrorDialog("Allowed keywords are: true, false, null", nil)
			return "", 0, false
		}
	case searchTypeString:
		typ = jsondocument.SearchString
	case searchTypeNumber:
		typ = jsondocument.SearchNumber
	}
	return search, typ, true
}

// doSearchDirection finds the next match in the requested direction.
func (w *searchBar) doSearchDirection(direction jsondocument.SearchDirection) {
	w.runSearch(direction, false)
}

func (w *searchBar) doReplace(direction jsondocument.SearchDirection) {
	w.runSearch(direction, true)
}

func (w *searchBar) runSearch(direction jsondocument.SearchDirection, replace bool) {
	search, typ, ok := w.selectedSearch()
	if !ok {
		return
	}
	if w.cancelSearch != nil {
		w.cancelSearch()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelSearch = cancel
	w.result.SetText(fmt.Sprintf("Searching %s...", w.searchType.Selected))
	startUID := w.u.selection.selectedUID
	go func() {
		uid, err := w.u.document.SearchDirection(ctx, startUID, search, typ, direction)
		fyne.Do(func() {
			if errors.Is(err, jsondocument.ErrCallerCanceled) {
				return
			} else if errors.Is(err, jsondocument.ErrNotFound) {
				w.result.SetText(fmt.Sprintf("No %s match found", w.searchType.Selected))
				return
			} else if err != nil {
				w.u.showErrorDialog("Search failed", err)
				return
			}
			if replace {
				if err := w.u.document.Replace(uid, typ, w.replaceEntry.Text); err != nil {
					w.u.showErrorDialog("Replace failed", err)
					return
				}
				w.u.markDirty()
				w.u.refreshEditedNode(uid)
				w.result.SetText("Replaced 1 match")
			} else {
				w.result.SetText("Match found")
			}
			w.u.tree.scrollTo(uid)
		})
	}()
}

func (w *searchBar) doReplaceAll() {
	search, typ, ok := w.selectedSearch()
	if !ok {
		return
	}
	count, err := w.u.document.ReplaceAll(context.Background(), search, typ, w.replaceEntry.Text)
	if err != nil {
		w.u.showErrorDialog("Replace all failed", err)
		return
	}
	if count > 0 {
		w.u.markDirty()
		w.u.tree.Refresh()
		if w.u.selection.selectedUID != "" {
			w.u.refreshEditedNode(w.u.selection.selectedUID)
		}
	}
	label := "matches"
	if count == 1 {
		label = "match"
	}
	w.result.SetText(fmt.Sprintf("Replaced %d %s", count, label))
}

func (w *searchBar) CreateRenderer() fyne.WidgetRenderer {
	searchRow := container.NewBorder(
		nil,
		nil,
		w.searchType,
		container.NewHBox(
			w.previous,
			w.searchButton,
			container.NewPadded(),
			layout.NewSpacer(),
			w.scrollTop,
			w.scrollBottom,
			w.collapseAll,
		),
		w.searchEntry,
	)
	c := container.NewVBox(searchRow, w.replace, w.result)
	return widget.NewSimpleRenderer(c)
}
