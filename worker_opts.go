package fun

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
)

// WorkerGroupConf describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
type WorkerGroupConf struct {
	// NumWorkers describes the number of parallel workers
	// processing the incoming stream items and running the map
	// function. All values less than 1 are converted to 1. Any
	// value greater than 1 will result in out-of-sequence results
	// in the output stream.
	NumWorkers int
	// ContinueOnPanic prevents the operations from halting when a
	// single processing function panics. In all modes mode panics
	// are converted to errors and propagated to the output
	// stream's Close() method,.
	ContinueOnPanic bool
	// ContinueOnError allows a processing function to return an
	// error and allow the work of the broader operation to
	// continue. Errors are aggregated propagated to the output
	// stream's Close() method.
	ContinueOnError bool
	// IncludeContextExpirationErrors changes the default handling
	// of context cancellation errors. By default all errors
	// rooted in context cancellation are not propagated to the
	// Close() method, however, when true, these errors are
	// captured. All other error handling semantics
	// (e.g. ContinueOnError) are applicable.
	IncludeContextExpirationErrors bool
	// ExcludedErrors is a list of that should not be included
	// in the collected errors of the
	// output. ers.ErrRecoveredPanic is always included and io.EOF
	// is never included.
	ExcludedErrors []error
	// ErrorHandler is used to collect and aggregate errors in
	// the collector. For operations with shorter runtime
	// `erc.Collector.Add` is a good choice, though different
	// strategies may make sense in different
	// cases. (erc.Collector has a mutex and stories collected
	// errors in memory.)
	ErrorHandler fn.Handler[error]
	// ErrorResolver should return an aggregated error collected
	// during the execution of worker
	// threads. `erc.Collector.Resolve` suffices when collecting
	// with an erc.Collector.
	ErrorResolver fn.Future[error]
}

// Validate ensures that the configuration is valid, and returns an
// error if there are impossible configurations
func (o *WorkerGroupConf) Validate() error {
	o.NumWorkers = max(1, o.NumWorkers)
	ehIsNotNil := o.ErrorHandler != nil
	erIsNotNil := o.ErrorResolver != nil

	return errors.Join(
		ers.Whenf(ehIsNotNil != erIsNotNil,
			"must configure error handler and resolver, or neither, eh=%t er=%t",
			ehIsNotNil, erIsNotNil),
	)
}

// CanContinueOnError checks an error, collecting it as needed using
// the WorkerGroupConf, and then returning true if processing should
// continue and false otherwise.
//
// Neither io.EOF nor ErrStreamContinue errors are ever observed.
// All panic errors are observed. Context cancellation errors are
// observed only when configured. as well as context cancellation
// errors when configured.
func (o WorkerGroupConf) CanContinueOnError(err error) bool {
	if err == nil {
		return true
	}

	switch {
	case errors.Is(err, ErrStreamContinue):
		return true
	case errors.Is(err, ers.ErrRecoveredPanic):
		o.ErrorHandler.SafeHandle(err)
		return o.ContinueOnPanic
	case ers.IsExpiredContext(err):
		if o.IncludeContextExpirationErrors {
			o.ErrorHandler.SafeHandle(err)
		}
		return false
	case ers.IsTerminating(err):
		return false
	default:
		o.ErrorHandler.SafeHandle(err)
		return o.ContinueOnError
	}
}

// WorkerGroupConfSet overrides the option with the provided option.
func WorkerGroupConfSet(opt *WorkerGroupConf) OptionProvider[*WorkerGroupConf] {
	return func(o *WorkerGroupConf) error { *o = *opt; return nil }
}

// WorkerGroupConfAddExcludeErrors appends the provided errors to the
// ExcludedErrors value. The provider will return an error if any of
// the input streams is ErrRecoveredPanic.
func WorkerGroupConfAddExcludeErrors(errs ...error) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		if ers.Is(ers.ErrRecoveredPanic, errs...) {
			return fmt.Errorf("cannot exclude recovered panics: %w", ers.ErrInvalidInput)
		}
		opts.ExcludedErrors = append(opts.ExcludedErrors, errs...)
		return nil
	}
}

// WorkerGroupConfIncludeContextErrors toggles the option that forces
// the operation to include context errors in the output. By default
// they are not included.
func WorkerGroupConfIncludeContextErrors() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.IncludeContextExpirationErrors = true; return nil }
}

// WorkerGroupConfContinueOnError toggles the option that allows the
// operation to continue when the operation encounters an
// error. Otherwise, any option will lead to an abort.
func WorkerGroupConfContinueOnError() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnError = true; return nil }
}

// WorkerGroupConfContinueOnPanic toggles the option that allows the
// operation to continue when encountering a panic.
func WorkerGroupConfContinueOnPanic() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnPanic = true; return nil }
}

// WorkerGroupConfWorkerPerCPU sets the number of workers to the
// number of detected CPUs by the runtime (e.g. runtime.NumCPU()).
func WorkerGroupConfWorkerPerCPU() OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = runtime.NumCPU(); return nil }
}

// WorkerGroupConfNumWorkers sets the number of workers
// configured. It is not possible to set this value to less than 1:
// negative values and 0 are always ignored.
func WorkerGroupConfNumWorkers(num int) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = max(1, num); return nil }
}

// WorkerGroupConfWithErrorCollector saves an error observer to the
// configuration. Typically implementations will provide some default
// error collection tool, and will only call the observer for non-nil
// errors. ErrorHandlers should be safe for concurrent use.
func WorkerGroupConfErrorHandler(observer fn.Handler[error]) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ErrorHandler = observer; return nil }
}

// WorkerGroupConfErrorResolver reports the errors collected by the
// observer. If the ErrorHandler is not set the resolver may be
// overridden. ErrorHandlers should be safe for concurrent use.
func WorkerGroupConfErrorResolver(resolver func() error) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ErrorResolver = resolver; return nil }
}

// WorkerGroupConfWithErrorCollector sets an error collector implementation for later
// use in the WorkerGroupOptions. The resulting function will only
// error if the collector is nil, however, this method will override
// an existing error collector.
//
// The ErrorCollector interface is typically provided by the
// `erc.Collector` type.
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
			WorkerGroupConfErrorHandler(ec.Add)(opts),
			WorkerGroupConfErrorResolver(ec.Resolve)(opts),
		)
	}
}

// WorkerGroupConfErrorCollectorPair uses an Handler/Generator pair to
// collect errors. A basic implementation, accessible via
// fun.MAKE.ErrorCollector() is suitable for this purpose.
func WorkerGroupConfErrorCollectorPair(ob fn.Handler[error], resolver fn.Future[error]) OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) (err error) {
		return ers.Join(
			WorkerGroupConfErrorHandler(ob)(opts),
			WorkerGroupConfErrorResolver(resolver)(opts),
		)
	}
}
