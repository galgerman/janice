package ui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	kxtheme "github.com/ErikKalkoken/fyne-kx/theme"
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

func TestThemeUsesModernAccentAndRoundedInputs(t *testing.T) {
	th := readableDisabledTheme{Theme: theme.DefaultTheme()}
	assert.Equal(t, accentDark, th.Color(theme.ColorNamePrimary, theme.VariantDark))
	assert.Equal(t, accentLight, th.Color(theme.ColorNamePrimary, theme.VariantLight))
	assert.Equal(t, float32(8), th.Size(theme.SizeNameInputRadius))
}

func TestThemeFollowsPinnedVariantNotSystemVariant(t *testing.T) {
	pinnedDark := readableDisabledTheme{Theme: kxtheme.DefaultWithFixedVariant(theme.VariantDark)}
	assert.Equal(t, accentDark, pinnedDark.Color(theme.ColorNamePrimary, theme.VariantLight))
	assert.Equal(t, color.NRGBA{R: 0x15, G: 0x17, B: 0x1d, A: 0xff}, pinnedDark.Color(theme.ColorNameBackground, theme.VariantLight))

	pinnedLight := readableDisabledTheme{Theme: kxtheme.DefaultWithFixedVariant(theme.VariantLight)}
	assert.Equal(t, accentLight, pinnedLight.Color(theme.ColorNamePrimary, theme.VariantDark))
}

func TestChromeDrawsThemedGradientAndEdge(t *testing.T) {
	test.NewTempApp(t)
	th := fyne.CurrentApp().Settings().Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	thickness := th.Size(theme.SizeNameSeparatorThickness)

	bottom := newChrome(edgeBottom, widget.NewLabel("content")).CreateRenderer().(*chromeRenderer)
	assert.Equal(t, th.Color(theme.ColorNameMenuBackground, variant), bottom.gradient.StartColor)
	assert.Equal(t, th.Color(theme.ColorNameBackground, variant), bottom.gradient.EndColor)
	assert.Equal(t, th.Color(theme.ColorNameSeparator, variant), bottom.line.FillColor)
	bottom.Layout(fyne.NewSize(100, 40))
	assert.Equal(t, 40-thickness, bottom.line.Position().Y)

	top := newChrome(edgeTop, widget.NewLabel("content")).CreateRenderer().(*chromeRenderer)
	assert.Equal(t, th.Color(theme.ColorNameBackground, variant), top.gradient.StartColor)
	assert.Equal(t, th.Color(theme.ColorNameMenuBackground, variant), top.gradient.EndColor)
	top.Layout(fyne.NewSize(100, 40))
	assert.Equal(t, float32(0), top.line.Position().Y)
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
		{"fileSave", "", fyne.KeyS, fyne.KeyModifierControl, false},
		{"fileSaveAs", "", fyne.KeyS, fyne.KeyModifierControl | fyne.KeyModifierShift, false},
		{"fileQuit", "", fyne.KeyQ, fyne.KeyModifierControl, false},
		{"fileSettings", "", fyne.KeyComma, fyne.KeyModifierControl, false},
		{"goBottom", "", fyne.KeyEnd, fyne.KeyModifierControl, false},
		{"goTop", "", fyne.KeyHome, fyne.KeyModifierControl, false},
		{"searchFind", "", fyne.KeyF, fyne.KeyModifierControl, false},
		{"searchReplace", "", fyne.KeyH, fyne.KeyModifierControl, false},

		{"fileNew", macOS, fyne.KeyN, fyne.KeyModifierSuper, false},
		{"fileOpen", macOS, fyne.KeyO, fyne.KeyModifierSuper, false},
		{"fileReload", macOS, fyne.KeyR, fyne.KeyModifierAlt, false},
		{"fileSave", macOS, fyne.KeyS, fyne.KeyModifierSuper, false},
		{"fileSaveAs", macOS, fyne.KeyS, fyne.KeyModifierSuper | fyne.KeyModifierShift, false},
		{"fileSettings", macOS, fyne.KeyComma, fyne.KeyModifierSuper, false},
		{"goBottom", macOS, fyne.KeyDown, fyne.KeyModifierSuper, false},
		{"goTop", macOS, fyne.KeyUp, fyne.KeyModifierSuper, false},
		{"searchFind", macOS, fyne.KeyF, fyne.KeyModifierSuper, false},
		{"searchReplace", macOS, fyne.KeyH, fyne.KeyModifierSuper, false},

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

func TestFindKeyHandler(t *testing.T) {
	var directions []jsondocument.SearchDirection
	h := findKeyHandler{onFind: func(direction jsondocument.SearchDirection) {
		directions = append(directions, direction)
	}}

	h.keyDown(&fyne.KeyEvent{Name: fyne.KeyF3})
	h.keyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	h.keyDown(&fyne.KeyEvent{Name: fyne.KeyF3})
	h.keyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	h.keyDown(&fyne.KeyEvent{Name: fyne.KeyF3})

	assert.Equal(t, []jsondocument.SearchDirection{
		jsondocument.SearchForward,
		jsondocument.SearchBackward,
		jsondocument.SearchForward,
	}, directions)
}

func TestReplaceAccordionStartsClosed(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)

	assert.False(t, u.searchBar.replace.Items[0].Open)
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
	uid := u.document.ChildUIDs("")[0]
	u.selectElement(uid)
	assert.Equal(t, "alpha", u.detail.keyEntry.Text)
	assert.Equal(t, "1", u.detail.valueEntry.Text)
	assert.True(t, u.applyKeyEdit(uid, "beta"))
	assert.True(t, u.applyValueEdit(uid, "2"))
	assert.True(t, u.dirty)
	assert.Equal(t, "beta", u.detail.keyEntry.Text)
	assert.Equal(t, "2", u.detail.valueEntry.Text)
	assert.Contains(t, u.window.Title(), "*")

	u.searchBar.searchType.SetSelected(searchTypeNumber)
	u.searchBar.searchEntry.SetText("2")
	u.searchBar.replaceEntry.SetText("3")
	u.searchBar.doReplaceAll()
	assert.Equal(t, json.Number("3"), u.document.Value(uid).Value)
	assert.Equal(t, "Replaced 1 match", u.searchBar.result.Text)
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

func loadTestDocument(t *testing.T, u *UI, content, name string) {
	t.Helper()
	ch := make(chan struct{})
	u.loadDocument(jsondocument.MakeURIReadCloser(strings.NewReader(content), name), func() {
		close(ch)
	})
	<-ch
}

func TestConfirmDiscardChangesRunsActionWhenDocumentIsClean(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()
	loadTestDocument(t, u, `{"alpha": 1}`, "dummy.json")

	var ran bool
	u.confirmDiscardChanges("Discard", func() { ran = true })
	assert.True(t, ran)
}

func TestConfirmDiscardChangesWaitsForConfirmationWhenDirty(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()
	loadTestDocument(t, u, `{"alpha": 1}`, "dummy.json")
	assert.True(t, u.applyValueEdit(u.document.ChildUIDs("")[0], "2"))
	assert.True(t, u.dirty)

	var ran bool
	u.confirmDiscardChanges("Discard", func() { ran = true })
	assert.False(t, ran, "action must wait until the user confirms")
}

func TestNewFileKeepsUnsavedDocumentUntilConfirmed(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()
	loadTestDocument(t, u, `{"alpha": 1}`, "dummy.json")
	assert.True(t, u.applyValueEdit(u.document.ChildUIDs("")[0], "2"))

	u.newFile()
	assert.True(t, u.dirty, "unsaved edits must survive an unconfirmed New File")
	assert.Equal(t, 2, u.document.Size())
}

const messyDoc = "{\r\n" +
	"\t\"zebra\"   :    12345678901234567890,\r\n" +
	"\t\"apple\": {\r\n" +
	"\t\t\t\"yankee\" : 1.10\r\n" +
	"\t},\r\n" +
	"    \"mango\" : [ 1e3,   true ]\r\n" +
	"}\r\n\r\n"

func loadFileIntoUI(t *testing.T, u *UI, path string) {
	t.Helper()
	reader, err := storage.Reader(storage.NewFileURI(path))
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan struct{})
	u.loadDocument(reader, func() { close(ch) })
	<-ch
}

