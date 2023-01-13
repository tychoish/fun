package fun

import "context"

// Iterator provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
type Iterator[T any] interface {
	Next(context.Context) bool
	Close(context.Context) error
	Value() T
}

type multiIterImpl[T any] struct {
	iters []Iterator[T]
	pipe  chan T
	value T
	wg    WaitGroup
	close context.CancelFunc
}

// Combine a set of related iterators into a single iterator. Starts a
// thread to consume from each iterator and does not
// otherwise guarantee the iterator's order.
func MergeIterators[T any](ctx context.Context, iters ...Iterator[T]) Iterator[T] {
	iter := &multiIterImpl[T]{
		iters: iters,
		pipe:  make(chan T),
	}
	iter.run(ctx)
	return iter
}

func (iter *multiIterImpl[T]) run(ctx context.Context) {
	ctx, iter.close = context.WithCancel(ctx)
	for _, it := range iter.iters {
		iter.wg.Add(1)
		go func(itr Iterator[T]) {
			defer iter.wg.Done()
			for itr.Next(ctx) {
				select {
				case <-ctx.Done():
					return
				case iter.pipe <- itr.Value():
					continue
				}
			}
		}(it)
	}
}

func (iter *multiIterImpl[T]) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case item := <-iter.pipe:
		iter.value = item
		return true
	}
}

func (iter *multiIterImpl[T]) Value() T { return iter.value }

func (iter *multiIterImpl[T]) Close(ctx context.Context) error {
	iter.close()
	iter.wg.Wait(ctx)

	errs := &ErrorCollector{errs: make([]error, 0, len(iter.iters))}
	wg := &WaitGroup{}
	for _, i := range iter.iters {
		wg.Add(1)
		go func(i Iterator[T]) {
			defer wg.Done()
			errs.Add(i.Close(ctx))

		}(i)
	}
	wg.Wait(ctx)

	return errs.Resolve()
}

type sliceIterImpl[T any] struct {
	vals []T
	idx  int
}

// SliceIterator produces an iterator for an arbitrary slice.
func SliceIterator[T any](in []T) Iterator[T] {
	return &sliceIterImpl[T]{
		vals: in,
	}
}

func (iter *sliceIterImpl[T]) Next(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	if iter.idx+2 > len(iter.vals) {
		return false
	}
	iter.idx++

	return true
}

func (iter *sliceIterImpl[T]) Value() T                      { return iter.vals[iter.idx] }
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
