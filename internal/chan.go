package internal

import (
	"context"
	"io"
)

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

type SendMode int

const (
	SendModeBlocking SendMode = iota
	SendModeNonBlocking
)

func Blocking(in bool) SendMode {
	if in {
		return SendModeBlocking
	}
	return SendModeNonBlocking
}

func SendOne[T any](ctx context.Context, mode SendMode, ch chan<- T, it T) error {
	switch mode {
	case SendModeBlocking:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- it:
			return nil
		}
	case SendModeNonBlocking:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- it:
			return nil
		default:
			return nil
		}
	default:
		return io.EOF
	}
}
