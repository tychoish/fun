package ers

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// When constructs an ers.Error-typed error value IF the conditional
// is true, and returns nil otherwise.
func When(cond bool, val any) error {
	if !cond {
		return nil
	}
	switch e := val.(type) {
	case error:
		return e
	case string:
		return Error(e)
	default:
		return fmt.Errorf("error=%T: %v", val, e)
	}
}

// Whenf constructs an error (using fmt.Errorf) IF the conditional is
// true, and returns nil otherwise.
func Whenf(cond bool, tmpl string, args ...any) error {
	if !cond {
		return nil
	}

	return fmt.Errorf(tmpl, args...)
}

// Wrap produces a wrapped error if the err is non-nil, wrapping the
// error with the provided annotation. When the error is nil, Wrap
// returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrap" but
// taking advantage of newer standard library error wrapping.
func Wrap(err error, annotation ...any) error {
	if Ok(err) {
		return nil
	}
	return Join(err, errors.New(fmt.Sprint(annotation...)))
}

// Wrapf produces a wrapped error, if the error is non-nil, with a
// formated wrap annotation. When the error is nil, Wrapf does not
// build an error and returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrapf" but
// taking advantage of newer standard library error wrapping.
func Wrapf(err error, tmpl string, args ...any) error {
	if Ok(err) {
		return nil
	}

	return Join(err, fmt.Errorf(tmpl, args...))
}

// Ignore discards an error.
func Ignore(_ error) { return } //nolint

// Cast converts an untyped/any object into an error, returning nil if
// the value is not an error or is an error of a nil type.
func Cast(e any) error { err, _ := e.(error); return err }

// Append adds one or more errors to the error slice, omitting all
// nil errors.
func Append(errs []error, es ...error) []error {
	for idx := range es {
		if IsError(es[idx]) {
			errs = append(errs, es[idx])
		}
	}
	return errs
}

// Ok returns true when the error is nil, and false otherwise. It
// should always be inlined, and mostly exists for clarity at call
// sites in bool/Ok check relevant contexts.
func Ok(err error) bool {
	switch e := err.(type) {
	case nil:
		return true
	case interface{ Ok() bool }:
		return e.Ok()
	default:
		return false
	}
}

// IsError returns true when the error is non-nill. Provides the
// inverse of Ok().
func IsError(err error) bool { return !Ok(err) }

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

// IsExpiredContext checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func IsExpiredContext(err error) bool { return Is(err, context.Canceled, context.DeadlineExceeded) }

// IsTerminating returns true if the error is one of the sentinel
// errors used by fun (and other packages!) to indicate that
// processing/iteration has terminated. (e.g. context expiration, or
// io.EOF.)
func IsTerminating(err error) bool {
	return Is(err, io.EOF, ErrCurrentOpAbort, context.Canceled, context.DeadlineExceeded)
}

// IsInvariantViolation returns true if the argument is or resolves to
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err := Cast(r)
	if r == nil || Ok(err) {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}
