package fn

import (
	"sync"

	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Handler describes a function that operates on a single object, but
// returns no output, and is used primarily for side effects,
// particularly around handling errors or collecting metrics. The
// Handler implementation here makes it possible to provide
// panic-safety for these kinds of functions or easily convert to
// other related types.
type Handler[T any] func(T)

// NewHandler produces an Handler[T] function as a helper.
func NewHandler[T any](in func(T)) Handler[T] { return in }

func NewNoopHandler[T any]() Handler[T] { return func(T) {} }

// JoinHandlers returns a function that combines a collection of
// handlers.
func JoinHandlers[T any](ops []Handler[T]) Handler[T] {
	return func(value T) {
		for idx := range ops {
			ft.ApplySafe(ops[idx], value)
		}
	}
}

// ErrorHandler creates a function that can be used to wrap a function
// call that returns a value and an error, to simplify some call sites.
func ErrorHandler[T any](eh Handler[error]) func(T, error) T {
	return func(v T, err error) T { eh(err); return v }
}

// Safe returns a function that will execute the underlying Handler
// function, only when the function is non-nil. Use WithRecover and
// RecoverPanic to handle panics.
func (of Handler[T]) Safe() Handler[T] { return of.callSafe }
func (of Handler[T]) callSafe(in T)    { ft.ApplyWhen(of != nil, of, in) }

// Handler provides a more expository operation to call a handler
// function.
func (of Handler[T]) Read(in T) { of(in) }

// WithRecover handles any panic encountered during the handler's execution
// and converts it to an error.
func (of Handler[T]) WithRecover(oe Handler[error]) Handler[T] {
	return func(in T) { oe(of.RecoverPanic(in)) }
}

// RecoverPanic runs the handler function with a panic handler and converts
// a possible panic to an error.
func (of Handler[T]) RecoverPanic(in T) error { return ft.WithRecoverApply(of, in) }

// If returns an handler that only executes the root handler if the
// condition is true.
func (of Handler[T]) If(cond bool) Handler[T] { return of.When(ft.Wrap(cond)) }

// When returns an handler function that only executes the handler
// function if the condition function returns true. The condition
// function is run every time the handler function runs.
func (of Handler[T]) When(cond func() bool) Handler[T] {
	return func(in T) { ft.ApplyWhen(cond(), of, in) }
}

// Skip runs a check before passing the object to the obsever, when
// the condition function--which can inspect the input object--returns
// true, the underlying Handler is called, otherwise, the observation
// is a noop.
func (of Handler[T]) Skip(hook func(T) bool) Handler[T] {
	return func(in T) { ft.ApplyWhen(hook != nil && hook(in), of, in) }
}

// Filter creates an handler that only executes the root handler Use
// this to process or transform the input before it is passed to the
// underlying handler. Use in combination with the Skip function to
// filter out non-actionable inputs.
func (of Handler[T]) Filter(filter func(T) T) Handler[T] {
	return func(in T) { of(filter(in)) }
}

// Join creates an handler function that runs both the root handler
// and the "next" handler.
func (of Handler[T]) Join(next Handler[T]) Handler[T] {
	return func(in T) { of(in); next(in) }
}

// PreHook provides the inverse operation to Join, running "prev"
// handler before the base handler.
//
// Returns itself if prev is nil.
func (of Handler[T]) PreHook(prev Handler[T]) Handler[T] { return prev.Join(of) }

// Chain calls the base handler, and then calls every handler in the chain.
func (of Handler[T]) Chain(chain ...Handler[T]) Handler[T] {
	return func(in T) {
		of(in)
		for idx := range chain {
			chain[idx](in)
		}
	}
}

// Once produces an handler function that runs exactly once, and
// successive executions of the handler are noops.
func (of Handler[T]) Once() Handler[T] {
	once := &sync.Once{}
	return func(in T) { once.Do(func() { of(in) }) }
}

// Lock returns an handler that is protected by a mutex. All
// execution's of the handler are isolated.
func (of Handler[T]) Lock() Handler[T] { return of.WithLock(&sync.Mutex{}) }

// WithLock protects the action of the handler with the provied mutex.
func (of Handler[T]) WithLock(mtx *sync.Mutex) Handler[T] {
	return func(in T) { defer internal.With(internal.Lock(mtx)); of(in) }
}

// WithLock protects the action of the handler with the provied mutex.
func (of Handler[T]) WithLocker(mtx sync.Locker) Handler[T] {
	return func(in T) { defer internal.WithL(internal.LockL(mtx)); of(in) }
}

// All processes all inputs with the specified handler.
func (of Handler[T]) All(in ...T) {
	for idx := range in {
		of(in[idx])
	}
}
