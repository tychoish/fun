// Package adt provides "atomic data types" as strongly-typed generic
// helpers for simple atomic operations (including sync.Map,
// sync.Pool, and a typed value).
package adt

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
)

// AtomicValue describes the public interface of the Atomic type. Use
// this definition to compose atomic types into other interfaces.
type AtomicValue[T any] interface {
	Load() T
	Store(T)
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
func Mnemonize[T any](in func() T) func() T { return ft.OnceDo(in) }

// Once provides a mnemonic form of sync.Once, caching and returning a
// value after the Do() function is called.
type Once[T any] struct {
	ctor    Atomic[func() T]
	once    sync.Once
	called  atomic.Bool
	defined atomic.Bool
	comp    T
}

// NewOnce creates a Once object and initializes it with the function
// provided. This is optional and this function can be later
// overridden by Set() or Do(). When the operation is complete, the
// Once operation has been
func NewOnce[T any](fn func() T) *Once[T] {
	o := &Once[T]{}
	o.defined.Store(true)
	o.ctor.Set(fn)
	return o
}

// Do runs the function provided, and caches the results. All
// subsequent calls to Do or Resolve() are noops. If multiple callers
// use Do/Resolve at the same time, like sync.Once.Do none will return
// until return until the first operation completes.
//
// Functions passed to Do should return values that are safe for
// concurrent access: while the Do/Resolve operations are synchronized,
// the return value from Do is responsible for its own
// synchronization.
func (o *Once[T]) Do(ctor func() T) {
	o.once.Do(func() { o.ctor.Set(ctor); o.defined.Store(true); o.populate() })
}

// Resolve runs the stored, if and only if it hasn't been run function
// and returns its output. Once the function has run, Resolve will
// continue to return the cached value.
func (o *Once[T]) Resolve() T { o.once.Do(o.populate); return o.comp }
func (o *Once[T]) populate()  { o.called.Store(true); o.comp = ft.SafeDo(o.ctor.Get()); o.ctor.Set(nil) }

// Set sets the constrctor/operation for the Once object, but does not
// execute the operation. The operation is atomic, is a noop after the
// operation has completed, will not reset the operation or the cached
// value.
func (o *Once[T]) Set(constr func() T) {
	ft.WhenCall(!o.Called(), func() { o.defined.Store(true); o.ctor.Set(constr) })
}

// Called returns true if the Once object has been called or is
// currently running, and false otherwise.
func (o *Once[T]) Called() bool { return o.called.Load() }

// Defined returns true when the function has been set. Use only for
// observational purpsoses. Though the value is stored in an atomic,
// it does reflect the state of the underlying operation.
func (o *Once[T]) Defined() bool { return o.defined.Load() }

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
func (a *Atomic[T]) Set(in T)   { a.Store(in) }
func (a *Atomic[T]) Store(in T) { a.val.Store(in) }

// Get resolves the atomic value, returning the zero value of the type
// T if the value is unset.
func (a *Atomic[T]) Get() T  { return a.Load() }
func (a *Atomic[T]) Load() T { return ft.SafeCast[T](a.val.Load()) }

// Swap does an in place exchange of the contents of a value
// exchanging the new value for the old. Unlike sync.Atomic.Swap() if
// new is nil, adt.Atomic.Swap() does NOT panic, and instead
// constructs the zero value of type T.
func (a *Atomic[T]) Swap(new T) (old T) {
	switch v := a.val.Swap(new).(type) {
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
func CompareAndSwap[T comparable, A AtomicValue[T]](a A, old, new T) bool {
	switch atom := any(a).(type) {
	case *Atomic[T]:
		return atom.val.CompareAndSwap(old, new)
	case *Synchronized[T]:
		defer With(Lock(&atom.mtx))
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

func Reset[T intish.Numbers, A AtomicValue[T]](a A) T {
	var delta T
	for {
		delta = a.Load()
		if CompareAndSwap(a, delta, 0) {
			break
		}
	}
	return delta
}

// IsAtomicZero checks an atomic value for a comparable type to see if
// it's zero. The ft.IsZero() function can't correctly check both that
// the Atomic is zero and that it holds a zero value, and because
// atomics need not be comparable this can't be a method on Atomic.
func IsAtomicZero[T comparable, A AtomicValue[T]](in A) bool {
	return isAtomicValueNil[T, A](in) || ft.IsZero(in.Load())
}

func isAtomicValueNil[T comparable, A AtomicValue[T]](in A) bool {
	switch v := any(in).(type) {
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

// SafeSet sets the atomic to the given value only if the value is not
// the Zero value for that type.
func SafeSet[T comparable, A AtomicValue[T]](atom A, value T) {
	if !ft.IsZero(value) && !isAtomicValueNil[T, A](atom) {
		atom.Store(value)
	}
}
