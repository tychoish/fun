package itertool

import (
	"context"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
)

// Reduce processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter *fun.Iterator[T],
	reducer func(T, O) (O, error),
	initalValue O,
) (value O, err error) {
	catcher := &erc.Collector{}

	defer func() { err = catcher.Resolve() }()
	defer erc.Check(catcher, iter.Close)
	defer erc.Recover(catcher)

	for {
		item, err := iter.ReadOne(ctx)
		if err != nil {
			return value, nil
		}

		value, err = reducer(item, value)
		if err != nil {
			catcher.Add(err)
			return value, err
		}
	}
}

// Contains processes an iterator of compareable type returning true
// after the first element that equals item, and false otherwise.
func Contains[T comparable](ctx context.Context, item T, iter *fun.Iterator[T]) bool {
	for {
		v, err := iter.ReadOne(ctx)
		if err != nil {
			break
		}
		if v == item {
			return true
		}
	}

	return false
}

// Uniq iterates over an iterator of comparable items, and caches them
// in a map, returning the first instance of each equivalent object,
// and skipping subsequent items
func Uniq[T comparable](iter *fun.Iterator[T]) *fun.Iterator[T] {
	set := dt.Mapify(map[T]struct{}{})

	return fun.Producer[T](func(ctx context.Context) (out T, _ error) {
		for iter.Next(ctx) {
			if val := iter.Value(); !set.Check(val) {
				set.SetDefault(val)
				return val, nil
			}
		}
		return out, io.EOF
	}).Iterator()
}

// DropZeroValues processes an iterator removing all zero values.
func DropZeroValues[T comparable](iter *fun.Iterator[T]) *fun.Iterator[T] {
	return fun.Producer[T](func(ctx context.Context) (out T, _ error) {
		for {
			item, err := iter.ReadOne(ctx)
			if err != nil {
				return out, err
			}

			if !fun.IsZero(item) {
				return item, nil
			}
		}
	}).Iterator()
}

// Chain, like merge, takes a sequence of iterators and produces a
// combined iterator. Chain processes each iterator provided in
// sequence, where merge reads from all iterators in at once.
func Chain[T any](iters ...*fun.Iterator[T]) *fun.Iterator[T] {
	pipe := fun.Blocking(make(chan T))

	init := fun.Operation(func(ctx context.Context) {
		defer pipe.Close()
		iteriter := dt.Sliceify(iters).Iterator()

		// use direct iteration because this function has full
		// ownership of the iterator and this is the easiest
		// way to make sure that the outer iterator aborts
		// when the context is canceled.
		for iteriter.Next(ctx) {
			iter := iteriter.Value()

			// iterate using the helper, so we get more
			// atomic iteration, and the ability to
			// respond to cancellation/blocking from the
			// outgoing function.
			for {
				val, err := iter.ReadOne(ctx)
				if err == nil {
					if pipe.Send().Check(ctx, val) {
						continue
					}
				}
				break
			}
		}
	}).Launch().Once()

	return pipe.Receive().Producer().PreHook(init).Iterator()
}
