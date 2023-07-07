package fun

import (
	"context"
	"fmt"
	"io"
	"strings"
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

// ErrorUnwindTransformer provides the ers.Unwind operation as a
// transform method, which consumes an error and produces a slice of
// its component errors. All errors are processed by the provided
// filter, and the transformer's context is not used. The error value
// of the Transform function is always nil.
func (Handlers) ErrorUnwindTransformer(filter ers.Filter) Transform[error, []error] {
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

// Sprintln constructs a future that calls fmt.Sprintln over the given
// variadic arguments.
func (Handlers) Sprintln(args ...any) Future[string] {
	return func() string { return fmt.Sprintln(args...) }
}

// Sprint constructs a future that calls fmt.Sprint over the given
// variadic arguments.
func (Handlers) Sprint(args ...any) Future[string] {
	return func() string { return fmt.Sprint(args...) }
}

// Sprintf produces a future that calls and returns fmt.Sprintf for
// the provided arguments when the future is called.
func (Handlers) Sprintf(tmpl string, args ...any) Future[string] {
	return func() string { return fmt.Sprintf(tmpl, args...) }
}

// Str provides a future that calls fmt.Sprint over a slice of
// any objects. Use fun.HF.Sprint for a variadic alternative.
func (Handlers) Str(args []any) Future[string] { return HF.Sprint(args...) }

// Strf produces a future that calls fmt.Sprintf for the given
// template string and arguments.
func (Handlers) Strf(tmpl string, args []any) Future[string] { return HF.Sprintf(tmpl, args...) }

// Strln constructs a future that calls fmt.Sprintln for the given
// arguments.
func (Handlers) Strln(args []any) Future[string] { return HF.Sprintln(args...) }

// StrJoinWith produces a future for strings.Join(), concatenating the
// elements in the input slice with the provided separator.
func (Handlers) StrJoinWith(input []string, sep string) Future[string] {
	return func() string { return strings.Join(input, sep) }
}

// StrConcatinate produces a future that joins a variadic sequence of
// strings into a single string.
func (Handlers) StrConcatinate(strs ...string) Future[string] {
	return func() string { return strings.Join(strs, "") }
}

// StrJoin produces a future that combines a slice of strings into a
// single string, joined without spaces.
func (Handlers) StrJoin(strs []string) Future[string] {
	return func() string { return strings.Join(strs, "") }
}

// Stringer converts a fmt.Stringer object/method call into a
// string-formatter.
func (Handlers) Stringer(op fmt.Stringer) Future[string] { return op.String }
