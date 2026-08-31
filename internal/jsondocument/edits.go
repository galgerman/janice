package jsondocument

import (
	"bytes"
	stdjson "encoding/json"
)

// nodeEdit is the replacement text for a node the user changed, held as the
// exact JSON that should appear in the saved file. Storing the encoded form
// here — rather than re-deriving it from the tree at save time — keeps the
// writer free of any knowledge of JSON value types.
type nodeEdit struct {
	key      string // encoded key text, without surrounding quotes
	value    string // encoded JSON value, e.g. `"abc"`, `1.10`, `true`
	hasKey   bool
	hasValue bool
}

func (e *nodeEdit) setKey(key string) {
	e.key, e.hasKey = key, true
}

func (e *nodeEdit) setValue(value string) {
	e.value, e.hasValue = value, true
}

// recordEdit returns the edit record for a node, creating it on first change.
func (j *JSONDocument) recordEdit(id int32) *nodeEdit {
	if j.edits == nil {
		j.edits = make(map[int32]*nodeEdit)
	}
	e, ok := j.edits[id]
	if !ok {
		e = &nodeEdit{}
		j.edits[id] = e
	}
	return e
}

// HasEdits reports whether any node has been changed since the document loaded.
func (j *JSONDocument) HasEdits() bool {
	return len(j.edits) > 0
}

func (j *JSONDocument) editsByID() map[int32]*nodeEdit {
	return j.edits
}

// encodeJSONString renders a Go string as a JSON string literal, quotes
// included.
func encodeJSONString(s string) string {
	var buf bytes.Buffer
	enc := stdjson.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""`
	}
	// Encode appends a newline.
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}

// ClearEdits forgets all recorded edits. Callers use this after saving, when
// the file on disk has caught up with the tree and becomes the new baseline for
// any further splicing.
func (j *JSONDocument) ClearEdits() {
	j.edits = nil
}
