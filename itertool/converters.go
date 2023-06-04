package itertool

import (
	"context"

	"github.com/tychoish/fun"
)

// CollectChannel converts and iterator to a channel. The iterator is
// not closed.
func CollectChannel[T any](ctx context.Context, iter fun.Iterator[T]) <-chan T {
	return collectChannel(ctx, 0, iter)
}

func collectChannel[T any](ctx context.Context, buf int, iter fun.Iterator[T]) chan T {
	out := make(chan T, buf)
	go func() {
		defer close(out)
		for {
			item, err := fun.IterateOne(ctx, iter)
			if err != nil || !fun.Blocking(out).Send().Check(ctx, item) {
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

// Slice produces an iterator for an arbitrary slice.
func Slice[T any](in []T) fun.Iterator[T] { return fun.Sliceify(in).Iterator() }

// Channel produces an iterator for a specified channel. The
// iterator does not start any background threads.
func Channel[T any](pipe <-chan T) fun.Iterator[T] {
	return fun.BlockingReceive(pipe).Producer().Generator()
}

// Variadic is a wrapper around Slice() for more ergonomic use at some
// call sites.
func Variadic[T any](in ...T) fun.Iterator[T] { return Slice(in) }

// FromMap converts a map into an iterator of fun.Pair objects. The
// iterator is panic-safe, and uses one go routine to track the
// progress through the map. As a result you should always, either
// exhaust the iterator, cancel the context that you pass to the
// iterator OR call iterator.Close().
//
// To use this iterator the items in the map are not copied, and the
// iteration order is randomized following the convention in go.
//
// Use in combination with other iterator processing tools
// (generators, observers, transformers, etc.) to limit the number of
// times a collection of data must be coppied.
func FromMap[K comparable, V any](in map[K]V) fun.Iterator[fun.Pair[K, V]] {
	return fun.Mapify(in).Iterator()
}
