package ers

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
const ErrInvariantViolation Error = Error("invariant violation")

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
const ErrRecoveredPanic Error = Error("recovered panic")

// ErrMalformedConfiguration indicates a configuration object that has
// failed validation.
const ErrMalformedConfiguration Error = Error("malformed configuration")

// ErrLimitExceeded is a constant sentinel error that indicates that a
// limit has been exceeded. These are generally retriable.
const ErrLimitExceeded Error = Error("limit exceeded")

// ErrInvalidInput indicates malformed input. These errors are not
// generally retriable.
const ErrInvalidInput Error = Error("invalid input")
