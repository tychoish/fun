package risky

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
)

// Slice converts an iterator into a slice: this will not abort or
// timeout if the iterator is blocking, and errors returned by the
// iterator's close method become panics.
func Slice[T any](iter *fun.Iterator[T]) []T {
	out := []T{}

	Observe(iter, func(in T) { out = append(out, in) })
	return out
}

// Observe is a fun.Observe wrapper that uses a background context,
// and converts the possible error returned by fun.Observe into a
// panic. In general fun.Observe only returns an error if the input
// iterator errors or the observer function panics.
func Observe[T any](iter *fun.Iterator[T], fn fun.Observer[T]) {
	fun.Invariant.Must(iter.Observe(context.Background(), fn))
}

// List is converts an iterator to a linked list, in the riskiest way
// possible.
func List[T any](iter *fun.Iterator[T]) *dt.List[T] {
	out := &dt.List[T]{}

	iter.Process(fun.Handle(func(in T) { out.PushBack(in) }).Processor()).
		Ignore().
		Block()

	return out
}
