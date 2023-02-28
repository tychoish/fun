package internal

import (
	"context"
	"io"
	"sync"
)

// use internally for iterations we know are cannot block. USE WITH CAUTION
var BackgroundContext = context.Background()

type SliceIterImpl[T any] struct {
	Vals  []T
	Index int
	val   *T
}

func NewSliceIter[T any](in []T) *SliceIterImpl[T] {
	return &SliceIterImpl[T]{
		Vals:  in,
		Index: -1,
	}
}

func (iter *SliceIterImpl[T]) Value() T     { return *iter.val }
func (iter *SliceIterImpl[T]) Close() error { return nil }
func (iter *SliceIterImpl[T]) Next(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	if iter.Index+1 >= len(iter.Vals) {
		return false
	}

	iter.Index++
	iter.val = &iter.Vals[iter.Index]

	return true
}

type ChannelIterImpl[T any] struct {
	Pipe   <-chan T
	value  T
	Error  error
	Ctx    context.Context
	Closer context.CancelFunc
}

func (iter *ChannelIterImpl[T]) Value() T { return iter.value }
func (iter *ChannelIterImpl[T]) Close() error {
	if iter.Closer != nil {
		iter.Closer()
	}

	return iter.Error
}
func (iter *ChannelIterImpl[T]) Next(ctx context.Context) bool {
	// check first because select statement ordering is non-deterministic
	if ctx.Err() != nil {
		return false
	}
	v, err := ReadOne(ctx, iter.Pipe)
	if err != nil {
		return false
	}
	iter.value = v
	return true
}

type MapIterImpl[T any] struct {
	ChannelIterImpl[T]
	WG sync.WaitGroup
}

func (iter *MapIterImpl[T]) Close() error {
	if iter.Closer != nil {
		iter.Closer()
	}
	iter.WG.Wait()
	return iter.Error
}

func ReadOne[T any](ctx context.Context, ch <-chan T) (T, error) {
	select {
	case <-ctx.Done():
		return *new(T), ctx.Err()
	case obj, ok := <-ch:
		if !ok {
			return *new(T), io.EOF
		}
		if err := ctx.Err(); err != nil {
			return *new(T), err
		}

		return obj, nil
	}
}
