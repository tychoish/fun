package fun

import (
	"context"
	"errors"
	"fmt"
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
			panic(fmt.Errorf("[%v]: %w", args[0], ErrInvariantViolation))
		default:
			panic(fmt.Errorf("[%s]: %w", fmt.Sprintln(args...), ErrInvariantViolation))
		}
	}
}

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
