package fun

import (
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// WorkerGroupConf describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
type WorkerGroupConf struct {
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
	// ErrorObserver is used to collect and aggregate errors in
	// the collector. For operations with shorter runtime
	// `erc.Collector.Add` is a good choice, though different
	// strategies may make sense in different
	// cases. (erc.Collector has a mutex and stories collected
	// errors in memory.)
	ErrorObserver Observer[error]
	// ErrorResolver should return an aggregated error collected
	// during the execution of worker
	// threads. `erc.Collector.Resolve` suffices when collecting
	// with an erc.Collector.
	ErrorResolver func() error
}

func (o *WorkerGroupConf) Validate() error {
	o.NumWorkers = internal.Max(1, o.NumWorkers)
	return nil
}

func (o WorkerGroupConf) CanContinueOnError(err error) bool {
	if err == nil {
		return true
	}

	hadPanic := errors.Is(err, ErrRecoveredPanic)

	switch {
	case hadPanic && !o.ContinueOnPanic:
		o.ErrorObserver(err)
		return false
	case hadPanic && o.ContinueOnPanic:
		o.ErrorObserver(err)
		return true
	case errors.Is(err, ErrIteratorSkip):
		return true
	case errors.Is(err, io.EOF):
		return false
	case ers.ContextExpired(err):
		if o.IncludeContextExpirationErrors {
			o.ErrorObserver(err)
		}

		return false
	default:
		o.ErrorObserver(err)
		return o.ContinueOnError
	}
}

type OptionProvider[T any] func(T) error

func JoinOptionProviders[T any](op ...OptionProvider[T]) OptionProvider[T] {
	switch len(op) {
	case 0:
		return func(T) error { return nil }
	case 1:
		return op[0]
	default:
		return op[0].Join(op[1:]...)
	}
}

func (op OptionProvider[T]) Apply(in T) error {
	err := op(in)
	switch validator := any(in).(type) {
	case interface{ Validate() error }:
		err = ers.Join(validator.Validate(), err)
	}
	return err

}
func (op OptionProvider[T]) Join(opps ...OptionProvider[T]) OptionProvider[T] {
	opps = append([]OptionProvider[T]{op}, opps...)
	return func(option T) (err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		for idx := range opps {
			if opps[idx] == nil {
				continue
			}
			err = ers.Join(opps[idx](option), err)
		}

		return err
	}
}

func WorkerGroupConfSet(opt *WorkerGroupConf) OptionProvider[*WorkerGroupConf] {
	return func(o *WorkerGroupConf) error { *o = *opt; return nil }
}

func WorkerGroupConfAddExcludeErrors(errs ...error) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		if ers.Is(ErrRecoveredPanic, errs...) {

			return fmt.Errorf("cannot exclude recovered panics: %w", ers.ErrInvalidInput)
		}
		opts.ExcludededErrors = append(opts.ExcludededErrors, errs...)
		return nil
	}
}

func WorkerGroupConfIncludeContextErrors() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.IncludeContextExpirationErrors = true; return nil }
}

func WorkerGroupConfContinueOnError() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnError = true; return nil }
}

func WorkerGroupConfContinueOnPanic() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnPanic = true; return nil }
}

func WorkerGroupConfWorkerPerCPU() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = runtime.NumCPU(); return nil }
}

func WorkerGroupConfNumWorkers(num int) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		if num <= 0 {
			num = 1
		}
		opts.NumWorkers = num
		return nil
	}
}

// WorkerGroupConfWithErrorCollector saves an error collector
func WorkerGroupConfErrorObserver(observer Observer[error]) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		opts.ErrorObserver = observer
		return nil
	}
}

func WorkerGroupConfErrorResolver(resolver func() error) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		opts.ErrorResolver = resolver
		return nil
	}
}

// WorkerGroupConfWithErrorCollector sets an error collector implementation for later
// use in the WorkerGroupOptions. The resulting function will only
// error if the collector is nil, however, this method will override
// an existing error collector.
//
// ErrorCollectors are used by some operations to collect, aggregate, and
// distribute errors from operations to the caller.
func WorkerGroupConfWithErrorCollector(
	ec interface {
		Add(error)
		Resolve() error
	},
) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) (err error) {
		if ec == nil {
			return errors.New("cannot use a nil error collector")
		}
		return ers.Join(
			WorkerGroupConfErrorObserver(ec.Add)(opts),
			WorkerGroupConfErrorResolver(ec.Resolve)(opts),
		)
	}
}
