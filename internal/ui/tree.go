package ui

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/ErikKalkoken/janice/internal/jsondocument"
)

// jsonTree shows a JSON document in a tree structure.
type jsonTree struct {
	widget.Tree
	findKeys findKeyHandler
	u        *UI
}

func newJSONTree(u *UI) *jsonTree {
	w := &jsonTree{u: u}
	w.findKeys.onFind = u.searchBar.doSearchDirection
	w.ExtendBaseWidget(w)

	w.ChildUIDs = func(id widget.TreeNodeID) []widget.TreeNodeID {
		return u.document.ChildUIDs(id)
	}
	w.IsBranch = func(id widget.TreeNodeID) bool {
		return u.document.IsBranch(id)
	}
	w.CreateNode = func(branch bool) fyne.CanvasObject {
		obj := newTreeNode(func() bool {
			return u.app.Preferences().BoolWithFallback(settingCompactTree, false)
		})
		obj.onSecondaryTap = func(event *fyne.PointEvent) {
			w.showPreviewKeyMenu(obj, event)
		}
		return obj
	}
	w.UpdateNode = func(uid widget.TreeNodeID, branch bool, co fyne.CanvasObject) {
		node := u.document.Value(uid)
		obj := co.(*treeNode)
		obj.uid = uid
		key := node.Key
		if u.document.IsJSONLines() && u.jsonLinesBar != nil {
			if preview, ok := u.document.JSONLinesRowPreview(uid, u.jsonLinesBar.selectedPreviewKey()); ok {
				key = fmt.Sprintf("%s — %s", key, preview)
			}
		}
		var text string
		switch v := node.Value; node.Type {
		case jsondocument.Array:
			if branch {
				if t := u.tree; t != nil && t.IsBranchOpen(uid) {
					text = ""
				} else {
					text = "[...]"
				}
			} else {
				text = "[]"
			}
		case jsondocument.Object:
			if branch {
				if t := u.tree; t != nil && t.IsBranchOpen(uid) {
					text = ""
				} else {
					text = "{...}"
				}
			} else {
				text = "{}"
			}
		case jsondocument.String:
			text = fmt.Sprintf("\"%s\"", v)
		case jsondocument.Number:
			text = string(v.(json.Number))
		case jsondocument.Boolean:
			text = fmt.Sprintf("%v", v)
		case jsondocument.Null:
			text = "null"
		default:
			text = fmt.Sprintf("%v", v)
		}
		obj.set(key, text, type2importance[node.Type])
	}
	w.OnSelected = func(uid widget.TreeNodeID) {
		u.selectElement(uid)
	}
	w.OnBranchOpened = func(uid widget.TreeNodeID) {
		if u.app.Preferences().BoolWithFallback(settingShowHierarchy, settingShowHierarchyDefault) {
			u.hierarchy.set(uid)
		}
	}
	return w
}

func (w *jsonTree) KeyDown(event *fyne.KeyEvent) {
	w.findKeys.keyDown(event)
}

func (w *jsonTree) KeyUp(event *fyne.KeyEvent) {
	w.findKeys.keyUp(event)
}

var _ desktop.Keyable = (*jsonTree)(nil)

func (w *jsonTree) showPreviewKeyMenu(node *treeNode, event *fyne.PointEvent) {
	uid := node.uid
	n := w.u.document.Value(uid)
	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Copy key", func() { w.u.app.Clipboard().SetContent(n.Key) }),
	}
	if value, ok := w.u.document.ScalarText(uid); ok {
		items = append(items, fyne.NewMenuItem("Copy value", func() { w.u.app.Clipboard().SetContent(value) }))
	}
	items = append(items, fyne.NewMenuItem("Copy hierarchy path", func() {
		w.u.app.Clipboard().SetContent(w.u.hierarchy.path(uid))
	}))
	if w.u.document.CanSetKey(uid) {
		items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem("Edit key...", func() { w.u.editKey(uid) }))
	}
	if _, ok := w.u.document.ScalarText(uid); ok {
		items = append(items, fyne.NewMenuItem("Edit value...", func() { w.u.editValue(uid) }))
	}
	if w.u.document.IsJSONLines() {
		if path, ok := w.u.document.PreviewPath(uid); ok {
			label := "Add to row key preview options"
			action := func() { w.u.jsonLinesBar.addPreviewKey(path) }
			if w.u.jsonLinesBar.hasPreviewKey(path) {
				label = "Remove from row key preview options"
				action = func() { w.u.jsonLinesBar.removePreviewKey(path) }
			}
			items = append(items, fyne.NewMenuItemSeparator(), fyne.NewMenuItem(label, action))
		}
	}
	menu := widget.NewPopUpMenu(fyne.NewMenu("", items...), w.u.window.Canvas())
	menu.ShowAtPosition(event.AbsolutePosition)
}

func (w *jsonTree) scrollTo(uid widget.TreeNodeID) {
	if uid == "" {
		return
	}
	p := w.u.document.Path(uid)
	for _, uid2 := range p {
		w.OpenBranch(uid2)
	}
	w.ScrollTo(uid)
	w.Select(uid)
}
