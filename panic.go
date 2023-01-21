package fun

import (
	"context"
	"fmt"
)

func Must[T any](arg T, err error) T {
	if err != nil {
		panic(err)
	}

	return arg
}

func Safe[T any](fn func() T) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	out = fn()
	return
}

func SafeCtx[T any](ctx context.Context, fn func(context.Context) T) (out T, err error) {
	return Safe(func() T {
		return fn(ctx)
	})
}
