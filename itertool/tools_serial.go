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
	iter fun.Iterator[T],
	reducer func(T, O) (O, error),
	initalValue O,
) (value O, err error) {
	value = initalValue
	catcher := &erc.Collector{}

	defer func() { err = catcher.Resolve() }()
	defer erc.Check(catcher, iter.Close)
	defer erc.Recover(catcher)

	for {
		var item T
		item, err = fun.IterateOne(ctx, iter)
		if err != nil {
			erc.When(catcher, !errors.Is(err, io.EOF), err)
			return
		}

		value, err = reducer(item, value)
		if err != nil {
			catcher.Add(err)
			return
		}
	}
}

// Contains processes an iterator of compareable type returning true
// after the first element that equals item, and false otherwise.
func Contains[T comparable](ctx context.Context, item T, iter fun.Iterator[T]) bool {
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

// Any, as a special case of Transform converts an iterator of any
// type and converts it to an iterator of any (e.g. interface{})
// values.
func Any[T any](iter fun.Iterator[T]) fun.Iterator[any] {
	return fun.Transform(iter, func(in T) (any, error) { return any(in), nil })
}

// Uniq iterates over an iterator of comparable items, and caches them
// in a map, returning the first instance of each equivalent object,
// and skipping subsequent items
func Uniq[T comparable](iter fun.Iterator[T]) fun.Iterator[T] {
	set := fun.Mapify(map[T]struct{}{})
	return fun.Generator(func(ctx context.Context) (T, error) {
		for iter.Next(ctx) {
			if val := iter.Value(); !set.Check(val) {
				set.SetDefault(val)
				return val, nil
			}
		}
		return fun.ZeroOf[T](), io.EOF
	})
}
