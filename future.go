package fun

import (
	"sync"

	"github.com/tychoish/fun/ft"
)

// Future is a basic function for providing a fun-style function type
// for a function object that will produce an object of the specified
// type.
type Future[T any] func() T

// Futureize is a simple wrapper to convert a function object to a
// Future[T] object.
func Futurize[T any](f func() T) Future[T] { return f }

// AsFuture wraps a value and returns a future object that, when
// called, will return the provided value.
func AsFuture[T any](in T) Future[T] { return func() T { return in } }

// Run executes the future.
func (f Future[T]) Run() T { return f() }

// Once returns a future that will only run the underlying future
// exactly once.
func (f Future[T]) Once() Future[T] { return ft.OnceDo(f) }

// Producer returns a producer function that wraps the future.
func (f Future[T]) Producer() Producer[T] { return ConsistentProducer(f) }

// Ignore produces a function that will call the Future but discards
// the output.
func (f Future[T]) Ignore() func() { return func() { _ = f() } }

// If produces a future that only runs when the condition value is
// true. If the condition is false, the future will return the zero
// value of T.
func (f Future[T]) If(cond bool) Future[T] { return func() T { return ft.WhenDo(cond, f) } }

// Not produces that only runs when the condition value is false. If
// the condition is true, the future will return the zero value of T.
func (f Future[T]) Not(cond bool) Future[T] { return f.If(!cond) }

// When produces a new future wrapping the input future that executes
// when the condition function returns true, returning the zero
// value for T when the condition is false. The condition value is
// checked every time the future is called.
func (f Future[T]) When(c func() bool) Future[T] { return func() T { return ft.WhenDo(c(), f) } }

// Locked returns a wrapped future that ensure that all calls to the
// future are protected by a mutex.
func (f Future[T]) Lock() Future[T] { return f.WithLock(&sync.Mutex{}) }

// PreHook unconditionally runs the provided function before running
// and returning the function.
func (f Future[T]) PreHook(fn func()) Future[T] { return func() T { fn(); return f() } }

// PostHook unconditionally runs the provided function after running
// the future. The hook runs in a defer statement.
func (f Future[T]) PostHook(fn func()) Future[T] { return func() T { defer fn(); return f() } }

// Slice returns a future-like function that wraps the output of the
// future as the first element in a slice.
func (f Future[T]) Slice() func() []T { return func() []T { return []T{f()} } }

// WithLock return a future that is protected with the provided mutex.
func (f Future[T]) WithLock(m *sync.Mutex) Future[T] {
	return func() T { defer with(lock(m)); return f() }
}

// Reduce takes the input future, the next future, and merges the
// results using the merge function.
func (f Future[T]) Reduce(merge func(T, T) T, next Future[T]) Future[T] {
	return func() T { return merge(f(), next()) }
}

// Join iteratively merges a collection of future operations.
func (f Future[T]) Join(merge func(T, T) T, ops ...Future[T]) Future[T] {
	return func() (out T) {
		out = f()
		for idx := range ops {
			out = merge(out, ops[idx]())
		}
		return out
	}
}
