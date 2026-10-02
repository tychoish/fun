package irt

import (
	"bufio"
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"reflect"
	"strconv"
)

// encodeJSONKey encodes k as a JSON object key (always a quoted
// string), following the rules encoding/json uses for map keys:
// strings and encoding.TextMarshaler values are encoded as strings and
// integer kinds are stringified. Any other key type is an error.
func encodeJSONKey(k any) ([]byte, error) {
	out, err := json.Marshal(k)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 && out[0] == '"' {
		return out, nil
	}
	switch reflect.ValueOf(k).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.AppendQuote(nil, string(out)), nil
	}
	return nil, fmt.Errorf("json: unsupported key type %T", k)
}

// trailingNewlineStripper strips the single trailing '\n' that json.Encoder
// appends after every Encode call, so that the encoded value can be embedded
// inside a larger JSON structure without an unwanted newline.
type trailingNewlineStripper struct{ w io.Writer }

func (s trailingNewlineStripper) Write(p []byte) (int, error) {
	if len(p) > 0 && p[len(p)-1] == '\n' {
		n, err := s.w.Write(p[:len(p)-1])
		return n + 1, err // report full length to avoid a spurious short-write error
	}
	return s.w.Write(p)
}

// MarshalToJSON encodes a sequence as a JSON array, writing directly to w.
// Returns an error if any element fails to marshal or if the writer fails.
// Callers that need buffered output should wrap w in a bufio.Writer.
func MarshalToJSON[T any](seq iter.Seq[T], w io.Writer) error {
	enc := json.NewEncoder(trailingNewlineStripper{w})
	first := true
	for value := range seq {
		if first {
			if _, err := io.WriteString(w, "["); err != nil {
				return err
			}
			first = false
		} else {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := enc.Encode(value); err != nil {
			return err
		}
	}
	var suffix string
	if first {
		suffix = "[]"
	} else {
		suffix = "]"
	}
	_, err := io.WriteString(w, suffix)
	return err
}

// MarshalToJSON2 encodes a pair sequence as a JSON object, writing directly to w.
// Returns an error if any key or value fails to marshal or if the writer fails.
// Callers that need buffered output should wrap w in a bufio.Writer.
func MarshalToJSON2[K any, V any](seq iter.Seq2[K, V], w io.Writer) error {
	enc := json.NewEncoder(trailingNewlineStripper{w})
	first := true
	for k, v := range seq {
		if first {
			if _, err := io.WriteString(w, "{"); err != nil {
				return err
			}
			first = false
		} else {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		key, err := encodeJSONKey(k)
		if err != nil {
			return err
		}
		if _, err := w.Write(key); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ":"); err != nil {
			return err
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
	}
	suffix := "}"
	if first {
		suffix = "{}"
	}
	_, err := io.WriteString(w, suffix)
	return err
}

// MarshalJSONL encodes a sequence in JSON Lines format: one JSON value per
// line, with each line terminated by a newline character. Returns an error if
// any element fails to marshal.
func MarshalJSONL[T any](seq iter.Seq[T]) ([]byte, error) {
	var buf bytes.Buffer
	if err := MarshalToJSONL(seq, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalToJSONL encodes a sequence in JSON Lines format, writing directly to
// w. Each element is written as a JSON value followed by a newline. Returns an
// error if any element fails to marshal or if the writer fails.
// Callers that need buffered output should wrap w in a bufio.Writer.
func MarshalToJSONL[T any](seq iter.Seq[T], w io.Writer) error {
	enc := json.NewEncoder(w)
	for value := range seq {
		if err := enc.Encode(value); err != nil {
			return err
		}
	}
	return nil
}

// MarshalJSON consumes a sequence and returns its JSON encoding as an
// array. Returns an error if any element fails to marshal.
func MarshalJSON[T any](seq iter.Seq[T]) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(trailingNewlineStripper{&buf})

	for value := range seq {
		if buf.Len() == 0 {
			must2(buf.WriteString("["))
		} else {
			must(buf.WriteByte(','))
		}

		if err := enc.Encode(value); err != nil {
			return nil, err
		}
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
func MarshalJSON2[A any, B any](seq iter.Seq2[A, B]) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(trailingNewlineStripper{&buf})

	for k, v := range seq {
		if buf.Len() == 0 {
			must2(buf.WriteString("{"))
		} else {
			must(buf.WriteByte(','))
		}

		key, err := encodeJSONKey(k)
		if err != nil {
			return nil, err
		}
		must2(buf.Write(key))
		must(buf.WriteByte(':'))

		if err := enc.Encode(v); err != nil {
			return nil, err
		}
	}
	if buf.Len() > 0 {
		must2(buf.WriteString("}"))
	} else {
		must2(buf.WriteString("{}"))
	}

	return buf.Bytes(), nil
}

// decodeJSONKey converts a JSON object key into A following the rules
// encoding/json uses for map keys: string kinds, integer kinds and
// types implementing encoding.TextUnmarshaler (on *A) are supported;
// interface types receive the key as a string. Anything else, or a key
// that cannot be parsed, is an error.
func decodeJSONKey[A any](text string) (key A, err error) {
	val := reflect.ValueOf(&key).Elem()
	kind := val.Kind()

	if tu, ok := any(&key).(encoding.TextUnmarshaler); ok && kind != reflect.String {
		return key, tu.UnmarshalText([]byte(text))
	}

	switch kind {
	case reflect.String:
		val.SetString(text)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(text, 10, val.Type().Bits())
		if err != nil {
			return key, err
		}
		val.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(text, 10, val.Type().Bits())
		if err != nil {
			return key, err
		}
		val.SetUint(n)
	case reflect.Interface:
		if val.NumMethod() != 0 {
			return key, fmt.Errorf("json: unsupported key type %T", key)
		}
		val.Set(reflect.ValueOf(text))
	default:
		return key, fmt.Errorf("json: unsupported key type %T", key)
	}
	return key, nil
}

// checkJSONEnd reports an error if anything other than whitespace
// follows the value the decoder has just finished reading.
func checkJSONEnd(dec *json.Decoder) error {
	_, err := dec.Token()
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errors.New("irt: unexpected data after JSON value")
	default:
		return err
	}
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
			return
		}

		if err := checkJSONEnd(dec); err != nil {
			yield(zero, err)
		}
	}
}

// UnmarshalJSON2 decodes a JSON object from the reader and yields each
// key-value pair as KV with a potential error. The reader is wrapped
// with bufio for efficient reading.
func UnmarshalJSON2[A any, B any](data io.Reader) iter.Seq2[KV[A, B], error] {
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

			text, ok := t.(string)
			if !ok {
				yield(zero, &json.SyntaxError{Offset: dec.InputOffset()})
				return
			}

			key, err := decodeJSONKey[A](text)
			if err != nil {
				yield(zero, err)
				return
			}

			// Decode the value
			var value B
			if err := dec.Decode(&value); err != nil {
				yield(zero, err)
				return
			}

			if !yield(MakeKV(key, value), nil) {
				return
			}
		}

		// read the closing brace: More also returns false at a
		// truncated input, which must be reported.
		if _, err := dec.Token(); err != nil {
			yield(zero, err)
			return
		}

		if err := checkJSONEnd(dec); err != nil {
			yield(zero, err)
		}
	}
}

