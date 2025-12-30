package fun

import (
	"context"

	"github.com/tychoish/fun/fnx"
)

// MAKE provides namespaced access to the constructors provided by the Constructors type.
var MAKE = Constructors{}

// The Constructors type serves to namespace constructors of common operations and specializations
// of generic functions provided by this package.
type Constructors struct{}

////////////////////////////////////////////////////////////////////////
//
// Worker Pool Tools

// OperationPool returns a fnx.Operation that, when called, processes the incoming stream of
// fnx.Operations, starts a go routine for running each element in the stream, (without any
// throttling or rate limiting) and then blocks until all operations have returned, or the context
// passed to the output function has been canceled.
//
// For more configuraable options, use the itertool.Worker() function which provides more
// configurability and supports both fnx.Operation and Worker functions.
func (Constructors) OperationPool(st *Stream[fnx.Operation]) fnx.Worker {
	return st.Parallel(fnx.MAKE.OperationHandler(), WorkerGroupConfDefaults())
}

// WorkerPool creates a work that processes a stream of worker functions, for simple and short
// total-duration operations. Every worker in the pool runs in it's own go routine, and there are no
// limits or throttling on the number of go routines. All errors are aggregated and in a single
// collector (erc.Stack) which is returned by the worker when the operation ends (if many Worker's
// error this may create memory pressure) and there's no special handling of panics.
//
// For more configuraable options, use the itertool.Worker() function which provides more
// configurability and supports both fnx.Operation and Worker functions.
func (Constructors) WorkerPool(st *Stream[fnx.Worker]) fnx.Worker {
	return st.Parallel(fnx.MAKE.WorkerHandler(), WorkerGroupConfDefaults())
}

// RunAllWorkers returns a Worker function that will run all of the Worker functions in the stream
// serially.
func (Constructors) RunAllWorkers(st *Stream[fnx.Worker]) fnx.Worker {
	return func(ctx context.Context) error { return st.ReadAll(fnx.MAKE.WorkerHandler()).Run(ctx) }
}

// RunAllOperations returns a worker function that will run all the fnx.Operation functions in the
// stream serially.
func (Constructors) RunAllOperations(st *Stream[fnx.Operation]) fnx.Worker {
	return func(ctx context.Context) error { return st.ReadAll(fnx.MAKE.OperationHandler()).Run(ctx) }
}
