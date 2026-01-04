package wpa

import (
	"context"
	"iter"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
)

// Job describes the union of the fnx.Worker and fnx.Operation types,
// allowing the wpa functions to operate on both types without runtime
// type casting.
type Job interface {
	fnx.Worker | fnx.Operation | Task | Thunk
	Job() func(context.Context) error
}

// Task represents a simple error-returning function that takes no
// arguments.  Tasks are more simple units of work for use with worker
// pool and execution functions in the wpa package.
type Task func() error

// Job evaluates converts a Task into a panic-safe worker function
// suitable for use in worker pool implementations (e.g.,
// wpa.RunWithPool). This exists to satisfy the wpa.Job type
// constraint, enabling interoperability with existing function types.
func (sf Task) Job() func(ctx context.Context) error { return fnx.MakeWorker(sf).WithRecover() }

// Thunk represents a single niladic function that can be used
// interchangeably in wpa pool and worker execution.
type Thunk func()

// Job converts a Thunk into a panic-safe worker function suitable for
// use in worker pool implementations (e.g., wpa.RunWithPool). This
// exists to satisfy the wpa.Job type constraint, enabling
// interoperability with existing function types.
func (tf Thunk) Job() func(ctx context.Context) error { return fnx.MakeOperation(tf).WithRecover() }

// Jobs is a simple wrapper to convert a sequence of Job objects to
// fnx.Workers.
func Jobs[T Job](seq iter.Seq[T]) iter.Seq[fnx.Worker] { return irt.Convert(seq, toWorker) }

func toWorker[J Job](in J) fnx.Worker { return in.Job() }
func withFilter[T Job](f erc.Filter) func(T) fnx.Worker {
	return func(j T) fnx.Worker { return toWorker(j).WithErrorFilter(f) }
}
