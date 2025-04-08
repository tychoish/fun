package itertool

import (
	"context"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)

// Uniq iterates over a stream of comparable items, and caches them
// in a map, returning the first instance of each equivalent object,
// and skipping subsequent items
func Uniq[T comparable](iter *fun.Stream[T]) *fun.Stream[T] {
	set := dt.NewMap(map[T]struct{}{})

	return fun.NewGenerator(func(ctx context.Context) (out T, _ error) {
		for iter.Next(ctx) {
			if val := iter.Value(); !set.Check(val) {
				set.SetDefault(val)
				return val, nil
			}
		}
		return out, io.EOF
	}).Stream().WithHook(func(out *fun.Stream[T]) { out.AddError(iter.Close()) })
}

// DropZeroValues processes a stream removing all zero values.
func DropZeroValues[T comparable](iter *fun.Stream[T]) *fun.Stream[T] {
	return fun.Generator[T](func(ctx context.Context) (out T, _ error) {
		for {
			item, err := iter.ReadOne(ctx)
			if err != nil {
				return out, err
			}

			if !ft.IsZero(item) {
				return item, nil
			}
		}
	}).Stream().WithHook(func(stream *fun.Stream[T]) {
		stream.AddError(iter.Close())
	})
}

// Contains processes a stream of compareable type returning true
// after the first element that equals item, and false otherwise.
func Contains[T comparable](ctx context.Context, item T, iter *fun.Stream[T]) bool {
	for {
		v, err := iter.ReadOne(ctx)
		if err != nil {
			break
		}
		if v == item {
			return true
		}
	}

	return false
}
