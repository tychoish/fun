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

func CountWraps[T any](in T) int {
	count := 1

	for {
		switch wi := any(in).(type) {
		case interface{ Unwrap() T }:
			count++
			in = wi.Unwrap()
		case interface{ Unwrap() []T }:
			count += len(wi.Unwrap())
			return count
		default:
			return count
		}
	}
}

// Unwind uses the Unwrap operation to build a list of the "wrapped"
// objects.
func Unwind[T any](in T) []T {
	var out []T

	for {
		switch wi := any(in).(type) {
		case interface{ Unwrap() []T }:
			items := wi.Unwrap()
			for _, i := range items {
				if any(i) == nil {
					continue
				}
				out = append(out, i)
			}

			return out
		case interface{ Unwrap() T }:
			in = wi.Unwrap()

			switch any(in).(type) {
			case nil:
				return out
			default:
				out = append(out, in)
			}
		case nil:
			return out
		default:
			return append(out, in)
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
