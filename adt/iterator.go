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
