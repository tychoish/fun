package itertool

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/tychoish/fun"
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

func (o Options) shouldCollectError(err error) bool {
	switch {
	case err == nil || errors.Is(err, io.EOF) || len(o.ExcludededErrors) > 1 && ers.Is(err, o.ExcludededErrors...):
		return false
	case errors.Is(err, ers.ErrRecoveredPanic) || (o.IncludeContextExpirationErrors && ers.ContextExpired(err)):
		return true
	default:
		return true
	}
}

type OptionProvider[T any] func(T) error

func Apply[T any](opt T, opts ...OptionProvider[T]) (err error) {
	defer func() { err = ers.Merge(err, ers.ParsePanic(recover())) }()
	return fun.Sliceify(opts).Process(func(_ context.Context, proc OptionProvider[T]) error {
		err = ers.Merge(proc(opt), err)
		return nil
	}).Block()
}

func ExcludeError(errs ...error) OptionProvider[*Options] {
	return func(opts *Options) error { opts.ExcludededErrors = errs; return nil }
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

func NumWorkers(num int) OptionProvider[*Options] {
	return func(opts *Options) error {
		if num <= 0 {
			return fmt.Errorf("cannot set workers to %d: %w", num, fun.ErrInvalidInput)
		}
		opts.NumWorkers = num
		return nil
	}
}
