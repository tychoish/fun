package pubsub

import (
	"context"
	"errors"
	"io"
	"iter"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/wpa"
)

// Stream provides a safe, context-respecting iteration/sequence
// paradigm, and entire tool kit for consumer functions, converters,
// and generation options.
//
// The implementation of Stream predated Go's native iterators, and
// Stream (originally named Iterator,) was an attempt to provide lazy,
// incremental execution and iteration-focused flows, with a decidedly
// functional and event-driven bent.
//
// This gave rise to the `fnx` package, and in many ways everything in
// the `tychoish/fun` package, all of which _do_ succeed at the aim of
// providing ergonomic, batteries-included programming models to
// address fundamental (and shared concerns.)
//
// Fundamentally, this type has been superceded by iterators which
// provide a much more reasonable and safe interface for common
// operations. However, being able to use simple context-aware
// functions as the basis for iteration/event-driven program has some
// potential utility, particularly for pub-sub use cases, and so it
// remains.
//
// Consider it deprecated: it will be removed in the future, but
// without a deadline. New code should use the iter.Seq[T] model, and the
// `fun/irt` package provides a lot of second-order tools for using
// iterators to ease the transition.
type Stream[T any] struct {
	operation func(context.Context) (T, error)
	value     T

	erc    erc.Collector
	closer struct {
		state atomic.Bool
		once  sync.Once
		hooks []fn.Handler[*Stream[T]]
		ops   []func()
	}
}

// MakeStream constructs a stream that calls the Future function
// once for every item, until it errors. Errors other than context
// cancellation errors and io.EOF are propgated to the stream's Close
// method.
func MakeStream[T any](gen func(context.Context) (T, error)) *Stream[T] {
	op, cancel := fnx.NewFuture(gen).WithCancel()
	st := &Stream[T]{operation: op}
	st.closer.ops = append(st.closer.ops, cancel)
	return st
}

// VariadicStream produces a stream from an arbitrary collection
// of objects, passed into the constructor.
func VariadicStream[T any](in ...T) *Stream[T] { return SliceStream(in) }

// ChannelStream exposes access to an existing "receive" channel as
// a stream.
func ChannelStream[T any](ch <-chan T) *Stream[T] {
	return MakeStream(stw.ChanBlockingReceive(ch).Read)
}

// SliceStream provides Stream access to the elements in a slice.
func SliceStream[T any](in []T) *Stream[T] {
	s := in
	idx := atomic.Int64{}
	idx.Store(-1)

	// Stream.ReadOne should take care of context
	// cancellation. There's no reason to cancel
	return MakeStream(func(context.Context) (out T, _ error) {
		next := idx.Add(1)

		if int(next) >= len(s) {
			return out, io.EOF
		}
		return s[next], nil
	})
}

// MergeStreams takes a collection of streams of the same type of
// objects and provides a single stream over these items.
//
// There are a collection of background threads, one for each input
// stream, which will iterate over the inputs and will provide the
// items to the output stream. These threads start on the first
// iteration and will return if this context is canceled.
//
// The stream will continue to produce items until all input
// streams have been consumed, the initial context is canceled, or
// the Close method is called, or all of the input streams have
// returned an error.
//
// Use MergeStreams when producing an item takes a non-trivial
// amount of time. Use ChainStreams or FlattenStreams if order is
// important. Use FlattenStream for larger numbers of streams.
//
// Deprecated: use irt.Chain() and iter.Seq[T] iterators instead.
func MergeStreams[T any](iters *Stream[*Stream[T]]) *Stream[T] {
	pipe := stw.ChanBlocking(make(chan T))
	ec := &erc.Collector{}
	ec.SetFilter(erc.NewFilter().WithoutContext().WithoutTerminating())

	wg := &fnx.WaitGroup{}
	init := fnx.Operation(func(ctx context.Context) {
		send := pipe.Send()

		iters.ReadAll(fnx.FromHandler(func(iter *Stream[T]) {
			// start a thread for reading this
			// sub-iterator.

			iter.Parallel(send.Write, wpa.WorkerGroupConfNumWorkers(pipe.Cap())).
				Operation(ec.Push).
				PostHook(func() { ec.Push(iter.Close()) }).
				Add(ctx, wg)
		})).Operation(ec.Push).Add(ctx, wg)

		wg.Operation().PostHook(pipe.Close).Background(ctx)
	}).Once()

	return MakeStream(fnx.NewFuture(pipe.Receive().Read).
		PreHook(init)).
		WithHook(func(st *Stream[T]) {
			fnx.WithContextCall(wg.Wait)
			st.AddError(ec.Resolve())
		})
}

