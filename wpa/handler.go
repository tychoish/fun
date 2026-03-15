package wpa

import (
	"context"
	"iter"

	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
)

// Handler describes worker pool interfaces for both Run-based (direct)
// and "For"-based (pull), worker pools.
type Handler[T any] interface {
	HandlerFor[T]
	HandlerRun[T]
}

// HandlerRun describes the interface used by "Run" based worker
// pools. These interfaces support a chaining-type configuration, all
// methods return a WorkerFor[T] instance that accepts the work
// iterator.
type HandlerRun[T any] interface {
	Run() WorkerFor[T]
	RunAll() WorkerFor[T]
	RunWithPool(opts ...opt.Provider[*WorkerGroupConf]) WorkerFor[T]
}

// HandlerFor describes the interface iterator-based worker pools.
type HandlerFor[T any] interface {
	For(seq iter.Seq[T]) iter.Seq[fnx.Worker]
	ForEach(seq iter.Seq[T]) WorkerPull[T]
}

// WorkerPull describes the interface that for pull-based worker pools
// where the errors must be iterated for work to progress.
type WorkerPull[T any] interface {
	Pull(context.Context) iter.Seq[error]
	PullAll(context.Context) iter.Seq[error]
	PullWithPool(context.Context, ...opt.Provider[*WorkerGroupConf]) iter.Seq[error]
}

// WithHandler provides an ergonomic bridge between sequences with
// arbitrary types and handler functions, and the worker pools that the
// execution in the Run and Pull operations.
func WithHandler[T any, H HandlerFunc[T]](op H) Handler[T] { return newHandler(op) }

// HandlerFunc is a type constraint that describes the union of
// fnx.Handler, fn.Handler and the Reader function type for use with
// Handler-based worker pools, particularly for the
// WithHandler().For() method and with the Run/RunAll/RunWithPool
// methods.
type HandlerFunc[T any] interface {
	fnx.Handler[T] | fn.Handler[T] | Reader[T]
	Job(T) func(context.Context) error
}

// Reader describes a common function signature for use in the
// WithHandler().For() operation.
type Reader[T any] func(T) error

// Job converts a Reader method into an fnx.Worker to satisfy the
// Handler[T] constraint for use with the WithHandler().For() operation.
func (hf Reader[T]) Job(in T) func(context.Context) error { return fnx.MakeHandler(hf).Job(in) }

// WorkerFor describes the interface for worker pool operations that
// take a sequence of job functions 'T' and return a single worker, that
// when run, executes the work. Implementations should aggregate errors
// from the worker pool into the workers' error.
type WorkerFor[T any] interface{ For(iter.Seq[T]) fnx.Worker }

// TaskHandler runs a task. Because this function which has the
// signature of an fnx.Handler function, can be used as a Handler for
// WithHandler operations, as needed.
func TaskHandler[J Job](ctx context.Context, job J) error { return toWorker(job).Run(ctx) }

type workerExec func(iter.Seq[fnx.Worker]) fnx.Worker

type handlerImpl[T any, H HandlerFunc[T]] struct{ hf H }

func newHandler[T any, H HandlerFunc[T]](in H) handlerImpl[T, H]     { return handlerImpl[T, H]{hf: in} }
func (h handlerImpl[T, H]) run() *runImpl[T, H]                      { return &runImpl[T, H]{hn: h} }
func (h handlerImpl[T, H]) into(in T) fnx.Worker                     { return h.hf.Job(in) }
func (h handlerImpl[T, H]) For(seq iter.Seq[T]) iter.Seq[fnx.Worker] { return irt.Convert(seq, h.into) }
func (h handlerImpl[T, H]) ForEach(seq iter.Seq[T]) WorkerPull[T]    { return h.run().each(seq) }
func (h handlerImpl[T, H]) Run() WorkerFor[T]                        { return h.run().with(Run) }
func (h handlerImpl[T, H]) RunAll() WorkerFor[T]                     { return h.run().with(RunAll[fnx.Worker]) }

func (h handlerImpl[T, H]) RunWithPool(opts ...opt.Provider[*WorkerGroupConf]) WorkerFor[T] {
	return h.run().with(workerForPool(opts))
}

type runImpl[T any, H HandlerFunc[T]] struct {
	hn  handlerImpl[T, H]
	seq iter.Seq[T]
	run func(iter.Seq[fnx.Worker]) fnx.Worker
}

func (r *runImpl[T, H]) For(seq iter.Seq[T]) fnx.Worker            { return r.run(r.hn.For(seq)) }
func (r *runImpl[T, H]) each(seq iter.Seq[T]) *runImpl[T, H]       { r.seq = seq; return r }
func (r *runImpl[T, H]) with(run workerExec) *runImpl[T, H]        { r.run = run; return r }
func (r *runImpl[T, H]) Pull(c context.Context) iter.Seq[error]    { return Pull(c, r.seq, r.hn.hf) }
func (r *runImpl[T, H]) PullAll(c context.Context) iter.Seq[error] { return PullAll(c, r.seq, r.hn.hf) }

func (r *runImpl[T, H]) PullWithPool(ctx context.Context, opts ...opt.Provider[*WorkerGroupConf]) iter.Seq[error] {
	return PullWithPool(ctx, r.seq, r.hn.hf, opts...)
}

func workerForPool(opts []opt.Provider[*WorkerGroupConf]) workerExec {
	return func(seq iter.Seq[fnx.Worker]) fnx.Worker { return RunWithPool(seq, opts...) }
}
