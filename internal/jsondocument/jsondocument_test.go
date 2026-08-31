package jsondocument_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/ErikKalkoken/janice/internal/jsondocument"
	"github.com/stretchr/testify/assert"
)

func TestJsonDocument(t *testing.T) {
	ctx := context.TODO()
	var dummy = binding.NewUntyped()
	j := jsondocument.New()
	data := map[string]any{
		"alpha": map[string]any{"charlie": map[string]any{"delta": 1}},
		"bravo": 2,
	}
	if err := j.Load(ctx, makeDataReader(data), dummy); err != nil {
		t.Fatal(err)
	}
	ids := j.ChildUIDs("")
	alphaID := ids[0]
	bravoID := ids[1]
	ids = j.ChildUIDs(alphaID)
	charlieID := ids[0]
	ids = j.ChildUIDs(charlieID)
	deltaID := ids[0]
	t.Run("should return tree size", func(t *testing.T) {
		assert.Equal(t, 5, j.Size())
	})
	t.Run("should return true when branch", func(t *testing.T) {
		assert.True(t, j.IsBranch(alphaID))
		assert.False(t, j.IsBranch(bravoID))
	})
	t.Run("should return value of parent node", func(t *testing.T) {
		got := j.Value(alphaID)
		want := jsondocument.Node{Key: "alpha", Value: jsondocument.Empty, Type: jsondocument.Object}
		assert.Equal(t, want, got)
	})
	t.Run("should return value of child node", func(t *testing.T) {
		got := j.Value(deltaID)
		want := jsondocument.Node{Key: "delta", Value: json.Number("1"), Type: jsondocument.Number}
		assert.Equal(t, want, got)
	})
	t.Run("should return empty path for parent node", func(t *testing.T) {
		got := j.Path(alphaID)
		assert.Len(t, got, 0)
	})
	t.Run("should return path for child node", func(t *testing.T) {
		got := j.Path(deltaID)
		want := []widget.TreeNodeID{alphaID, charlieID}
		assert.Equal(t, want, got)
	})
	t.Run("should return parent of a normal node", func(t *testing.T) {
		got := j.Parent(deltaID)
		assert.Equal(t, charlieID, got)
	})
	t.Run("should return parent of a top node", func(t *testing.T) {
		got := j.Parent(alphaID)
		assert.Equal(t, "", got)
	})
}
func TestJsonDocumentLoad(t *testing.T) {
	ctx := context.TODO()
	var dummy = binding.NewUntyped()
	t.Run("can load object with values of all types and sort keys", func(t *testing.T) {
		// given
		j := jsondocument.New()
		data := map[string]any{
			"bravo":   5,
			"alpha":   "abc",
			"echo":    []any{1, 2},
			"charlie": true,
			"delta":   nil,
			"foxtrot": map[string]any{"child": 1},
		}
		// when
		err := j.Load(ctx, makeDataReader(data), dummy)
		// then
		if assert.NoError(t, err) {
			assert.Equal(t, 10, j.Size())
			for i, id := range j.ChildUIDs("") {
				node := j.Value(id)
				switch i {
				case 0:
					assert.Equal(t, node, jsondocument.Node{Key: "alpha", Value: "abc", Type: jsondocument.String})
				case 1:
					assert.Equal(t, node, jsondocument.Node{Key: "bravo", Value: json.Number("5"), Type: jsondocument.Number})
				case 2:
					assert.Equal(t, node, jsondocument.Node{Key: "charlie", Value: true, Type: jsondocument.Boolean})
				case 3:
					assert.Equal(t, node, jsondocument.Node{Key: "delta", Value: nil, Type: jsondocument.Null})
				case 4:
					assert.Equal(t, node, jsondocument.Node{Key: "echo", Value: jsondocument.Empty, Type: jsondocument.Array})
					for n, childId := range j.ChildUIDs(id) {
						node := j.Value(childId)
						switch n {
						case 0:
							assert.Equal(t, node.Value, json.Number("1"))
						case 1:
							assert.Equal(t, node.Value, json.Number("2"))
						}
					}
				case 5:
					assert.Equal(t, node, jsondocument.Node{Key: "foxtrot", Value: jsondocument.Empty, Type: jsondocument.Object})
					for n, childId := range j.ChildUIDs(id) {
						node := j.Value(childId)
						switch n {
						case 0:
							assert.Equal(t, node, jsondocument.Node{Key: "child", Value: json.Number("1"), Type: jsondocument.Number})
						}
					}
				}
			}
		}
	})
	t.Run("can load array", func(t *testing.T) {
		// given
		j := jsondocument.New()
		data := []any{"one", "two"}
		// when
		err := j.Load(ctx, makeDataReader(data), dummy)
		// then
		if assert.NoError(t, err) {
			assert.Equal(t, 3, j.Size())
			for i, id := range j.ChildUIDs("") {
				node := j.Value(id)
				switch i {
				case 0:
					assert.Equal(t, node, jsondocument.Node{Key: "[0]", Value: "one", Type: jsondocument.String})
				case 1:
					assert.Equal(t, node, jsondocument.Node{Key: "[1]", Value: "two", Type: jsondocument.String})
				}
			}
		}
	})
	t.Run("can load JSON and update progress", func(t *testing.T) {
		// given
		info := binding.NewUntyped()
		j := jsondocument.New()
		j.ProgressUpdateTick = 1
		data := []any{"one", "two"}
		// when
		err := j.Load(ctx, makeDataReader(data), info)
		// then
		if assert.NoError(t, err) {
			assert.Equal(t, 3, j.Size())
			x, err := info.Get()
			if assert.NoError(t, err) {
				p := x.(jsondocument.ProgressInfo)
				assert.Equal(t, 3, p.Size)
				assert.Equal(t, 3, p.CurrentStep)
				assert.Equal(t, 3, p.TotalSteps)
			}
		}
	})
}

