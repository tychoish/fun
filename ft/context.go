package ft

import (
	"context"
	"time"
)

// WithContextTimeoutCall runs the function, which is the same type as
// fnx.Operation, with a new context that expires after the specified duration.
func WithContextTimeoutCall(dur time.Duration, op func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	op(ctx)
}

// WithContextCall runs the function, which is the same type as a
// fnx.Operation, with a new context that is canceled after the
// function exits.
func WithContextCall(op func(context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	op(ctx)
}

// WithContextCallOk runs the function, with a new context that is
// canceled after the function exits.
func WithContextCallOk(op func(context.Context) bool) bool { return WithContextDo(op) }

// WithContextCallErr runs the function, which is the same type as a
// fnx.Worker, with a new context that is canceled after the
// function exits.
func WithContextCallErr(op func(context.Context) error) error { return WithContextDo(op) }

// WithContextDo runs a function with a new context that is canceled
// after the function returns.
func WithContextDo[T any](op func(context.Context) T) T {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return op(ctx)
}

// WithContextDoOk runs a function with a new context that is canceled
// after the function returns.
func WithContextDoOk[T any](op func(context.Context) (T, bool)) (T, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return op(ctx)
}

// WithContextDoErr runs a function, which is the same type as
// fnx..Future, with a new context that is canceled after the function
// returns.
func WithContextDoErr[T any](op func(context.Context) (T, error)) (out T, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return op(ctx)
}

// WrapWithContextCall wraps a context-accepting function into a
// no-argument function by capturing the provided context.
func WrapWithContextCall(ctx context.Context, op func(context.Context)) func() {
	return func() { op(ctx) }
}

// WrapWithContextDo wraps a context-accepting function that returns a
// value into a no-argument function by capturing the provided
// context.
func WrapWithContextDo[T any](ctx context.Context, op func(context.Context) T) func() T {
	return func() T { return op(ctx) }
}

// WrapWithContextDoOk wraps a context-accepting function that returns
// a value and bool into a no-argument function by capturing the
// provided context.
func WrapWithContextDoOk[T any](ctx context.Context, op func(context.Context) (T, bool)) func() (T, bool) {
	return func() (T, bool) { return op(ctx) }
}

// WrapWithContextDoErr wraps a context-accepting function that
// returns a value and error into a no-argument function by capturing
// the provided context.
func WrapWithContextDoErr[T any](ctx context.Context, op func(context.Context) (T, error)) func() (T, error) {
	return func() (T, error) { return op(ctx) }
}
