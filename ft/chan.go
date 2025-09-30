package ft

import "context"

// Send executes the provided future function and sends its result to the channel.
func Send[T any](ch chan<- T, fut func() T) { ch <- fut() }

// Wait blocks until the channel produces a value, always discarding the value.
func Wait[T any](ch <-chan T) { <-ch }

// Recv receives and returns a value from the channel, blocking according to the Go channel semantics.
func Recv[T any](ch <-chan T) T { return <-ch }

// ContextErrorChannel returns a channel that will receive the context's error
// when the context is done. The channel is closed after sending the error.
func ContextErrorChannel(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		defer close(out)
		defer Send(out, ctx.Err)
		Wait(ctx.Done())
	}()

	return out
}
