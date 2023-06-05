package pubsub

import (
	"context"

	"github.com/tychoish/fun"
)

// Populate publishes items from the input iterator to the broker in
// question.
//
// You may call Populate in a go routine. The error returned is the
// result the iterator's close method.
func Populate[T any](ctx context.Context, iter *fun.Iterator[T], broker *Broker[T]) error {
	for {
		val, err := iter.ReadOne(ctx)
		if err != nil {
			break
		}
		broker.Publish(ctx, val)
	}

	return iter.Close()
}
