package ers

import (
	"errors"

	"github.com/tychoish/fun/internal"
)

// Unwind assembles the full "unwrapped" list of all component
// errors. Supports error implementations where the Unwrap() method
// returns either error or []error.
//
// Unwind provides a special case of the dt.Unwind operation.
//
// If an error type implements interface{ Unwind() []error }, this
// takes precedence over Unwrap when unwinding errors, to better
// support the Stack type and others where the original error is
// nested within the unwrapped error objects.
func Unwind(in error) []error { return internal.Unwind(in) }

// New constructs an error object that uses the Error as the
// underlying type.
func New(str string) error { return Error(str) }

// As is a wrapper around errors.As to allow ers to be a drop in
// replacement for errors.
func As(err error, target any) bool { return errors.As(err, target) }

// Unwrap is a wrapper around errors.Unwrap to allow ers to be a drop in
// replacement for errors.
func Unwrap(err error) error { return errors.Unwrap(err) }

// Is returns true if the error is one of the target errors, (or one
// of it's constituent (wrapped) errors is a target error. ers.Is uses
// errors.Is.
func Is(err error, targets ...error) bool {
	for _, target := range targets {
		if err == nil && target != nil {
			continue
		}
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