// JoinStreams takes a sequence of streams and produces a combined
// stream. JoinStreams processes items sequentially from each
// stream. By contrast, MergeStreams constructs a stream that reads
// all of the items from the input streams in parallel, and returns
// items in an arbitrary order.
//
// Use JoinStreams or FlattenStreams if order is important. Use
// FlattenStream for larger numbers of streams. Use MergeStreams
// when producing an item takes a non-trivial amount of time.
func JoinStreams[T any](iters ...*Stream[T]) *Stream[T] { return new(Stream[T]).Join(iters...) }

func (st *Stream[T]) doClose() {
	st.closer.once.Do(func() {
		st.closer.state.Store(true)
		fn.JoinHandlers(irt.Slice(st.closer.hooks)).Read(st)
		irt.Apply(irt.Remove(irt.Slice(st.closer.ops), func(op func()) bool { return op == nil }), func(op func()) { op() })
	})
}

// Close terminates the stream and returns any errors collected
// during iteration. If the stream allocates resources, this
// will typically release them, but close may not block until all
// resources are released.
//
// Close is safe to call more than once and always resolves the error handler (e.g. AddError),.
func (st *Stream[T]) Close() error { st.doClose(); return st.erc.Resolve() }

// WithHook constructs a stream from the future. The
// provided hook function will run during the Stream's Close()
// method.
func (st *Stream[T]) WithHook(hook fn.Handler[*Stream[T]]) *Stream[T] {
	st.closer.hooks = append(st.closer.hooks, hook)

	return st
}

// AddError can be used by calling code to add errors to the
// stream, which are merged.
//
// AddError is not safe for concurrent use (with regards to other
// AddError calls or Close).
func (st *Stream[T]) AddError(e error) { st.erc.Push(e) }

// ErrorHandler provides access to the AddError method as an error observer.
func (st *Stream[T]) ErrorHandler() fn.Handler[error] { return st.erc.Push }

// Value returns the object at the current position in the
// stream. It's often used with Next() for looping over the
// stream.
//
// Value and Next cannot be done safely when the stream is being
// used concrrently. Use ReadOne or the Future method.
func (st *Stream[T]) Value() T { return st.value }

// Next advances the stream (using ReadOne) and caches the current
// value for access with the Value() method. When Next is true, the
// Value() will return the next item. When false, either the stream
// has been exhausted (e.g. the Future function has returned io.EOF)
// or the context passed to Next has been canceled.
//
// Using Next/Value cannot be done safely if stream is accessed from
// multiple go routines concurrently. In these cases use ReadOne
// directly, or use Split to create a stream that safely draws
// items from the parent stream.
func (st *Stream[T]) Next(ctx context.Context) bool {
	if val, err := st.Read(ctx); err == nil {
		st.value = val
		return true
	}
	return false
}

func (st *Stream[T]) readOneHandleError(err error) error {
	if err != nil {
		return erc.Join(err, st.Close())
	}
	return nil
}

// Read returns a single value from the stream. This operation IS safe
// for concurrent use.
//
// Read returns the io.EOF error when the stream has been
// exhausted, a context expiration error or the underlying error
// produced by the stream. All errors produced by Read are
// terminal and indicate that no further iteration is possible.
func (st *Stream[T]) Read(ctx context.Context) (out T, err error) {
	defer func() { err = st.readOneHandleError(err) }()

	if err = ctx.Err(); err != nil {
		return out, err
	} else if st.closer.state.Load() || st.operation == nil {
		return out, io.EOF
	}

	for {
		out, err = st.operation(ctx)
		switch {
		case err == nil:
			return out, nil
		case errors.Is(err, ers.ErrCurrentOpSkip):
			continue
		case ers.IsTerminating(err):
			return out, err
		case ers.IsExpiredContext(err):
			return out, err
		case errors.Is(err, ers.ErrRecoveredPanic):
			st.AddError(err)
			return out, err
		default:
			st.AddError(err)
			return out, err
		}
	}
}

