package itertool

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)

// Uniq iterates over a stream of comparable items, and caches them
// in a map, returning the first instance of each equivalent object,
// and skipping subsequent items
func Uniq[T comparable](iter *fun.Stream[T]) *fun.Stream[T] {
	set := &dt.Set[T]{}

	return iter.Filter(func(in T) bool { return !set.AddCheck(in) })
}

// DropZeroValues processes a stream removing all zero values.
//
// Deprecated: use `fun.Stream[T].Filter(ft.NotZero)` instead
func DropZeroValues[T comparable](iter *fun.Stream[T]) *fun.Stream[T] {
	return iter.Filter(ft.NotZero)
}

// Contains processes a stream of compareable type returning true
// after the first element that equals item, and false otherwise.
func Contains[T comparable](ctx context.Context, item T, iter *fun.Stream[T]) bool {
	for {
		v, err := iter.Read(ctx)
		if err != nil {
			return false
		}
		if v == item {
			return true
		}
	}
}
