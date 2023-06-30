package ers

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Wrap produces a wrapped error if the err is non-nil, wrapping the
// error with the provided annotation. When the error is nil, Wrap
// returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrap" but
// taking advantage of newer standard library error wrapping.
func Wrap(err error, annotation ...any) error {
	if OK(err) {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprint(annotation...), err)
}

// Wrapf produces a wrapped error, if the error is non-nil, with a
// formated wrap annotation. When the error is nil, Wrapf does not
// build an error and returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrapf" but
// taking advantage of newer standard library error wrapping.
func Wrapf(err error, tmpl string, args ...any) error {
	if OK(err) {
		return nil
	}

	return fmt.Errorf("%s: %w", fmt.Sprintf(tmpl, args...), err)
}

// ContextExpired checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func ContextExpired(err error) bool { return Is(err, context.Canceled, context.DeadlineExceeded) }

// OK returns true when the error is nil, and false otherwise. It
// should always be inlined, and mostly exists for clarity at call
// sites in bool/OK check relevant contexts.
func OK(err error) bool { return err == nil }

// IsError returns true when the error is non-nill. Provides the
// inverse of OK().
func IsError(err error) bool { return !OK(err) }

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

// IsTerminating returns true if the error is one of the sentinel
// errors used by fun (and other packages!) to indicate that
// processing/iteration has terminated. (e.g. context expiration, or
// io.EOF.)
func IsTerminating(err error) bool {
	return Is(err, io.EOF, context.Canceled, context.DeadlineExceeded)
}

// IsInvariantViolation returns true if the argument is or resolves to
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err := Cast(r)
	if r == nil || OK(err) {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}

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
