// Package adt provides "atomic data types" as strongly-typed generic
// helpers for simple atomic operations (including sync.Map,
// sync.Pool, and a typed value).
package adt

import (
	"fmt"
	"sync/atomic"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/stw"
)

// AtomicValue describes the public interface of the Atomic type. Use
// this definition to compose atomic types into other interfaces.
type AtomicValue[T any] interface {
	Load() T
	Store(T)
	Swap(T) T
}

// Atomic is a very simple atomic Get/Set operation, providing a
// generic type-safe implementation wrapping sync/atomic.Value. The
// primary caveat is that interface types are not compatible with
// adt.Atomic as a result of the standard library's underlying atomic
// type. To store interface objects atomically you can wrap the
// object in a function, using ft.Wrapper.
type Atomic[T any] struct{ val atomic.Value }

// NewAtomic creates a new Atomic Get/Set value with the initial value
// already set. This is a helper for creating a new atomic with a
// default value set.
func NewAtomic[T any](initial T) *Atomic[T] { a := &Atomic[T]{}; a.Set(initial); return a }

// Set atomically sets the value of the Atomic.
func (a *Atomic[T]) Set(in T) { a.Store(in) }

// Store saves the value in the atomic.
func (a *Atomic[T]) Store(in T) { a.val.Store(in) }

// Get resolves the atomic value, returning the zero value of the type
// T if the value is unset.
func (a *Atomic[T]) Get() T { return a.Load() }

// Load returns the value stored in the atomic. It mirrors the
// standard library's interface for atomics.
func (a *Atomic[T]) Load() T           { return a.cast(a.val.Load()) }
func (*Atomic[T]) cast(in any) (out T) { out, _ = in.(T); return }

// Swap does an in place exchange of the contents of a value
// exchanging the new value for the old. Unlike sync.Atomic.Swap() if
// new is nil, adt.Atomic.Swap() does NOT panic, and instead
// constructs the zero value of type T.
func (a *Atomic[T]) Swap(newVal T) (old T) {
	switch v := a.val.Swap(newVal).(type) {
	case T:
		return v
	default:
		return old
	}
}

// CompareAndSwap exposes the CompareAndSwap option for atomics that
// store values of comparable types. Only supports the Atomic and
// Synchronized types, as well as any type that implement a
// CompareAndSwap method for old/new values of T. Panics for all other
// types.
func CompareAndSwap[T comparable, A AtomicValue[T]](a A, oldVal, newVal T) bool {
	switch atom := any(a).(type) {
	case *Atomic[T]:
		return atom.val.CompareAndSwap(oldVal, newVal)
	case *Synchronized[T]:
		defer With(Lock(&atom.mtx))
		if atom.obj == oldVal {
			atom.obj = newVal
			return true
		}
		return false
	case interface{ CompareAndSwap(a, b T) bool }:
		return atom.CompareAndSwap(oldVal, newVal)
	default:
		panic(fmt.Errorf("compare and swap operation not supported: %w", ers.ErrInvariantViolation))
	}
}

// Reset sets the atomic, for an atomic value that holds a number, to
// 0, and returns the previously stored value.
func Reset[T stw.Integers, A AtomicValue[T]](a A) T {
	var delta T
	for {
		delta = a.Load()
		if CompareAndSwap(a, delta, 0) {
			break
		}
	}
	return delta
}

func isAtomicValueNil[T comparable, A AtomicValue[T]](in A) bool {
	switch v := any(in).(type) {
	case *Atomic[T]:
		return v == nil
	case *Synchronized[T]:
		return v == nil
	default:
		return v == nil
	}
}

// SafeSet sets the atomic to the given value only if the value is not
// the Zero value for that type.
func SafeSet[T comparable, A AtomicValue[T]](atom A, value T) {
	if stw.IsZero(value) || isAtomicValueNil[T, A](atom) {
		return
	}

	atom.Store(value)
}
