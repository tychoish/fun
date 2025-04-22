package ers

import (
	"context"
	"fmt"
	"io"

	"github.com/tychoish/fun/internal"
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

// Strings renders (using the Error() method) a slice of errors into a
// slice of their string values.
func Strings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for idx := range errs {
		if !Ok(errs[idx]) {
			out = append(out, errs[idx].Error())
		}
	}

	return out
}

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

// IsExpiredContext checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func IsExpiredContext(err error) bool { return Is(err, context.Canceled, context.DeadlineExceeded) }

// IsTerminating returns true if the error is one of the sentinel
// errors used by fun (and other packages!) to indicate that
// processing/iteration has terminated. (e.g. context expiration, or
// io.EOF.)
func IsTerminating(err error) bool { return Is(err, io.EOF, ErrCurrentOpAbort, ErrContainerClosed) }
