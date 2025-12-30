package fun

import (
	"context"
	"fmt"
	"strings"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
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

////////////////////////////////////////////////////////////////////////
//
// Error Handling Tools

// ErrorStream provides a stream that provides access to the error collector.
func (Constructors) ErrorStream(ec *erc.Collector) *Stream[error] {
	return IteratorStream(ec.Iterator())
}

// ErrorHandler constructs an error observer that only calls the wrapped observer when the error
// passed is non-nil.
func (Constructors) ErrorHandler(of fn.Handler[error]) fn.Handler[error] {
	return func(err error) { ft.ApplyWhen(err != nil, of, err) }
}

// Recover catches a panic, turns it into an error and passes it to the provided observer function.
func (Constructors) Recover(ob fn.Handler[error]) { ob(erc.ParsePanic(recover())) }

// ErrorHandlerWithoutCancelation wraps and returns an error handler that filters all nil errors and
// errors that are rooted in context Cancellation from the wrapped Handler.
func (Constructors) ErrorHandlerWithoutCancelation(of fn.Handler[error]) fn.Handler[error] {
	return of.Skip(func(err error) bool { return err != nil && !ers.IsExpiredContext(err) })
}

// ErrorHandlerWithoutTerminating wraps an error observer and only calls the underlying observer if
// the input error is non-nil and is not one of the "terminating" errors used by this package
// (e.g. io.EOF and similar errors). Context cancellation errors can and should be filtered
// separately.
func (Constructors) ErrorHandlerWithoutTerminating(of fn.Handler[error]) fn.Handler[error] {
	return of.Skip(func(err error) bool { return err != nil && !ers.IsTerminating(err) })
}

// ErrorHandlerWithAbort creates a new error handler that--ignoring nil and context expiration
// errors--will call the provided context cancellation function when it receives an error.
//
// Use the Chain and Join methods of handlers to further process the error.
func (Constructors) ErrorHandlerWithAbort(cancel context.CancelFunc) fn.Handler[error] {
	return func(err error) {
		if err == nil || ers.IsExpiredContext(err) {
			return
		}

		cancel()
	}
}

// ErrorUnwindTransformer provides the ers.Unwind operation as a transform method, which consumes an
// error and produces a slice of its component errors. All errors are processed by the provided
// filter, and the transformer's context is not used. The error value of the Transform function is
// always nil.
func (Constructors) ErrorUnwindTransformer(filter erc.Filter) fn.Converter[error, []error] {
	return func(err error) []error {
		unwound := ers.Unwind(err)
		out := make([]error, 0, len(unwound))
		for idx := range unwound {
			if e := filter(unwound[idx]); e != nil {
				out = append(out, e)
			}
		}
		return out
	}
}

// ConvertErrorsToStrings makes a Converter function that translates slices of errors to slices of
// errors.
func (Constructors) ConvertErrorsToStrings() fn.Converter[[]error, []string] {
	return func(errs []error) []string {
		out := make([]string, 0, len(errs))
		for idx := range errs {
			if !ers.IsOk(errs[idx]) {
				out = append(out, errs[idx].Error())
			}
		}

		return out
	}
}

////////////////////////////////////////////////////////////////////////
//
// Strings

// Sprintln constructs a future that calls fmt.Sprintln over the given variadic arguments.
func (Constructors) Sprintln(args ...any) fn.Future[string] {
	return func() string { return fmt.Sprintln(args...) }
}

// Sprint constructs a future that calls fmt.Sprint over the given variadic arguments.
func (Constructors) Sprint(args ...any) fn.Future[string] {
	return func() string { return fmt.Sprint(args...) }
}

// Sprintf produces a future that calls and returns fmt.Sprintf for the provided arguments when the
// future is called.
func (Constructors) Sprintf(tmpl string, args ...any) fn.Future[string] {
	return func() string { return fmt.Sprintf(tmpl, args...) }
}

// Str provides a future that calls fmt.Sprint over a slice of any objects. Use fun.MAKE.Sprint for
// a variadic alternative.
func (Constructors) Str(args []any) fn.Future[string] { return MAKE.Sprint(args...) }

// Strf produces a future that calls fmt.Sprintf for the given template string and arguments.
func (Constructors) Strf(tmpl string, args []any) fn.Future[string] {
	return MAKE.Sprintf(tmpl, args...)
}

// Strln constructs a future that calls fmt.Sprintln for the given arguments.
func (Constructors) Strln(args []any) fn.Future[string] { return MAKE.Sprintln(args...) }

// StrJoin like Strln and Sprintln create a concatenated string representation of a sequence of
// values, however StrJoin omits the final new line character that Sprintln adds. This is similar in
// functionality MAKE.Sprint() or MAKE.Str() but ALWAYS adds a space between elements.
func (Constructors) StrJoin(args []any) fn.Future[string] {
	return func() string { out := fmt.Sprintln(args...); return out[:max(0, len(out)-1)] }
}

// StrConcatinate produces a future that joins a variadic sequence of strings into a single string.
func (Constructors) StrConcatinate(strs ...string) fn.Future[string] {
	return MAKE.StrSliceConcatinate(strs)
}

// StrSliceConcatinate produces a future for strings.Join(), concatenating the elements in the input
// slice with the provided separator.
func (Constructors) StrSliceConcatinate(input []string) fn.Future[string] {
	return MAKE.StringsJoin(input, "")
}

// StringsJoin produces a future that combines a slice of strings into a single string, joined with
// the separator.
func (Constructors) StringsJoin(strs []string, sep string) fn.Future[string] {
	return func() string { return strings.Join(strs, sep) }
}

// Stringer converts a fmt.Stringer object/method call into a string-formatter.
func (Constructors) Stringer(op fmt.Stringer) fn.Future[string] { return op.String }
