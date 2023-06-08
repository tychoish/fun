// Package adt provides "atomic data types" as strongly-typed generic
// helpers for simple atomic operations (including sync.Map,
// sync.Pool, and a typed value).
package adt

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun"
)

// AtomicValue describes the public interface of the Atomic type. Use
// this definition to compose atomic types into other interfaces.
type AtomicValue[T any] interface {
	Get() T
	Set(T)
	Swap(T) T
}

// Mnemonize, like adt.Once, provides a way to lazily resolve and
// cache a value. Mnemonize takes an input function that returns a
// type and returns a function of the same signature. When the
// function is called the first time it caches the value and returns
// it henceforth.
//
// While the function produced by Mnemonize is safe to use
// concurrently, there is no provision for protecting mutable types
// returned by the function and concurrent modification of mutable
// returned values is a race.
func Mnemonize[T any](in func() T) func() T {
	once := &sync.Once{}
	var value T

	return func() T {
		once.Do(func() { value = in() })
		return value
	}
}

// Once provides a mnemonic form of sync.Once, caching and returning a
// value after the Do() function is called.
type Once[T any] struct {
	act sync.Once
	val T
}

// Do runs the function provided and returns its value. All subsequent
// calls to Do return the value of the first function passed to do. If
// multiple callers use Do at the same time, like sync.Once.Do none
// will return until return until the first operation completes.
//
// Functions passed to Do should return values that are safe for
// concurrent access: while the Do operation is synchronized, the
// return value from Do is responsible for its own synchronization.
func (o *Once[T]) Do(constr func() T) T { o.act.Do(func() { o.val = constr() }); return o.val }

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
func (a *Atomic[T]) Get() (out T) { return castOrZero[T](a.val.Load()) }

// Swap does an in place exchange of the contents of a value
// exchanging the new value for the old. Unlike sync.Atomic.Swap() if
// new is nil, adt.Atomic.Swap() does NOT panic, and instead
// constructs the zero value of type T.
func (a *Atomic[T]) Swap(new T) (old T) {
	switch v := a.val.Swap(castOrZero[T](new)).(type) {
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
func CompareAndSwap[T comparable](a AtomicValue[T], old, new T) bool {
	switch atom := a.(type) {
	case *Atomic[T]:
		return atom.val.CompareAndSwap(old, new)
	case *Synchronized[T]:
		defer atom.withLock()()
		if atom.obj == old {
			atom.obj = new
			return true
		}
		return false
	case interface{ CompareAndSwap(a, b T) bool }:
		return atom.CompareAndSwap(old, new)
	default:
		panic(fmt.Errorf("compare and swap operation not supported: %w", fun.ErrInvariantViolation))
	}
}

// IsAtomicZero checks an atomic value for a comparable type to see if
// it's zero. The fun.IsZero() function can't correctly check both that
// the Atomic is zero and that it holds a zero value, and because
// atomics need not be comparable this can't be a method on Atomic.
func IsAtomicZero[T comparable](in AtomicValue[T]) bool {
	return isAtomicValueNil(in) || fun.IsZero(in.Get())
}

func isAtomicValueNil[T comparable](in AtomicValue[T]) bool {
	switch v := in.(type) {
	case *Atomic[T]:
		return v == nil
	case *Synchronized[T]:
		return v == nil
	case interface{ IsZero() bool }:
		return v.IsZero()
	default:
		return v == nil
	}
}

// castOrZero takes a value of any type, and if that value is nil,
// returns the zero value of the specified type. Otherwise,
// castOrZero coerces the value into T and returns it.
func castOrZero[T any](val any) (out T) {
	switch converted := val.(type) {
	case T:
		return converted
	default:
		return out
	}
}

// SafeSet sets the atomic to the given value only if the value is not
// the Zero value for that type.
func SafeSet[T comparable](atom AtomicValue[T], value T) {
	if !fun.IsZero(value) && !isAtomicValueNil(atom) {
		atom.Set(value)
	}
}
