package fun

import (
	"sync"
	"time"

	"github.com/tychoish/fun/ft"
)

// Future is a basic function for providing a fun-style function type
// for a function object that will produce an object of the specified
// type.
type Future[T any] func() T

// Futureize is a simple wrapper to convert a function object to a
// Future[T] object.
//
// Deprecated: Use MakeFuture instead.
func Futurize[T any](f func() T) Future[T] { return f }

// MakeFuture constructs a Future[T] object from a function
// object.
func MakeFuture[T any](f func() T) Future[T] { return f }

// AsFuture wraps a value and returns a future object that, when
// called, will return the provided value.
func AsFuture[T any](in T) Future[T] { return ft.Wrapper(in) }

// Translate converts a future from one type to another.
func Translate[T any, O any](in Future[T], tfn func(T) O) Future[O] {
	return func() O { return tfn(in()) }
}

// Resolve executes the future and returns its value.
func (f Future[T]) Resolve() T { return f() }

// Once returns a future that will only run the underlying future
// exactly once.
func (f Future[T]) Once() Future[T] { return sync.OnceValue(f) }

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

// WithLock return a future that is protected with the provided mutex.
func (f Future[T]) WithLock(m sync.Locker) Future[T] {
	return func() T { defer with(lock(m)); return f() }
}

// PreHook unconditionally runs the provided function before running
// and returning the function.
func (f Future[T]) PreHook(fn func()) Future[T] { return func() T { fn(); return f() } }

// PostHook unconditionally runs the provided function after running
// the future. The hook runs in a defer statement.
func (f Future[T]) PostHook(fn func()) Future[T] { return func() T { defer fn(); return f() } }

// Slice returns a future-like function that wraps the output of the
// future as the first element in a slice.
func (f Future[T]) Slice() func() []T { return func() []T { return []T{f()} } }

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

func (f Future[T]) TTL(dur time.Duration) Future[T] {
	resolver := ttlExec[T](dur)
	return func() T { return resolver(f) }
}

func (f Future[T]) Limit(in int) Future[T] {
	resolver := limitExec[T](in)
	return func() T { return resolver(f) }
}
