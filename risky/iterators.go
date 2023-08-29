package risky

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
)

// Slice converts an iterator into a slice: this will not abort or
// timeout if the iterator is blocking. The iterator's close method is
// not processed.
func Slice[T any](iter *fun.Iterator[T]) []T {
	out := dt.Slice[T]{}
	out.Populate(iter).Ignore().Wait()
	return out
}

// Observe is a fun.Observe wrapper that uses a background context,
// and converts the possible error returned by fun.Observe into a
// panic. In general fun.Observe only returns an error if the input
// iterator errors (or panics) or the handler function panics.
func Observe[T any](iter *fun.Iterator[T], fn fun.Handler[T]) {
	iter.Process(fn.Processor()).Must().Wait()
}

// List converts an iterator to a linked list, without passing .
func List[T any](iter *fun.Iterator[T]) *dt.List[T] {
	out := &dt.List[T]{}

	Observe(iter, out.PushBack)

	return out
}
