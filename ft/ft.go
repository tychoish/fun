package ft

import "iter"

// Wrap produces a function that always returns the value
// provided. Useful for bridging interface paradigms, and for storing
// interface-typed objects in atomics.
func Wrap[T any](in T) func() T { return func() T { return in } }

// Flip takes two arguments and returns them in the opposite
// order. Intended to wrap other functions to reduce the friction when
// briding APIs.
func Flip[A any, B any](first A, second B) (B, A) { return second, first }

// Convert takes a sequence of A and converts it, lazily into a
// sequence of B, using the mapper function.
func Convert[A any, B any](mapper func(A) B, values iter.Seq[A]) iter.Seq[B] {
	return func(yield func(B) bool) {
		for input := range values {
			if !yield(mapper(input)) {
				return
			}
		}
	}
}

// Slice returns a slice for the variadic arguments. Useful for
// adapting functions that take slice arguments where it's easier to
// pass values variadicly.
func Slice[T any](items ...T) []T { return items }
