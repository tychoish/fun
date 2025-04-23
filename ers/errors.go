package ers

import (
	"errors"
)

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
