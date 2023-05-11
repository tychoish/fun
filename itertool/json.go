package itertool

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/tychoish/fun"
)

type errIter[T any] struct{ err error }

func (e errIter[T]) Close() error              { return e.err }
func (_ errIter[T]) Next(context.Context) bool { return false }
func (_ errIter[T]) Value() T                  { return fun.ZeroOf[T]() }

// MarshalJSON is useful for implementing json.Marshaler methods
// from iterator-supporting types. Wrapping the standard library's
// json.Marshal method, it produces a byte array of encoded JSON
// documents.
func MarshalJSON[T any](ctx context.Context, iter fun.Iterator[T]) ([]byte, error) {
	buf := &bytes.Buffer{}
	_, _ = buf.Write([]byte("["))
	first := true
	for {
		val, err := fun.IterateOne(ctx, iter)
		if err != nil {
			break
		}
		if first {
			first = false
		} else {
			_, _ = buf.Write([]byte(","))
		}

		it, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(it)
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}
	_, _ = buf.Write([]byte("]"))

	return buf.Bytes(), nil
}

// UnmarshalJSON reads a JSON input and produces an iterator of the
// items. The implementation reads all items from the slice before
// returning. Any errors encountered are propgated to the Close method
// of the iterator.
func UnmarshalJSON[T any](in []byte) fun.Iterator[T] {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return errIter[T]{err: err}
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
