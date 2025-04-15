// Package fun is a zero-dependency collection of tools and idoms that
// takes advantage of generics. Streams, error handling, a
// native-feeling Set type, and a simple pub-sub framework for
// distributing messages in fan-out patterns.
package fun

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// ErrStreamContinue instructs consumers of Streams and related
// processors that run groups. Equivalent to the "continue" keyword in
// other contexts.
const ErrStreamContinue ers.Error = ers.ErrCurrentOpSkip

// Stream provides a safe, context-respecting iteration/sequence
// paradigm, and entire tool kit for consumer functions, converters,
// and generation options.
//
// As the basis and heart of a programming model, streams make it
// possible to think about groups or sequences of objects or work,
// that can be constructed and populated lazily, and provide a
// collection of interfaces for processing and manipulating data.
//
// Beyond the stream interactive tools provided in this package, the
// itertool package provdes some additional helpers and tools, while
// the adt and dt packages provide simple types and tooling built
// around these streams
//
// The canonical way to use a stream is with the core Next()
// Value() and Close() methods: Next takes a context and advances the
// stream. Next, which is typically called in single-clause for loop
// (e.g. as in while loop) returns false when the stream has no
// items, after which the stream should be closed and cannot be
// re-started. When Next() returns true, the stream is advanced, and
// the output of Value() will provide the value at the current
// position in the stream. Next() will block if the stream has not
// been closed, and the operation with Produces or Generates new items
// for the stream blocks, (or continues iterating) until the
// stream is exhausted, or closed.
//
// However, additional methods, such as ReadOne, the Generator()
// function (which is a wrapper around ReadOne) provide a different
// iteraction paradigm: they combine the Next() and value operations
// into a single function call. When the stream is exhausted these
// methods return the `io.EOF` error.
//
// In all cases, checking the Close() value of the stream makes it
// possible to see any errors encountered during the operation of the
// stream.
//
// Using Next/Value cannot be used concurrently, as there is no way to
// synchronize the Next/Value calls with respect to eachother: it's
// possible in this mode to both miss and/or get duplicate values from
// the stream in this case. If the generator/generator function in
// the stream is safe for concurrent use, then ReadOne can be used
// safely. As a rule, all tooling in the fun package uses ReadOne
// except in a few cases where a caller has exclusive access to the
// stream.
type Stream[T any] struct {
	operation Generator[T]
	value     T

	err struct {
		once    sync.Once
		handler fn.Handler[error]
		future  fn.Future[error]
	}

	closer struct {
		state atomic.Bool
		once  sync.Once
		hooks []fn.Handler[*Stream[T]]
		ops   []func()
	}
}

// MakeStream constructs a stream that calls the Generator function
// once for every item, until it errors. Errors other than context
// cancellation errors and io.EOF are propgated to the stream's Close
// method.
func MakeStream[T any](gen Generator[T]) *Stream[T] {
	op, cancel := gen.WithCancel()
	st := &Stream[T]{operation: op}
	st.closer.ops = append(st.closer.ops, cancel)
	return st
}

// VariadicStream produces a stream from an arbitrary collection
// of objects, passed into the constructor.
func VariadicStream[T any](in ...T) *Stream[T] { return SliceStream(in) }

// ChannelStream exposes access to an existing "receive" channel as
// a stream.
func ChannelStream[T any](ch <-chan T) *Stream[T] { return BlockingReceive(ch).Stream() }

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

// ConvertStream processes the input stream of type T into an
// output stream of type O. It's implementation uses the Generator,
// will continue producing values as long as the input stream
// produces values, the context isn't canceled, or exhausted.
func ConvertStream[T, O any](iter *Stream[T], op Transform[T, O]) *Stream[O] {
	return op.Process(iter)
}

