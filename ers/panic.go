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
		default:
			return Join(fmt.Errorf("%v", err), ErrRecoveredPanic)
		}
	}
	return nil
}

// Check, like Safe, runs a function without arguments that does not
// produce an error, and, if the function panics, converts it into an
// error.
func Check(fn func()) (err error) {
	defer func() { err = Join(err, ParsePanic(recover())) }()
	fn()
	return
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() { err = Join(err, ParsePanic(recover())) }()
	out = fn()
	return
}
