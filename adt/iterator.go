package adt

import (
	"context"
	"io"
	"sync"

	"github.com/tychoish/fun"
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
	return syncIterImpl[T]{
		mtx:  mtx,
		iter: iter,
	}
}

func (iter syncIterImpl[T]) Unwrap() fun.Iterator[T] { return iter.iter }

type syncIterImpl[T any] struct {
	mtx  *sync.Mutex
	iter fun.Iterator[T]
}

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

	if err := ctx.Err(); err != nil {
		return fun.ZeroOf[T](), err
	}

	if iter.iter.Next(ctx) {
		return iter.iter.Value(), nil
	}

	return fun.ZeroOf[T](), io.EOF
}
