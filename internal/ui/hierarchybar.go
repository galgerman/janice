package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type hierarchyBar struct {
	widget.BaseWidget

	label *widget.Label
	u     *UI
}

func newHierarchyBar(u *UI) *hierarchyBar {
	w := &hierarchyBar{label: widget.NewLabel("Hierarchy: No item selected"), u: u}
	w.label.TextStyle.Bold = true
	w.ExtendBaseWidget(w)
	w.Hide()
	return w
}

func (w *hierarchyBar) set(uid widget.TreeNodeID) {
	keys := make([]string, 0)
	for _, pathUID := range append(w.u.document.Path(uid), uid) {
		key := w.u.document.Value(pathUID).Key
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	w.label.SetText("Hierarchy: " + formatHierarchy(keys))
}

func formatHierarchy(rawKeys []string) string {
	keys := make([]string, 0, len(rawKeys))
	for _, key := range rawKeys {
		if index, ok := arrayIndex(key); ok {
			if len(keys) == 0 {
				keys = append(keys, strconv.Itoa(index+1))
			} else {
				keys[len(keys)-1] += "[" + strconv.Itoa(index+1) + "]"
			}
			continue
		}
		keys = append(keys, key)
	}
	return strings.Join(keys, " -> ")
}

func (w *hierarchyBar) reset() {
	w.label.SetText("Hierarchy: No item selected")
}

func arrayIndex(key string) (int, bool) {
	if len(key) < 3 || key[0] != '[' || key[len(key)-1] != ']' {
		return 0, false
	}
	index, err := strconv.Atoi(key[1 : len(key)-1])
	return index, err == nil
}

func (w *hierarchyBar) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewHScroll(w.label))
}
