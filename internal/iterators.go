package internal

import (
	"context"
	"errors"
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

type GeneratorIterator[T any] struct {
	Operation func(context.Context) (T, error)
	Closer    context.CancelFunc
	Error     func() error
	closed    atomic.Bool
	value     T
	opErr     error
}

func NewGeneratorIterator[T any](op func(ctx context.Context) (T, error)) *GeneratorIterator[T] {
	return &GeneratorIterator[T]{Operation: op}
}

func (iter *GeneratorIterator[T]) Value() T { return iter.value }
func (iter *GeneratorIterator[T]) Close() error {
	if iter.closed.CompareAndSwap(false, true) {
		if closer := iter.Closer; closer != nil {
			closer()
		}
	}
	if iter.Error != nil {
		return MergeErrors(iter.Error(), iter.opErr)
	}
	return iter.opErr
}

func (iter *GeneratorIterator[T]) Next(ctx context.Context) bool {
	if ctx.Err() != nil || iter.Operation == nil || iter.closed.Load() {
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
	default:
		iter.opErr = err
	}
	return false
}

type SyncIterImpl[T any] struct {
	Mtx  *sync.Mutex
	Iter interface {
		Next(context.Context) bool
		Value() T
		Close() error
	}
}

func (iter SyncIterImpl[T]) Next(ctx context.Context) bool {
	iter.Mtx.Lock()
	defer iter.Mtx.Unlock()

	return iter.Iter.Next(ctx)
}

func (iter SyncIterImpl[T]) Close() error {
	iter.Mtx.Lock()
	defer iter.Mtx.Unlock()

	return iter.Iter.Close()
}

func (iter SyncIterImpl[T]) Value() T {
	iter.Mtx.Lock()
	defer iter.Mtx.Unlock()

	return iter.Iter.Value()
}

func (iter SyncIterImpl[T]) ReadOne(ctx context.Context) (T, error) {
	iter.Mtx.Lock()
	defer iter.Mtx.Unlock()

	if err := ctx.Err(); err != nil {
		return ZeroOf[T](), err
	}

	if iter.Iter.Next(ctx) {
		return iter.Iter.Value(), nil
	}

	return ZeroOf[T](), io.EOF
}
