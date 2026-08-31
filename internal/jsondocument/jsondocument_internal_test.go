package jsondocument

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestLoadFile(t *testing.T) {
	ctx := context.Background()
	test.NewTempApp(t) // calling Fyne features requires a Fyne app to exist
	t.Run("should return load and unmarshaled data from stream", func(t *testing.T) {
		// given
		dat := []byte(`{"alpha":"two"}`)
		r := MakeURIReadCloser(bytes.NewReader(dat), "test")
		j := New()
		// when
		got, err := j.load(ctx, r)
		// then
		if assert.NoError(t, err) {
			want := &orderedObject{keys: []string{"alpha"}, values: []any{"two"}}
			assert.Equal(t, want, got)
		}

	})
	t.Run("should return error when stream can not be unmarshaled", func(t *testing.T) {
		// given
		r := MakeURIReadCloser(strings.NewReader("invalid JSON"), "test")
		j := New()
		// when
		_, err := j.load(ctx, r)
		// then
		assert.Error(t, err)
	})
}

func TestAddNode(t *testing.T) {
	ctx := context.Background()
	test.NewTempApp(t) // calling Fyne features requires a Fyne app to exist
	t.Run("can add root node", func(t *testing.T) {
		j := New()
		j.initialize(10)
		id, err := j.addNode(ctx, -1, "", Empty, Array)
		if assert.NoError(t, err) {
			assert.Equal(t, 1, j.Size())
			assert.Equal(t, Node{Key: "", Value: Empty, Type: Array}, j.values[id])
		}
	})
	t.Run("can add valid parent node", func(t *testing.T) {
		j := New()
		j.initialize(10)
		j.addNode(ctx, -1, "", Empty, Array)
		id, err := j.addNode(ctx, 0, "alpha", "one", String)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, j.Size())
			assert.Equal(t, Node{Key: "alpha", Value: "one", Type: String}, j.values[id])
		}
	})
	t.Run("can add valid child node", func(t *testing.T) {
		j := New()
		j.initialize(10)
		j.addNode(ctx, -1, "", Empty, Array)
		id1, _ := j.addNode(ctx, 0, "alpha", "one", String)
		id2, err := j.addNode(ctx, id1, "bravo", "two", String)
		if assert.NoError(t, err) {
			assert.Equal(t, 3, j.Size())
			assert.Equal(t, Node{Key: "bravo", Value: "two", Type: String}, j.values[id2])
		}
	})
	t.Run("should return error when parent UID does not exist", func(t *testing.T) {
		j := New()
		j.initialize(10)
		j.addNode(ctx, -1, "", Empty, Array)
		_, err := j.addNode(ctx, 5, "alpha", "one", String)
		assert.Error(t, err)
	})
}

func TestWildcard2Regex(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"test*", "^test.*$"},
		{"*test", "^.*test$"},
		{"test", "^test$"},
		{"first*second", "^first.*second$"},
	}
	for _, tc := range cases {
		got := wildCardToRegexp(tc.in)
		assert.Equal(t, tc.want, got)
	}
}

// func TestMemoryUsage(t *testing.T) {
// 	size := int32(1_000_000)
// 	ctx := context.Background()
// 	PrintMemUsage()
// 	j := New()
// 	j.initialize(size + 1)
// 	root, _ := j.addNode(ctx, -1, "", Empty, Array)
// 	for i := range size {
// 		k := strconv.Itoa(int(i))
// 		j.addNode(ctx, root, k, i, Number)
// 	}
// 	PrintMemUsage()
// 	t.Fail()
// }

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// Search walks node ids in numeric order instead of materializing a flattened
// tree. That is only equivalent while ids are assigned depth-first, so guard
// the invariant here.
func TestNodeIDsFollowDepthFirstOrder(t *testing.T) {
	j := New()
	src := `{"b":{"d":1,"c":[10,{"z":2}]},"a":[{"y":3}],"e":null}`
	err := j.Load(context.Background(), MakeURIReadCloser(strings.NewReader(src), "t.json"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	var depthFirst func(id int32) []int32
	depthFirst = func(id int32) []int32 {
		result := []int32{id}
		for _, childID := range j.ids[id] {
			result = append(result, depthFirst(childID)...)
		}
		return result
	}
	want := make([]int32, j.n)
	for i := range want {
		want[i] = int32(i)
	}
	assert.Equal(t, want, depthFirst(0))
}

// JSONLinesRowIndex binary searches the row ids, which requires them ascending.
func TestJSONLinesRowIDsAscend(t *testing.T) {
	j := New()
	src := "{\"a\":1}\n{\"b\":{\"c\":2}}\n{\"d\":[3,4]}\n"
	err := j.Load(context.Background(), MakeURIReadCloser(strings.NewReader(src), "t.jsonl"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, slices.IsSorted(j.ids[0]), "row ids must ascend, got %v", j.ids[0])
	for row := range j.ids[0] {
		uid, ok := j.JSONLinesRowUID(row)
		if assert.True(t, ok) {
			assert.Equal(t, row, j.JSONLinesRowIndex(uid))
		}
	}
}

func TestDocumentRecordsEdits(t *testing.T) {
	j := New()
	src := `{"alpha":1,"bravo":"x"}`
	err := j.Load(context.Background(), MakeURIReadCloser(strings.NewReader(src), "t.json"), binding.NewUntyped())
	if !assert.NoError(t, err) {
		return
	}
	assert.False(t, j.HasEdits())

	alpha := j.ChildUIDs("")[0]
	bravo := j.ChildUIDs("")[1]
	assert.NoError(t, j.SetScalarValue(alpha, "42"))
	assert.NoError(t, j.SetKey(bravo, "charlie"))
	assert.True(t, j.HasEdits())

	edits := j.editsByID()
	assert.Len(t, edits, 2)

	valueEdit := edits[uid2id(alpha)]
	assert.True(t, valueEdit.hasValue)
	assert.Equal(t, "42", valueEdit.value)
	assert.False(t, valueEdit.hasKey)

	keyEdit := edits[uid2id(bravo)]
	assert.True(t, keyEdit.hasKey)
	assert.Equal(t, "charlie", keyEdit.key)
	assert.False(t, keyEdit.hasValue)

	// Reloading a document starts from a clean slate.
	err = j.Load(context.Background(), MakeURIReadCloser(strings.NewReader(src), "t.json"), binding.NewUntyped())
	assert.NoError(t, err)
	assert.False(t, j.HasEdits())
}