func TestJSONLinesLoad(t *testing.T) {
	t.Run("loads multiple JSON values as rows", func(t *testing.T) {
		j := jsondocument.New()
		data := "{\"id\":1,\"name\":\"Alpha\",\"nested\":{\"value\":true}}\n" +
			"{\"id\":2,\"name\":\"Bravo\",\"active\":false}\n"
		err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(data), "test.jsonl"), binding.NewUntyped())
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, j.IsJSONLines())
		assert.Equal(t, 2, j.JSONLinesRowCount())

		first, ok := j.JSONLinesRowUID(0)
		if assert.True(t, ok) {
			assert.Equal(t, 0, j.JSONLinesRowIndex(first))
			preview, found := j.JSONLinesRowPreview(first, "name")
			assert.True(t, found)
			assert.Equal(t, "Alpha", preview)

			children := j.ChildUIDs(first)
			assert.NotEmpty(t, children)
			assert.Equal(t, 0, j.JSONLinesRowIndex(children[0]))
			for _, child := range children {
				if j.Value(child).Key != "nested" {
					continue
				}
				nestedChildren := j.ChildUIDs(child)
				if assert.Len(t, nestedChildren, 1) {
					path, found := j.PreviewPath(nestedChildren[0])
					assert.True(t, found)
					assert.Equal(t, "/nested/value", path)
					preview, found := j.JSONLinesRowPreview(first, path)
					assert.True(t, found)
					assert.Equal(t, "true", preview)
				}
			}
		}

		second, ok := j.JSONLinesRowUID(1)
		if assert.True(t, ok) {
			preview, found := j.JSONLinesRowPreview(second, "active")
			assert.True(t, found)
			assert.Equal(t, "false", preview)
			_, found = j.JSONLinesRowPreview(second, "missing")
			assert.False(t, found)
		}
	})

	t.Run("keeps a regular JSON array as JSON", func(t *testing.T) {
		j := jsondocument.New()
		err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(`[1, 2]`), "test.json"), binding.NewUntyped())
		assert.NoError(t, err)
		assert.False(t, j.IsJSONLines())
		assert.Equal(t, 0, j.JSONLinesRowCount())
	})

	t.Run("recognizes a single-row jsonl file by extension", func(t *testing.T) {
		j := jsondocument.New()
		err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(`{"name":"Only row"}`), "test.jsonl"), binding.NewUntyped())
		assert.NoError(t, err)
		assert.True(t, j.IsJSONLines())
		assert.Equal(t, 1, j.JSONLinesRowCount())
	})

	t.Run("reports errors in later rows", func(t *testing.T) {
		j := jsondocument.New()
		data := "{\"id\":1}\n{not json}\n"
		err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(data), "test.jsonl"), binding.NewUntyped())
		assert.Error(t, err)
	})
}

