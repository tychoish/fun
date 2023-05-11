package internal

import (
	"context"
	"sync"
)

func Mnemonize[T any](in func() T) func() T {
	once := &sync.Once{}
	var value T

	return func() T {
		once.Do(func() { value = in() })
		return value
	}
}

func MnemonizeContext[T any](in func(context.Context) T) func(context.Context) T {
	once := &sync.Once{}
	var value T

	return func(ctx context.Context) T {
		once.Do(func() { value = in(ctx) })
		return value
	}
}