// ReadAll provides a function consumes all items in the stream with
// the provided processor function.
//
// All panics are converted to errors and propagated in the response
// of the worker, and abort the processing. If the handler function
// returns ers.ErrCurrentOpSkip, processing continues. All other errors
// abort processing and are returned by the worker.
func (st *Stream[T]) ReadAll(fn fnx.Handler[T]) fnx.Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover()), st.Close()) }()
		for {
			item, err := st.Read(ctx)
			if err == nil {
				err = fn(ctx, item)
			}
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
	}
}

// Parallel produces a worker that, when executed, will
// iteratively processes the contents of the stream. The options
// control the error handling and parallelism semantics of the
// operation.
//
// This is the work-house operation of the package, and can be used as
// the basis of worker pools, even processing, or message dispatching
// for pubsub queues and related systems.
func (st *Stream[T]) Parallel(fn fnx.Handler[T], opts ...opt.Provider[*wpa.WorkerGroupConf]) fnx.Worker {
	return func(ctx context.Context) error {
		return wpa.RunWithPool(
			irt.Convert(
				st.Iterator(ctx),
				func(in T) fnx.Worker {
					return func(ctx context.Context) error { return fn.Read(ctx, in) }
				},
			),
			opts...,
		).Run(ctx)
	}
}

// Filter passes every item in the stream and, if the check function
// returns true propagates it to the output stream.  There is no
// buffering, and check functions should return quickly. For more
// advanced use, consider using itertool.Map().
func (st *Stream[T]) Filter(check func(T) bool) *Stream[T] {
	return MakeStream(fnx.NewFuture(st.operation).Filter(check)).WithHook(st.CloseHook())
}

// Reduce processes a stream with a reducer function. The output
// function is a Future operation which runs synchronously, and no
// processing happens before future is called. If the reducer
// function returns ers.ErrCurrentOpSkip, the output value is ignored,
// and the reducer operation continues.
//
// If the underlying stream returns an error, it's returned by the
// close method of this the new stream. If the reducer function
// returns an error, that error is returned either by the Read or
// Close methods of the stream. If the underlying stream terminates
// cleanly, then the reducer will return it's last value without
// error. Otherwise any error returned by the Reduce method, other
// than ers.ErrCurrentOpSkip, is propagated to the caller.
//
// The "previous" value for the first reduce option is the zero value
// for the type T.
func (st *Stream[T]) Reduce(reducer func(T, T) (T, error)) *Stream[T] {
	var value T
	return MakeStream(func(ctx context.Context) (_ T, err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()

		for {
			item, err := st.Read(ctx)
			if err != nil {
				return value, nil
			}

			out, err := reducer(item, value)
			switch {
			case err == nil:
				value = out
				continue
			case ers.Is(err, ers.ErrCurrentOpSkip):
				continue
			default:
				return value, err
			}
		}
	}).WithHook(st.CloseHook())
}

// Count returns the number of items observed by the stream. Callers
// should still manually call Close on the stream.
func (st *Stream[T]) Count(ctx context.Context) (count int) {
	st.ReadAll(fnx.FromHandler(func(T) { count++ })).Ignore().Run(ctx)
	return count
}

