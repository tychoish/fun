package fun

import (
	"context"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// Handler describes a function that operates on a single object, but
// returns no output, and is used primarily for side effects,
// particularly around handling errors or collecting metrics. The
// Handler implementation here makes it possible to provide
// panic-safety for these kinds of functions or easily convert to
// other related types.
type Handler[T any] func(T)

// Handle produces an Handler[T] function as a helper.
func Handle[T any](in func(T)) Handler[T] { return in }

// HandlePassthrough creates a bit of syntactic sugar to handle the
// _second_ return value of a function with a provided handler
// function while returning the first. This is often useful for
// processing the error value of a function with a handler while
// returning the first (potentially zero) value.
func HandlePassthrough[T any, O any](hf Handler[O]) func(T, O) T {
	return func(first T, second O) T { hf(second); return first }
}

// Handler provides a more expository operation to call a handler
// function.
func (of Handler[T]) Handle(in T) { of(in) }

// WithRecover handles any panic encountered during the handler's execution
// and converts it to an error.
func (of Handler[T]) WithRecover(oe Handler[error]) Handler[T] {
	return func(in T) { oe(of.RecoverPanic(in)) }
}

// RecoverPanic runs the handler function with a panic handler and converts
// a possible panic to an error.
func (of Handler[T]) RecoverPanic(in T) error { return ers.Check(of.Capture(in)) }

// Worker captures a variable and returns a worker function which
// will, when executed, observe the input value. These worker
// functions, use the Safe-mode of execution.
func (of Handler[T]) Worker(in T) Worker {
	return func(context.Context) error { return of.RecoverPanic(in) }
}

// Operation captures a variable and converts an Handler into a wait
// function that observes the value when the Operation runs.
func (of Handler[T]) Operation(in T) Operation { return func(context.Context) { of(in) } }

// Capture returns a function that handles the specified value,
// but only when executed later.
func (of Handler[T]) Capture(in T) func() { return func() { of(in) } }

// Processor converts the handler to an handler function. The
// Processor will always return nil, and the context is ignored.
func (of Handler[T]) Processor() Processor[T] { return ProcessifyHandler(of) }

// Iterator produces a worker that processes every item in the
// iterator with the handler function function.
func (of Handler[T]) Iterator(iter *Iterator[T]) Worker { return iter.Observe(of) }

// If returns an handler that only executes the root handler if the
// condition is true.
func (of Handler[T]) If(cond bool) Handler[T] { return of.When(ft.Wrapper(cond)) }

// When returns an handler function that only executes the handler
// function if the condition function returns true. The condition
// function is run every time the handler function runs.
func (of Handler[T]) When(cond func() bool) Handler[T] {
	return func(in T) { ft.WhenCall(cond(), of.Capture(in)) }
}

// Skip runs a check before passing the object to the obsever, when
// the condition function--which can inspect the input object--returns
// true, the underlying Handler is called, otherwise, the observation
// is a noop.
func (of Handler[T]) Skip(hook func(T) bool) Handler[T] {
	return func(in T) { ft.WhenHandle(hook, of, in) }
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
func (of Handler[T]) Join(next Handler[T]) Handler[T] { return func(in T) { of(in); next(in) } }

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
func (of Handler[T]) WithLock(mtx sync.Locker) Handler[T] {
	return func(in T) { mtx.Lock(); defer mtx.Unlock(); of(in) }
}

// All processes all inputs with the specified handler.
func (of Handler[T]) All(in ...T) {
	for idx := range in {
		of(in[idx])
	}
}
