package ui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/ErikKalkoken/janice/internal/jsondocument"
	"github.com/stretchr/testify/assert"
)

func TestAddToListWithRotation(t *testing.T) {
	t.Run("can add to empty list", func(t *testing.T) {
		var l []string
		l2 := addToListWithRotation(l, "alpha", 5)
		assert.Equal(t, []string{"alpha"}, l2)
	})
	t.Run("should insert new items on top", func(t *testing.T) {
		var l = []string{"alpha"}
		l2 := addToListWithRotation(l, "bravo", 5)
		assert.Equal(t, []string{"bravo", "alpha"}, l2)
	})
	t.Run("should throw away bottom item to keep max length", func(t *testing.T) {
		var l = []string{"alpha", "bravo", "charlie"}
		l2 := addToListWithRotation(l, "delta", 3)
		assert.Equal(t, []string{"delta", "alpha", "bravo"}, l2)
	})
	t.Run("should insert new on top and remove duplicates", func(t *testing.T) {
		var l = []string{"alpha", "bravo"}
		l2 := addToListWithRotation(l, "bravo", 5)
		assert.Equal(t, []string{"bravo", "alpha"}, l2)
	})
}

func TestFormatHierarchy(t *testing.T) {
	assert.Equal(t, "1 -> input -> launchers[2] -> data", formatHierarchy([]string{"[0]", "input", "launchers", "[1]", "data"}))
}

func TestCompactTreeNodeIsShorter(t *testing.T) {
	normal := newTreeNode(func() bool { return false })
	compact := newTreeNode(func() bool { return true })
	assert.Less(t, compact.MinSize().Height, normal.MinSize().Height)
}

func TestDisabledTextIsReadableInDarkMode(t *testing.T) {
	got := readableDisabledTheme{Theme: theme.DefaultTheme()}.Color(theme.ColorNameDisabled, theme.VariantDark)
	assert.Equal(t, color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}, got)
}

func TestFontSizeScale(t *testing.T) {
	assert.Equal(t, float32(0.85), fontSizeScale(fontSizeSmall))
	assert.Equal(t, float32(1), fontSizeScale(fontSizeDefault))
	assert.Equal(t, float32(1.2), fontSizeScale(fontSizeLarge))
	assert.Equal(t, float32(1.4), fontSizeScale(fontSizeExtraLarge))
}

func TestThemeScalesTextOnly(t *testing.T) {
	base := theme.DefaultTheme()
	scaled := readableDisabledTheme{Theme: base, textScale: 1.2}
	assert.InDelta(t, base.Size(theme.SizeNameText)*1.2, scaled.Size(theme.SizeNameText), 0.001)
	assert.Equal(t, base.Size(theme.SizeNamePadding), scaled.Size(theme.SizeNamePadding))
}

func TestViewOptionsUpdatePreferences(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)

	u.setCompactTree(true)
	assert.True(t, a.Preferences().Bool(settingCompactTree))
	assert.True(t, u.viewCompactTree.Checked)

	u.setShowHierarchy(false)
	assert.False(t, a.Preferences().BoolWithFallback(settingShowHierarchy, true))
	assert.False(t, u.viewShowHierarchy.Checked)
	assert.True(t, u.hierarchy.Hidden)
}

