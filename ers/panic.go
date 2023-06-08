package ers

import (
	"errors"
	"fmt"
)

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
var ErrRecoveredPanic = errors.New("recovered panic")

func ParsePanic(r any) error {
	if r != nil {
		switch err := r.(type) {
		case error:
			return Merge(err, ErrRecoveredPanic)
		default:
			return Merge(fmt.Errorf("%v", err), ErrRecoveredPanic)
		}
	}
	return nil
}

// Check, like Safe, runs a function without arguments that does not
// produce an error, and, if the function panics, converts it into an
// error.
func Check(fn func()) (err error) {
	defer func() { err = Merge(err, ParsePanic(recover())) }()
	fn()
	return
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() { err = Merge(err, ParsePanic(recover())) }()
	out = fn()
	return
}

// Protect wraps a function with a panic handler, that will parse and
// attach the content of the pantic to the error output (while
// maintaining the functions orginial error.) All handled panics will
// be annotated with fun.ErrRecoveredPanic.
func Protect[I any, O any](fn func(I) (O, error)) func(I) (O, error) {
	return func(in I) (out O, err error) {
		defer func() { err = Merge(err, ParsePanic(recover())) }()
		return fn(in)
	}
}
