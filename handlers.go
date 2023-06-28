package fun

import (
	"context"
	"io"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// HF provides namespaced access to the Handlers/constructors provided
// by the handler's type.
var HF Handlers = Handlers{}

// The Handlers type serves to namespace constructors of common
// operations and specializations of generic functions provided by
// this package.
type Handlers struct{}

// ProcessWorker constructs a Processor function for running Worker
// functions. Use in combination with Process and ProcessParallel, and
// to build worker pools.
//
// The Handlers type serves to namespace these constructors, for
// interface clarity purposes. Use the HF variable to access this
// method as in:
//
//	fun.HF.ProcessWorker()
func (Handlers) ProcessWorker() Processor[Worker] {
	return func(ctx context.Context, wf Worker) error { return wf(ctx) }
}

// ProcessOperation constructs a Processor function for running Worker
// functions. Use in combination with Process and ProcessParallel, and
// to build worker pools.
//
// The Handlers type serves to namespace these constructors, for
// interface clarity purposes. Use the HF variable to access this
// method as in:
//
//	fun.HF.ProcessOperation()
func (Handlers) ProcessOperation() Processor[Operation] {
	return func(ctx context.Context, op Operation) error { return op.Safe()(ctx) }
}

// ErrorProcessor produces an error Processor function for errors that
// only calls the input Processor if the error is non-nil.
func (Handlers) ErrorProcessor(pf Processor[error]) Processor[error] {
	return func(ctx context.Context, in error) error {
		return ft.WhenDo(in != nil, func() error { return pf(ctx, in) })
	}
}

// ErrorObserver constructs an error observer that only calls the
// wrapped observer when the error passed is non-nil.
func (Handlers) ErrorObserver(of Observer[error]) Observer[error] {
	return func(err error) {
		if err != nil {
			of(err)
		}
	}
}

// ErrorCollector provides a basic error aggregation facility that
// collects non-nil errors, and adds them to a slice internally, which
// is accessible via the producer. The operation of the observer and
// producer are protexted by a shared mutex.
func (Handlers) ErrorCollector() (ob Observer[error], prod Producer[[]error]) {
	var errs []error

	ob = func(err error) { errs = append(errs, err) }
	prod = func(_ context.Context) ([]error, error) { return errs, nil }
	mtx := &sync.Mutex{}

	return HF.ErrorObserver(ob).WithLock(mtx), prod.WithLock(mtx)
}

// ErrorObserverWithoutEOF wraps an error observer and propogates all
// non-error and non-io.EOF errors to the underlying observer.
func (Handlers) ErrorObserverWithoutEOF(of Observer[error]) Observer[error] {
	return of.Skip(func(err error) bool { return err != nil && !ers.Is(err, io.EOF) })
}

// ErrorObserverWithoutTerminating wraps an error observer and only
// calls the underlying observer if the input error is non-nil and is
// not one of the "terminating" errors used by this package
// (e.g. io.EOF and the context cancellation errors).
func (Handlers) ErrorObserverWithoutTerminating(of Observer[error]) Observer[error] {
	return of.Skip(func(err error) bool { return err != nil && !ers.IsTerminating(err) })
}

// ErrorUndindTransformer provides the ers.Unwind operation as a
// transform method, which consumes an error and produces a slice of
// its component errors. All errors are processed by the provided
// filter, and the transformer's context is not used. The error value
// of the Transform function is always nil.
func (Handlers) ErrorUndindTransformer(filter ers.Filter) Transform[error, []error] {
	return func(ctx context.Context, err error) ([]error, error) {
		unwound := ers.Unwind(err)
		out := make([]error, 0, len(unwound))
		for idx := range unwound {
			if e := filter(unwound[idx]); e != nil {
				out = append(out, e)
			}
		}
		return out, nil
	}
}
