package itertool

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/tychoish/fun"
)

// MarshalJSON is useful for implementing json.Marshaler methods
// from iterator-supporting types. Wrapping the standard library's
// json.Marshal method, it produces a byte array of encoded JSON
// documents.
func MarshalJSON[T any](ctx context.Context, iter fun.Iterator[T]) ([]byte, error) {
	items := [][]byte{}
	var size int
	for iter.Next(ctx) {
		it, err := json.Marshal(iter.Value())
		if err != nil {
			return nil, err
		}
		items = append(items, it)
		size += len(it) + 1
	}
	if err := iter.Close(ctx); err != nil {
		return nil, err
	}

	out := make([]byte, 0, size+2)
	out = append([]byte("["), bytes.Join(items, []byte(","))...)
	out = append(out, []byte("]")...)
	return out, nil
}

type errIter[T any] struct{ err error }

func (e errIter[T]) Close(context.Context) error { return e.err }
func (_ errIter[T]) Next(context.Context) bool   { return false }
func (_ errIter[T]) Value() T                    { return *new(T) }

// UnmarshalJSON reads a JSON input and produces an iterator of the
// items. The implementation reads all items from the slice before
// returning.
func UnmarshalJSON[T any](in []byte) fun.Iterator[T] {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return errIter[T]{err: err}
	}
	out := make([]T, len(rv))
	for idx := range out {
		if err := json.Unmarshal(rv[idx], &out[idx]); err != nil {
			return errIter[T]{err: err}
		}
	}

	return Slice(out)
}
