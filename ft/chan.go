package ft

import (
	"context"

	"github.com/tychoish/fun/risky"
)

// ContextErrorChannel returns a channel that will receive the context's error
// when the context is done. The channel is closed after sending the error.
func ContextErrorChannel(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		defer close(out)
		defer risky.Send(out, ctx.Err)
		risky.Wait(ctx.Done())
	}()

	return out
}
