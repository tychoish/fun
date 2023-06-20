package ers

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
const ErrInvariantViolation = Error("invariant violation")

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
const ErrRecoveredPanic = Error("recovered panic")

// ErrMalformedConfiguration indicates a configuration object that has
// failed validation.
const ErrMalformedConfiguration = Error("malformed configuration")

const ErrLimitExceeded = Error("limit exceeded")

const ErrInvalidInput = Error("invalid input")
