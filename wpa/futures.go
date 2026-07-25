package wpa

import (
	"context"
	"errors"
	"iter"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
)

// FutureJob describes the union of the fnx.Future, wpa.Result, and
// wpa.Producer types, allowing the ResolveFutures family to operate on
// any of them without runtime type casting.
type FutureJob[T any] interface {
	fnx.Future[T] | Result[T] | Producer[T]
	Job(context.Context) (T, error)
}

// Producer is a niladic value-producing function, the value analog of
// wpa.Thunk. Mirrors fn.Future[T] but is a wpa-local type so it can carry
// a Job method without modifying the fn package.
type Producer[T any] func() T

// Job converts a Producer into a panic-safe, value-producing function
// suitable for use with the ResolveFutures family. This exists to
// satisfy the wpa.FutureJob type constraint.
func (pf Producer[T]) Job(ctx context.Context) (T, error) {
	return fnx.WrapFuture(fn.Future[T](pf))(ctx)
}

// Result is a niladic value-producing function that can also fail, the
// value analog of wpa.Task.
type Result[T any] func() (T, error)

// Job converts a Result into a panic-safe, value-producing function
// suitable for use with the ResolveFutures family. This exists to
// satisfy the wpa.FutureJob type constraint.
func (rf Result[T]) Job(ctx context.Context) (T, error) { return fnx.MakeFuture(rf).WithRecover()(ctx) }

// resolveFuture returns a converter, suitable for use as the op
// argument to irt.With2/irt.Pool3 (or any other irt combinator that
// maps a single value into a pair), that resolves a FutureJob into its
// (value, error) result.
func resolveFuture[V any, T FutureJob[V]](ctx context.Context) func(T) (V, error) {
	return func(job T) (V, error) { return job.Job(ctx) }
}

// ResolveFutures resolves futures sequentially, yielding a (value,
// error) pair per item. Abort-on-error semantics mirror Pull:
// terminating errors (io.EOF, ers.ErrCurrentOpAbort) stop the sequence
// without yielding a final pair; ers.ErrCurrentOpSkip is skipped
// entirely (no pair yielded for it).
func ResolveFutures[V any, T FutureJob[V]](ctx context.Context, seq iter.Seq[T]) iter.Seq2[V, error] {
	return func(yield func(V, error) bool) {
		for v, err := range irt.With2(seq, resolveFuture[V, T](ctx)) {
			switch {
			case err == nil:
				if !yield(v, nil) {
					return
				}
			case ers.IsTerminating(err):
				return
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			default:
				yield(v, err)
				return
			}
		}
	}
}

// ResolveFuturesAll resolves every future, yielding a (value, error)
// pair per item regardless of error (mirrors PullAll).
func ResolveFuturesAll[V any, T FutureJob[V]](ctx context.Context, seq iter.Seq[T]) iter.Seq2[V, error] {
	return irt.With2(seq, resolveFuture[V, T](ctx))
}

// ResolveFuturesWithPool resolves futures concurrently via irt.Pool3,
// yielding (value, error) pairs as they complete (out of order).
// Mirrors PullWithPool.
func ResolveFuturesWithPool[V any, T FutureJob[V]](
	ctx context.Context,
	seq iter.Seq[T],
	opts ...opt.Provider[*WorkerGroupConf],
) iter.Seq2[V, error] {
	opts = append(
		opts,
		WorkerGroupConfDisableErrorCollector(),
		WorkerGroupConfCustomValidatorAppend(func(conf *WorkerGroupConf) error {
			return ers.When(conf.ErrorCollector != nil, "cannot define a custom error collector for wpa pooled operations")
		}),
	)
	conf := &WorkerGroupConf{}
	if err := opt.Join(opts...).Apply(conf); err != nil {
		var zero V
		return irt.Two(zero, err)
	}

	resolve := resolveFuture[V, T](ctx)
	return irt.Pool3(ctx, conf.NumWorkers, seq, func(job T) (V, error) {
		v, err := resolve(job)
		return v, conf.filterPreserving(err)
	})
}
