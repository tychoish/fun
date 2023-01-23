package fun

import (
	"context"
	"fmt"
)

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T {
	if err != nil {
		panic(err)
	}

	return arg
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	out = fn()
	return
}

// SafeCtx provides a variant of the Safe function that takes a
// context.
func SafeCtx[T any](ctx context.Context, fn func(context.Context) T) (out T, err error) {
	return Safe(func() T {
		return fn(ctx)
	})
}