// Transform processes a stream passing each element through a
// transform function. The type of the stream is the same for the
// output. Use Convert stream to change the type of the value.
func (st *Stream[T]) Transform(op Transform[T, T]) *Stream[T] { return op.Process(st) }

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
func MergeStreams[T any](iters *Stream[*Stream[T]]) *Stream[T] {
	pipe := Blocking(make(chan T))
	es := &ers.Stack{}
	mu := &sync.Mutex{}

	eh := MAKE.ErrorHandlerWithoutTerminating(es.Handler()).WithLock(mu)
	ep := fn.MakeFuture(es.Future()).WithLock(mu)

	init := Operation(func(ctx context.Context) {
		wg := &WaitGroup{}
		wctx, cancel := context.WithCancel(ctx)
		send := pipe.Send()

		iters.ReadAll(func(iter *Stream[T]) {
			send.Consume(iter).Operation(eh).Add(ctx, wg)
		}).Operation(eh).Add(wctx, wg)

		wg.Operation().
			PostHook(cancel).
			PostHook(pipe.Close).
			Background(ctx)
	}).Go().Once()

	return pipe.Receive().
		Generator().
		PreHook(init).
		Stream().
		WithErrorCollector(eh, ep)
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

// FlattenStreams combines input streams into a single output stream.
//
// There is no buffering of the flattened stream, but elements in
// the output stream are drawn from the input streams sequentially
// and in order: both order of the input stream and the items in all
// constituent streams are reflected in the output.
//
// Use FlattenStreams or ChainStreams if order is important. Use
// FlattenStream for larger numbers of streams. Use MergeStreams
// when producing an item takes a non-trivial amount of time.
func FlattenStreams[T any](iters *Stream[*Stream[T]]) *Stream[T] {
	pipe := Blocking(make(chan T))

	eh, ep := MAKE.ErrorCollector()
	eh = MAKE.ErrorHandlerWithoutTerminating(eh)

	return pipe.
		Generator().
		PreHook(Operation(
			func(ctx context.Context) {
				defer pipe.Close()

				send := pipe.Send() // input end of the pipe

				iters.ReadAll(func(in *Stream[T]) {
					// write the contents of the input stream
					// to the input end of the pipe
					send.Handler().ReadAll(in).Observe(ctx, eh)
				}).Observe(ctx, eh)
			}).
			Go().Once(),
		).Stream().WithErrorCollector(eh, ep)
}

func (st *Stream[T]) doClose() {
	st.closer.once.Do(func() {
		st.ensureErrorHandler()

		st.closer.state.Store(true)

		fn.JoinHandlers(st.closer.hooks).Handle(st)

		ft.Call(ft.JoinFuture(st.closer.ops))
	})
}

// Close terminates the stream and returns any errors collected
// during iteration. If the stream allocates resources, this
// will typically release them, but close may not block until all
// resources are released.
func (st *Stream[T]) Close() error { st.doClose(); return st.err.future() }

func (*Stream[T]) joinTwoErrs(e1, e2 error) error { return ers.Join(e1, e2) }

func (st *Stream[T]) WithErrorCollector(ec fn.Handler[error], er fn.Future[error]) *Stream[T] {
	st.err.handler = ec.Join(st.err.handler)
	st.err.future = er.Join(st.joinTwoErrs, st.err.future)
	return st
}

// WithHook constructs a stream from the generator. The
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
func (st *Stream[T]) AddError(e error) {
	st.ensureErrorHandler()
	st.err.handler(e)
}

// ErrorHandler provides access to the AddError method as an error observer.
func (st *Stream[T]) ErrorHandler() fn.Handler[error] { st.ensureErrorHandler(); return st.err.handler }

// Generator provides access to the contents of the stream as a
// Generator function.
func (st *Stream[T]) Generator() Generator[T] { return st.ReadOne }

// Value returns the object at the current position in the
// stream. It's often used with Next() for looping over the
// stream.
//
// Value and Next cannot be done safely when the stream is being
// used concrrently. Use ReadOne or the Generator method.
func (st *Stream[T]) Value() T { return st.value }

// Next advances the stream (using ReadOne) and caches the current
// value for access with the Value() method. When Next is true, the
// Value() will return the next item. When false, either the stream
// has been exhausted (e.g. the Generator function has returned io.EOF)
// or the context passed to Next has been canceled.
//
// Using Next/Value cannot be done safely if stream is accessed from
// multiple go routines concurrently. In these cases use ReadOne
// directly, or use Split to create a stream that safely draws
// items from the parent stream.
func (st *Stream[T]) Next(ctx context.Context) bool {
	if val, err := st.ReadOne(ctx); err == nil {
		st.value = val
		return true
	}
	return false
}

// ReadOne advances the stream and returns the value as a single
// option. This operation IS safe for concurrent use.
//
// ReadOne returns the io.EOF error when the stream has been
// exhausted, a context expiration error or the underlying error
// produced by the stream. All errors produced by ReadOne are
// terminal and indicate that no further iteration is possible.
func (st *Stream[T]) ReadOne(ctx context.Context) (out T, err error) {
	defer func() { ft.WhenCall(err != nil, st.doClose) }()

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
		case errors.Is(err, ErrStreamContinue):
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
// of the worker, and abort the processing. If the processor function
// returns ErrStreamContinue, processing continues. All other errors
// abort processing and are returned by the worker.
func (st *Stream[T]) ReadAll(fn fn.Handler[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover()), st.Close()) }()
		for {
			item, err := st.ReadOne(ctx)
			switch {
			case err == nil:
				fn(item)
			case ers.IsExpiredContext(err):
				return err
			case ers.IsTerminating(err):
				return nil
			default:
				// this is (realistically) only context
				// cancellation errors, because ReadOne puts
				// all errors into the stream's
				// Close() method.
				return err
			}
		}
	}
}

