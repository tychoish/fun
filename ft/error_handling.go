package ft

import (
	"errors"
	"fmt"

	"github.com/tychoish/fun/ers"
)

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T {
	CallWhen(err != nil, func() { panic(errors.Join(err, ers.ErrInvariantViolation)) })
	return arg
}

// Check can wrap a the callsite of a function that returns a value
// and an error, and returns (zero, false) if the error is non, nil,
// and (value, true) if the error is nil.
func Check[T any](value T, err error) (zero T, _ bool) {
	if err != nil {
		return zero, false
	}
	return value, true
}

func WrapCheck[T any](value T, err error) func() (T, bool) {
	return func() (T, bool) { return Check(value, err) }
}

// MustBeOk raises an invariant violation if the ok value is false,
// and returns the first value if the second value is ok. Useful as
// in:
//
//	out := ft.MustBeOk(func() (string, bool) { return "hello world", true })
func MustBeOk[T any](out T, ok bool) T {
	CallWhen(!ok, func() { panic(fmt.Errorf("check failed: %w", ers.ErrInvariantViolation)) })
	return out
}

// Ignore is a noop, but can be used to annotate operations rather
// than assigning to the empty identifier:
//
//	_ = operation()
//	ft.Ignore(operation())
func Ignore[T any](_ T) { return } //nolint:staticcheck

// IgnoreFirst takes two arguments and returns only the second, for
// use in wrapping functions that return two values.
func IgnoreFirst[A any, B any](_ A, b B) B { return b }

// IgnoreSecond takes two arguments and returns only the first, for
// use when wrapping functions that return two values.
func IgnoreSecond[A any, B any](a A, _ B) A { return a }
