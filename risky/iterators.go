package risky

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

func Slice[T any](iter fun.Iterator[T]) []T {
	out := []T{}
	Observe(iter, func(in T) { out = append(out, in) })
	return out
}

func Observe[T any](iter fun.Iterator[T], fn fun.Observer[T]) {
	fun.InvariantMust(fun.Observe(internal.BackgroundContext, iter, fn))
}

// IterateOneBlocking has the same semantics as IterateOne except it
// uses a blocking context, and if the iterator is blocking and there
// are no more items, IterateOneBlocking will never return. Use with
// caution, and in situations where you understand the iterator's
// implementation.
func IterateOne[T any](iter fun.Iterator[T]) (T, error) {
	return fun.IterateOne(internal.BackgroundContext, iter)
}

func IgnoreObserver[T any](_ T) {}

func BackgroundWorker(ctx context.Context, fn fun.WorkerFunc) {
	fn.Background(ctx, IgnoreObserver[error])
}