// MarshalText marshals a sequence to bytes by trying TextMarshaler, JSONMarshaler,
// MarshalYAML, Marshal, string/[]byte conversions, or falling back to JSON encoding.
func MarshalText[T any](seq iter.Seq[T]) ([]byte, error) {
	var buf bytes.Buffer
	var (
		payload []byte
		err     error
	)
	for value := range seq {
		switch vt := any(value).(type) {
		case encoding.TextMarshaler:
			payload, err = vt.MarshalText()
		case json.Marshaler:
			payload, err = vt.MarshalJSON()
		case interface{ MarshalYAML() ([]byte, error) }:
			payload, err = vt.MarshalYAML()
		case interface{ Marshal() ([]byte, error) }:
			payload, err = vt.Marshal()
		case string:
			payload, err = []byte(vt), nil
		case []byte:
			payload, err = vt, nil
		default:
			payload, err = json.Marshal(vt)
		}
		if err != nil {
			return nil, err
		}
		must2(buf.Write(payload))
		payload = payload[:0]
	}
	if buf.Len() == 0 {
		return []byte{}, nil
	}
	return buf.Bytes(), nil
}

// MarshalBinary marshals a sequence to bytes by trying BinaryMarshaler, Marshal,
// MarshalBSON, or []byte conversions. Returns an error for unsupported types.
func MarshalBinary[T any](seq iter.Seq[T]) ([]byte, error) {
	var buf bytes.Buffer
	var (
		payload []byte
		err     error
	)
	for value := range seq {
		switch vt := any(value).(type) {
		case encoding.BinaryMarshaler:
			payload, err = vt.MarshalBinary()
		case interface{ Marshal() ([]byte, error) }:
			payload, err = vt.Marshal()
		case interface{ MarshalBSON() ([]byte, error) }:
			payload, err = vt.MarshalBSON()
		case []byte:
			payload, err = vt, nil
		default:
			payload, err = nil, fmt.Errorf("could not marshal %T, implement econding.BinaryMarshaler", value)
		}
		if err != nil {
			return nil, err
		}
		must2(buf.Write(payload))
		payload = payload[:0]
	}
	if buf.Len() == 0 {
		return []byte{}, nil
	}
	return buf.Bytes(), nil
}
