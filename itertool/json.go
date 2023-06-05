package itertool

import (
	"context"
	"encoding/json"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

type errIter[T any] struct{ err error }

func (e errIter[T]) Close() error              { return e.err }
func (_ errIter[T]) Next(context.Context) bool { return false }
func (_ errIter[T]) Value() T                  { return fun.ZeroOf[T]() }

// MarshalJSON is useful for implementing json.Marshaler methods
// from iterator-supporting types. Wrapping the standard library's
// json encoding tools.
//
// The contents of the iterator are marshaled as elements in an JSON
// array.
func MarshalJSON[T any](ctx context.Context, iter fun.Iterable[T]) ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
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

		if err := enc.Encode(val); err != nil {
			return nil, err
		}
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
func UnmarshalJSON[T any](in []byte) *fun.Iterator[T] {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return fun.StaticProducer(fun.ZeroOf[T](), err).Generator()
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
