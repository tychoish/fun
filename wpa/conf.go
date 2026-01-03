package wpa

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/opt"
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

// WorkerGroupConfDefaults returns a configuration provider that sets recommended defaults
// for worker pool operations.
//
// Default Settings:
//   - ContinueOnError: true (workers continue processing jobs after encountering errors)
//   - NumWorkers: runtime.NumCPU() (one worker per CPU core)
//   - ContinueOnPanic: false (workers stop on panic, but panics are converted to errors)
//   - IncludeContextExpirationErrors: false (context cancellation errors are not collected)
//
// This configuration is suitable for most batch processing workloads where you want
// maximum throughput and want to collect all errors for later analysis.
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
// ExcludedErrors value. The provider will return an error if any of
// the input streams is ErrRecoveredPanic.
func WorkerGroupConfAddExcludeErrors(errs ...error) opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error {
		if ers.Is(ers.ErrRecoveredPanic, errs...) {
			return fmt.Errorf("cannot exclude recovered panics: %w", ers.ErrInvalidInput)
		}
		opts.ExcludedErrors = append(opts.ExcludedErrors, errs...)
		return nil
	}
}

// WorkerGroupConfIncludeContextErrors enables collection of context cancellation errors.
//
// When enabled:
//   - Context cancellation errors (context.Canceled, context.DeadlineExceeded) are
//     included in the error collector and returned to the caller
//   - Useful for distinguishing between job failures and explicit cancellation
//
// When disabled (default):
//   - Context cancellation errors are not collected
//   - Workers stop processing when context is cancelled, but no error is returned
//   - This is typically the desired behavior since context cancellation is usually
//     intentional and not an error condition
//
// Note: Workers always respect context cancellation and stop processing; this setting
// only controls whether cancellation is reported as an error.
func WorkerGroupConfIncludeContextErrors() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.IncludeContextExpirationErrors = true; return nil }
}

// WorkerGroupConfContinueOnError enables continue-on-error semantics for worker pool operations.
//
// When enabled:
//   - Workers continue processing subsequent jobs after encountering an error
//   - All errors are collected and aggregated for return to the caller
//   - The pool continues running until all jobs are processed or the context is cancelled
//
// When disabled (default):
//   - A worker stops processing its shard of jobs upon encountering an error
//   - Other workers continue processing their shards
//   - Errors are still collected and returned
//
// This setting does not affect panic recovery (see WorkerGroupConfContinueOnPanic) or
// terminating errors (io.EOF, ers.ErrCurrentOpAbort), which have special handling.
func WorkerGroupConfContinueOnError() opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) error { opts.ContinueOnError = true; return nil }
}

// WorkerGroupConfContinueOnPanic enables continue-on-panic semantics for worker pool operations.
//
// When enabled:
//   - Workers continue processing subsequent jobs after recovering from a panic
//   - Panics are converted to errors (via Job() method) and collected
//   - The worker continues processing its shard of jobs
//
// When disabled (default):
//   - A worker stops processing its shard of jobs after recovering from a panic
//   - The panic is still converted to an error and collected
//   - Other workers continue processing their shards
//
// Note: All panics are always recovered and converted to errors; this setting only
// controls whether the worker continues processing after recovery. Unrecovered panics
// will crash the program.
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
// use in the WorkerGroupOptions. The resulting function will only
// error if the collector is nil, however, this method will override
// an existing error collector.
//
// The ErrorCollector interface is typically provided by the
// `erc.Collector` type.
//
// ErrorCollectors are used by some operations to collect, aggregate, and
// distribute errors from operations to the caller.
func WorkerGroupConfWithErrorCollector(ec *erc.Collector) opt.Provider[*WorkerGroupConf] {
	return func(opts *WorkerGroupConf) (err error) {
		if ec == nil {
			return ers.Wrap(ers.ErrInvalidInput, "cannot use a nil error collector")
		}
		opts.ErrorCollector = ec
		return nil
	}
}
