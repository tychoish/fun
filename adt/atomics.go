package adt

import "github.com/tychoish/fun"

// IsAtomicZero checks an atomic value for a comparable type to see if
// it's zero. The fun.IsZero() function can't correctly check both that
// the Atomic is zero and that it holds a zero value, and because
// atomics need not be comparable this can't be a method on Atomic.
func IsAtomicZero[T comparable, A *fun.Atomic[T]](in *fun.Atomic[T]) bool {
	if in == nil {
		return true
	}
	return fun.IsZero(in.Get())
}

// SafeSet sets the atomic to the given value only if the value is not
// the Zero value for that type.
func SafeSet[T comparable](atom *fun.Atomic[T], value T) {
	if !fun.IsZero(value) {
		atom.Set(value)
	}
}
