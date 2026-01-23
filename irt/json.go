package irt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"iter"
)

// MarshalJSON consumes a sequence and returns its JSON encoding as an
// array. Returns an error if any element fails to marshal.
func MarshalJSON[T any](seq iter.Seq[T]) ([]byte, error) {
	var buf bytes.Buffer

	for value := range seq {
		if buf.Len() == 0 {
			must2(buf.WriteString("["))
		} else {
			must(buf.WriteByte(','))
		}

		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}

		must2(buf.Write(data))
	}
	if buf.Len() > 0 {
		must2(buf.WriteString("]"))
	} else {
		must2(buf.WriteString("[]"))
	}

	return buf.Bytes(), nil
}

// MarshalJSON2 consumes a pair sequence with string keys and
// returns its JSON encoding as an object. Returns an error if any
// element fails to marshal.
func MarshalJSON2[A ~string, B any](seq iter.Seq2[A, B]) ([]byte, error) {
	var buf bytes.Buffer

	for k, v := range seq {
		if buf.Len() == 0 {
			must2(buf.WriteString("{"))
		} else {
			must(buf.WriteByte(','))
		}

		must2(buf.Write(must2(json.Marshal(k))))
		must(buf.WriteByte(':'))

		value, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		must2(buf.Write(value))
	}
	if buf.Len() > 0 {
		must2(buf.WriteString("}"))
	} else {
		must2(buf.WriteString("{}"))
	}

	return buf.Bytes(), nil
}

// UnmarshalJSON decodes a JSON stream from a reader and yields each
// element with a potential error. The reader is wrapped with bufio for
// efficient reading.
func UnmarshalJSON[T any](data io.Reader) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T

		dec := json.NewDecoder(bufio.NewReader(data))

		// Read opening bracket
		if t, err := dec.Token(); err != nil {
			yield(zero, err)
			return
		} else if delim, ok := t.(json.Delim); !ok || delim != '[' {
			yield(zero, &json.SyntaxError{Offset: dec.InputOffset()})
			return
		}

		// Decode elements until we reach the end of the array
		for dec.More() {
			var elem T
			if err := dec.Decode(&elem); err != nil {
				yield(zero, err)
				return
			}

			if !yield(elem, nil) {
				return
			}
		}

		// Read closing bracket
		if _, err := dec.Token(); err != nil {
			yield(zero, err)
		}
	}
}

func castOk[T any](in any) (out T, ok bool) { out, ok = in.(T); return }
func cast[T any](in any) T                  { return first(castOk[T](in)) }

// UnmarshalJSON2 decodes a JSON object from the reader and yields each
// key-value pair as KV with a potential error. The reader is wrapped
// with bufio for efficient reading.
func UnmarshalJSON2[A ~string, B any](data io.Reader) iter.Seq2[KV[A, B], error] {
	return func(yield func(KV[A, B], error) bool) {
		var zero KV[A, B]

		dec := json.NewDecoder(bufio.NewReader(data))

		// Read opening brace
		t, err := dec.Token()
		if err != nil {
			yield(zero, err)
			return
		}

		// Check if it's the start of an object
		if delim, ok := t.(json.Delim); !ok || delim != '{' {
			yield(zero, &json.SyntaxError{Offset: dec.InputOffset()})
			return
		}

		// Decode key-value pairs until we reach the end of the object
		for dec.More() {
			// Decode the key
			t, err := dec.Token()
			if err != nil {
				yield(zero, err)
				return
			}

			key := cast[string](t)

			// Decode the value
			var value B
			if err := dec.Decode(&value); err != nil {
				yield(zero, err)
				return
			}

			if !yield(MakeKV(A(key), value), nil) {
				return
			}
		}
		// we don't have to read the closing brace: more only returns false when the next
		// character is a closing brace, so this can't error, unless Peek is broken
	}
}
