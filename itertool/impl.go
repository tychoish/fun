// Package itertool provides a set of functional helpers for
// managinging and using fun.Iterator implementations, including a
// parallel Map/Reduce, Merge, and other convenient tools.
package itertool

import (
	"context"

	"github.com/tychoish/fun"
)

// Merge combines a set of related iterators into a single
// iterator. Starts a thread to consume from each iterator and does
// not otherwise guarantee the iterator's order.
func Merge[T any](iters ...*fun.Iterator[T]) *fun.Iterator[T] {
	pipe := make(chan T)

	init := fun.WaitFunc(func(ctx context.Context) {
		wg := &fun.WaitGroup{}
		wctx, cancel := context.WithCancel(ctx)

		// start a go routine for every iterator, to read from
		// the incoming iterator and push it to the pipe
		for idx := range iters {
			proc := iters[idx].Producer()
			fun.WaitFunc(func(ctx context.Context) {
				for {
					if value, ok := proc.Check(wctx); !ok || !fun.BlockingSend(pipe).Check(wctx, value) {
						return
					}
				}
			}).Add(wctx, wg)
		}
		wg.WaitFunc().AddHook(func() { cancel(); close(pipe) }).Go(ctx)
	}).Once()

	return fun.Generator(func(ctx context.Context) (T, error) {
		init(ctx)
		return fun.Blocking(pipe).Receive().Read(ctx)
	})
}
