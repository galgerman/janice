package jsondocument

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

// Size of the read buffer used while decoding a document.
const decodeBufferSize = 64 * 1024

// orderedObject is a JSON object that remembers the key order of the source
// document. Unmarshalling into a map discards that order, which would make it
// impossible to write a document back the way it was read.
// Keys and values are held in parallel slices indexed together.
type orderedObject struct {
	keys   []string
	values []any
}

func (o *orderedObject) add(key string, value any) {
	o.keys = append(o.keys, key)
	o.values = append(o.values, value)
}

// newDecoder returns an iterator for reading successive JSON values from r.
func newDecoder(r io.Reader) *jsoniter.Iterator {
	return jsoniter.Parse(json, r, decodeBufferSize)
}

// decodeOrdered reads the next JSON value from iter. Objects are returned as
// [orderedObject], everything else matches what the standard library produces.
// It returns [io.EOF] once the stream is exhausted, so callers can read the
// successive values of a JSON Lines stream.
func decodeOrdered(iter *jsoniter.Iterator) (any, error) {
	next := iter.WhatIsNext()
	if iter.Error != nil {
		return nil, iter.Error // io.EOF once the stream is fully consumed
	}
	if next == jsoniter.InvalidValue {
		return nil, fmt.Errorf("invalid JSON value")
	}
	data := readOrdered(iter)
	// Reaching the end of the reader while completing the final value is normal.
	if iter.Error != nil && !errors.Is(iter.Error, io.EOF) {
		return nil, iter.Error
	}
	return data, nil
}

func readOrdered(iter *jsoniter.Iterator) any {
	switch iter.WhatIsNext() {
	case jsoniter.ObjectValue:
		o := &orderedObject{}
		iter.ReadObjectCB(func(it *jsoniter.Iterator, key string) bool {
			o.add(key, readOrdered(it))
			return true
		})
		return o
	case jsoniter.ArrayValue:
		a := make([]any, 0)
		iter.ReadArrayCB(func(it *jsoniter.Iterator) bool {
			a = append(a, readOrdered(it))
			return true
		})
		return a
	case jsoniter.StringValue:
		return iter.ReadString()
	case jsoniter.NumberValue:
		return iter.ReadNumber()
	case jsoniter.BoolValue:
		return iter.ReadBool()
	case jsoniter.NilValue:
		iter.ReadNil()
		return nil
	default:
		iter.ReportError("readOrdered", "unrecognized JSON value")
		return nil
	}
}

// isJSONNumber reports whether text is a valid JSON number literal.
func isJSONNumber(text string) bool {
	if text == "" {
		return false
	}
	d := stdjson.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return false
	}
	if _, ok := v.(stdjson.Number); !ok {
		return false
	}
	return d.More() == false && d.InputOffset() == int64(len(text))
}
