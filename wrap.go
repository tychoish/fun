package fun

// Wrapper produces a function that always returns the value
// provided. Useful for bridging interface paradigms, and for storing
// interface-typed objects in atomics.
func Wrapper[T any](in T) func() T { return func() T { return in } }

// Unwrap is a generic equivalent of the `errors.Unwrap()` function
// for any type that implements an `Unwrap() T` method. useful in
// combination with Is.
func Unwrap[T any](in T) T {
	u, ok := doUnwrap(in)
	if !ok {
		return ZeroOf[T]()
	}
	return u.Unwrap()
}

// UnwrapedRoot unwinds a wrapped object and returns the innermost
// non-nil wrapped item
func UnwrapedRoot[T any](in T) T {
	for {
		u, ok := doUnwrap(in)
		if !ok {
			return in
		}
		in = u.Unwrap()
	}
}

// Unwind uses the Unwrap operation to build a list of the "wrapped"
// objects.
func Unwind[T any](in T) []T {
	out := []T{}

	out = append(out, in)
	for {
		switch wi := any(in).(type) {
		case wrapped[T]:
			in = wi.Unwrap()
			out = append(out, in)
		case wrappedMany[T]:
			for _, item := range wi.Unwrap() {
				out = append(out, Unwind(item)...)
			}

			return out
		default:
			return out
		}
	}
}

type wrapped[T any] interface{ Unwrap() T }
type wrappedMany[T any] interface{ Unwrap() []T }

func doUnwrap[T any](in T) (wrapped[T], bool) { u, ok := any(in).(wrapped[T]); return u, ok }

// Zero returns the zero-value for the type T of the input argument.
func Zero[T any](T) T { return ZeroOf[T]() }

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
		return in == Zero(in)
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