// Split produces an arbitrary number of streams which divide the
// input. The division is lazy and depends on the rate of consumption
// of output streams, but every item from the input stream is sent
// to exactly one output stream, each of which can be safely used
// from a different go routine.
//
// The input stream is not closed after the output streams are
// exhausted. There is one background go routine that reads items off
// of the input stream, which starts when the first output stream
// is advanced: be aware that canceling this context will effectively
// cancel all streams.
func (st *Stream[T]) Split(num int) []*Stream[T] {
	if num <= 0 {
		return nil
	}
	pipe := stw.ChanBlocking(make(chan T))

	setup := st.Parallel(pipe.Send().Write, wpa.WorkerGroupConfNumWorkers(pipe.Cap())).PostHook(pipe.Close).Operation(st.AddError).Go().Once()
	output := make([]*Stream[T], num)
	for idx := range output {
		output[idx] = MakeStream(fnx.NewFuture(pipe.Receive().Read).PreHook(setup)).WithHook(st.CloseHook())
	}

	return output
}

// CloseHook returns a function that can be passed to the WithHook() method on a _new_ stream that wraps this stream, so that
// the other stream will call the inner stream's close method and include the inner stream's errors.
func (st *Stream[T]) CloseHook() func(*Stream[T]) {
	return func(next *Stream[T]) { next.AddError(st.Close()) }
}

// Join merges multiple streams processing and producing their results
// sequentially, and without starting any go routines. Otherwise
// similar to Flatten (which processes each stream in parallel).
func (st *Stream[T]) Join(iters ...*Stream[T]) *Stream[T] {
	proc := fnx.NewFuture(st.Read)
	for idx := range iters {
		proc = proc.Join(iters[idx].Read)
	}
	return MakeStream(proc)
}

// Slice converts a stream to the slice of it's values, and
// closes the stream at the when the stream has been exhausted..
//
// In the case of an error in the underlying stream the output slice
// will have the values encountered before the error.
func (st *Stream[T]) Slice(ctx context.Context) (out []T, _ error) {
	return out, st.ReadAll(fnx.FromHandler(func(in T) { out = append(out, in) })).Run(ctx)
}

// Buffer adds a buffer in the queue using a channel as buffer to
// smooth out iteration performance, if the iteration function (future) and the
// consumer both take time, even a small buffer will improve the
// throughput of the system and prevent both components of the system
// from blocking on eachother.
//
// The ordering of elements in the output stream is the same as the
// order of elements in the input stream.
func (st *Stream[T]) Buffer(n int) *Stream[T] {
	buf := stw.ChanBlocking(make(chan T, n))
	pipe := st.Parallel(buf.Send().Write, wpa.WorkerGroupConfNumWorkers(n)).Operation(st.ErrorHandler()).PostHook(buf.Close).Go().Once()
	return MakeStream(fnx.NewFuture(buf.Receive().Read).PreHook(pipe))
}

// BufferParallel processes the input queue and stores
// those items in a channel (like Buffer); however, unlike Buffer, multiple workers
// consume the input stream: as a result the order of the elements
// in the output stream is not the same as the input order.
//
// Otherwise, the two Buffer methods are equivalent and serve the same
// purpose: process the items from a stream without blocking the
// consumer of the stream.
func (st *Stream[T]) BufferParallel(n int) *Stream[T] {
	buf := stw.ChanBlocking(make(chan T, n))

	return MakeStream(fnx.NewFuture(buf.Receive().Read).PreHook(
		st.Parallel(
			buf.Send().Write,
			wpa.WorkerGroupConfNumWorkers(n),
		).Operation(st.ErrorHandler()).
			PostHook(buf.Close).Go().Once(),
	)).WithHook(st.CloseHook())
}

// Channel proides access to the contents of the stream as a
// channel. The channel is closed when the stream is exhausted.
func (st *Stream[T]) Channel(ctx context.Context) <-chan T { return st.BufferedChannel(ctx, 0) }

// BufferedChannel provides access to the content of the stream with
// a buffered channel that is closed when the stream is
// exhausted.
func (st *Stream[T]) BufferedChannel(ctx context.Context, size int) <-chan T {
	ch := make(chan T, size)
	out := stw.ChanBlocking(ch)

	st.ReadAll(out.Send().Write).
		PostHook(out.Close).
		Operation(st.AddError).
		Launch(ctx)

	return ch
}

// Iterator converts a pubsub.Stream[T] into a native go iterator.
func (st *Stream[T]) Iterator(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			item, err := st.Read(ctx)
			if err != nil || !yield(item) {
				return
			}
		}
	}
}
