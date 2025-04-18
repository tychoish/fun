package ers

import (
	"context"
	"errors"
	"io"
)

// ErrImmutabilityViolation is an returned by some operations or
// invariant violations when an operation attempts to modify logically
// immutable value.
const ErrImmutabilityViolation Error = Error("immutability violation")

// ErrMalformedConfiguration indicates a configuration object that has
// failed validation.
const ErrMalformedConfiguration Error = Error("malformed configuration")

// ErrLimitExceeded is a constant sentinel error that indicates that a
// limit has been exceeded. These are generally retriable.
const ErrLimitExceeded Error = Error("limit exceeded")

// ErrInvalidRuntimeType signals a type error encountered at
// runtime. Typically included as a component in an aggregate
// invariant violation error.
const ErrInvalidRuntimeType Error = Error("invalid type at runtime")

// ErrCurrentOpAbort is used to signal that a retry function or other
// looping operation that the loop should exit. ErrCurrentOpAbort
// should be handled like "break", and should not be returned to
// callers or aggregated with other errors.
const ErrCurrentOpAbort Error = Error("abort current operation")

// ErrCurrentOpSkip is used to signal that a retry function or other
// looping loop should continue. In most cases, this error should not
// be returned to callers or aggregated with other errors.
const ErrCurrentOpSkip Error = Error("skip current operation")

// ErrContainerClosed is returned for operations against a container
// that has been closed or finished.
const ErrContainerClosed Error = Error("container is closed")

// ErrNotImplemented indicates an unfinished implementation. These
// errors are not generally retriable.
const ErrNotImplemented Error = Error("not implemented")

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
const ErrRecoveredPanic Error = Error("recovered panic")

// ErrInvalidInput indicates malformed input. These errors are not
// generally retriable.
const ErrInvalidInput Error = Error("invalid input")

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
const ErrInvariantViolation Error = Error("invariant violation")

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
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err, _ := r.(error)

	if r == nil || Ok(err) {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}