func (st *Stream[T]) ensureErrorHandler() { st.err.once.Do(st.setupDefaultErrorHandler) }
func (st *Stream[T]) setupDefaultErrorHandler() {
	st.err.handler, st.err.future = MAKE.ErrorCollector()
}

// Filter passes every item in the stream and, if the check function
// returns true propagates it to the output stream.  There is no
// buffering, and check functions should return quickly. For more
// advanced use, consider using itertool.Map()
func (st *Stream[T]) Filter(check func(T) bool) *Stream[T] {
	return NewGenerator(func(ctx context.Context) (out T, _ error) {
		for {
			item, err := st.ReadOne(ctx)
			switch {
			case err == nil:
				if check(item) {
					return item, nil
				}
			case ers.Is(err, ErrStreamContinue):
				continue
			default:
				return out, err
			}
		}
	}).Stream()
}

// Any, as a special case of Transform converts a stream of any
// type and converts it to a stream of any (e.g. interface{})
// values.
func (st *Stream[T]) Any() *Stream[any] {
	return Converter(func(in T) any { return any(in) }).Process(st)
}

// Reduce processes a stream with a reducer function. The output
// function is a Generator operation which runs synchronously, and no
// processing happens before generator is called. If the reducer
// function returns, ErrStreamContinue, the output value is ignored, and
// the reducer operation continues. io.EOR errors are not propagated
// to the caller, and in all situations, the last value produced by
// the reducer is returned with an error.
//
// The "previous" value for the first reduce option is the zero value
// for the type T.
func (st *Stream[T]) Reduce(
	reducer func(T, T) (T, error),
) Generator[T] {
	var value T
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()

		for {
			item, err := st.ReadOne(ctx)
			if err != nil {
				return value, nil
			}

			out, err := reducer(item, value)
			switch {
			case err == nil:
				value = out
				continue
			case errors.Is(err, ErrStreamContinue):
				continue
			case ers.Is(err, io.EOF, ers.ErrCurrentOpAbort):
				return value, nil
			default:
				return value, err
			}
		}
	}
}

// Count returns the number of items observed by the stream. Callers
// should still manually call Close on the stream.
func (st *Stream[T]) Count(ctx context.Context) (count int) {
	for proc := st.Generator(); ft.IsOk(proc.Check(ctx)); {
		count++
	}
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
	pipe := Blocking(make(chan T))

	setup := pipe.Send().Consume(st).PostHook(pipe.Close).Ignore().Go().Once()
	output := make([]*Stream[T], num)
	for idx := range output {
		output[idx] = pipe.Generator().PreHook(setup).Stream()
	}

	return output
}

// Join merges multiple streams processing and producing their results
// sequentially, and without starting any go routines. Otherwise
// similar to Flatten (which processes each stream in parallel).
func (st *Stream[T]) Join(iters ...*Stream[T]) *Stream[T] {
	proc := st.Generator()
	for idx := range iters {
		proc = proc.Join(iters[idx].ReadOne)
	}
	return proc.Stream()
}

// Slice converts a stream to the slice of it's values, and
// closes the stream at the when the stream has been exhausted..
//
// In the case of an error in the underlying stream the output slice
// will have the values encountered before the error.
func (st *Stream[T]) Slice(ctx context.Context) (out []T, _ error) {
	return out, st.ReadAll(func(in T) { out = append(out, in) }).Run(ctx)
}

// Channel proides access to the contents of the stream as a
// channel. The channel is closed when the stream is exhausted.
func (st *Stream[T]) Channel(ctx context.Context) <-chan T { return st.BufferedChannel(ctx, 0) }

