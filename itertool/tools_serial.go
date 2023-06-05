package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
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
func Contains[T comparable](ctx context.Context, item T, iter fun.Iterable[T]) bool {
	for {
		v, err := fun.IterateOne(ctx, iter)
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
	set := fun.Mapify(map[T]struct{}{})

	return fun.Producer[T](func(ctx context.Context) (T, error) {
		for iter.Next(ctx) {
			if val := iter.Value(); !set.Check(val) {
				set.SetDefault(val)
				return val, nil
			}
		}
		return fun.ZeroOf[T](), io.EOF
	}).Generator()
}

// DropZeroValues processes an iterator removing all zero values.
func DropZeroValues[T comparable](iter fun.Iterable[T]) *fun.Iterator[T] {
	return fun.Producer[T](func(ctx context.Context) (T, error) {
		for {
			item, err := fun.IterateOne(ctx, iter)
			if err != nil {
				return fun.ZeroOf[T](), err
			}

			if !fun.IsZero(item) {
				return item, nil
			}
		}
	}).Generator()
}

// Chain, like merge, takes a sequence of iterators and produces a
// combined iterator. Chain processes each iterator provided in
// sequence, where merge reads from all iterators in at once.
func Chain[T any](iters ...fun.Iterable[T]) *fun.Iterator[T] {
	pipe := make(chan T)
	init := fun.WaitFunc(func(ctx context.Context) {
		defer close(pipe)

	GROUP:
		for _, iter := range iters {

		ITERATOR:
			for {
				val, err := fun.IterateOne(ctx, iter)

				switch {
				case err == nil:
					if fun.Blocking(pipe).Send().Check(ctx, val) {
						continue ITERATOR
					}
				case errors.Is(err, io.EOF):
					continue GROUP
				}
				return
			}
		}
	}).Launch().Once()

	return fun.Generator(func(ctx context.Context) (T, error) {
		init(ctx)
		return fun.Blocking(pipe).Receive().Read(ctx)
	})
}
