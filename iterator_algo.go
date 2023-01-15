package fun

import (
	"context"
)

// CollectIteratorChannel converts and iterator to a channel.
func CollectIteratorChannel[T any](ctx context.Context, iter Iterator[T]) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for iter.Next(ctx) {
			select {
			case <-ctx.Done():
				return
			case out <- iter.Value():
				continue
			}
		}
	}()
	return out
}

// CollectIterator converts an iterator to the slice of it's values.
//
// In the case of an error in the underlying iterator the output slice
// will have the values encountered before the error.
func CollectIterator[T any](ctx context.Context, iter Iterator[T]) ([]T, error) {
	out := []T{}
	err := ForEach(ctx, iter, func(_ context.Context, in T) error { out = append(out, in); return nil })
	return out, err
}

// ForEach passes each item in the iterator through the specified
// handler function, return an error if the handler function errors.
func ForEach[T any](ctx context.Context, iter Iterator[T], fn func(context.Context, T) error) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &ErrorCollector{}

	defer func() { err = catcher.Resolve() }()
	defer catcher.Recover()
	defer catcher.CheckCtx(ctx, iter.Close)

	for iter.Next(ctx) {
		if ferr := fn(ctx, iter.Value()); ferr != nil {
			catcher.Add(ferr)
			break
		}
	}

	return
}

// FilterIterator passes all objects in an iterator through the
// specified filter function. If the filter function errors, the
// operation aborts and the error is reported by the returned
// iterator's Close method. If the include boolean is true the result
// of the function is included in the output iterator, otherwise the
// operation is skipped.
//
// The output iterator is produced iteratively as the returned
// iterator is consumed.
func FilterIterator[T any](
	ctx context.Context,
	iter Iterator[T],
	fn func(ctx context.Context, input T) (output T, include bool, err error),
) Iterator[T] {
	out := new(mapIterImpl[T])
	pipe := make(chan T)
	out.pipe = pipe

	var iterCtx context.Context
	iterCtx, out.closer = context.WithCancel(ctx)

	out.wg.Add(1)
	go func() {
		catcher := &ErrorCollector{}
		defer out.wg.Done()
		defer func() { out.err = catcher.Resolve() }()
		defer catcher.Recover()
		defer catcher.CheckCtx(ctx, iter.Close)
		defer close(pipe)

		for iter.Next(iterCtx) {
			o, include, err := fn(iterCtx, iter.Value())
			if err != nil {
				catcher.Add(err)
				break
			}
			if !include {
				continue
			}
			select {
			case <-iterCtx.Done():
			case pipe <- o:
			}
		}
	}()
	return out
}

// MapIterator provides an orthodox functional map implementation based around
// fun.Iterator. Operates in asynchronous/streaming manner, so that
// the output Iterator must be consumed.
func MapIterator[T any, O any](
	ctx context.Context,
	iter Iterator[T],
	mapper func(context.Context, T) (O, error),
) Iterator[O] {
	out := new(mapIterImpl[O])
	pipe := make(chan O)
	out.pipe = pipe
	var iterCtx context.Context
	iterCtx, out.closer = context.WithCancel(ctx)

	out.wg.Add(1)
	go func() {
		catcher := &ErrorCollector{}

		defer out.wg.Done()
		defer func() { out.err = catcher.Resolve() }()
		defer catcher.Recover()
		defer catcher.CheckCtx(ctx, iter.Close)
		defer close(pipe)

		for iter.Next(iterCtx) {
			o, err := mapper(iterCtx, iter.Value())
			if err != nil {
				catcher.Add(err)
				continue
			}
			select {
			case <-iterCtx.Done():
				return
			case pipe <- o:
			}
		}
	}()

	return out
}

// ReduceIterator processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func ReduceIterator[T any, O any](
	ctx context.Context,
	iter Iterator[T],
	reducer func(context.Context, T, O) (O, error),
	initalValue O,
) (value O, err error) {
	value = initalValue
	catcher := &ErrorCollector{}

	defer func() { err = catcher.Resolve() }()
	defer catcher.Recover()
	defer catcher.CheckCtx(ctx, iter.Close)

	for iter.Next(ctx) {
		value, err = reducer(ctx, iter.Value(), value)
		if err != nil {
			catcher.Add(err)
			return
		}
	}

	return
}
