package ers

import (
	"fmt"
)

// Recovery catches a panic, turns it into an error and passes it to
// the provided observer function.
func Recover(ob func(error)) { ob(ParsePanic(recover())) }

// ParsePanic converts a panic to an error, if it is not, and attaching
// the ErrRecoveredPanic error to that error. If no panic is
// detected, ParsePanic returns nil.
func ParsePanic(r any) error {
	if r != nil {
		switch err := r.(type) {
		case error:
			return Join(err, ErrRecoveredPanic)
		case string:
			return Join(New(err), ErrRecoveredPanic)
		case []error:
			return Join(err...)
		default:
			return Join(fmt.Errorf("[%T]: %v", err, err), ErrRecoveredPanic)
		}
	}
	return nil
}

// NewInvariantViolation creates a new error object, which always
// includes
func NewInvariantViolation(args ...any) error {
	switch len(args) {
	case 0:
		return ErrInvariantViolation
	case 1:
		switch ei := args[0].(type) {
		case error:
			return Join(ei, ErrInvariantViolation)
		case string:
			return Join(New(ei), ErrInvariantViolation)
		case func() error:
			return Join(ei(), ErrInvariantViolation)
		default:
			return Join(fmt.Errorf("%v", args[0]), ErrInvariantViolation)
		}
	default:
		return extractAndJoin(args, ErrInvariantViolation)
	}
}

// WithRecoverCall runs a function without arguments that does not
// produce an error, and, if the function panics, converts it into an
// error.
func WithRecoverCall(fn func()) (err error) {
	defer func() { err = ParsePanic(recover()) }()
	fn()
	return
}

func WrapWithRecoverCall(fn func()) func() error {
	return func() error { return WithRecoverCall(fn) }
}
func WrapRecoverDo[T any](fn func() T) func() (T, error) {
	return func() (T, error) { return WithRecoverDo(fn) }
}
func WrapRecoverOK[T any](fn func() (T, error)) func() (T, bool) {
	return func() (T, bool) { return WithRecoverOK(fn) }
}

// WithRecoverDo runs a function with a panic handler that converts the panic
// to an error.
func WithRecoverDo[T any](fn func() T) (out T, err error) {
	defer func() { err = ParsePanic(recover()) }()
	out = fn()
	return
}

// WithRecoverOK runs a function and returns true if there are no errors and
// no panics the bool output value is true, otherwise it is false.
func WithRecoverOK[T any](fn func() (T, error)) (out T, ok bool) {
	defer func() { ok = !IsError(ParsePanic(recover())) && ok }()
	if value, err := fn(); OK(err) {
		out, ok = value, true
	}
	return
}