func TestSaveRewritesOnlyTheEditedBytes(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()

	path := filepath.Join(t.TempDir(), "doc.json")
	assert.NoError(t, os.WriteFile(path, []byte(messyDoc), 0o644))
	loadFileIntoUI(t, u, path)

	apple := u.document.ChildUIDs("")[1]
	yankee := u.document.ChildUIDs(apple)[0]
	assert.True(t, u.applyValueEdit(yankee, "2.20"))

	u.saveFile()

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, strings.Replace(messyDoc, "1.10", "2.20", 1), string(got))
	assert.False(t, u.dirty)
	assert.False(t, u.document.HasEdits())
}

func TestSaveWithoutEditsLeavesTheFileByteIdentical(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()

	path := filepath.Join(t.TempDir(), "doc.json")
	assert.NoError(t, os.WriteFile(path, []byte(messyDoc), 0o644))
	loadFileIntoUI(t, u, path)

	u.saveFile()

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, messyDoc, string(got))
}

func TestSecondSaveSplicesAgainstTheAlreadySavedFile(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()

	path := filepath.Join(t.TempDir(), "doc.json")
	assert.NoError(t, os.WriteFile(path, []byte(messyDoc), 0o644))
	loadFileIntoUI(t, u, path)

	apple := u.document.ChildUIDs("")[1]
	assert.True(t, u.applyValueEdit(u.document.ChildUIDs(apple)[0], "2.20"))
	u.saveFile()

	// A second, unrelated edit must splice into the file saved a moment ago.
	assert.True(t, u.applyKeyEdit(u.document.ChildUIDs("")[0], "giraffe"))
	u.saveFile()

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	want := strings.Replace(messyDoc, "1.10", "2.20", 1)
	want = strings.Replace(want, `"zebra"`, `"giraffe"`, 1)
	assert.Equal(t, want, string(got))
	assert.False(t, u.dirty)
}

func TestSaveJSONLinesTouchesOnlyTheEditedRow(t *testing.T) {
	a := test.NewTempApp(t)
	u, err := NewUI(a)
	assert.NoError(t, err)
	u.window.Show()

	const rows = "{\"id\": 1,  \"msg\":\"a\"}\r\n" +
		"{\"id\":2,\"ms\" : 0.30000000000000004}\n" +
		"{\"id\":3}\n"
	path := filepath.Join(t.TempDir(), "doc.jsonl")
	assert.NoError(t, os.WriteFile(path, []byte(rows), 0o644))
	loadFileIntoUI(t, u, path)

	row2, ok := u.document.JSONLinesRowUID(1)
	assert.True(t, ok)
	assert.True(t, u.applyValueEdit(u.document.ChildUIDs(row2)[1], "0.5"))
	u.saveFile()

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, strings.Replace(rows, "0.30000000000000004", "0.5", 1), string(got))
}
