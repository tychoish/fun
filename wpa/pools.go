package wpa

import (
	"context"
	"errors"
	"iter"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
)

// Job describes the union of the fnx.Worker and fnx.Operation types, allowing the wpa functions to
// operate on both types without runtime type casting.
type Job interface {
	fnx.Worker | fnx.Operation | Task | Thunk
	Job() func(context.Context) error
}

// Task represents a simple error-returning function that takes no arguments.  Tasks are more simple
// units of work for use with worker pool and execution functions in the wpa package.
type Task func() error

// Thunk represents a single niladic function that can be used interchangeably in wpa pool and
// worker execution.
type Thunk func()

// Job converts a Thunk into a panic-safe worker function suitable for use in
// worker pool implementations (e.g., wpa.RunWithPool). This exists to satisfy the wpa.Job
// type constraint, enabling interoperability with existing function types.
func (tf Thunk) Job() func(context.Context) error { return fnx.MakeOperation(tf).WithRecover() }

// Job converts a Task into a panic-safe worker function suitable for use in
// worker pool implementations (e.g., wpa.RunWithPool). This exists to satisfy the wpa.Job
// type constraint, enabling interoperability with existing function types.
func (sf Task) Job() func(context.Context) error { return fnx.MakeWorker(sf).WithRecover() }

func jobAsRunnable[T Job](in T) func(context.Context) error { return in.Job() }
func jobAsWorker[T Job](in T) fnx.Worker                    { return in.Job() }

func jobConverterForPool[T Job](filter erc.Filter) func(T) fnx.Worker {
	return func(in T) fnx.Worker { return jobAsWorker(in).WithErrorFilter(filter) }
}

// Run executes jobs sequentially with fail-fast error handling. Panics are converted to errors.
// Returns immediately on first error (except ers.ErrCurrentOpSkip which is ignored). Terminating
// errors (io.EOF, ers.ErrCurrentOpAbort) return nil. Context errors abort execution. Jobs are
// executed serially.
//
// Execution of job processing does not begin until the returned worker function is called.
func Run[T Job](seq iter.Seq[T]) fnx.Worker {
	return func(ctx context.Context) error {
		for job := range irt.Convert(seq, jobAsRunnable) {
			err := job(ctx)
			switch {
			case err == nil:
				continue
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			case ers.IsExpiredContext(err):
				return err
			case ers.IsTerminating(err):
				return nil
			default:
				return err
			}
		}
		return nil
	}
}

// RunAll executes all jobs sequentially, collecting all errors. Panics are converted to errors.
// Unlike Run, RunAll continues executing remaining jobs after errors. The returned error aggregates
// all errors collected during exeuction. Jobs are executed serially.
//
// Execution of job processing does not begin until the returned worker function is called.
func RunAll[T Job](seq iter.Seq[T]) fnx.Worker {
	return func(ctx context.Context) error {
		ec := &erc.Collector{}
		for job := range irt.Convert(seq, jobAsWorker) {
			job.Operation(ec.Push).Run(ctx)
		}
		return ec.Resolve()
	}
}

// RunWithPool executes jobs concurrently using a worker pool. Panics are converted to errors.  The
// number of workers and error handling is configurable, via the WorkerGroupConf configuration
// options, the default number of workers is the value of runtime.NumCPU(). While continue-on-error,
// continue-on-panic, custom error filtering is possible. Execution is concurrent, and there are no
// ordering guarantees. RunWithPool blocks until all tasks have returned. The return value is always
// the aggregated errors for all jobs that produced an error (or panic.).
//
// Execution of job processing does not begin until the returned worker function is called.
func RunWithPool[T Job](seq iter.Seq[T], opts ...opt.Provider[*WorkerGroupConf]) fnx.Worker {
	return func(ctx context.Context) error {
		conf := &WorkerGroupConf{}
		if err := opt.Join(opts...).Apply(conf); err != nil {
			return err
		}

		wg := &fnx.WaitGroup{}

		jobs := irt.Convert(seq, jobConverterForPool[T](conf.Filter))
		for shard := range irt.Shard(ctx, conf.NumWorkers, jobs) {
			wg.Launch(ctx, Run(shard).
				WithRecover().
				Ignore())
		}

		wg.Wait(ctx)
		return conf.ErrorCollector.Resolve()
	}
}
