package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/ErikKalkoken/janice/internal/jsondocument"
)

// jsonTree shows a JSON document in a tree structure.
type jsonTree struct {
	widget.Tree
	u *UI
}

func newJSONTree(u *UI) *jsonTree {
	w := &jsonTree{u: u}
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
			x := v.(float64)
			text = strconv.FormatFloat(x, 'f', -1, 64)
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

func (w *jsonTree) showPreviewKeyMenu(node *treeNode, event *fyne.PointEvent) {
	path, ok := w.u.document.PreviewPath(node.uid)
	if !ok {
		return
	}
	label := "Add to row key preview options"
	action := func() { w.u.jsonLinesBar.addPreviewKey(path) }
	if w.u.jsonLinesBar.hasPreviewKey(path) {
		label = "Remove from row key preview options"
		action = func() { w.u.jsonLinesBar.removePreviewKey(path) }
	}
	menu := widget.NewPopUpMenu(fyne.NewMenu("", fyne.NewMenuItem(label, action)), w.u.window.Canvas())
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
