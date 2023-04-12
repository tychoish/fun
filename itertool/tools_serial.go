package itertool

import (
	"context"

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

	for iter.Next(ctx) {
		value, err = reducer(iter.Value(), value)
		if err != nil {
			catcher.Add(err)
			return
		}
	}

	return
}
