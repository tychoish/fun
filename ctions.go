package fun

import (
	"context"

	"github.com/tychoish/fun/internal"
)

// Ptr returns a pointer for the object. Useful for creating values
func Ptr[T any](in T) *T { return &in }

// ContextExpired checks the context's Err method and returns true
// when it is non-nil, and false otherwise. Provided for
// expressiveness.
func ContextExpired(ctx context.Context) bool { return ctx.Err() != nil }

// ReadOnce reads one item from the channel, and returns it. ReadOne
// returns early if the context is canceled (ctx.Err()) or the channel
// is closed (io.EOF).
func ReadOne[T any](ctx context.Context, ch <-chan T) (T, error) {
	return internal.ReadOne(ctx, ch)
}

func Default[T comparable](input T, defaultValue T) T {
	if IsZero(input) {
		return defaultValue
	}
	return input
}

// WhenCall runs a function when condition is true, and is a noop
// otherwise.
func WhenCall(cond bool, op func()) {
	if !cond {
		return
	}
	op()
}

// WhenDo calls the function when the condition is true, and returns
// the result, or if the condition is false, the operation is a noop,
// and returns zero-value for the type.
func WhenDo[T any](cond bool, op func() T) T {
	if !cond {
		return ZeroOf[T]()
	}
	return op()
}

// Contain returns true if an element of the slice is equal to the
// item.
func Contains[T comparable](item T, slice []T) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

// Apply processes an input slice, with the provided function,
// returning a new slice that holds the result.
func Apply[T any](fn func(T) T, in []T) []T {
	out := make([]T, len(in))

	for idx := range in {
		out[idx] = fn(in[idx])
	}

	return out
}
