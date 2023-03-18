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
