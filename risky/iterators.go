package risky

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/fnx"
)

// Slice converts a stream into a slice: this will not abort or
// timeout if the stream is blocking. The streams close method is
// not processed.
func Slice[T any](iter *fun.Stream[T]) []T { return fnx.NewFuture(iter.Slice).Force().Resolve() }
