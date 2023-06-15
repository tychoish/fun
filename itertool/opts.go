package itertool

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// Options describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
type Options struct {
	// NumWorkers describes the number of parallel workers
	// processing the incoming iterator items and running the map
	// function. All values less than 1 are converted to 1. Any
	// value greater than 1 will result in out-of-sequence results
	// in the output iterator.
	NumWorkers int
	// ContinueOnPanic prevents the operations from halting when a
	// single processing function panics. In all modes mode panics
	// are converted to errors and propagated to the output
	// iterator's Close() method,.
	ContinueOnPanic bool
	// ContinueOnError allows a processing function to return an
	// error and allow the work of the broader operation to
	// continue. Errors are aggregated propagated to the output
	// iterator's Close() method.
	ContinueOnError bool
	// IncludeContextExpirationErrors changes the default handling
	// of context cancellation errors. By default all errors
	// rooted in context cancellation are not propagated to the
	// Close() method, however, when true, these errors are
	// captured. All other error handling semantics
	// (e.g. ContinueOnError) are applicable.
	IncludeContextExpirationErrors bool
	// ExcludededErrors is a list of that should not be included
	// in the collected errors of the
	// output. fun.ErrRecoveredPanic is always included and io.EOF
	// is never included.
	ExcludededErrors []error
}

func (o *Options) init() {
	o.NumWorkers = internal.Max(1, o.NumWorkers)
}

func (opts Options) HandleAbortableErrors(of fun.Observer[error], err error) bool {
	hadPanic := errors.Is(err, fun.ErrRecoveredPanic)

	switch {
	case hadPanic && !opts.ContinueOnPanic:
		of(err)
		return false
	case hadPanic && opts.ContinueOnPanic:
		of(err)
		return true
	case errors.Is(err, fun.ErrIteratorSkip):
		return true
	case errors.Is(err, io.EOF):
		return false
	case ers.ContextExpired(err):
		fun.WhenCall(opts.IncludeContextExpirationErrors, func() { of(err) })
		return false
	default:
		of(err)
		return opts.ContinueOnError
	}
}

type OptionProvider[T any] func(T) error

func Apply[T any](opt T, opts ...OptionProvider[T]) (err error) {
	defer func() { err = ers.Merge(err, ers.ParsePanic(recover())) }()
	dt.Sliceify(opts).Observe(func(proc OptionProvider[T]) {
		err = ers.Merge(proc(opt), err)
	})
	return err
}

func Set(opt *Options) OptionProvider[*Options] {
	return func(o *Options) error { *o = *opt; return nil }
}

func AddExcludeErrors(errs ...error) OptionProvider[*Options] {
	return func(opts *Options) error {
		if ers.Is(fun.ErrRecoveredPanic, errs...) {
			return fmt.Errorf("cannot exclude recovered panics: %w", fun.ErrInvalidInput)
		}
		opts.ExcludededErrors = append(opts.ExcludededErrors, errs...)
		return nil
	}
}
func IncludeContextErrors() OptionProvider[*Options] {
	return func(opts *Options) error { opts.IncludeContextExpirationErrors = true; return nil }
}
func ContinueOnError() OptionProvider[*Options] {
	return func(opts *Options) error { opts.ContinueOnError = true; return nil }
}
func ContinueOnPanic() OptionProvider[*Options] {
	return func(opts *Options) error { opts.ContinueOnPanic = true; return nil }
}
func WorkerPerCPU() OptionProvider[*Options] {
	return func(opts *Options) error { opts.NumWorkers = runtime.NumCPU(); return nil }
}

func NumWorkers(num int) OptionProvider[*Options] {
	return func(opts *Options) error {
		if num <= 0 {
			num = 1
		}
		opts.NumWorkers = num
		return nil
	}
}
