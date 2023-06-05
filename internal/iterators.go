package internal

import (
	"context"
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
