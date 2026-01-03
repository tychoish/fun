package fnx

import (
	"context"
	"sync"

	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
)

// MAKE provides namespaced access to the constructors provided by the Constructors type.
var MAKE = Constructors{}

// The Constructors type serves to namespace constructors of common operations and specializations
// of generic functions provided by this package.
type Constructors struct{}

// OperationHandler constructs a Handler function for running Worker functions. Use with streams to
// build worker pools.
func (Constructors) OperationHandler() Handler[Operation] {
	return func(ctx context.Context, op Operation) error { return op.WithRecover().Run(ctx) }
}

// WorkerHandler constructs a Handler function for running Worker functions. Use with streams to
// build worker pools.
//
// The WorkerHandler provides no panic protection.
func (Constructors) WorkerHandler() Handler[Worker] {
	return func(ctx context.Context, op Worker) error { return op.Run(ctx) }
}

// ConvertOperationToWorker provides a converter function to produce Worker functions from Operation
// functions. The errors produced by the worker functions--if any--are the recovered panics from the
// inner operation.
func (Constructors) ConvertOperationToWorker() Converter[Operation, Worker] {
	return MakeConverter(func(o Operation) Worker { return o.WithRecover() })
}

// ConvertWorkerToOperation converts Worker functions to Operation function, capturing their errors
// with the provided error handler.
func (Constructors) ConvertWorkerToOperation(eh fn.Handler[error]) Converter[Worker, Operation] {
	return MakeConverter(func(wf Worker) Operation { return wf.Operation(eh) })
}

// ContextChannelWorker creates a worker function that wraps a context and will--when called--block
// until the context is done, returning the context's cancellation error. Unless provided with a
// custom context that can be canceled but does not return an error (which would break many common
// assumptions regarding contexts,) this worker will always return an error.
func (Constructors) ContextChannelWorker(ctx context.Context) Worker {
	return MAKE.ErrorChannelWorker(ft.ContextErrorChannel(ctx))
}

// ErrorChannelWorker constructs a worker from an error channel. The resulting worker blocks until
// an error is produced in the error channel, the error channel is closed, or the worker's context
// is canceled. If the channel is closed, the worker will return a nil error, and if the context is
// canceled, the worker will return a context error. In all other cases the work will propagate the
// error (or nil) received from the channel.
//
// You can call the resulting worker function more than once.
func (Constructors) ErrorChannelWorker(ch <-chan error) Worker {
	return Worker(func(ctx context.Context) error {
		if ch == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ch:
			return err
		}
	})
}

// Signal is a wrapper around the common pattern where signal channels are closed to pass
// termination and blocking notifications between go routines. The constructor returns two
// functions: a closer operation--func()--and a Worker that waits for the closer to be triggered.
//
// The closer is safe to call multiple times.
func (Constructors) Signal() (func(), Worker) {
	sig := make(chan struct{})
	closer := sync.OnceFunc(func() { close(sig) })

	return closer, Worker(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
		case <-sig:
		}
		return ctx.Err()
	}).Once()
}
