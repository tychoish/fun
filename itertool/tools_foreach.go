package itertool

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
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
var _ interface{ Safe() fun.Worker } = new(fun.WaitFunc)

// Worker takes iterators of fun.Worker or fun.WaitFunc lambdas
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
func Worker[OP fun.Worker | fun.WaitFunc](
	ctx context.Context,
	iter *fun.Iterator[OP],
	opts Options,
) error {
	return Process(ctx, iter, func(ctx context.Context, op OP) error {
		return any(op).(interface {
			Safe() fun.Worker
		}).Safe()(ctx)
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
) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &erc.Collector{}
	wg := &fun.WaitGroup{}
	defer func() { err = catcher.Resolve() }()
	defer erc.Check(catcher, iter.Close)
	defer erc.Recover(catcher)

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	splits := iter.Split(opts.NumWorkers)

	for idx := range splits {
		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, splits[idx], fn, cancel)
	}

	wg.Wait(internal.BackgroundContext)
	return
}

func forEachWorker[T any](
	ctx context.Context,
	catcher *erc.Collector,
	wg *fun.WaitGroup,
	opts Options,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	abort func(),
) {
	defer wg.Done()
	defer erc.RecoverHook(catcher, func() {
		if ctx.Err() != nil {
			return
		}
		if !opts.ContinueOnPanic {
			abort()
			return
		}

		wg.Add(1)
		go forEachWorker(ctx, catcher, wg, opts, iter, fn, abort)
	})
	for iter.Next(ctx) {
		if err := fn(ctx, iter.Value()); err != nil {
			erc.When(catcher, opts.shouldCollectError(err), err)

			if opts.ContinueOnError {
				continue
			}

			abort()
			return
		}
	}
}
