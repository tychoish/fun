package fun

import (
	"context"
	"sync"
)

// Processor are generic functions that take an argument (and a
// context) and return an error. They're the type of function used by
// the itertool.Process/itertool.ParallelForEach and useful in other
// situations as a compliment to fun.Worker and WaitFunc.
//
// In general the implementations of the methods for processing
// functions are wrappers around their similarly named fun.Worker
// analogues.
type Processor[T any] func(context.Context, T) error

// Run executes the ProcessFunc but creates a context within the
// function (decended from the context provided in the arguments,)
// that is canceled when Run() returns to avoid leaking well behaved
// resources outside of the scope of the function execution. Run can
// also be passed as a Process func.
func (pf Processor[T]) Run(ctx context.Context, in T) error { return pf.Worker(in).Run(ctx) }

// Block runs the ProcessFunc with a context that will never be
// canceled.
func (pf Processor[T]) Block(in T) error                       { return pf.Worker(in).Block() }
func (pf Processor[T]) Wait(in T, of Observer[error]) WaitFunc { return pf.Worker(in).Wait(of) }
func (pf Processor[T]) Safe(ctx context.Context, in T) error   { return pf.Worker(in).Safe(ctx) }

func (pf Processor[T]) Worker(in T) Worker {
	return func(ctx context.Context) error { return pf(ctx, in) }
}

func (pf Processor[T]) Once() Processor[T] {
	once := &sync.Once{}
	var err error
	return func(ctx context.Context, in T) error {
		once.Do(func() { err = pf(ctx, in) })
		return err
	}
}
