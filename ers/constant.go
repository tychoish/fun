// Package ers provides some very basic error aggregating and handling
// tools, as a companion to erc.
//
// The packages are similar, though ers is smaller and has no
// dependencies outside of a few packages in the standard library,
// whereas erc is more tightly integrated into fun's ecosystem and
// programming model.
//
// ers has an API that is equivalent to the standard library errors
// package, with some additional tools and minor semantic
// differences.
package ers

import (
	"errors"
)

// Error is a type alias for building/declaring sentinel errors
// as constants.
//
// In addition to nil error interface values, the Empty string is,
// considered equal to nil errors for the purposes of Is(). errors.As
// correctly handles unwrapping and casting Error-typed error objects.
type Error string

// New constructs an error object that uses the Error as the
// underlying type.
func New(str string) error { return Error(str) }

// As is a wrapper around errors.As to allow ers to be a drop in
// replacement for errors.
func As(err error, target any) bool { return errors.As(err, target) }

// Unwrap is a wrapper around errors.Unwrap to allow ers to be a drop in
// replacement for errors.
func Unwrap(err error) error { return errors.Unwrap(err) }

// Error implements the error interface for ConstError.
func (e Error) Error() string { return string(e) }

// Err returns the Error object as an error object (e.g. that
// implements the error interface.) Provided for more ergonomic conversions.
func (e Error) Err() error { return e }

// Is Satisfies the Is() interface without using reflection.
func (e Error) Is(err error) bool {
	switch {
	case err == nil && e == "":
		return e == ""
	case (err == nil) != (e == ""):
		return false
	default:
		switch x := err.(type) {
		case Error:
			return x == e
		default:
			return false
		}
	}
}
