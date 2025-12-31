package wpa

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
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
	// ErrorCollector provides a way to connect an existing error
	// collector to a worker group.
	ErrorCollector *erc.Collector
}

// Validate ensures that the configuration is valid, and returns an
// error if there are impossible configurations.
func (o *WorkerGroupConf) Validate() error {
	o.NumWorkers = max(1, o.NumWorkers)

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
		o.ErrorCollector.Push(err)
		return o.ContinueOnPanic
	case ers.IsExpiredContext(err):
		o.ErrorCollector.If(o.IncludeContextExpirationErrors, err)
		return false
	default:
		o.ErrorCollector.Push(err)
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
func WorkerGroupConfDefaults() fnx.OptionProvider[*WorkerGroupConf] {
	return fnx.JoinOptionProviders(
		WorkerGroupConfContinueOnError(),
		WorkerGroupConfWorkerPerCPU(),
	)
}

// WorkerGroupConfSet overrides the option with the provided option.
func WorkerGroupConfSet(opt *WorkerGroupConf) fnx.OptionProvider[*WorkerGroupConf] {
	return func(o *WorkerGroupConf) error { *o = *opt; return nil }
}

// WorkerGroupConfAddExcludeErrors appends the provided errors to the
// ExcludedErrors value. The provider will return an error if any of
// the input streams is ErrRecoveredPanic.
func WorkerGroupConfAddExcludeErrors(errs ...error) fnx.OptionProvider[*WorkerGroupConf] {
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
func WorkerGroupConfIncludeContextErrors() fnx.OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.IncludeContextExpirationErrors = true; return nil }
}

// WorkerGroupConfContinueOnError toggles the option that allows the
// operation to continue when the operation encounters an
// error. Otherwise, any option will lead to an abort.
func WorkerGroupConfContinueOnError() fnx.OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnError = true; return nil }
}

// WorkerGroupConfContinueOnPanic toggles the option that allows the
// operation to continue when encountering a panic.
func WorkerGroupConfContinueOnPanic() fnx.OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnPanic = true; return nil }
}

// WorkerGroupConfWorkerPerCPU sets the number of workers to the
// number of detected CPUs by the runtime (e.g. runtime.NumCPU()).
func WorkerGroupConfWorkerPerCPU() fnx.OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = runtime.NumCPU(); return nil }
}

// WorkerGroupConfNumWorkers sets the number of workers
// configured. It is not possible to set this value to less than 1:
// negative values and 0 are always ignored.
func WorkerGroupConfNumWorkers(num int) fnx.OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.NumWorkers = max(1, num); return nil }
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
func WorkerGroupConfWithErrorCollector(ec *erc.Collector) fnx.OptionProvider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) (err error) {
		if ec == nil {
			return ers.Wrap(ers.ErrInvalidInput, "cannot use a nil error collector")
		}
		opts.ErrorCollector = ec
		return nil
	}
}