func TestMakeShortCut(t *testing.T) {
	const macOS = "darwin"
	cases := []struct {
		name         string
		goos         string
		wantKey      fyne.KeyName
		wantModifier fyne.KeyModifier
		wantIsError  bool
	}{
		{"fileNew", "", fyne.KeyN, fyne.KeyModifierControl, false},
		{"fileOpen", "", fyne.KeyO, fyne.KeyModifierControl, false},
		{"fileReload", "", fyne.KeyR, fyne.KeyModifierAlt, false},
		{"fileQuit", "", fyne.KeyQ, fyne.KeyModifierControl, false},
		{"fileSettings", "", fyne.KeyComma, fyne.KeyModifierControl, false},
		{"goBottom", "", fyne.KeyEnd, fyne.KeyModifierControl, false},
		{"goTop", "", fyne.KeyHome, fyne.KeyModifierControl, false},

		{"fileNew", macOS, fyne.KeyN, fyne.KeyModifierSuper, false},
		{"fileOpen", macOS, fyne.KeyO, fyne.KeyModifierSuper, false},
		{"fileReload", macOS, fyne.KeyR, fyne.KeyModifierAlt, false},
		{"fileSettings", macOS, fyne.KeyComma, fyne.KeyModifierSuper, false},
		{"goBottom", macOS, fyne.KeyDown, fyne.KeyModifierSuper, false},
		{"goTop", macOS, fyne.KeyUp, fyne.KeyModifierSuper, false},

		{"invalid", "", fyne.KeyN, fyne.KeyModifierControl, true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s %s %v", tc.name, tc.goos, tc.wantIsError), func(t *testing.T) {
			gotShortCut, gotErr := makeShortCut(tc.name, tc.goos)
			if tc.wantIsError {
				assert.Error(t, gotErr)
			} else {
				if assert.NoError(t, gotErr) {
					assert.Equal(t, tc.wantKey, gotShortCut.KeyName)
					assert.Equal(t, tc.wantModifier, gotShortCut.Modifier)
				}
			}
		})
	}
}

func TestCanLoadDocument(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()
	x := jsondocument.MakeURIReadCloser(strings.NewReader(`{"alpha": 1}`), "dummy")
	ch := make(chan struct{})
	u.loadDocument(x, func() {
		close(ch)
	})
	<-ch
	assert.Equal(t, 2, u.document.Size())
}

func TestCanLoadJSONLinesDocument(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()
	u.jsonLinesBar.addPreviewKey("/name")
	x := jsondocument.MakeURIReadCloser(strings.NewReader("{\"name\":\"Alpha\",\"input\":{}}\n{\"name\":\"Bravo\"}\n"), "dummy.jsonl")
	ch := make(chan struct{})
	u.loadDocument(x, func() {
		close(ch)
	})
	<-ch

	assert.True(t, u.document.IsJSONLines())
	assert.False(t, u.jsonLinesBar.Hidden)
	assert.Equal(t, []string{noJSONLinesPreview, "/name"}, u.jsonLinesBar.previewKey.Options)
	assert.Equal(t, []string{"/name"}, u.app.Preferences().StringList(preferenceJSONLinesPreviewKeys))
	assert.Equal(t, 0, u.jsonLinesBar.currentRow)
	assert.Equal(t, "Row 1 of 2", u.jsonLinesBar.rowIndicator.Text)
	assert.False(t, u.hierarchy.Hidden)
	assert.Equal(t, "Hierarchy: 1", u.hierarchy.label.Text)
	first, ok := u.document.JSONLinesRowUID(0)
	assert.True(t, ok)
	assert.Equal(t, first, u.selection.selectedUID)
	for _, child := range u.document.ChildUIDs(first) {
		if u.document.Value(child).Key == "input" {
			u.tree.OnBranchOpened(child)
			assert.Equal(t, "Hierarchy: 1 -> input", u.hierarchy.label.Text)
		}
	}

	u.jsonLinesBar.previewKey.SetSelected("/name")
	assert.Equal(t, "/name", u.jsonLinesBar.selectedPreviewKey())
	node := newTreeNode()
	u.tree.UpdateNode(first, true, node)
	assert.Equal(t, "[0] — Alpha :", node.key.Text)

	u.jsonLinesBar.goToRow(1)
	second, ok := u.document.JSONLinesRowUID(1)
	assert.True(t, ok)
	assert.Equal(t, second, u.selection.selectedUID)
	assert.Equal(t, "Row 2 of 2", u.jsonLinesBar.rowIndicator.Text)
	assert.False(t, u.jsonLinesBar.previous.Disabled())
	assert.True(t, u.jsonLinesBar.next.Disabled())

	u.jsonLinesBar.previewKey.removeOption("/name")
	assert.Equal(t, []string{noJSONLinesPreview}, u.jsonLinesBar.previewKey.Options)
	assert.Empty(t, u.app.Preferences().StringList(preferenceJSONLinesPreviewKeys))
}
