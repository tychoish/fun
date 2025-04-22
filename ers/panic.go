package ers

import (
	"errors"
	"fmt"
)

// Recover catches a panic, turns it into an error and passes it to
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
			out := make([]error, len(err), len(err)+1)
			copy(out, err)
			return Join(append(out, ErrRecoveredPanic)...)
		default:
			return fmt.Errorf("[%T]: %v: %w", err, err, ErrRecoveredPanic)
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
			return fmt.Errorf("%v: %w", args[0], ErrInvariantViolation)
		}
	default:
		args, errs := ExtractErrors(args)
		out := append(make([]error, 0, len(errs)+2), ErrInvariantViolation)
		if len(args) > 0 {
			out = append(out, errors.New(fmt.Sprintln(args...)))
		}
		return Join(append(out, errs...)...)
	}
}

// IsInvariantViolation returns true if the argument is or resolves to
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err, _ := r.(error)

	if r == nil || Ok(err) {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}

// WithRecoverCall runs a function without arguments that does not
// produce an error and, if the function panics, converts it into an
// error.
func WithRecoverCall(fn func()) (err error) {
	defer func() { err = ParsePanic(recover()) }()
	fn()
	return
}

// WrapRecoverCall wraps a function without arguments that does not
// produce an error with one that does produce an error. When called,
// the new function will never panic but returns an error if the input
// function panics.
func WrapRecoverCall(fn func()) func() error { return func() error { return WithRecoverCall(fn) } }

// WithRecoverDo runs a function with a panic handler that converts
// the panic to an error.
func WithRecoverDo[T any](fn func() T) (_ T, err error) {
	defer func() { err = ParsePanic(recover()) }()
	return fn(), nil
}

// WithRecoverApply runs a function with a panic handler that converts
// the panic to an error.
func WithRecoverApply[T any](op func(T), in T) (err error) {
	defer func() { err = ParsePanic(recover()) }()
	op(in)
	return
}

// WithRecoverOk runs a function and returns true if there are no errors and
// no panics the bool output value is true, otherwise it is false.
func WithRecoverOk[T any](fn func() (T, error)) (zero T, ok bool) {
	defer func() { ok = !IsError(ParsePanic(recover())) && ok }()
	if value, err := fn(); Ok(err) {
		return value, Ok(err)
	}
	return zero, false
}

// WrapRecoverDo wraps a function that returns a single value, with
// one that returns that argument and an error if the underlying
// function panics.
func WrapRecoverDo[T any](fn func() T) func() (T, error) {
	return func() (T, error) { return WithRecoverDo(fn) }
}

// WrapRecoverOk takes a function that returns an error and a value,
// and returns a wrapped function that also catches any panics in the
// input function, and returns the value and a boolean that is true if
// the input function does not return an error or panic.
func WrapRecoverOk[T any](fn func() (T, error)) func() (T, bool) {
	return func() (T, bool) { return WithRecoverOk(fn) }
}
