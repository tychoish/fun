package adt

import (
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/ft"
)

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
//
// Deprecated: Use sync.OnceValue from the standard library. Be aware that this will have slightly different around panic handling.
func Mnemonize[T any](in func() T) func() T { return ft.OnceDo(in) }

// Once provides a mnemonic form of sync.Once, caching and returning a
// value after the Do() function is called.
//
// Panics are only thrown when the underlying constructor is called
// (and it panics.) Nil constructors are ignored and subsequent
// attempts to access the value will return the zero value for the
// return type.
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
// Once value is populated and the .Resolve() method will return the value.
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
func (o *Once[T]) Do(ctor func() T)     { o.once.Do(func() { o.ctor.Set(ctor); o.populate() }) }
func (o *Once[T]) Call(ctor func() T) T { o.Do(ctor); return o.comp }

// Resolve runs the stored, if and only if it hasn't been run function
// and returns its output. If the constructor hasn't been populated,
// as with Set(), then Resolve() will return the zero value for
// T. Once the function has run, Resolve will continue to return the
// cached value.
func (o *Once[T]) Resolve() T { o.once.Do(o.populate); return o.comp }

func (o *Once[T]) populate() {
	ft.WhenCall(o.called.CompareAndSwap(false, true), func() { o.comp = ft.SafeDo(o.ctor.Get()); o.ctor.Set(nil) })
}

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
