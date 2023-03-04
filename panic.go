package fun

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/fun/internal"
)

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
var ErrInvariantViolation = errors.New("invariant violation")

// Invariant panics if the condition is false Invariant panics,
// passing an error that is rooted by ErrInvariantViolation.
func Invariant(cond bool, args ...any) {
	if !cond {
		switch len(args) {
		case 0:
			panic(ErrInvariantViolation)
		case 1:
			switch ei := args[0].(type) {
			case error:
				panic(&internal.MergedError{Current: ei, Wrapped: ErrInvariantViolation})
			case string:
				panic(&internal.MergedError{Current: errors.New(ei), Wrapped: ErrInvariantViolation})
			default:
				panic(fmt.Errorf("[%v]: %w", args[0], ErrInvariantViolation))
			}
		default:
			if err, ok := args[0].(error); ok {
				panic(&internal.MergedError{
					Current: fmt.Errorf("[%s]", args[1:]),
					Wrapped: &internal.MergedError{Current: err, Wrapped: ErrInvariantViolation},
				})
			}
			panic(fmt.Errorf("[%s]: %w", fmt.Sprintln(args...), ErrInvariantViolation))
		}
	}
}

// InvariantMust raises an invariant error if the error is not
// nil. The content of the panic is both--via wrapping--an
// ErrInvariantViolation and the error itself.
func InvariantMust(err error, args ...any) {
	if err == nil {
		return
	}

	panic(&internal.MergedError{
		Current: fmt.Errorf("%s: %w", fmt.Sprint(args...), err),
		Wrapped: ErrInvariantViolation,
	})
}

// InvariantCheck calls the function and if it returns an error panics
// with an ErrInvariantViolation error, wrapped with the error of the
// function, and any annotation arguments.
func InvariantCheck(fn func() error, args ...any) { InvariantMust(fn(), args...) }

// IsInvariantViolation returns true if the argument is or resolves to
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err, ok := r.(error)
	if r == nil || !ok {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T {
	if err != nil {
		panic(err)
	}

	return arg
}

// Check, like safe and SafeCtx runs a function without arguments that
// does not produce an error, and, if the function panics, converts it
// into an error.
func Check(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = buildRecoverError(r)
		}
	}()

	fn()
	return
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = buildRecoverError(r)
		}
	}()
	out = fn()
	return
}

func buildRecoverError(r any) error {
	switch in := r.(type) {
	case error:
		return fmt.Errorf("panic: %w", in)
	default:
		return fmt.Errorf("panic: %v", in)
	}
}

// SafeCtx provides a variant of the Safe function that takes a
// context.
func SafeCtx[T any](ctx context.Context, fn func(context.Context) T) (out T, err error) {
	return Safe(func() T {
		return fn(ctx)
	})
}
