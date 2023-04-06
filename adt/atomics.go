// Package adt provides "atomic data types" as strongly-typed generic
// helpers for simple atomic operations (including sync.Map,
// sync.Pool, and a typed value).
package adt

import (
	"sync/atomic"

	"github.com/tychoish/fun"
)

// Atomic is a very simple atomic Get/Set operation, providing a
// generic type-safe implementation wrapping sync/atomic.Value.
type Atomic[T any] struct{ val atomic.Value }

// NewAtomic creates a new Atomic Get/Set value with the initial value
// already set.
func NewAtomic[T any](initial T) *Atomic[T] { a := &Atomic[T]{}; a.Set(initial); return a }

// Set atomically sets the value of the Atomic.
func (a *Atomic[T]) Set(in T) { a.val.Store(in) }

// Get resolves the atomic value, returning the zero value of the type
// T if the value is unset.
func (a *Atomic[T]) Get() T { return fun.ZeroWhenNil[T](a.val.Load()) }

// Swap does an in place exchange of the contents of a value
// exchanging the new value for the old. Unlike sync.Atomic.Swap() if
// new is nil, adt.Atomic.Swap() does NOT panic, and instead
// constructs the zero value of type T.
func (a *Atomic[T]) Swap(new T) (old T) {
	v := a.val.Swap(fun.ZeroWhenNil[T](new))
	if v == nil {
		return old
	}
	return v.(T)
}

// CompareAndSwap exposes the CompareAndSwap option for atomics that
// store values of comparable types.
func CompareAndSwap[T comparable](a *Atomic[T], old, new T) bool {
	return a.val.CompareAndSwap(old, new)
}

// IsAtomicZero checks an atomic value for a comparable type to see if
// it's zero. The fun.IsZero() function can't correctly check both that
// the Atomic is zero and that it holds a zero value, and because
// atomics need not be comparable this can't be a method on Atomic.
func IsAtomicZero[T comparable](in *Atomic[T]) bool {
	if in == nil {
		return true
	}
	return fun.IsZero(in.Get())
}

// SafeSet sets the atomic to the given value only if the value is not
// the Zero value for that type.
func SafeSet[T comparable](atom *Atomic[T], value T) {
	if !fun.IsZero(value) && atom != nil {
		atom.Set(value)
	}
}
