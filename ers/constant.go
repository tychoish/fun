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

// Error is a type alias for building/declaring sentinel errors
// as constants.
//
// In addition to nil error interface values, the Empty string is,
// considered equal to nil errors for the purposes of Is(). errors.As
// correctly handles unwrapping and casting Error-typed error objects.
type Error string

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

////////////////////////////////////////////////////////////////////////
//
// Sentinel Errors
//
////////////////////////////////////////////////////////////////////////

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
