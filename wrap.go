package fun

import (
	"context"
	"io"

	"github.com/tychoish/fun/internal"
)

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
		if !IsWrapped(in) {
			return in
		}
		in = Unwrap(in)
	}
}

// Unwind uses the Unwrap operation to build a list of the "wrapped"
// objects.
func Unwind[T any](in T) []T {
	out := []T{}

	out = append(out, in)
	for {
		u, ok := doUnwrap(in)
		if !ok {
			break
		}
		in = u.Unwrap()
		out = append(out, in)
	}
	return out
}

// UnwindIterator unwinds an object as in unwind but produces the
// result as an Iterator. The iterative approach may be more ergonomic
// in some situations, but also eliminates the need to create a copy
// the unwound stack of objects to a slice.
func UnwindIterator[T any](root T) Iterator[T] {
	var next *T
	next = &root
	return Generator(func(context.Context) (T, error) {
		if next == nil {
			return internal.ZeroOf[T](), io.EOF
		}
		item := *next

		wrapped, ok := doUnwrap(item)
		if !ok {
			next = nil
		} else {
			ni := wrapped.Unwrap()
			next = &ni
		}

		return item, nil
	})
}

type wrapped[T any] interface{ Unwrap() T }

func doUnwrap[T any](in T) (wrapped[T], bool) { u, ok := any(in).(wrapped[T]); return u, ok }

// IsWrapped returns true if the object is wrapped (e.g. implements an
// Unwrap() method returning its own type). and false otherwise.
func IsWrapped[T any](in T) bool { return Is[wrapped[T]](in) }

// Zero returns the zero-value for the type T of the input argument.
func Zero[T any](T) T { return ZeroOf[T]() }

// ZeroOf returns the zero-value for the type T specified as an
// argument.
func ZeroOf[T any]() T { return internal.ZeroOf[T]() }

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