func TestJsonDocumentExtract(t *testing.T) {
	ctx := context.TODO()
	var dummy = binding.NewUntyped()
	j := jsondocument.New()
	data := map[string]any{
		"alpha": map[string]any{"charlie": map[string]any{"delta": 1}},
		"bravo": []any{1, 2, 3},
	}
	if err := j.Load(ctx, makeDataReader(data), dummy); err != nil {
		t.Fatal(err)
	}
	ids := j.ChildUIDs("")
	alphaID := ids[0]
	bravoID := ids[1]
	ids = j.ChildUIDs(alphaID)
	charlieID := ids[0]
	ids = j.ChildUIDs(charlieID)
	deltaID := ids[0]
	t.Run("can extract object", func(t *testing.T) {
		got, err := j.Extract(alphaID)
		if assert.NoError(t, err) {
			want, _ := json.Marshal(map[string]any{"charlie": map[string]any{"delta": float64(1)}})
			assert.Equal(t, want, got)
		}
	})
	t.Run("can extract array", func(t *testing.T) {
		got, err := j.Extract(bravoID)
		if assert.NoError(t, err) {
			want, _ := json.Marshal([]any{float64(1), float64(2), float64(3)})
			assert.Equal(t, want, got)
		}
	})
	t.Run("should return error when trying to extract normal value", func(t *testing.T) {
		_, err := j.Extract(deltaID)
		assert.Error(t, err)
	})
}

func TestJSONDocumentEditingAndMarshal(t *testing.T) {
	j := jsondocument.New()
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(`{"name":"Alpha","count":1,"active":true}`), "test.json"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	children := j.ChildUIDs("")
	byKey := make(map[string]string)
	for _, uid := range children {
		byKey[j.Value(uid).Key] = uid
	}
	assert.NoError(t, j.SetKey(byKey["name"], "title"))
	assert.Error(t, j.SetKey(byKey["count"], "title"))
	assert.NoError(t, j.SetScalarValue(byKey["name"], "Bravo"))
	assert.NoError(t, j.SetScalarValue(byKey["count"], "2.5"))
	assert.Error(t, j.SetScalarValue(byKey["active"], "yes"))
	assert.NoError(t, j.SetScalarValue(byKey["active"], "false"))
	text, ok := j.ScalarText(byKey["count"])
	assert.True(t, ok)
	assert.Equal(t, "2.5", text)

	got, err := j.Marshal()
	assert.NoError(t, err)
	assert.JSONEq(t, `{"title":"Bravo","count":2.5,"active":false}`, string(got))
}

func TestJSONLinesMarshal(t *testing.T) {
	j := jsondocument.New()
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader("{\"id\":1}\n{\"id\":2}\n"), "test.jsonl"), binding.NewUntyped())
	assert.NoError(t, err)
	got, err := j.Marshal()
	assert.NoError(t, err)
	assert.Equal(t, "{\"id\":1}\n{\"id\":2}\n", string(got))
}

func TestJSONType(t *testing.T) {
	cases := []struct {
		typ  jsondocument.JSONType
		name string
	}{
		{jsondocument.Array, "array"},
		{jsondocument.Boolean, "boolean"},
		{jsondocument.Null, "null"},
		{jsondocument.Number, "number"},
		{jsondocument.Object, "object"},
		{jsondocument.String, "string"},
		{jsondocument.Undefined, "undefined"},
		{jsondocument.Unknown, "unknown"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("can return name of type %T as string", tc.typ), func(t *testing.T) {
			got := fmt.Sprint(tc.typ)
			assert.Equal(t, tc.name, got)
		})
	}
}

