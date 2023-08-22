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

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// HF provides namespaced access to the Handlers/constructors provided
// by the handler's type.
var HF = Handlers{}

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

// RunOperations returns a Operation that, when called, processes the incoming
// iterator of Operations, starts a go routine running each, and wait
// function and then blocks until all operations have returned, or the
// context passed to the output function has been canceled.
func (Handlers) OperationPool(iter *Iterator[Operation]) Operation {
	return func(ctx context.Context) {
		wg := &WaitGroup{}
		defer wg.Wait(ctx)

		wg.Add(1)

		go func() {
			defer wg.Done()
			_ = iter.Observe(ctx, func(fn Operation) { wg.Launch(ctx, fn) })
		}()
	}
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
	return func(ctx context.Context, op Operation) error { return op.WithRecover()(ctx) }
}

// ErrorProcessor produces an error Processor function for errors that
// only calls the input Processor if the error is non-nil.
func (Handlers) ErrorProcessor(pf Processor[error]) Processor[error] {
	return func(ctx context.Context, in error) error {
		return ft.WhenDo(in != nil, func() error { return pf(ctx, in) })
	}
}

// Recovery catches a panic, turns it into an error and passes it to
// the provided observer function.
func (Handlers) Recover(ob Handler[error]) { ob(ers.ParsePanic(recover())) }

// ErrorHandler constructs an error observer that only calls the
// wrapped observer when the error passed is non-nil.
func (Handlers) ErrorHandler(of Handler[error]) Handler[error] {
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
func (Handlers) ErrorCollector() (ob Handler[error], prod Future[[]error]) {
	var errs []error
	ob = func(err error) { errs = append(errs, err) }
	prod = func() []error { return errs }
	mtx := &sync.Mutex{}
	return HF.ErrorHandler(ob).WithLock(mtx), prod.WithLock(mtx)
}

// ErrorHandlerWithoutEOF wraps an error observer and propagates all
// non-error and non-io.EOF errors to the underlying observer.
func (Handlers) ErrorHandlerWithoutEOF(of Handler[error]) Handler[error] {
	return of.Skip(func(err error) bool { return err != nil && !ers.Is(err, io.EOF) })
}

// ErrorHandlerWithoutTerminating wraps an error observer and only
// calls the underlying observer if the input error is non-nil and is
// not one of the "terminating" errors used by this package
// (e.g. io.EOF and the context cancellation errors).
func (Handlers) ErrorHandlerWithoutTerminating(of Handler[error]) Handler[error] {
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

// ErrorHandlerSingle creates an Handler/Future pair for errors that
// that, with a lightweight concurrency control, captures the first
// non-nil error it encounters.
func (Handlers) ErrorHandlerSingle() (Handler[error], Future[error]) {
	var latch = &atomic.Bool{}
	var setter = &sync.Once{}
	var cache error

	return func(err error) {
			if err == nil {
				return
			}

			if latch.CompareAndSwap(false, true) {
				setter.Do(func() { cache = err })
			}
		},
		func() error {
			if !latch.Load() {
				return nil
			}
			// must wait for setter to resolve
			setter.Do(func() {})
			return cache
		}
}

// ErrorHandlerWithAbort creates a new error handler that--ignoring
// nil and context expiration errors--will call the provided context
// cancellation function when it receives an error.
//
// Use the Chain and Join methods of handlers to further process the
// error.
func (Handlers) ErrorHandlerWithAbort(cancel context.CancelFunc) Handler[error] {
	return func(err error) {
		if err == nil || ers.ContextExpired(err) {
			return
		}

		cancel()
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

// StrConcatinate produces a future that joins a variadic sequence of
// strings into a single string.
func (Handlers) StrConcatinate(strs ...string) Future[string] {
	return HF.StrJoin(strs, "")
}

// StrJoin produces a future that combines a slice of strings into a
// single string, joined with the separator.
func (Handlers) StrJoin(strs []string, sep string) Future[string] {
	return func() string { return strings.Join(strs, sep) }
}

// StrJoinWith produces a future for strings.Join(), concatenating the
// elements in the input slice with the provided separator.
func (Handlers) StrSliceConcatinate(input []string) Future[string] {
	return HF.StrJoin(input, "")
}

// Stringer converts a fmt.Stringer object/method call into a
// string-formatter.
func (Handlers) Stringer(op fmt.Stringer) Future[string] { return op.String }

// Lines provides a fun.Iterator access over the contexts of a
// (presumably plaintext) io.Reader, using the bufio.Scanner. During
// iteration the leading and trailing space is also trimmed.
func (Handlers) Lines(reader io.Reader) *Iterator[string] {
	scanner := bufio.NewScanner(reader)
	return Generator(func(ctx context.Context) (string, error) {
		if !scanner.Scan() {
			return "", ers.Join(io.EOF, scanner.Err())
		}
		return strings.TrimSpace(scanner.Text()), nil
	})
}

// Itoa produces a Transform function that converts integers into
// strings.
func (Handlers) Itoa() Transform[int, string] {
	return Converter(func(in int) string { return fmt.Sprint(in) })
}

// Atoi produces a Transform function that converts strings into
// integers.
func (Handlers) Atoi() Transform[string, int] { return ConverterErr(strconv.Atoi) }

// Counter produces an iterator that, starting at 1, yields
// monotonically increasing integers until the maximum is reached.
func (Handlers) Counter(max int) *Iterator[int] {
	state := &atomic.Int64{}
	return MakeProducer(func() (int, error) {
		if prev := int(state.Add(1)); prev <= max {
			return prev, nil
		}

		return -1, io.EOF
	}).Iterator()

}
