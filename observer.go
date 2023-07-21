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

// Safe handles any panic encountered during the observer's execution
// and converts it to an error.
func (of Handler[T]) Safe(oe Handler[error]) Handler[T] { return func(in T) { oe(of.Check(in)) } }

// Check runs the observer function with a panic handler and converts
// a possible panic to an error.
func (of Handler[T]) Check(in T) error { return ers.Check(of.Capture(in)) }

// Worker captures a variable and returns a worker function which
// will, when executed, observe the input value. These worker
// functions, use the Safe-mode of execution.
func (of Handler[T]) Worker(in T) Worker { return func(context.Context) error { return of.Check(in) } }

// Operation captures a variable and converts an Handler into a wait
// function that observes the value when the Operation runs.
func (of Handler[T]) Operation(in T) Operation { return func(context.Context) { of(in) } }

// Caputre returns a function that observes the specified variable,
// but only when executed later.
func (of Handler[T]) Capture(in T) func() { return func() { of(in) } }

// Processor converts the observer to an observer function. The
// Processor will always return nil, and the context is ignored.
func (of Handler[T]) Processor() Processor[T] {
	return func(_ context.Context, in T) error { return of.Check(in) }
}

// Iterator produces a worker that processes every item in the
// iterator with the observer function.
func (of Handler[T]) Iterator(iter *Iterator[T]) Worker {
	return func(ctx context.Context) error { return iter.Observe(ctx, of) }
}

// If returns an observer that only executes the root observer if the
// condition is true.
func (of Handler[T]) If(cond bool) Handler[T] { return of.When(ft.Wrapper(cond)) }

// When returns an observer function that only executes the observer
// function if the condition function returns true. The condition
// function is run every time the observer function runs.
func (of Handler[T]) When(cond func() bool) Handler[T] {
	return func(in T) { ft.WhenCall(cond(), of.Capture(in)) }
}

// Skip runs a check before passing the object to the obsever, when
// the condition function--which can inspect the input object--returns
// true, the underlying Handler is called, otherwise, the observation
// is a noop.
func (of Handler[T]) Skip(hook func(T) bool) Handler[T] {
	return func(in T) { ft.WhenHandle(in, hook, of) }
}

// Filter creates an observer that only executes the root observer Use
// this to process or transform the input before it is passed to the
// underlying observer. Use in combination with the Skip function to
// filter out non-actionable inputs.
func (of Handler[T]) Filter(filter func(T) T) Handler[T] {
	return func(in T) { of(filter(in)) }
}

// Once produces an observer function that runs exactly once, and
// successive executions of the observer are noops.
func (of Handler[T]) Once() Handler[T] {
	once := &sync.Once{}
	return func(in T) { once.Do(func() { of(in) }) }
}

// Lock returns an observer that is protected by a mutex. All
// execution's of the observer are isolated.
func (of Handler[T]) Lock() Handler[T] { return of.WithLock(&sync.Mutex{}) }

// WithLock protects the action of the observer with the provied mutex.
func (of Handler[T]) WithLock(mtx *sync.Mutex) Handler[T] {
	return func(in T) { mtx.Lock(); defer mtx.Unlock(); of(in) }
}

// Join creates an observer function that runs both the root observer
// and the "next" observer.
func (of Handler[T]) Join(next Handler[T]) Handler[T] { return func(in T) { of(in); next(in) } }
