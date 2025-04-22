package fun

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
)

// MAKE provides namespaced access to the constructors provided by the
// Constructors type.
var MAKE = Constructors{}

// The Constructors type serves to namespace constructors of common
// operations and specializations of generic functions provided by
// this package.
type Constructors struct{}

// OperationPool returns a Operation that, when called, processes the
// incoming stream of Operations, starts a go routine for running
// each element in the stream, (without any throttling or rate
// limiting) and then blocks until all operations have returned, or
// the context passed to the output function has been canceled.
//
// For more configuraable options, use the itertool.Worker() function
// which provides more configurability and supports both Operation and
// Worker functions.
func (Constructors) OperationPool(iter *Stream[Operation]) Operation {
	return func(ctx context.Context) {
		wg := &WaitGroup{}

		wg.Launch(ctx, iter.ReadAll(func(fn Operation) {
			wg.Launch(ctx, fn)
		}).Ignore())

		wg.Wait(ctx)
	}
}

// WorkerPool creates a work that processes a stream of worker
// functions, for simple and short total-duration operations. Every
// worker in the pool runs in it's own go routine, and there are no
// limits or throttling on the number of go routines. All errors are
// aggregated and in a single collector (erc.Stack) which is returned
// by the worker when the operation ends (if many Worker's error this
// may create memory pressure) and there's no special handling of panics.
//
// For more configuraable options, use the itertool.Worker() function
// which provides more configurability and supports both Operation and
// Worker functions.
func (Constructors) WorkerPool(iter *Stream[Worker]) Worker {
	return func(ctx context.Context) error {
		ec := &erc.Collector{}
		eh := MAKE.ErrorHandlerWithoutTerminating(ec.Push)

		wg := &WaitGroup{}

		wg.Launch(ctx, iter.ReadAll(func(fn Worker) {
			wg.Launch(ctx, fn.Operation(eh))
		}).Operation(eh))

		wg.Wait(ctx)
		ec.Push(ctx.Err())

		return ec.Resolve()
	}
}

func (Constructors) ErrorStream(ec *erc.Collector) *Stream[error] {
	return SeqStream(ec.Generator())
}

// ProcessOperation constructs a Handler function for running Worker
// functions. Use in combination with Process and ProcessParallel, and
// to build worker pools.
//
// The Handlers type serves to namespace these constructors, for
// interface clarity purposes. Use the HF variable to access this
// method as in:
//
//	fun.MAKE.ProcessOperation()
func (Constructors) ProcessOperation() Handler[Operation] {
	return func(ctx context.Context, op Operation) error { return op.WithRecover().Run(ctx) }
}

// Signal is a wrapper around the common pattern where signal channels
// are closed to pass termination and blocking notifications between
// go routines. The constructor returns two functions: a closer
// operation--func()--and a Worker that waits for the closer to be
// triggered.
//
// The closer is safe to call multiple times. The worker ALWAYS
// returns the context cancellation error if its been canceled even if
// the signal channel was closed.
func (Constructors) Signal() (func(), Worker) {
	sig := make(chan struct{})
	closer := sync.OnceFunc(func() { close(sig) })

	return closer, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
		case <-sig:
		}
		return ctx.Err()
	}

}

// ErrorHandler constructs an error observer that only calls the
// wrapped observer when the error passed is non-nil.
func (Constructors) ErrorHandler(of fn.Handler[error]) fn.Handler[error] {
	return func(err error) { ft.WhenApply(err != nil, of, err) }
}

// Recover catches a panic, turns it into an error and passes it to
// the provided observer function.
func (Constructors) Recover(ob fn.Handler[error]) { ob(ers.ParsePanic(recover())) }

// ErrorHandlerWithoutCancelation wraps and returns an error handler
// that filters all nil errors and errors that are rooted in context
// Cancellation from the wrapped Handler.
func (Constructors) ErrorHandlerWithoutCancelation(of fn.Handler[error]) fn.Handler[error] {
	return of.Skip(func(err error) bool { return err != nil && !ers.IsExpiredContext(err) })
}

// ErrorHandlerWithoutTerminating wraps an error observer and only
// calls the underlying observer if the input error is non-nil and is
// not one of the "terminating" errors used by this package
// (e.g. io.EOF and similar errors). Context cancellation errors can
// and should be filtered separately.
func (Constructors) ErrorHandlerWithoutTerminating(of fn.Handler[error]) fn.Handler[error] {
	return of.Skip(func(err error) bool {
		return err != nil && !ers.IsTerminating(err)
	})
}

