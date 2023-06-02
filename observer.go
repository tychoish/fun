package fun

import (
	"context"

	"github.com/tychoish/fun/internal"
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
func (of Observer[T]) Safe(in T) (err error) {
	return internal.MergeErrors(Safe(func() error { of(in); return nil }))
}

// Worker captures a variable and returns a worker function which
// will, when executed, observe the input value. These worker
// functions, use the Safe-mode of execution.
func (of Observer[T]) Worker(in T) Worker { return func(context.Context) error { return of.Safe(in) } }

// Wait captures a variable and converts an Observer into a wait
// function that observes the value when the WaitFunc runs.
func (of Observer[T]) Wait(in T) WaitFunc { return func(context.Context) { of(in) } }

// Processor converts the observer to an observer function. The
// Processor will always return nil, and the context is ignored.
func (of Observer[T]) Processor() Processor[T] {
	return func(_ context.Context, in T) error { of(in); return nil }
}
