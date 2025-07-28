package ft

import "context"

func Send[T any](ch chan<- T, fut func() T) { ch <- fut() }
func Wait[T any](ch <-chan T)               { <-ch }
func Recv[T any](ch <-chan T) T             { return <-ch }

func ContextErrorChannel(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		defer close(out)
		defer Send(out, ctx.Err)
		Wait(ctx.Done())
	}()

	return out
}
