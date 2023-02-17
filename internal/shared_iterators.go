package internal

import (
	"context"
	"sync"
)

type SliceIterImpl[T any] struct {
	Vals  []T
	Index int
	val   *T
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
	select {
	case <-ctx.Done():
		return false
	case val, ok := <-iter.Pipe:
		if !ok {
			return false
		}
		iter.value = val
		return true
	}
}

type MapIterImpl[T any] struct {
	ChannelIterImpl[T]
	WG sync.WaitGroup
}

func (iter *MapIterImpl[T]) Close() error {
	iter.Closer()
	iter.WG.Wait()
	return iter.ChannelIterImpl.Close()
}
