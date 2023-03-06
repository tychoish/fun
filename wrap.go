package fun

// Is a generic version of `errors.Is` that takes advantage of the
// Unwrap function, and is useful for checking if an object of an
// interface type is or wraps an implementation of the type
// parameter.
func Is[T any](in any) bool {
	for {
		if _, ok := in.(T); ok {
			return true
		}
		if in = Unwrap(in); in == nil {
			return false
		}
	}
}

// Unwrap is a generic equivalent of the `errors.Unwrap()` function
// for any type that implements an `Unwrap() T` method. useful in
// combination with Is.
func Unwrap[T any](in T) T {
	u, ok := any(in).(interface{ Unwrap() T })
	if !ok {
		return ZeroOf[T]()
	}
	return u.Unwrap()
}

// Zero returns the zero-value for the type T of the input argument.
func Zero[T any](in T) T { return ZeroOf[T]() }

// ZeroOf returns the zero-value for the type T specified as an
// argument.
func ZeroOf[T any]() T { return *new(T) }

// IsZero returns true if the input value compares "true" to the zero
// value for the type of the argument. If the type implements an
// IsZero() method (e.g. time.Time), then IsZero returns that value,
// otherwise, IsZero constructs a zero valued object of type T and
// compares the input value to the zero value.
func IsZero[T comparable](in T) bool {
	switch val := any(in).(type) {
	case interface{ IsZero() bool }:
		return val.IsZero()
	default:
		return val == Zero(in)
	}
}

// ZeroWhenNil takes a value of any type, and if that value is nil,
// returns the zero value of the specified type. Otherwise,
// ZeroWhenNil coerces the value into T and returns it. If the input
// value does not match the output type of the function, ZeroWhenNil
// panics with an ErrInvariantViolation.
func ZeroWhenNil[T any](val any) T {
	if val == nil {
		return ZeroOf[T]()
	}
	out, ok := val.(T)

	Invariant(ok, "unexpected type mismatch")

	return out
}
