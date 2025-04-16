package itertool

import (
	"github.com/tychoish/fun"
)

// MergeSlices converts a stream of slices to an flattened
// stream of their elements.
func MergeSlices[T any](iter *fun.Stream[[]T]) *fun.Stream[T] {
	return fun.MergeStreams(fun.MakeConverter(fun.SliceStream[T]).Stream(iter))
}
