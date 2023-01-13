package fun

import "context"

// Convert an arbitrary iterator to a channel.
func Channel[T any](ctx context.Context, iter Iterator[T]) <-chan T {
	out := make(chan T)
	go func() {
		for iter.Next(ctx) {
			select {
			case <-ctx.Done():
			case out <- iter.Value():
			}
		}
	}()
	return out
}

// ForEach passes each item in the iterator through the specified
// handler function, return an error if the handler function errors.
func ForEach[T any](ctx context.Context, iter Iterator[T], fn func(T) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for iter.Next(ctx) {
		if err := fn(iter.Value()); err != nil {
			return err
		}
	}

	return iter.Close(ctx)
}

// Map provides an orthodox functional map implementation based around
// fun.Iterator. Operates in asynchronous/streaming manner, so that
// the output Iterator must be consumed.
func Map[T any, O any](
	ctx context.Context,
	iter Iterator[T],
	mapper func(context.Context, T) (O, error),
) Iterator[O] {
	out := new(mapIterImpl[O])
	pipe := make(chan O)
	out.pipe = pipe
	catcher := &ErrorCollector{}
	var iterCtx context.Context
	iterCtx, out.closer = context.WithCancel(ctx)

	out.wg.Add(1)
	go func() {
		defer out.wg.Done()
		for iter.Next(iterCtx) {
			o, err := mapper(iterCtx, iter.Value())
			if err != nil {
				catcher.Add(err)
				continue
			}
			select {
			case <-iterCtx.Done():
			case pipe <- o:
			}
		}
		// use parent context to avoid not being able to close
		// the input iterator
		catcher.Add(iter.Close(ctx))
		out.err = catcher.Resolve()
	}()

	return out
}

// Reduce processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter Iterator[T],
	reducer func(context.Context, T, O) (O, error),
	initalValue O,
) (O, error) {
	value := initalValue
	var err error
	for iter.Next(ctx) {
		value, err = reducer(ctx, iter.Value(), value)
		if err != nil {
			return *new(O), err
		}
	}
	if err = iter.Close(ctx); err != nil {
		return *new(O), err
	}

	return value, nil
}
