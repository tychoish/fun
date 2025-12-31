package pubsub

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/opt"
	"github.com/tychoish/fun/wpa"
)

// Convert takes an input stream of one type, and returns a function which takes an
// fnx.Converter function that returns the output stream.  and converts it to a stream of the
// another type. All errors from the original stream are propagated to the output stream.
func Convert[T, O any](op fnx.Converter[T, O]) interface {
	Stream(*Stream[T]) *Stream[O]
	Parallel(*Stream[T], ...opt.Provider[*wpa.WorkerGroupConf]) *Stream[O]
} {
	return &converter[T, O]{op: op}
}

// ConvertFn simplifies calls to pubsub.Convert for fn.Convert types.
func ConvertFn[T any, O any](op fn.Converter[T, O]) interface {
	Stream(*Stream[T]) *Stream[O]
	Parallel(*Stream[T], ...opt.Provider[*wpa.WorkerGroupConf]) *Stream[O]
} {
	return &converter[T, O]{op: fnx.MakeConverter(op)}
}

type converter[T any, O any] struct {
	op fnx.Converter[T, O]
}

func (converter[T, O]) zero() (v O) { return v }

func (c *converter[T, O]) Stream(st *Stream[T]) *Stream[O] {
	return MakeStream(func(ctx context.Context) (out O, _ error) {
		for {
			item, err := st.Read(ctx)
			if err != nil {
				// you'd think that you'd need to
				// filter out Continue and other
				// signals here, but the Read method
				// on the stream will do that making
				// these unreachable.
				return c.zero(), err
			}

			out, err = c.op.Convert(ctx, item)
			switch {
			case err == nil:
				return out, nil
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			default:
				return c.zero(), err
			}
		}
	}).WithHook(func(outer *Stream[O]) { outer.AddError(st.Close()) })
}

// Parallel runs the input stream through the transform
// operation and produces an output stream, much like
// convert. However, the Parallel implementation has
// configurable parallelism, and error handling with the
// WorkerGroupConf options.
func (c converter[T, O]) Parallel(
	iter *Stream[T],
	opts ...opt.Provider[*wpa.WorkerGroupConf],
) *Stream[O] {
	output := Blocking(make(chan O))

	conf := &wpa.WorkerGroupConf{}
	if err := opt.Join(opts...).Apply(conf); err != nil {
		return MakeStream(fnx.MakeFuture(func() (O, error) { return c.zero(), err }))
	}

	setup := iter.Parallel(
		c.mapPullProcess(output.Send().Write, conf),
		wpa.WorkerGroupConfSet(conf),
	).PostHook(output.Close).Operation(conf.ErrorCollector.Push).Go().Once()

	return MakeStream(fnx.NewFuture(output.Receive().Read).PreHook(setup)).
		WithHook(func(out *Stream[O]) {
			out.AddError(iter.Close())
			out.AddError(conf.ErrorCollector.Resolve())
		})
}

// mapPullProcess returns a processor which consumes "input"  items, passes
// them to the transform function, and then "sends" them with the
// provided processor function.
func (c converter[T, O]) mapPullProcess(
	output fnx.Handler[O],
	opts *wpa.WorkerGroupConf,
) fnx.Handler[T] {
	mpf := c.op.WithRecover()
	return func(ctx context.Context, in T) error {
		val, err := mpf(ctx, in)
		if err != nil {
			if opts.CanContinueOnError(err) {
				return nil
			}
			return io.EOF
		}

		if !output.Check(ctx, val) {
			return io.EOF
		}

		return nil
	}
}