func TestJsonDocumentSearchKey(t *testing.T) {
	ctx := context.TODO()
	var dummy = binding.NewUntyped()
	j := jsondocument.New()
	data := map[string]any{
		"alpha": []any{1, 2, 3},
		"bravo": map[string]any{
			"charlie": 5,
			"delta":   map[string]any{"echo": 1, "foxtrot": 2},
		},
		"delta": 42,
		"golf": []any{
			9,
			map[string]any{"alpha": 99, "echo": 5, "india": 9},
		},
	}
	if err := j.Load(ctx, makeDataReader(data), dummy); err != nil {
		t.Fatal(err)
	}
	ids := j.ChildUIDs("")
	alpha1ID, bravoID, delta1ID, golfID := ids[0], ids[1], ids[2], ids[3]
	ids = j.ChildUIDs(bravoID)
	charlieID, delta2ID := ids[0], ids[1]
	ids = j.ChildUIDs(delta2ID)
	echo1ID, foxtrotID := ids[0], ids[1]
	ids = j.ChildUIDs(golfID)
	ids2 := j.ChildUIDs(ids[1])
	alpha2ID, echo2ID, indiaID := ids2[0], ids2[1], ids2[2]
	cases := []struct {
		startUID   string
		key        string
		foundUID   string
		shouldFind bool
	}{
		{delta2ID, "delta", delta1ID, true},
		{echo1ID, "echo", echo2ID, true},
		{"", "alpha", alpha1ID, true},
		{"", "bravo", bravoID, true},
		{"", "charlie", charlieID, true},
		{"", "delta", delta2ID, true},
		{"", "echo", echo1ID, true},
		{"", "foxtrot", foxtrotID, true},
		{"", "golf", golfID, true},
		{"", "india", indiaID, true},
		{bravoID, "foxtrot", foxtrotID, true},
		{echo1ID, "india", indiaID, true},
		{golfID, "echo", echo2ID, true},
		{alpha1ID, "alpha", alpha2ID, true},
		{delta1ID, "delta", indiaID, false},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("can find key %s from %v (%d)", tc.key, j.Value(tc.startUID), i+1), func(t *testing.T) {
			got, err := j.Search(ctx, tc.startUID, tc.key, jsondocument.SearchKey)
			if !tc.shouldFind {
				if !assert.ErrorIs(t, err, jsondocument.ErrNotFound) {
					panic("STOP")
				}
			} else if assert.NoError(t, err) {
				assert.Equal(t, tc.foundUID, got)
			}
		})
	}

}

func TestJsonDocumentSearchValue(t *testing.T) {
	ctx := context.TODO()
	var dummy = binding.NewUntyped()
	j := jsondocument.New()
	data := map[string]any{
		"alpha": 1,
		"bravo": 2,
		"charlie": map[string]any{
			"delta": "johnny",
		},
		"foxtrot": []any{4, 5, 2},
		"golf": map[string]any{
			"hotel":    true,
			"india":    false,
			"november": nil,
		},
	}
	if err := j.Load(ctx, makeDataReader(data), dummy); err != nil {
		t.Fatal(err)
	}
	ids := j.ChildUIDs("")
	alphaID, bravoID, charlieID, foxtrotID, golfID := ids[0], ids[1], ids[2], ids[3], ids[4]
	ids = j.ChildUIDs(charlieID)
	deltaID := ids[0]
	ids = j.ChildUIDs(foxtrotID)
	foxtrotID1, foxtrotID2, foxtrotID3 := ids[0], ids[1], ids[2]
	ids = j.ChildUIDs(golfID)
	hotelID, indiaID, novemberID := ids[0], ids[1], ids[2]
	cases := []struct {
		startUID   string
		value      string
		searchType jsondocument.SearchType
		foundUID   string
		shouldFind bool
	}{
		{bravoID, "2", jsondocument.SearchNumber, foxtrotID3, true},
		{"", "1", jsondocument.SearchNumber, alphaID, true},
		{"", "2", jsondocument.SearchNumber, bravoID, true},
		{"", "johnny", jsondocument.SearchString, deltaID, true},
		{"", "4", jsondocument.SearchNumber, foxtrotID1, true},
		{"", "5", jsondocument.SearchNumber, foxtrotID2, true},
		{deltaID, "5", jsondocument.SearchNumber, foxtrotID2, true},
		{"", "true", jsondocument.SearchKeyword, hotelID, true},
		{"", "false", jsondocument.SearchKeyword, indiaID, true},
		{"", "null", jsondocument.SearchKeyword, novemberID, true},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("can find value %s from %v (%d)", tc.value, j.Value(tc.startUID), i+1), func(t *testing.T) {
			got, err := j.Search(ctx, tc.startUID, tc.value, tc.searchType)
			if !tc.shouldFind {
				assert.ErrorIs(t, err, jsondocument.ErrNotFound)
			} else if assert.NoError(t, err) {
				assert.Equal(t, tc.foundUID, got)
			}
		})
	}

}

