package wpa

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
)

// WorkerGroupConf describes the runtime options to several operations
// operations. The zero value of this struct provides a usable strict
// operation.
type WorkerGroupConf struct {
	// NumWorkers describes the number of parallel workers
	// processing the incoming items and running the map
	// function. All values less than 1 are converted to 1. Any
	// value greater than 1 will result in out-of-sequence results
	// in the output.
	NumWorkers int
	// ContinueOnPanic prevents the operations from halting when a
	// single processing function panics. In all modes mode panics
	// are converted to errors and propagated to the caller.
	ContinueOnPanic bool
	// ContinueOnError allows a processing function to return an
	// error and allow the work of the broader operation to
	// continue. Errors are aggregated propagated to the caller.
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
	// DisableErrorCollection disables the automatic and default
	// error collection. When true, no default ErrorCollector is
	// used.
	DisableErrorCollection bool
	// ErrorCollector provides a way to connect an existing error
	// collector to a worker group.
	ErrorCollector *erc.Collector
	// CustomValidators are used by some worker group
	// implementations to add specific validation rules.
	CustomValidators []func(*WorkerGroupConf) error
}

// Validate ensures that the configuration is valid, and returns an
// error if there are impossible configurations.
func (o *WorkerGroupConf) Validate() error {
	o.NumWorkers = max(1, o.NumWorkers)
	if err := erc.JoinSeq(irt.Convert(irt.Slice(o.CustomValidators), func(op func(*WorkerGroupConf) error) error { return op(o) })); err != nil {
		return err
	}

	if o.ErrorCollector == nil {
		o.ErrorCollector = &erc.Collector{}
	}

	return nil
}

// CanContinueOnError checks an error, collecting it as needed using
// the WorkerGroupConf, and then returning true if processing should
// continue and false otherwise.
//
// Neither io.EOF nor ers.ErrCurrentOpSkip errors are ever observed.
// All panic errors are observed. Context cancellation errors are
// observed only when configured. as well as context cancellation
// errors when configured.
func (o *WorkerGroupConf) CanContinueOnError(err error) (out bool) {
	if err == nil {
		return true
	}

	switch {
	case errors.Is(err, ers.ErrCurrentOpSkip):
		return true
	case ers.IsTerminating(err):
		return false
	case errors.Is(err, ers.ErrRecoveredPanic):
		o.ErrorCollector.If(!o.DisableErrorCollection, err)
		return o.ContinueOnPanic
	case ers.IsExpiredContext(err):
		o.ErrorCollector.If(!o.DisableErrorCollection && o.IncludeContextExpirationErrors, err)
		return false
	default:
		o.ErrorCollector.If(!o.DisableErrorCollection, err)
		return o.ContinueOnError
	}
}

// Filter is a method which can be used as an erc.Filter
// function, and used in worker pool implementations process as
// configured.
func (o *WorkerGroupConf) Filter(err error) error {
	switch {
	case err == nil:
		return nil
	case o.CanContinueOnError(err):
		return ers.ErrCurrentOpSkip
	default:
		return err
	}
}

// WorkerGroupConfDefaults sets the "continue-on-error" option and the
// "number-of-worers-equals-numcpus" options.
func WorkerGroupConfDefaults() opt.Provider[*WorkerGroupConf] {
	return opt.Join(
		WorkerGroupConfContinueOnError(),
		WorkerGroupConfWorkerPerCPU(),
	)
}

// WorkerGroupConfSet overrides the option with the provided option.
func WorkerGroupConfSet(opt *WorkerGroupConf) opt.Provider[*WorkerGroupConf] {
	return func(o *WorkerGroupConf) error { *o = *opt; return nil }
}

// WorkerGroupConfAddExcludeErrors appends the provided errors to the
// ExcludedErrors value. The provider will never exclude an error that
// is rooted in ers.ErrRecoveredPanic..
func WorkerGroupConfAddExcludeErrors(errs ...error) opt.Provider[*WorkerGroupConf] {
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
func WorkerGroupConfIncludeContextErrors() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.IncludeContextExpirationErrors = true; return nil }
}

// WorkerGroupConfContinueOnError toggles the option that allows the
// operation to continue when the operation encounters an
// error. Otherwise, any option will lead to an abort.
func WorkerGroupConfContinueOnError() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnError = true; return nil }
}

// WorkerGroupConfContinueOnPanic toggles the option that allows the
// operation to continue when encountering a panic.
func WorkerGroupConfContinueOnPanic() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnPanic = true; return nil }
}

// WorkerGroupConfWorkerPerCPU sets the number of workers to the
// number of detected CPUs by the runtime (e.g. runtime.NumCPU()).
func WorkerGroupConfWorkerPerCPU() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = runtime.NumCPU(); return nil }
}

// WorkerGroupConfNumWorkers sets the number of workers
// configured. It is not possible to set this value to less than 1:
// negative values and 0 are always ignored.
func WorkerGroupConfNumWorkers(num int) opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = max(1, num); return nil }
}

// WorkerGroupConfWithErrorCollector sets an error collector implementation for later
// use in the Worker GroupOptions.
func WorkerGroupConfWithErrorCollector(ec *erc.Collector) opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		if ec == nil {
			return ers.Wrap(ers.ErrInvalidInput, "cannot use a nil error collector")
		}
		opts.ErrorCollector = ec
		return nil
	}
}

// WorkerGroupConfDisableErrorCollector disales the default error collector, that collects all
// non-filtered errors. Use this option for all long-running operations that may collect a large
// number of errors. These could be.
func WorkerGroupConfDisableErrorCollector() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.DisableErrorCollection = true; return nil }
}

// WorkerGroupConfCustomValidatorAppend provides a chance to add additional worker group
// configuration checks (or in practice, default settings), when callers need to enforce more strict
// validation constraints.
func WorkerGroupConfCustomValidatorAppend(vf func(*WorkerGroupConf) error) opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		opts.CustomValidators = append(opts.CustomValidators, vf)
		return nil
	}
}

// WorkerGroupConfCustomValidatorReset removes all custom validation functions.
func WorkerGroupConfCustomValidatorReset() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.CustomValidators = nil; return nil }
}
