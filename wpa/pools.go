package wpa

import (
	"context"
	"errors"
	"iter"
	"sync"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
)

// Run executes jobs sequentially with abort-on-error
// handling. Panics are converted to errors.  Run returns immediately
// on the first error (except ers.ErrCurrentOpSkip which is
// ignored). Terminating errors (io.EOF, ers.ErrCurrentOpAbort) return
// nil. Context errors abort execution. Jobs are executed serially.
//
// Execution of job processing does not begin until the returned
// worker function is called. The error value of the worker function
// is the (filtered) error produced by the erroring job, (or nil, if
// there were no errors.)
func Run[T Job](seq iter.Seq[T]) fnx.Worker {
	return func(ctx context.Context) error {
		ec := &erc.Collector{}
		irt.Apply(Pull(ctx, seq, fnx.Handler[T](TaskHandler[T])), ec.Push)
		return ec.Resolve()
	}
}

// RunAll executes all jobs sequentially, collecting all
// errors. Panics are converted to errors.  Unlike Run, RunAll
// exeuctes all jobs, even after an error.
//
// Execution of job processing does not begin until the returned
// worker function is called. The worker's error is the aggregate
// error for all jobs.
func RunAll[T Job](seq iter.Seq[T]) fnx.Worker {
	return func(ctx context.Context) error {
		ec := &erc.Collector{}

		irt.Apply(PullAll(ctx, seq, fnx.Handler[T](TaskHandler[T])), ec.Push)

		return ec.Resolve()
	}
}

// RunWithPool executes jobs concurrently using a worker pool. Panics
// are converted to errors; continue-on-error, continue-on-panic, and
// custom error filtering are available via the configuration
// options. Worker pool size defaults to the number of CPUs, but is
// also configurable.
//
// Execution of job processing does not begin until the returned
// worker function is called. RunWithPool blocks until all tasks have
// returned, and the return value is always the aggregated errors for
// all jobs that produced an error (or panic).
func RunWithPool[T Job](seq iter.Seq[T], opts ...opt.Provider[*WorkerGroupConf]) fnx.Worker {
	return func(ctx context.Context) error {
		conf := &WorkerGroupConf{}
		if err := opt.Join(opts...).Apply(conf); err != nil {
			return err
		}

		wg := &fnx.WaitGroup{}

		withFilter := withFilter[T](conf.Filter)

		for shard := range irt.Shard(ctx, conf.NumWorkers, seq) {
			wg.Launch(ctx, Run(irt.Convert(shard, withFilter)).
				WithRecover().
				Ignore())
		}

		wg.Wait(ctx)
		return conf.ErrorCollector.Resolve()
	}
}

// Pull processes items sequentially with abort-on-error semantics,
// although ers.ErrCurrentOpSkipped are ignored. Terminating errors
// (io.EOF, ers.ErrCurrentOpAbort) stop processing without yielding an
// error
//
// Execution is lazy and depends on a consumer of the output sequence
// consuming the sequence.
func Pull[T any, HF Handler[T]](ctx context.Context, seq iter.Seq[T], hf HF) iter.Seq[error] {
	return func(yield func(error) bool) {
	WORKLOAD:
		for job := range WithHandler(hf).For(seq) {
			err := job.Run(ctx)

			switch {
			case err == nil:
				continue WORKLOAD
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue WORKLOAD
			case ers.IsTerminating(err):
				return
			case errors.Is(err, ers.ErrRecoveredPanic):
				fallthrough
			case ers.IsExpiredContext(err):
				fallthrough
			default:
				yield(err)
				return
			}
		}
	}
}

// PullAll processes all items sequentially returning all non-nil
// errors to the output sequence.
//
// Execution is lazy and depends on a consumer of the output sequence
// consuming the sequence.
func PullAll[T any, HF Handler[T]](ctx context.Context, seq iter.Seq[T], hf HF) iter.Seq[error] {
	return func(yield func(error) bool) {
		for job := range WithHandler(hf).For(seq) {
			if err := job.Run(ctx); err != nil && !yield(err) {
				return
			}
		}
	}
}

// PullWithPool processes items concurrently using a worker pool and
// yields errors to the returned sequence. Worker pool size and error
// handling behavior and filtering are configurable.
//
// Execution is lazy, and does not begin until the sequence is
// iterated. While there is no _extra_ buffering, each of the pool's
// worker effectively buffers an item from the pool, if the consumer
// backs up.
func PullWithPool[T any, HF Handler[T]](
	ctx context.Context,
	seq iter.Seq[T],
	hf HF,
	opts ...opt.Provider[*WorkerGroupConf],
) iter.Seq[error] {
	opts = append(opts,
		WorkerGroupConfDisableErrorCollector(),
		WorkerGroupConfCustomValidatorAppend(func(conf *WorkerGroupConf) error {
			return ers.When(conf.ErrorCollector != nil, "cannot define a custom error collector for streaming operations")
		}),
	)
	conf := &WorkerGroupConf{}
	if err := opt.Join(opts...).Apply(conf); err != nil {
		return irt.One(err)
	}

	wg := &sync.WaitGroup{}
	ch := make(chan error)

	return irt.WithHooks(
		// input
		irt.Channel(ctx, ch),
		// before:
		func() {
			wg.Add(conf.NumWorkers)
			go func() {
				wg.Wait()
				close(ch)
			}()

			for shard := range irt.Shard(ctx, conf.NumWorkers, seq) {
				go func() {
					defer wg.Done()
					for job := range WithHandler(hf).For(shard) {
						if err := conf.Filter(job.Run(ctx)); err != nil {
							select {
							case <-ctx.Done():
								return
							case ch <- err:
							}
						}
					}
				}()
			}
		},
		// after:
		nil,
		// func() {
		// },
	)
}
