package itertool

import (
	"context"

	"github.com/tychoish/fun"
)

// CollectChannel converts and iterator to a channel. The iterator is
// not closed.
func CollectChannel[T any](ctx context.Context, iter fun.Iterator[T]) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			item, err := fun.IterateOne(ctx, iter)
			if err != nil || !fun.Blocking(out).Check(ctx, item) {
				return
			}
		}
	}()
	return out
}

// CollectSlice converts an iterator to the slice of it's values, and
// closes the iterator at the when the iterator has been exhausted..
//
// In the case of an error in the underlying iterator the output slice
// will have the values encountered before the error.
func CollectSlice[T any](ctx context.Context, iter fun.Iterator[T]) ([]T, error) {
	out := []T{}
	return out, fun.Observe(ctx, iter, func(in T) { out = append(out, in) })
}
