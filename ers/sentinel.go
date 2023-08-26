package ers

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
const ErrInvariantViolation Error = Error("invariant violation")

// ErrImmutabilityViolation is an returned by some operations or
// invariant violations when an operation attempts to modify logically
// immutable value.
const ErrImmutabilityViolation Error = Error("immutability violation")

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
