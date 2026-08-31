package jsondocument

import (
	"bufio"
	"fmt"
	"io"
)

// WriteSpliced copies the original document from src to dst, substituting only
// the keys and values the user edited. Every other byte — indentation, spacing,
// line endings, number formatting, escape sequences, key order — is passed
// through untouched, so a saved file differs from the original only where it
// was actually edited.
//
// src must be the document this tree was loaded from. The walk re-derives node
// ids from the source and checks each one against the tree, so a source that
// has drifted is reported as an error rather than silently mis-spliced.
func (j *JSONDocument) WriteSpliced(src io.Reader, dst io.Writer) error {
	if j.n == 0 {
		return fmt.Errorf("document is empty")
	}
	w := &splicer{
		src:   bufio.NewReaderSize(src, 64*1024),
		dst:   bufio.NewWriterSize(dst, 64*1024),
		doc:   j,
		edits: j.edits,
	}
	if err := w.run(); err != nil {
		return err
	}
	return w.dst.Flush()
}

// splicer walks the source document in the same depth-first order the loader
// used, so the id it assigns to each value is the id that value has in the
// tree. That correspondence is what lets an edit recorded against a node id be
// applied to the right bytes without storing any offsets during load.
type splicer struct {
	src   *bufio.Reader
	dst   *bufio.Writer
	doc   *JSONDocument
	edits map[int32]*nodeEdit
	next  int32 // id of the next value to be entered
}

func (w *splicer) run() error {
	if w.doc.isJSONLines {
		// The array holding the rows is synthetic: it owns id 0 but occupies no
		// bytes in the source, which is a stream of separate top level values.
		w.next++
		for {
			done, err := w.atEOF()
			if err != nil {
				return err
			}
			if done {
				break
			}
			if err := w.value(w.take()); err != nil {
				return err
			}
		}
	} else {
		if err := w.leadingSpace(); err != nil {
			return err
		}
		if err := w.value(w.take()); err != nil {
			return err
		}
		if err := w.trailingSpace(); err != nil {
			return err
		}
	}
	if w.next != w.doc.n {
		return fmt.Errorf("source no longer matches the loaded document: found %d nodes, expected %d", w.next, w.doc.n)
	}
	return nil
}

// take reserves the next node id in depth-first order.
func (w *splicer) take() int32 {
	id := w.next
	w.next++
	return id
}

// atEOF copies any run of whitespace and reports whether the source is spent.
func (w *splicer) atEOF() (bool, error) {
	if err := w.leadingSpace(); err != nil {
		return false, err
	}
	if _, err := w.src.Peek(1); err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}

// leadingSpace copies whitespace through verbatim.
func (w *splicer) leadingSpace() error {
	for {
		b, err := w.src.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !isSpace(b) {
			return w.src.UnreadByte()
		}
		if err := w.dst.WriteByte(b); err != nil {
			return err
		}
	}
}

func (w *splicer) trailingSpace() error {
	if err := w.leadingSpace(); err != nil {
		return err
	}
	if _, err := w.src.Peek(1); err == nil {
		return fmt.Errorf("unexpected trailing content in source")
	}
	return nil
}

// value walks one JSON value, which the tree records under id.
func (w *splicer) value(id int32) error {
	b, err := w.src.Peek(1)
	if err != nil {
		return fmt.Errorf("unexpected end of source: %w", err)
	}
	kind, err := kindOf(b[0])
	if err != nil {
		return err
	}
	if int(id) >= len(w.doc.values) || w.doc.values[id].Type != kind {
		return fmt.Errorf("source no longer matches the loaded document at node %d", id)
	}
	switch kind {
	case Object:
		return w.object(id)
	case Array:
		return w.array(id)
	}
	// A scalar is the only thing an edit can replace outright.
	if e := w.edits[id]; e != nil && e.hasValue {
		if err := w.scalar(false); err != nil {
			return err
		}
		_, err := w.dst.WriteString(e.value)
		return err
	}
	return w.scalar(true)
}

