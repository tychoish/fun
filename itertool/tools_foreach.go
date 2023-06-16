package itertool

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
)

// Process provides a (potentially) more sensible alternate name for
// ParallelForEach, but otherwise is identical.
func Process[T any](
	ctx context.Context,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	optp ...OptionProvider[*Options],
) error {
	return ParallelForEach(ctx, iter, fn, optp...)
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
	optp ...OptionProvider[*Options],
) error {
	return Process(ctx, iter, func(ctx context.Context, op OP) error {
		return any(op).(interface{ Safe() fun.Worker }).Safe()(ctx)
	}, optp...)
}

// ParallelForEach processes the iterator in parallel, and is
// essentially an iterator-driven worker pool. The input iterator is
// split dynamically into iterators for every worker (determined by
// Options.NumWorkers,) with the division between workers determined
// by their processing speed (e.g. workers should not suffer from
// head-of-line blocking,) and input iterators are consumed (safely)
// as work is processed.
func ParallelForEach[T any](
	ctx context.Context,
	iter *fun.Iterator[T],
	fn fun.Processor[T],
	optp ...OptionProvider[*Options],
) (err error) {
	opts := &Options{}
	opts.init()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = Apply(opts, optp...)
	fun.WhenCall(err != nil, cancel)

	ec := &erc.Collector{}
	wg := &fun.WaitGroup{}

	operation := fn.Safe().FilterErrors(makeErrorFilter(ec.Add, opts))

	splits := iter.Split(opts.NumWorkers)
	for idx := range splits {
		operation.ReadAll(splits[idx].Producer()).
			Wait(func(err error) { fun.WhenCall(errors.Is(err, io.EOF), cancel) }).
			Add(ctx, wg)
	}

	wg.Operation().Block()
	ec.Add(iter.Close())
	return ec.Resolve()
}

func makeErrorFilter(
	oberr fun.Observer[error],
	opts *Options,
) func(error) error {
	return func(err error) error {
		if opts.HandleAbortableErrors(oberr, err) {
			return nil
		}
		return io.EOF
	}
}
