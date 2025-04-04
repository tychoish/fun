package risky

import (
	"github.com/tychoish/fun"
)

// Slice converts an iterator into a slice: this will not abort or
// timeout if the iterator is blocking. The iterator's close method is
// not processed.
func Slice[T any](iter *fun.Iterator[T]) []T { return fun.NewProducer(iter.Slice).Force().Resolve() }

// Observe is a fun.Handler wrapper that uses a background context,
// and converts the possible error returned by fun.Observe into a
// panic. In general fun.Observe only returns an error if the input
// iterator errors (or panics) or the handler function panics.
func Observe[T any](iter *fun.Iterator[T], fn fun.Handler[T]) { iter.Observe(fn).Ignore().Wait() }
