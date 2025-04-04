package risky

import (
	"github.com/tychoish/fun"
)

// Slice converts an iterator into a slice: this will not abort or
// timeout if the iterator is blocking. The iterator's close method is
// not processed.
func Slice[T any](iter *fun.Iterator[T]) []T { return fun.NewProducer(iter.Slice).Force().Resolve() }
