package fun

// Wrapper produces a function that always returns the value
// provided. Useful for bridging interface paradigms, and for storing
// interface-typed objects in atomics.
func Wrapper[T any](in T) func() T { return func() T { return in } }

// Unwrap is a generic equivalent of the `errors.Unwrap()` function
// for any type that implements an `Unwrap() T` method. useful in
// combination with Is.
func Unwrap[T any](in T) (out T) {
	switch wi := any(in).(type) {
	case interface{ Unwrap() T }:
		return wi.Unwrap()
	default:
		return out
	}
}

// UnwrapedRoot unwinds a wrapped object and returns the innermost
// non-nil wrapped item
func UnwrapedRoot[T any](in T) T {
	for {
		switch wi := any(in).(type) {
		case interface{ Unwrap() T }:
			in = wi.Unwrap()
		default:
			return in
		}
	}
}

// Unwind uses the Unwrap operation to build a list of the "wrapped"
// objects.
func Unwind[T any](in T) []T {
	out := Sliceify([]T{})

	switch any(in).(type) {
	case nil:
		return nil
	default:
		out.Add(in)
	}

	for {
		switch wi := any(in).(type) {
		case interface{ Unwrap() []T }:
			Sliceify(wi.Unwrap()).Observe(func(in T) { out.Extend(Unwind(in)) })
			return out
		case interface{ Unwrap() T }:
			in = wi.Unwrap()

			switch any(in).(type) {
			case nil:
				return out
			default:
				out.Add(in)
			}
		default:
			return out
		}
	}
}

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
		var comp T
		return in == comp
	}
}

func IsType[T any](in any) bool         { _, ok := in.(T); return ok }
func Cast[T any](in any) (v T, ok bool) { v, ok = in.(T); return }
