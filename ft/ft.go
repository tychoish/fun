// Package ft provides high-level function tools for manipulating common function objects and types.
package ft

// Noop returns the input value.
func Noop[T any](in T) T { return in }

// Zero returns the zero value for a given type. The compiler can't
// determine the type automatically (typically), so have to invoke
// this as `Zero[int]()`.
func Zero[T any]() (zero T) { return zero }

// ZeroFor returns the zero value for a given type, but ignores the input. This makes it easier to use than Zero in cases where you just need a zero value of a type you already have.
func ZeroFor[T any](_ T) T { return Zero[T]() }

// Wrap produces a function that always returns the value
// provided. Useful for bridging interface paradigms, and for storing
// interface-typed objects in atomics.
func Wrap[T any](in T) func() T { return func() T { return in } }

// Flip takes two arguments and returns them in the opposite
// order. Intended to wrap other functions to reduce the friction when
// briding APIs.
func Flip[A any, B any](first A, second B) (B, A) { return second, first }

// Slice returns a slice for the variadic arguments. Useful for
// adapting functions that take slice arguments where it's easier to
// pass values variadicly.
func Slice[T any](items ...T) []T { return items }