func TestSearchPreviousAndReplace(t *testing.T) {
	j := jsondocument.New()
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(`{"first":"same","second":"same","count":1}`), "test.json"), binding.NewUntyped())
	assert.NoError(t, err)
	idsByKey := make(map[string]string)
	for _, uid := range j.ChildUIDs("") {
		idsByKey[j.Value(uid).Key] = uid
	}
	firstID, secondID, countID := idsByKey["first"], idsByKey["second"], idsByKey["count"]

	got, err := j.SearchDirection(context.Background(), secondID, "same", jsondocument.SearchString, jsondocument.SearchBackward)
	assert.NoError(t, err)
	assert.Equal(t, firstID, got)
	got, err = j.SearchDirection(context.Background(), "", "same", jsondocument.SearchString, jsondocument.SearchBackward)
	assert.NoError(t, err)
	assert.Equal(t, secondID, got)

	count, err := j.ReplaceAll(context.Background(), "same", jsondocument.SearchString, "changed")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, "changed", j.Value(firstID).Value)
	assert.NoError(t, j.Replace(countID, jsondocument.SearchNumber, "2"))
	assert.Equal(t, json.Number("2"), j.Value(countID).Value)
}

func makeDataReader(data any) fyne.URIReadCloser {
	x, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	r := bytes.NewReader(x)
	return jsondocument.MakeURIReadCloser(r, "test")
}

func TestMarshalPreservesObjectKeyOrder(t *testing.T) {
	j := jsondocument.New()
	src := `{"zebra":1,"apple":{"yankee":2,"bravo":3},"mango":[{"zulu":4,"alpha":5}],"empty":{},"none":[]}`
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(src), "test.json"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	got, err := j.Marshal()
	if !assert.NoError(t, err) {
		return
	}
	var compact bytes.Buffer
	if !assert.NoError(t, json.Compact(&compact, got)) {
		return
	}
	assert.Equal(t, src, compact.String())
}

func TestExtractPreservesObjectKeyOrder(t *testing.T) {
	j := jsondocument.New()
	src := `{"outer":{"zebra":1,"apple":2,"mango":{"yankee":3,"bravo":4}}}`
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(src), "test.json"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	got, err := j.Extract(j.ChildUIDs("")[0])
	if assert.NoError(t, err) {
		assert.Equal(t, `{"zebra":1,"apple":2,"mango":{"yankee":3,"bravo":4}}`, string(got))
	}
}

func TestJSONLinesMarshalPreservesKeyOrder(t *testing.T) {
	j := jsondocument.New()
	src := "{\"zebra\":1,\"apple\":2}\n{\"mango\":3,\"bravo\":4}\n"
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(src), "test.jsonl"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	got, err := j.Marshal()
	if assert.NoError(t, err) {
		assert.Equal(t, src, string(got))
	}
}

func TestNumbersKeepTheirOriginalLiteral(t *testing.T) {
	j := jsondocument.New()
	src := `{"id":12345678901234567890,"price":1.10,"exp":1e3,"tiny":0.30000000000000004,"neg":-0}`
	err := j.Load(context.Background(), jsondocument.MakeURIReadCloser(strings.NewReader(src), "t.json"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	want := map[string]string{
		"id":    "12345678901234567890",
		"price": "1.10",
		"exp":   "1e3",
		"tiny":  "0.30000000000000004",
		"neg":   "-0",
	}
	for _, uid := range j.ChildUIDs("") {
		key := j.Value(uid).Key
		got, ok := j.ScalarText(uid)
		if assert.True(t, ok, key) {
			assert.Equal(t, want[key], got, "literal for %s", key)
		}
	}
	out, err := j.Marshal()
	if assert.NoError(t, err) {
		var compact bytes.Buffer
		assert.NoError(t, json.Compact(&compact, out))
		assert.Equal(t, src, compact.String())
	}
}
