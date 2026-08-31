package jsondocument

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"fyne.io/fyne/v2/data/binding"
	"github.com/stretchr/testify/assert"
)

func loadForSplice(t *testing.T, src, name string) *JSONDocument {
	t.Helper()
	j := New()
	err := j.Load(context.Background(), MakeURIReadCloser(strings.NewReader(src), name), binding.NewUntyped())
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func splice(t *testing.T, j *JSONDocument, src string) string {
	t.Helper()
	var out bytes.Buffer
	if err := j.WriteSpliced(strings.NewReader(src), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// The awkward formatting here is the point: odd indentation, tabs, CRLF, a
// trailing blank line, unicode escapes and numbers that no float round-trip
// would survive.
const messyJSON = "{\r\n" +
	"\t\"zebra\"   :    12345678901234567890,\r\n" +
	"\t\"apple\": {\r\n" +
	"\t\t\t\"yankee\" : 1.10,\r\n" +
	"\t\t\t\"bravo\"  : \"caf\\u00e9\"\r\n" +
	"\t},\r\n" +
	"    \"mango\" : [ 1e3,   true ,null ],\r\n" +
	"\t\"empty\": {}\r\n" +
	"}\r\n\r\n"

func TestSpliceWithoutEditsIsByteIdentical(t *testing.T) {
	j := loadForSplice(t, messyJSON, "t.json")
	assert.Equal(t, messyJSON, splice(t, j, messyJSON))
}

func TestSpliceReplacesOnlyTheEditedValue(t *testing.T) {
	j := loadForSplice(t, messyJSON, "t.json")
	apple := j.ChildUIDs("")[1]
	yankee := j.ChildUIDs(apple)[0]
	assert.NoError(t, j.SetScalarValue(yankee, "2.20"))

	want := strings.Replace(messyJSON, "1.10", "2.20", 1)
	assert.Equal(t, want, splice(t, j, messyJSON))
}

func TestSpliceReplacesOnlyTheEditedKey(t *testing.T) {
	j := loadForSplice(t, messyJSON, "t.json")
	zebra := j.ChildUIDs("")[0]
	assert.NoError(t, j.SetKey(zebra, "giraffe"))

	want := strings.Replace(messyJSON, `"zebra"`, `"giraffe"`, 1)
	assert.Equal(t, want, splice(t, j, messyJSON))
}

func TestSpliceEditsNestedArrayElement(t *testing.T) {
	j := loadForSplice(t, messyJSON, "t.json")
	mango := j.ChildUIDs("")[2]
	assert.NoError(t, j.SetScalarValue(j.ChildUIDs(mango)[1], "false"))

	want := strings.Replace(messyJSON, "true ,null", "false ,null", 1)
	assert.Equal(t, want, splice(t, j, messyJSON))
}

func TestSpliceEscapesEditedStrings(t *testing.T) {
	j := loadForSplice(t, messyJSON, "t.json")
	apple := j.ChildUIDs("")[1]
	bravo := j.ChildUIDs(apple)[1]
	assert.NoError(t, j.SetScalarValue(bravo, "a\"b\\c\nd"))

	got := splice(t, j, messyJSON)
	assert.Contains(t, got, `"a\"b\\c\nd"`)
	// The untouched escape sequence on the neighbouring line is left alone.
	assert.Contains(t, got, "12345678901234567890")
}

const messyJSONL = "{\"id\": 1,  \"msg\":\"a\"}\r\n" +
	"{\"id\":2,\"meta\":{ \"ms\" : 0.30000000000000004 }}\n" +
	"{\"id\":3,\"msg\" : \"c\"}\n"

func TestSpliceJSONLinesTouchesOnlyTheEditedRow(t *testing.T) {
	j := loadForSplice(t, messyJSONL, "t.jsonl")
	assert.Equal(t, messyJSONL, splice(t, j, messyJSONL), "no edits must round-trip exactly")

	row2, ok := j.JSONLinesRowUID(1)
	assert.True(t, ok)
	meta := j.ChildUIDs(row2)[1]
	assert.NoError(t, j.SetScalarValue(j.ChildUIDs(meta)[0], "0.5"))

	want := strings.Replace(messyJSONL, "0.30000000000000004", "0.5", 1)
	assert.Equal(t, want, splice(t, j, messyJSONL))
}

func TestSpliceRejectsASourceThatNoLongerMatches(t *testing.T) {
	j := loadForSplice(t, messyJSON, "t.json")
	zebra := j.ChildUIDs("")[0]
	assert.NoError(t, j.SetScalarValue(zebra, "1"))

	var out bytes.Buffer
	err := j.WriteSpliced(strings.NewReader(`{"zebra": 1, "extra": 2}`), &out)
	assert.Error(t, err, "a source that drifted from the loaded tree must be refused")
}

func TestSpliceHandlesAwkwardShapes(t *testing.T) {
	cases := map[string]string{
		"top level array":       "[ 1,\t2 ,\r\n  {\"a\":[]} ]\n",
		"empty containers":      "{\"a\":{},\"b\":[],\"c\":{\"d\":[{}]}}",
		"deep nesting":          "[[[[[[[[[[1]]]]]]]]]]",
		"escapes and unicode":   `{"a":"\"\\\/\b\f\n\r\t\u00e9","b":"caf\u00e9 \u2713"}`,
		"no trailing newline":   "{\"a\":1}",
		"exotic numbers":        "[0,-0,1e3,1E-3,-1.5e+10,0.30000000000000004,12345678901234567890]",
		"whitespace everywhere": "  {\n\n \"a\"\t:\r\n [ \t1 , 2 ]  \n }  \n ",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			j := loadForSplice(t, src, "t.json")
			assert.Equal(t, src, splice(t, j, src))
		})
	}
}

func TestSpliceJSONLinesAwkwardShapes(t *testing.T) {
	cases := map[string]string{
		"no trailing newline": "{\"a\":1}\n{\"b\":2}",
		"blank lines between": "{\"a\":1}\n\n\n{\"b\":2}\n",
		"crlf throughout":     "{\"a\":1}\r\n{\"b\":2}\r\n",
		"arrays as rows":      "[1,2]\n[3]\n",
		"leading whitespace":  "  {\"a\":1}\n  {\"b\":2}\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			j := loadForSplice(t, src, "t.jsonl")
			assert.Equal(t, src, splice(t, j, src))
		})
	}
}
