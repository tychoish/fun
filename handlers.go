package fun

import (
	"context"
	"io"

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

func (Handlers) ErrorProcessor(pf Processor[error]) Processor[error] {
	return func(ctx context.Context, in error) error {
		return ft.WhenDo(in != nil, func() error { return pf(ctx, in) })
	}
}

func (Handlers) ErrorObserver(of Observer[error]) Observer[error] {
	return of.Filter(func(err error) bool { return err != nil })
}

func (Handlers) ErrorObserverWithoutEOF(of Observer[error]) Observer[error] {
	return of.Filter(func(err error) bool { return err != nil && !ers.Is(err, io.EOF) })
}

func (Handlers) ErrorObserverWithoutTerminating(of Observer[error]) Observer[error] {
	return of.Filter(func(err error) bool { return err != nil && !ers.IsTerminating(err) })
}

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
