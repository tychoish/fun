package internal

import (
	"context"
	"errors"
	"io"
)

var ErrSkippedNonBlockingChannelOperation = errors.New("skipped non-blocking channel operation")

// ZeroOf returns the zero-value for the type T specified as an
// argument.
func ZeroOf[T any]() T { var out T; return out }

func ReadOne[T any](ctx context.Context, ch <-chan T) (T, error) {
	select {
	case <-ctx.Done():
		return ZeroOf[T](), ctx.Err()
	case obj, ok := <-ch:
		if !ok {
			return ZeroOf[T](), io.EOF
		}
		if err := ctx.Err(); err != nil {
			return ZeroOf[T](), err
		}

		return obj, nil
	}
}

func NonBlockingReadOne[T any](ctx context.Context, ch <-chan T) (T, error) {
	select {
	case <-ctx.Done():
		return ZeroOf[T](), ctx.Err()
	case obj, ok := <-ch:
		if !ok {
			return ZeroOf[T](), io.EOF
		}
		if err := ctx.Err(); err != nil {
			return ZeroOf[T](), err
		}

		return obj, nil
	default:
		return ZeroOf[T](), ErrSkippedNonBlockingChannelOperation
	}
}
