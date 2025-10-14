package fnx

import (
	"context"
	"sync"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// Converter is a function type that converts T objects int objects of
// type O.
type Converter[T any, O any] func(context.Context, T) (O, error)

// MakeConverter builds a Transform function out of an equivalent function
// that doesn't take a context or return an error.
func MakeConverter[T any, O any](op func(T) O) Converter[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in), nil }
}

// MakeCovnerterOk builds a Transform function from a function that
// converts between types T and O, but that returns a boolean/check
// value. When the converter function returns false the
// transform function returns a ers.ErrCurrentOpSkip error.
func MakeCovnerterOk[T any, O any](op func(T) (O, bool)) Converter[T, O] {
	return func(_ context.Context, in T) (out O, err error) {
		var ok bool
		out, ok = op(in)
		if !ok {
			err = ers.ErrCurrentOpSkip
		}

		return
	}
}

// MakeConverterErr constructs a Transform function from an analogous
// function that does not take a context.
func MakeConverterErr[T any, O any](op func(T) (O, error)) Converter[T, O] {
	return func(_ context.Context, in T) (O, error) { return op(in) }
}

// Lock returns a Transform function that's executed the root function
// inside of the sope of a mutex.
func (mpf Converter[T, O]) Lock() Converter[T, O] {
	mu := &sync.Mutex{}
	return mpf.WithLock(mu)
}

// WithLock returns a Transform function inside of the scope of the
// provided mutex.
func (mpf Converter[T, O]) WithLock(mu *sync.Mutex) Converter[T, O] {
	return func(ctx context.Context, val T) (O, error) {
		defer internal.With(internal.Lock(mu))
		return mpf.Convert(ctx, val)
	}
}

// WithRecover returns a Transform function that catches a panic, converts
// the panic object to an error if needed, and aggregates that with
// the Transform function's error.
func (mpf Converter[T, O]) WithRecover() Converter[T, O] {
	return func(ctx context.Context, val T) (_ O, err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()
		return mpf(ctx, val)
	}
}

// Wait calls the transform function passing a context that cannot expire.
func (mpf Converter[T, O]) Wait(in T) (O, error) { return mpf.Convert(context.Background(), in) }

// Convert uses the converter function to transform a value from one
// type (T) to another (O).
func (mpf Converter[T, O]) Convert(ctx context.Context, in T) (O, error) { return mpf(ctx, in) }