// ErrorUnwindTransformer provides the ers.Unwind operation as a
// transform method, which consumes an error and produces a slice of
// its component errors. All errors are processed by the provided
// filter, and the transformer's context is not used. The error value
// of the Transform function is always nil.
func (Constructors) ErrorUnwindTransformer(filter erc.Filter) Converter[error, []error] {
	return func(_ context.Context, err error) ([]error, error) {
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

// ErrorHandlerWithAbort creates a new error handler that--ignoring
// nil and context expiration errors--will call the provided context
// cancellation function when it receives an error.
//
// Use the Chain and Join methods of handlers to further process the
// error.
func (Constructors) ErrorHandlerWithAbort(cancel context.CancelFunc) fn.Handler[error] {
	return func(err error) {
		if err == nil || ers.IsExpiredContext(err) {
			return
		}

		cancel()
	}
}

func (Constructors) ConvertErrorsToStrings() Converter[[]error, []string] {
	return MakeConverter(func(errs []error) []string {
		out := make([]string, 0, len(errs))
		for idx := range errs {
			if !ers.IsOk(errs[idx]) {
				out = append(out, errs[idx].Error())
			}
		}

		return out
	})

}

// Sprintln constructs a future that calls fmt.Sprintln over the given
// variadic arguments.
func (Constructors) Sprintln(args ...any) fn.Future[string] {
	return func() string { return fmt.Sprintln(args...) }
}

// Sprint constructs a future that calls fmt.Sprint over the given
// variadic arguments.
func (Constructors) Sprint(args ...any) fn.Future[string] {
	return func() string { return fmt.Sprint(args...) }
}

// Sprintf produces a future that calls and returns fmt.Sprintf for
// the provided arguments when the future is called.
func (Constructors) Sprintf(tmpl string, args ...any) fn.Future[string] {
	return func() string { return fmt.Sprintf(tmpl, args...) }
}

// Str provides a future that calls fmt.Sprint over a slice of
// any objects. Use fun.MAKE.Sprint for a variadic alternative.
func (Constructors) Str(args []any) fn.Future[string] { return MAKE.Sprint(args...) }

// Strf produces a future that calls fmt.Sprintf for the given
// template string and arguments.
func (Constructors) Strf(tmpl string, args []any) fn.Future[string] {
	return MAKE.Sprintf(tmpl, args...)
}

// Strln constructs a future that calls fmt.Sprintln for the given
// arguments.
func (Constructors) Strln(args []any) fn.Future[string] { return MAKE.Sprintln(args...) }

// StrConcatinate produces a future that joins a variadic sequence of
// strings into a single string.
func (Constructors) StrConcatinate(strs ...string) fn.Future[string] {
	return MAKE.StrJoin(strs, "")
}

// StrJoin produces a future that combines a slice of strings into a
// single string, joined with the separator.
func (Constructors) StrJoin(strs []string, sep string) fn.Future[string] {
	return func() string { return strings.Join(strs, sep) }
}

// StrSliceConcatinate produces a future for strings.Join(), concatenating the
// elements in the input slice with the provided separator.
func (Constructors) StrSliceConcatinate(input []string) fn.Future[string] {
	return MAKE.StrJoin(input, "")
}

// Stringer converts a fmt.Stringer object/method call into a
// string-formatter.
func (Constructors) Stringer(op fmt.Stringer) fn.Future[string] { return op.String }

// Lines provides a fun.Stream access over the contexts of a
// (presumably plaintext) io.Reader, using the bufio.Scanner.
func (Constructors) Lines(reader io.Reader) *Stream[string] {
	scanner := bufio.NewScanner(reader)
	return MakeGenerator(func() (string, error) {
		if !scanner.Scan() {
			return "", erc.Join(io.EOF, scanner.Err())
		}
		return scanner.Text(), nil
	}).Stream()
}

// LinesWithSpaceTrimed provides a stream with access to the
// line-separated content of an io.Reader, line Lines(), but with the
// leading and trailing space trimmed from each line.
func (Constructors) LinesWithSpaceTrimed(reader io.Reader) *Stream[string] {
	return MAKE.Lines(reader).Transform(MakeConverter(strings.TrimSpace))
}

// Itoa produces a Transform function that converts integers into
// strings.
func (Constructors) Itoa() Converter[int, string] {
	return MakeConverter(func(in int) string { return fmt.Sprint(in) })
}

// Atoi produces a Transform function that converts strings into
// integers.
func (Constructors) Atoi() Converter[string, int] { return MakeConverterErr(strconv.Atoi) }

// Counter produces a stream that, starting at 1, yields
// monotonically increasing integers until the maximum is reached.
func (Constructors) Counter(maxVal int) *Stream[int] {
	state := &atomic.Int64{}
	return MakeGenerator(func() (int, error) {
		if prev := int(state.Add(1)); prev <= maxVal {
			return prev, nil
		}

		return -1, io.EOF
	}).Stream()
}
