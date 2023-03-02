package fun

import (
	"context"

	"github.com/tychoish/fun/internal"
)

// ReadOnce reads one item from the channel, and returns it. ReadOne
// returns early if the context is canceled (ctx.Err()) or the channel
// is closed (io.EOF).
func ReadOne[T any](ctx context.Context, ch <-chan T) (T, error) {
	return internal.ReadOne(ctx, ch)
}

// Process calls a function for every item passed to it. Process is a
// noop if no items are passed.
//
// One potential use case is with erc.Collectors, as in:
//
//	ec := &erc.Collector
//	fun.Process(ec.Add, err, err1, err2, err3, file.Close())
func Process[T any](fn func(T), items ...T) { ProcessAll(fn, items) }

// ProcessAll is a non-varadic version of Process().
func ProcessAll[T any](fn func(T), items []T) {
	for idx := range items {
		fn(items[idx])
	}
}
