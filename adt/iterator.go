package adt

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
)

type syncIterImpl[T any] struct {
	mtx  *sync.Mutex
	iter fun.Iterator[T]
}

// NewIterator constructs an iterator that wraps the input iterator
// but that protects every single operations against the iterator with
// the mutext. Be careful, however, because single logical operations
// (e.g. advance with Next() and access with Value()) are not isolated
// with regards to eachother so multiple goroutines can have logical
// races if both are iterating concurrently. As a special case the
// fun.IterateOne function provides a special case that allows for
// safe, concurrent iteration.
func NewIterator[T any](mtx *sync.Mutex, iter fun.Iterator[T]) fun.Iterator[T] {
	return syncIterImpl[T]{mtx: mtx, iter: iter}
}

func (iter syncIterImpl[T]) Unwrap() fun.Iterator[T] { return iter.iter }

func (iter syncIterImpl[T]) Next(ctx context.Context) bool {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Next(ctx)
}

func (iter syncIterImpl[T]) Close() error {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Close()
}

func (iter syncIterImpl[T]) Value() T {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Value()
}

func (iter syncIterImpl[T]) ReadOne(ctx context.Context) (T, error) {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return fun.IterateOne(ctx, iter.iter)
}
