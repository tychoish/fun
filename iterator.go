package fun

import (
	"context"
	"sync"
)

// Iterator provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
type Iterator[T any] interface {
	Next(context.Context) bool
	Close(context.Context) error
	Value() T
}

// MergeIterators combines a set of related iterators into a single
// iterator. Starts a thread to consume from each iterator and does
// not otherwise guarantee the iterator's order.
func MergeIterators[T any](ctx context.Context, iters ...Iterator[T]) Iterator[T] {
	pipe := make(chan T)

	iter := &mapIterImpl[T]{
		channelIterImpl: channelIterImpl[T]{pipe: pipe},
	}

	ctx, iter.closer = context.WithCancel(ctx)
	for _, it := range iters {
		iter.wg.Add(1)
		go func(itr Iterator[T]) {
			defer iter.wg.Done()
			for itr.Next(ctx) {
				select {
				case <-ctx.Done():
					return
				case pipe <- itr.Value():
					continue
				}
			}
		}(it)
	}

	// when all workers conclude, close the pipe.
	go func() { iter.wg.Wait(ctx); close(pipe) }()

	return iter
}

type sliceIterImpl[T any] struct {
	vals []T
	idx  int
	val  *T
}

// SliceIterator produces an iterator for an arbitrary slice.
func SliceIterator[T any](in []T) Iterator[T] {
	return &sliceIterImpl[T]{
		vals: in,
		idx:  -1,
	}
}

func (iter *sliceIterImpl[T]) Next(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	if iter.idx+1 >= len(iter.vals) {
		return false
	}
	iter.idx++
	iter.val = &iter.vals[iter.idx]

	return true
}

func (iter *sliceIterImpl[T]) Value() T                      { return *iter.val }
func (iter *sliceIterImpl[T]) Close(_ context.Context) error { return nil }

type channelIterImpl[T any] struct {
	pipe  <-chan T
	value T
	err   error
}

// ChannelIterator produces an aterator for a specified channel. The
// iterator does not start any background threads.
func ChannelIterator[T any](pipe <-chan T) Iterator[T] {
	return &channelIterImpl[T]{pipe: pipe}
}

func (iter *channelIterImpl[T]) Next(ctx context.Context) bool {
	// check first because select statement ordering is non-deterministic
	if ctx.Err() != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case val, ok := <-iter.pipe:
		if !ok {
			return false
		}

		iter.value = val
		return true
	}
}

func (iter *channelIterImpl[T]) Value() T                      { return iter.value }
func (iter *channelIterImpl[T]) Close(_ context.Context) error { return iter.err }

type mapIterImpl[T any] struct {
	channelIterImpl[T]
	wg     WaitGroup
	closer context.CancelFunc
}

func (iter *mapIterImpl[T]) Close(ctx context.Context) error {
	iter.closer()
	iter.wg.Wait(ctx)
	return iter.channelIterImpl.Close(ctx)
}

type syncIterImpl[T any] struct {
	mtx  *sync.RWMutex
	iter Iterator[T]
}

// MakeSynchronizedIterator produces wraps an existing iterator with
// one that is protected by a mutex. The underling implementation
// provides an Unwrap method.
func MakeSynchronizedIterator[T any](in Iterator[T]) Iterator[T] {
	return syncIterImpl[T]{
		mtx:  &sync.RWMutex{},
		iter: in,
	}
}

func (iter syncIterImpl[T]) Unwrap() Iterator[T] { return iter.iter }

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
