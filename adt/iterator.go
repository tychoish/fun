package adt

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// NewIterator constructs an iterator that wraps the input iterator
// but that protects every single operations against the iterator with
// the mutext. Be careful, however, because single logical operations
// (e.g. advance with Next() and access with Value()) are not isolated
// with regards to eachother so multiple goroutines can have logical
// races if both are iterating concurrently. As a special case the
// fun.IterateOne function allows for safe, concurrent iteration of
// these iterators.
func NewIterator[T any](mtx *sync.Mutex, iter fun.Iterator[T]) fun.Iterator[T] {
	return syncIterImpl[T]{internal.SyncIterImpl[T]{Mtx: mtx, Iter: iter}}
}

type syncIterImpl[T any] struct {
	internal.SyncIterImpl[T]
}

func (iter syncIterImpl[T]) Unwrap() fun.Iterator[T] { return iter.Iter }
func (iter syncIterImpl[T]) ReadOne(ctx context.Context) (T, error) {
	return iter.SyncIterImpl.ReadOne(ctx)
}
