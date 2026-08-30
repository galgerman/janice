package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// treeNode represents a node in a JSON document tree.
type treeNode struct {
	widget.BaseWidget

	compact        func() bool
	key            *widget.Label
	onSecondaryTap func(*fyne.PointEvent)
	uid            widget.TreeNodeID
	value          *widget.Label
}

func (w *treeNode) TappedSecondary(event *fyne.PointEvent) {
	if w.onSecondaryTap != nil {
		w.onSecondaryTap(event)
	}
}

// newTreeNode returns a new instance of the [treeNode] widget.
func newTreeNode(compact ...func() bool) *treeNode {
	w := &treeNode{
		key:   widget.NewLabel(""),
		value: widget.NewLabel(""),
	}
	if len(compact) > 0 {
		w.compact = compact[0]
	}
	w.ExtendBaseWidget(w)
	return w
}

func (w *treeNode) set(key string, value string, importance widget.Importance) {
	w.key.SetText(fmt.Sprintf("%s :", key))
	w.value.Importance = importance
	w.value.Text = strings.ReplaceAll(value, "\n", " ")
	w.value.Refresh()
	w.value.Truncation = fyne.TextTruncateEllipsis
}

func (w *treeNode) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewBorder(nil, nil, w.key, nil, w.value)
	c := container.New(&treeNodeLayout{node: w}, content)
	return widget.NewSimpleRenderer(c)
}

type treeNodeLayout struct {
	node *treeNode
}

func (l *treeNodeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Resize(size)
}

func (l *treeNodeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := objects[0].MinSize()
	if l.node.compact != nil && l.node.compact() {
		size.Height = fyne.Max(size.Height-12, 20)
	}
	return size
}
