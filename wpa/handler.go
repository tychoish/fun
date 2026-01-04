package wpa

import (
	"context"
	"iter"

	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
)

// Handler provides a type constraint for processing data, to be used
// with the WithHandler().For() function in order to use the
// Run/RunAll/RunWithPool functions with arbitrary sequences.
type Handler[T any] interface {
	fnx.Handler[T] | fn.Handler[T] | Reader[T]
	Job(T) func(context.Context) error
}

// Reader describes a common function signature for use in the
// WithHandler().For() operation.
type Reader[T any] func(T) error

// Job converts a Reader method into an fnx.Worker to satisfy the
// Handler[T] constraint for use with the WithHandler().For() operation.
func (hf Reader[T]) Job(in T) func(context.Context) error { return fnx.MakeHandler(hf).Job(in) }

// TaskHandler runs a task. Because this function which has the signature of an fnx.Handler function,
// can be used as a Handler for WithHandler operations, as needed.
func TaskHandler[J Job](ctx context.Context, job J) error { return toWorker(job).Run(ctx) }

// WithHandler provides an ergonomic bridge between sequences with
// arbitrary types and handler functions, and the worker pools that the execution in the Run and Pull operations.
func WithHandler[T any, H Handler[T]](op H) interface {
	For(seq iter.Seq[T]) iter.Seq[fnx.Worker]
	Run() interface{ For(iter.Seq[T]) fnx.Worker }
	RunAll() interface{ For(iter.Seq[T]) fnx.Worker }
	RunWithPool(opts ...opt.Provider[*WorkerGroupConf]) interface{ For(iter.Seq[T]) fnx.Worker }

	ForEach(seq iter.Seq[T]) interface {
		Pull(context.Context) iter.Seq[error]
		PullAll(context.Context) iter.Seq[error]
		PullWithPool(context.Context, ...opt.Provider[*WorkerGroupConf]) iter.Seq[error]
	}
} {
	return newHandler(op)
}

type handlerImpl[T any, H Handler[T]] struct {
	hf H
}

func newHandler[T any, H Handler[T]](in H) handlerImpl[T, H] { return handlerImpl[T, H]{hf: in} }
func (h handlerImpl[T, H]) runner() *runnerImpl[T, H]        { return &runnerImpl[T, H]{hn: h} }
func (h handlerImpl[T, H]) into(in T) fnx.Worker             { return h.hf.Job(in) }
func (h handlerImpl[T, H]) For(seq iter.Seq[T]) iter.Seq[fnx.Worker] {
	return irt.Convert(seq, h.into)
}

func (h handlerImpl[T, H]) ForEach(seq iter.Seq[T]) interface {
	Pull(context.Context) iter.Seq[error]
	PullAll(context.Context) iter.Seq[error]
	PullWithPool(context.Context, ...opt.Provider[*WorkerGroupConf]) iter.Seq[error]
} {
	return h.runner().each(seq)
}

func (h handlerImpl[T, H]) Run() interface{ For(iter.Seq[T]) fnx.Worker } {
	return h.runner().with(Run)
}

func (h handlerImpl[T, H]) RunAll() interface{ For(iter.Seq[T]) fnx.Worker } {
	return h.runner().with(RunAll[fnx.Worker])
}

func runWithConfiguredPool(opts []opt.Provider[*WorkerGroupConf]) func(iter.Seq[fnx.Worker]) fnx.Worker {
	return func(seq iter.Seq[fnx.Worker]) fnx.Worker {
		return RunWithPool(seq, opts...)
	}
}

func (h handlerImpl[T, H]) RunWithPool(opts ...opt.Provider[*WorkerGroupConf]) interface{ For(iter.Seq[T]) fnx.Worker } {
	return h.runner().with(runWithConfiguredPool(opts))
}

type runnerImpl[T any, H Handler[T]] struct {
	hn  handlerImpl[T, H]
	seq iter.Seq[T]
	run func(iter.Seq[fnx.Worker]) fnx.Worker
}

func (r *runnerImpl[T, H]) For(seq iter.Seq[T]) fnx.Worker { return r.run(r.hn.For(seq)) }

func (r *runnerImpl[T, H]) each(seq iter.Seq[T]) *runnerImpl[T, H] { r.seq = seq; return r }
func (r *runnerImpl[T, H]) with(run func(iter.Seq[fnx.Worker]) fnx.Worker) *runnerImpl[T, H] {
	r.run = run
	return r
}

func (r *runnerImpl[T, H]) Pull(ctx context.Context) iter.Seq[error] {
	return Pull(ctx, r.seq, r.hn.hf)
}

func (r *runnerImpl[T, H]) PullAll(ctx context.Context) iter.Seq[error] {
	return PullAll(ctx, r.seq, r.hn.hf)
}

func (r *runnerImpl[T, H]) PullWithPool(ctx context.Context, opts ...opt.Provider[*WorkerGroupConf]) iter.Seq[error] {
	return PullWithPool(ctx, r.seq, r.hn.hf, opts...)
}
