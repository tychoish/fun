package risky

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// Slice converts an iterator into a slice: this will not abort or
// timeout if the iterator is blocking, and errors returned by the
// iterator's close method become panics.
func Slice[T any](iter fun.Iterator[T]) []T {
	out := []T{}
	Observe(iter, func(in T) { out = append(out, in) })
	return out
}

// Observe is a fun.Observe wrapper that uses a background context,
// and converts the possible error returned by fun.Observe into a
// panic. In general fun.Observe only returns an error if the input
// iterator errors or the observer function panics.
func Observe[T any](iter fun.Iterator[T], fn fun.Observer[T]) {
	fun.InvariantMust(fun.Observe(internal.BackgroundContext, iter, fn))
}

// IterateOne has the same semantics as IterateOne except it
// uses a blocking context, and if the iterator is blocking and there
// are no more items, IterateOneBlocking will never return. Use with
// caution, and in situations where you understand the iterator's
// implementation.
func IterateOne[T any](iter fun.Iterator[T]) (T, error) {
	return fun.IterateOne(internal.BackgroundContext, iter)
}

// IgnoreObserver is a fun.Observer[T] function that ignores its
// input.
func IgnoreObserver[T any](_ T) {}
