package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// Process provides a (potentially) more sensible alternate name for
// ParallelForEach, but otherwise is identical.
func Process[T any](
	ctx context.Context,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	opts Options,
) error {
	return ParallelForEach(ctx, iter, fn, opts)
}

// compile-time assertions that both worker types support the "safe"
// interface needed for the Worker() tool.
var _ interface{ Safe() fun.Worker } = new(fun.Worker)
var _ interface{ Safe() fun.Worker } = new(fun.Operation)

// Worker takes iterators of fun.Worker or fun.Operation lambdas
// and processes them in according to the configuration.
//
// All operations functions are processed using their respective
// Safe() methods, which means that the functions themselves will
// never panic, and the ContinueOnPanic option will not impact the
// outcome of the operation (unless the iterator returns a nil
// operation.)
//
// This operation is particularly powerful in combination with the
// iterator for a pubsub.Distributor, interfaces which provide
// synchronized, blocking, and destructive (e.g. so completed
// workloads do not remain in memory) containers.
//
// Worker is implemented using ParallelForEach.
func Worker[OP fun.Worker | fun.Operation](
	ctx context.Context,
	iter *fun.Iterator[OP],
	opts Options,
) error {
	return Process(ctx, iter, func(ctx context.Context, op OP) error {
		return any(op).(interface{ Safe() fun.Worker }).Safe()(ctx)
	}, opts)
}

// ParallelForEach processes the iterator in parallel, and is
// essentially an iterator-driven worker pool. The input iterator is
// split dynamically into iterators for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input iterators are consumed (safely)
// as work is processed.
//
// Because there is no output in these operations
// Options.OutputBufferSize is ignored.
func ParallelForEach[T any](
	ctx context.Context,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	opts Options,
) error {
	opts.init()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ec := &erc.Collector{}
	wg := &fun.WaitGroup{}

	splits := iter.Split(opts.NumWorkers)

	for idx := range splits {
		forEachWorker(ec, opts, splits[idx], fn).
			Wait(func(err error) {
				fun.WhenCall(errors.Is(err, io.EOF), cancel)
			}).Add(ctx, wg)
	}

	wg.Operation().Block()
	ec.Add(iter.Close())
	return ec.Resolve()
}

func forEachWorker[T any](
	ec *erc.Collector,
	opts Options,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
) fun.Worker {
	return func(ctx context.Context) error {
		for {
			value, err := iter.ReadOne(ctx)
			if err != nil {
				return nil
			}

			if err := func(value T) (err error) {
				defer func() { err = erc.Merge(err, ers.ParsePanic(recover())) }()

				return fn(ctx, value)
			}(value); err != nil {
				erc.When(ec, opts.shouldCollectError(err), err)

				hadPanic := errors.Is(err, fun.ErrRecoveredPanic)

				switch {
				case hadPanic && !opts.ContinueOnPanic:
					return io.EOF
				case hadPanic && opts.ContinueOnPanic:
					continue
				case !opts.ContinueOnError || ers.IsTerminating(err):
					return io.EOF
				}
			}
		}
	}
}
