package ers

import (
	"fmt"
)

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

// Check, like Safe, runs a function without arguments that does not
// produce an error, and, if the function panics, converts it into an
// error.
func Check(fn func()) (err error) {
	defer func() { err = ParsePanic(recover()) }()
	fn()
	return
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() { err = ParsePanic(recover()) }()
	out = fn()
	return
}

// SafeOK runs a function and returns true if there are no errors and
// no panics the bool output value is true, otherwise it is false.
func SafeOK[T any](fn func() (T, error)) (out T, ok bool) {
	defer func() { ok = !IsError(ParsePanic(recover())) && ok }()
	if value, err := fn(); OK(err) {
		out, ok = value, true
	}
	return
}
