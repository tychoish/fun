package itertool

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
)

type syncIterImpl[T any] struct {
	mtx  *sync.RWMutex
	iter fun.Iterator[T]
}

func (iter syncIterImpl[T]) Unwrap() fun.Iterator[T] { return iter.iter }
func (iter syncIterImpl[T]) Next(ctx context.Context) bool {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Next(ctx)
}

func (iter syncIterImpl[T]) Close(ctx context.Context) error {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Close(ctx)
}

func (iter syncIterImpl[T]) Value() T {
	iter.mtx.RLock()
	defer iter.mtx.RUnlock()

	return iter.iter.Value()
}

// Synchronized produces wraps an existing iterator with one that is
// protected by a mutex. The underling implementation provides an
// Unwrap method.
func Synchronize[T any](in fun.Iterator[T]) fun.Iterator[T] {
	return syncIterImpl[T]{&sync.RWMutex{}, in}
}