func (w *splicer) object(id int32) error {
	if err := w.byteThrough(); err != nil { // '{'
		return err
	}
	for {
		if err := w.leadingSpace(); err != nil {
			return err
		}
		b, err := w.src.Peek(1)
		if err != nil {
			return fmt.Errorf("unterminated object: %w", err)
		}
		if b[0] == '}' {
			return w.byteThrough()
		}
		if b[0] == ',' {
			if err := w.byteThrough(); err != nil {
				return err
			}
			continue
		}
		if b[0] != '"' {
			return fmt.Errorf("expected object key, found %q", b[0])
		}
		memberID := w.take()
		if e := w.edits[memberID]; e != nil && e.hasKey {
			if err := w.stringToken(false); err != nil {
				return err
			}
			if _, err := w.dst.WriteString(encodeJSONString(e.key)); err != nil {
				return err
			}
		} else if err := w.stringToken(true); err != nil {
			return err
		}
		if err := w.leadingSpace(); err != nil {
			return err
		}
		b, err = w.src.Peek(1)
		if err != nil || b[0] != ':' {
			return fmt.Errorf("expected ':' after object key")
		}
		if err := w.byteThrough(); err != nil {
			return err
		}
		if err := w.leadingSpace(); err != nil {
			return err
		}
		if err := w.value(memberID); err != nil {
			return err
		}
	}
}

func (w *splicer) array(id int32) error {
	if err := w.byteThrough(); err != nil { // '['
		return err
	}
	for {
		if err := w.leadingSpace(); err != nil {
			return err
		}
		b, err := w.src.Peek(1)
		if err != nil {
			return fmt.Errorf("unterminated array: %w", err)
		}
		switch b[0] {
		case ']':
			return w.byteThrough()
		case ',':
			if err := w.byteThrough(); err != nil {
				return err
			}
			continue
		}
		if err := w.value(w.take()); err != nil {
			return err
		}
	}
}

// scalar consumes one string, number, boolean or null token, copying it to the
// output only when emit is set.
func (w *splicer) scalar(emit bool) error {
	b, err := w.src.Peek(1)
	if err != nil {
		return err
	}
	if b[0] == '"' {
		return w.stringToken(emit)
	}
	for {
		c, err := w.src.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if isSpace(c) || c == ',' || c == '}' || c == ']' {
			return w.src.UnreadByte()
		}
		if emit {
			if err := w.dst.WriteByte(c); err != nil {
				return err
			}
		}
	}
}

// stringToken consumes a quoted string including its escapes.
func (w *splicer) stringToken(emit bool) error {
	c, err := w.src.ReadByte()
	if err != nil {
		return err
	}
	if c != '"' {
		return fmt.Errorf("expected string, found %q", c)
	}
	if emit {
		if err := w.dst.WriteByte(c); err != nil {
			return err
		}
	}
	escaped := false
	for {
		c, err := w.src.ReadByte()
		if err != nil {
			return fmt.Errorf("unterminated string: %w", err)
		}
		if emit {
			if err := w.dst.WriteByte(c); err != nil {
				return err
			}
		}
		if escaped {
			escaped = false
			continue
		}
		switch c {
		case '\\':
			escaped = true
		case '"':
			return nil
		}
	}
}

// byteThrough copies exactly one byte.
func (w *splicer) byteThrough() error {
	c, err := w.src.ReadByte()
	if err != nil {
		return err
	}
	return w.dst.WriteByte(c)
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// kindOf maps the first byte of a value to its JSON type.
func kindOf(b byte) (JSONType, error) {
	switch {
	case b == '{':
		return Object, nil
	case b == '[':
		return Array, nil
	case b == '"':
		return String, nil
	case b == 't' || b == 'f':
		return Boolean, nil
	case b == 'n':
		return Null, nil
	case b == '-' || (b >= '0' && b <= '9'):
		return Number, nil
	}
	return Undefined, fmt.Errorf("unexpected character %q in source", b)
}
