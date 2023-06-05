package fun

import (
	"context"
	"sync"
)

// Observer describes a function that operates on a single object, but
// returns no output, and is used primarly for side effects,
// particularly around handling errors or collecting metrics. The
// Observer implementation here makes it possible to provide
// panic-safety for these kinds of functions or easily convert to
// other related types.
type Observer[T any] func(T)

// Safe handles any panic encountered during the observer's execution
// and converts it to an error.
func (of Observer[T]) Safe(oe Observer[error]) Observer[T] { return func(in T) { oe(of.Check(in)) } }

// Check runs the observer function with a panic handler and converts
// a possible panic to an error.
func (of Observer[T]) Check(in T) error { return Check(func() { of(in) }) }

// Worker captures a variable and returns a worker function which
// will, when executed, observe the input value. These worker
// functions, use the Safe-mode of execution.
func (of Observer[T]) Worker(in T) Worker { return func(context.Context) error { return of.Check(in) } }

// Wait captures a variable and converts an Observer into a wait
// function that observes the value when the WaitFunc runs.
func (of Observer[T]) Wait(in T) WaitFunc { return func(context.Context) { of(in) } }

// Caputre returns a function that observes the specified variable,
// but only when executed later.
func (of Observer[T]) Capture(in T) func() { return func() { of(in) } }

// Processor converts the observer to an observer function. The
// Processor will always return nil, and the context is ignored.
func (of Observer[T]) Processor() Processor[T] {
	return func(_ context.Context, in T) error { return of.Check(in) }
}

// Iterator produces a worker that processes every item in the
// iterator with the observer function.
func (of Observer[T]) Iterator(iter *Iterator[T]) Worker {
	return func(ctx context.Context) error { return iter.Observe(ctx, of) }
}

// If returns an observer that only executes the root observer if the
// condition is true.
func (of Observer[T]) If(cond bool) Observer[T] { return of.When(Wrapper(cond)) }

// When returns an observer function that only executes the observer
// function if the condition function returns true. The condition
// function is run every time the observer function runs.
func (of Observer[T]) When(cond func() bool) Observer[T] {
	return func(in T) { WhenCall(cond(), of.Capture(in)) }
}

// Filter creates an observer that only executes the root observer
// when the condition function--which can inspect the input
// object--returns true. Use this to filter out nil inputs, or
// unactionable inputs.
func (of Observer[T]) Filter(cond func(T) bool) Observer[T] {
	return func(in T) { WhenCall(cond(in), of.Capture(in)) }
}

// Once produces an observer function that runs exactly once, and
// successive executions of the observer are noops.
func (of Observer[T]) Once() Observer[T] {
	once := &sync.Once{}
	return func(in T) { once.Do(func() { of(in) }) }
}

// Lock returns an observer that is protected by a mutex. All
// execution's of the observer are isolated.
func (of Observer[T]) Lock() Observer[T] {
	mtx := &sync.Mutex{}
	return func(in T) { mtx.Lock(); defer mtx.Unlock(); of(in) }
}

// Chain creates an observer function that runs both the root observer
// and the "next" observer.
func (of Observer[T]) Chain(next Observer[T]) Observer[T] { return func(in T) { of(in); next(in) } }
