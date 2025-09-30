package ers

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/tychoish/fun/internal"
)

// IsOk returns true when the error is nil, and false otherwise. It
// should always be inlined, and mostly exists for clarity at call
// sites in bool/IsOk check relevant contexts.
func IsOk(err error) bool {
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
func IsError(err error) bool { return !IsOk(err) }

// Cast returns e if e is an error and otherwise returns nil.
func Cast(e any) error { err, _ := e.(error); return err }

// IsExpiredContext checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func IsExpiredContext(err error) bool { return Is(err, context.Canceled, context.DeadlineExceeded) }

// IsTerminating returns true if the error is one of the sentinel
// errors used by fun (and other packages!) to indicate that
// processing/iteration has terminated. (e.g. context expiration, or
// io.EOF.)
func IsTerminating(err error) bool { return Is(err, io.EOF, ErrCurrentOpAbort, ErrContainerClosed) }

// IsInvariantViolation returns true if the argument is or resolves to
// ers.ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	if err := Cast(r); IsError(err) {
		return errors.Is(err, ErrInvariantViolation)
	}

	return false
}

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

// If constructs an ers.Error-typed error value IF the conditional
// is true, and returns nil otherwise.
func If(cond bool, err error) error {
	if !cond {
		return nil
	}

	return err
}

// When constructs an error from the provided string when the condition is true, and returns nil otherwise.
func When(cond bool, val string) error {
	if !cond {
		return nil
	}

	return New(val)
}

// Whenf constructs an error (using fmt.Errorf) IF the conditional is
// true, and returns nil otherwise.
func Whenf(cond bool, tmpl string, args ...any) error {
	if !cond {
		return nil
	}

	return fmt.Errorf(tmpl, args...)
}

// Wrap returns an annotated error if the input error is non-nil, and
// returns nil otherwise.
//
// This, roughly mirrors the api "github/pkg/errors.Wrap" but takes
// advantage of newer standard library error wrapping tools.
func Wrap(err error, annotation string) error {
	if IsError(err) {
		return fmt.Errorf("%w: %s", err, annotation)
	}
	return nil
}

// Wrapf returns an annotated error using the given sprintf-style
// template and arguments, and returns nil otherwise.
//
// This, roughly mirrors the api "github/pkg/errors.Wrapf" but takes
// advantage of newer standard library error wrapping tools.
func Wrapf(err error, tmpl string, args ...any) error {
	if IsError(err) {
		return fmt.Errorf(fmt.Sprint("%w: ", tmpl),
			append([]any{err}, args...)...,
		)
	}
	return nil
}