// BufferedChannel provides access to the content of the stream with
// a buffered channel that is closed when the stream is
// exhausted.
func (st *Stream[T]) BufferedChannel(ctx context.Context, size int) <-chan T {
	out := Blocking(make(chan T, size))

	out.Handler().
		ReadAll(st).
		PostHook(out.Close).
		Operation(st.AddError).
		Launch(ctx)

	return out.Channel()
}

// MarshalJSON is useful for implementing json.Marshaler methods
// from stream-supporting types. Wrapping the standard library's
// json encoding tools.
//
// The contents of the stream are marshaled as elements in an JSON
// array.
func (st *Stream[T]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	_ = buf.WriteByte('[')
	first := true

	// decide to capture a context in the
	// stream or not care
	ctx := context.TODO()

	for val := range st.Seq(ctx) {
		if first {
			first = false
		} else {
			_ = buf.WriteByte(',')
		}

		if err := enc.Encode(val); err != nil {
			return nil, err
		}
	}

	_ = buf.WriteByte(']')

	return buf.Bytes(), nil
}

// UnmarshalJSON reads a byte-array of input data that contains a JSON
// array and then processes and returns that data iteratively.
//
// To handle streaming data from an io.Reader that contains a stream
// of line-separated json documents, use itertool.JSON.
func (st *Stream[T]) UnmarshalJSON(in []byte) error {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return err
	}
	var idx int
	st.operation = st.operation.Join(func(_ context.Context) (out T, err error) {
		if idx >= len(rv) {
			return out, io.EOF
		}
		err = json.Unmarshal(rv[idx], &out)
		if err == nil {
			idx++
		}
		return
	})
	return nil
}

// ReadAllParallel produces a worker that, when executed, will
// iteratively processes the contents of the stream. The options
// control the error handling and parallelism semantics of the
// operation.
//
// This is the work-house operation of the package, and can be used as
// the basis of worker pools, even processing, or message dispatching
// for pubsub queues and related systems.
func (st *Stream[T]) ReadAllParallel(
	fn Handler[T],
	optp ...OptionProvider[*WorkerGroupConf],
) Worker {
	return func(ctx context.Context) error {
		opts := &WorkerGroupConf{}
		if err := JoinOptionProviders(optp...).Apply(opts); err != nil {
			return err
		}

		if opts.ErrorHandler == nil {
			opts.ErrorHandler, opts.ErrorResolver = MAKE.ErrorCollector()
		}

		fn.WithRecover().WithErrorFilter(opts.ErrorFilter).
			ReadAll(st).
			StartGroup(ctx, opts.NumWorkers).
			Ignore().Run(ctx)

		return opts.ErrorResolver()
	}
}

// Buffer adds a buffer in the queue using a channel as buffer to
// smooth out iteration performance, if the iteration (generator) and the
// consumer both take time, even a small buffer will improve the
// throughput of the system and prevent both components of the system
// from blocking on eachother.
//
// The ordering of elements in the output stream is the same as the
// order of elements in the input stream.
func (st *Stream[T]) Buffer(n int) *Stream[T] {
	buf := Blocking(make(chan T, n))
	pipe := buf.Send().Consume(st).Operation(st.ErrorHandler()).PostHook(buf.Close).Go().Once()
	return buf.Generator().PreHook(pipe).Stream()
}

// ParallelBuffer, like buffer, process the input queue and stores
// those items in a channel; however, unlike Buffer, multiple workers
// consume the input stream: as a result the order of the elements
// in the output stream is not the same as the input order.
//
// Otherwise, the two Buffer methods are equivalent and serve the same
// purpose: process the items from a stream without blocking the
// consumer of the stream.
func (st *Stream[T]) ParallelBuffer(n int) *Stream[T] {
	buf := Blocking(make(chan T, n))

	return buf.Generator().PreHook(
		st.ReadAllParallel(
			buf.Send().Handler(),
			WorkerGroupConfNumWorkers(n),
		).Operation(st.ErrorHandler()).
			PostHook(buf.Close).Go().Once(),
	).Stream()
}

// Seq converts a fun.Stream[T] into a native go iterator.
func (st *Stream[T]) Seq(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			item, err := st.ReadOne(ctx)
			if err != nil || !yield(item) {
				return
			}
		}
	}
}
