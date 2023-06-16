package itertool

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// WorkerGroupOptions describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
type WorkerGroupOptions struct {
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

func (o *WorkerGroupOptions) init() {
	o.NumWorkers = internal.Max(1, o.NumWorkers)
}

func (o WorkerGroupOptions) CanContinueOnError(of fun.Observer[error], err error) bool {
	if err == nil {
		return true
	}

	hadPanic := errors.Is(err, fun.ErrRecoveredPanic)

	switch {
	case hadPanic && !o.ContinueOnPanic:
		of(err)
		return false
	case hadPanic && o.ContinueOnPanic:
		of(err)
		return true
	case errors.Is(err, fun.ErrIteratorSkip):
		return true
	case errors.Is(err, io.EOF):
		return false
	case ers.ContextExpired(err):
		if o.IncludeContextExpirationErrors {
			of(err)
		}

		return false
	default:
		of(err)
		return o.ContinueOnError
	}
}

type OptionProvider[T any] func(T) error

func Apply[T any](opt T, opts ...OptionProvider[T]) (err error) {
	defer func() { err = ers.Merge(err, ers.ParsePanic(recover())) }()
	for idx := range opts {
		err = ers.Merge(opts[idx](opt), err)
	}
	return err
}

func Set(opt *WorkerGroupOptions) OptionProvider[*WorkerGroupOptions] {
	return func(o *WorkerGroupOptions) error { *o = *opt; return nil }
}

func AddExcludeErrors(errs ...error) OptionProvider[*WorkerGroupOptions] {
	return func(opts *WorkerGroupOptions) error {
		if ers.Is(fun.ErrRecoveredPanic, errs...) {
			return fmt.Errorf("cannot exclude recovered panics: %w", fun.ErrInvalidInput)
		}
		opts.ExcludededErrors = append(opts.ExcludededErrors, errs...)
		return nil
	}
}
func IncludeContextErrors() OptionProvider[*WorkerGroupOptions] {
	return func(opts *WorkerGroupOptions) error { opts.IncludeContextExpirationErrors = true; return nil }
}
func ContinueOnError() OptionProvider[*WorkerGroupOptions] {
	return func(opts *WorkerGroupOptions) error { opts.ContinueOnError = true; return nil }
}
func ContinueOnPanic() OptionProvider[*WorkerGroupOptions] {
	return func(opts *WorkerGroupOptions) error { opts.ContinueOnPanic = true; return nil }
}
func WorkerPerCPU() OptionProvider[*WorkerGroupOptions] {
	return func(opts *WorkerGroupOptions) error { opts.NumWorkers = runtime.NumCPU(); return nil }
}

func NumWorkers(num int) OptionProvider[*WorkerGroupOptions] {
	return func(opts *WorkerGroupOptions) error {
		if num <= 0 {
			num = 1
		}
		opts.NumWorkers = num
		return nil
	}
}
