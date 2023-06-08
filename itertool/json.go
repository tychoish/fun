package itertool

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// MarshalJSON is useful for implementing json.Marshaler methods
// from iterator-supporting types. Wrapping the standard library's
// json encoding tools.
//
// The contents of the iterator are marshaled as elements in an JSON
// array.
func MarshalJSON[T any](ctx context.Context, iter *fun.Iterator[T]) ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	_, _ = buf.Write([]byte("["))
	first := true
	for {
		val, err := iter.ReadOne(ctx)
		if err != nil {
			break
		}
		if first {
			first = false
		} else {
			_, _ = buf.Write([]byte(","))
		}

		if err := enc.Encode(val); err != nil {
			return nil, err
		}
	}

	_, _ = buf.Write([]byte("]"))

	if err := iter.Close(); err != nil {
		// TODO try and marshal with a producer that makes an
		// actual error
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalJSON reads a JSON input and produces an iterator of the
// items. The implementation reads all items from the slice before
// returning. Any errors encountered are propgated to the Close method
// of the iterator.
func UnmarshalJSON[T any](in []byte) *fun.Iterator[T] {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		var next T
		return fun.StaticProducer(next, err).Iterator()
	}
	var idx int
	return fun.Generator(func(ctx context.Context) (out T, err error) {
		if idx >= len(rv) {
			return out, io.EOF
		}
		err = json.Unmarshal(rv[idx], &out)
		if err != nil {
			return
		}
		idx++
		return
	})
}
