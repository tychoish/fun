package dt

import "github.com/tychoish/fun/internal"

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

// Unwind uses the Unwrap operation to build a list of the "wrapped"
// objects.
func Unwind[T any](in T) []T { return internal.Unwind(in) }
