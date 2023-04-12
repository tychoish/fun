package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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
	WG     sync.WaitGroup
}

func (iter *ChannelIterImpl[T]) Value() T { return iter.value }
func (iter *ChannelIterImpl[T]) Close() error {
	if iter.Closer != nil {
		iter.Closer()
	}
	iter.WG.Wait()
	return iter.Error
}
func (iter *ChannelIterImpl[T]) Next(ctx context.Context) bool {
	// check first because select statement ordering is non-deterministic
	if ctx.Err() != nil {
		return false
	}
	v, err := ReadOne(ctx, iter.Pipe)
	if err != nil {
		if errors.Is(err, io.EOF) {
			_ = iter.Close()
		}
		return false
	}
	iter.value = v
	return true
}

type GeneratorIterator[T any] struct {
	Operation func(context.Context) (T, error)
	Closer    context.CancelFunc
	Error     error
	closed    atomic.Bool
	value     T
}

func (iter *GeneratorIterator[T]) Value() T { return iter.value }
func (iter *GeneratorIterator[T]) Close() error {
	if iter.Closer != nil {
		iter.Closer()
	}
	iter.closed.Store(true)
	return iter.Error
}

func (iter *GeneratorIterator[T]) Next(ctx context.Context) bool {
	if ctx.Err() != nil || iter.closed.Load() {
		fmt.Println("one", iter.closed.Load(), ctx.Err())
		return false
	}

	v, err := iter.Operation(ctx)
	switch {
	case err == nil:
		iter.value = v
		return true
	case errors.Is(err, context.Canceled):
	case errors.Is(err, context.DeadlineExceeded):
	case errors.Is(err, io.EOF):
		_ = iter.Close()
	default:
		iter.Error = err
		_ = iter.Close()
	}
	return false
}
