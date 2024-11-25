// Package fun is a zero-dependency collection of tools and idoms that
// takes advantage of generics. Iterators, error handling, a
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
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// ErrIteratorSkip instructs consumers of Iterators and related
// processors that run groups. Equivalent to the "continue" keyword in
// other contexts.
const ErrIteratorSkip ers.Error = ers.ErrCurrentOpSkip

// Iterator provides a safe, context-respecting iteration/sequence
// paradigm, and entire tool kit for consumer functions, converters,
// and generation options.
//
// As the basis and heart of a programming model, iterators make it
// possible to think about groups or sequences of objects or work,
// that can be constructed and populated lazily, and provide a
// collection of interfaces for processing and manipulating data.
//
// Beyond the iterator interactive tools provided in this package, the
// itertool package provdes some additional helpers and tools, while
// the adt and dt packages provide simple iterations and tooling
// around iterators.
//
// The canonical way to use an iterator is with the core Next()
// Value() and Close() methods: Next takes a context and advances the
// iterator. Next, which is typically called in single-clause for loop
// (e.g. as in while loop) returns false when the iterator has no
// items, after which the iterator should be closed and cannot be
// re-started. When Next() returns true, the iterator is advanced, and
// the output of Value() will provide the value at the current
// position in the iterator. Next() will block if the iterator has not
// been closed, and the operation with Produces or Generates new items
// for the iterator blocks, (or continues iterating) until the
// iterator is exhausted, or closed.
//
// However, additional methods, such as ReadOne, the Producer()
// function (which is a wrapper around ReadOne) provide a different
// iteraction paradigm: they combine the Next() and value operations
// into a single function call. When the iterator is exhausted these
// methods return the `io.EOF` error.
//
// In all cases, checking the Close() value of the iterator makes it
// possible to see any errors encountered during the operation of the
// iterator.
//
// Using Next/Value cannot be used concurrently, as there is no way to
// synchronize the Next/Value calls with respect to eachother: it's
// possible in this mode to both miss and/or get duplicate values from
// the iterator in this case. If the generator/producer function in
// the iterator is safe for concurrent use, then ReadOne can be used
// safely. As a rule, all tooling in the fun package uses ReadOne
// except in a few cases where a caller has exclusive access to the
// iterator.
type Iterator[T any] struct {
	operation Producer[T]
	value     T

	err struct {
		handler Handler[error]
		future  Future[error]
	}

	closer struct {
		state atomic.Bool
		once  sync.Once
		op    context.CancelFunc
	}
}

// Generator creates an iterator that produces new values, using the
// generator function provided. This implementation does not create
// any background go routines, and the iterator will produce values
// until the function returns an error or the Close() method is
// called. Any non-nil error returned by the generator function is
// propagated to the close method, as long as it is not a context
// cancellation error or an io.EOF error.
func Generator[T any](op Producer[T]) *Iterator[T] { return op.Iterator() }

// VariadicIterator produces an iterator from an arbitrary collection
// of objects, passed into the constructor.
func VariadicIterator[T any](in ...T) *Iterator[T] { return SliceIterator(in) }

// ChannelIterator exposes access to an existing "receive" channel as
// an iterator.
func ChannelIterator[T any](ch <-chan T) *Iterator[T] { return BlockingReceive(ch).Iterator() }

// SliceIterator provides Iterator access to the elements in a slice.
func SliceIterator[T any](in []T) *Iterator[T] {
	s := in
	var idx = -1
	return Producer[T](func(ctx context.Context) (out T, _ error) {
		if len(s) <= idx+1 {
			return out, io.EOF
		}
		idx++
		return s[idx], ctx.Err()
	}).Iterator()
}

// ConvertIterator processes the input iterator of type T into an
// output iterator of type O. It's implementation uses the Generator,
// will continue producing values as long as the input iterator
// produces values, the context isn't canceled, or exhausted.
func ConvertIterator[T, O any](iter *Iterator[T], op Transform[T, O]) *Iterator[O] {
	return op.Process(iter)
}

// Transform processes an iterator passing each element through a
// transform function. The type of the iterator is the same for the
// output. Use Convert iterator to change the type of the value.
func (i *Iterator[T]) Transform(op Transform[T, T]) *Iterator[T] { return op.Process(i) }

// MergeIterators takes a collection of iterators of the same type of
// objects and provides a single iterator over these items.
//
// There are a collection of background threads which will iterate
// over the inputs and will provide the items to the output
// iterator. These threads start on the first iteration and will exit
// if this context is canceled.
//
// The iterator will continue to produce items until all input
// iterators have been consumed, the initial context is canceled, or
// the Close method is called, or all of the input iterators have
// returned an error.
func MergeIterators[T any](iters ...*Iterator[T]) *Iterator[T] {
	pipe := Blocking(make(chan T))

	es := &ers.Stack{}
	mu := &sync.Mutex{}
	eh := HF.ErrorHandlerWithoutEOF(es.Handler()).WithLock(mu)
	ep := Futurize(es.Future()).WithLock(mu)

	init := Operation(func(ctx context.Context) {
		wg := &WaitGroup{}
		wctx, cancel := context.WithCancel(ctx)

		// start a go routine for every iterator, to read from
		// the incoming iterator and push it to the pipe

		send := pipe.Send()
		for idx := range iters {
			send.Consume(iters[idx]).Operation(eh).Add(wctx, wg)
		}

		wg.Operation().PostHook(cancel).PostHook(pipe.Close).Background(ctx)
	}).Once()

	return pipe.Receive().
		Producer().
		PreHook(init).
		IteratorWithErrorCollector(eh, ep)
}

func (i *Iterator[T]) doClose() {
	i.closer.once.Do(func() { i.closer.state.Store(true); ft.SafeCall(i.closer.op) })
}

// Close terminates the iterator and returns any errors collected
// during iteration. If the iterator allocates resources, this
// will typically release them, but close may not block until all
// resources are released.
func (i *Iterator[T]) Close() error { i.doClose(); return ft.SafeDo(i.err.future) }

// AddError can be used by calling code to add errors to the
// iterator, which are merged.
//
// AddError is not safe for concurrent use (with regards to other
// AddError calls or Close).
func (i *Iterator[T]) AddError(e error) { i.err.handler(e) }

// ErrorHandler provides access to the AddError method as an error observer.
func (i *Iterator[T]) ErrorHandler() Handler[error] { return i.err.handler }

// Producer provides access to the contents of the iterator as a
// Producer function.
func (i *Iterator[T]) Producer() Producer[T] { return i.ReadOne }

// Value returns the object at the current position in the
// iterator. It's often used with Next() for looping over the
// iterator.
//
// Value and Next cannot be done safely when the iterator is bueing
// used concrrently. Use ReadOne or the Prodicer
func (i *Iterator[T]) Value() T { return i.value }

// Next advances the iterator (using ReadOne) and caches the current
// value for access with the Value() method. When Next is true, the
// Value() will return the next item. When false, either the iterator
// has been exhausted (e.g. the Producer function has returned io.EOF)
// or the context passed to Next has been canceled.
//
// Using Next/Value cannot be done safely if iterator is accessed from
// multiple go routines concurrently. In these cases use ReadOne
// directly, or use Split to create an iterator that safely draws
// items from the parent iterator.
func (i *Iterator[T]) Next(ctx context.Context) bool {
	if i.operation == nil || i.closer.state.Load() || ctx.Err() != nil {
		return false
	}

	val, err := i.ReadOne(ctx)
	if err == nil {
		i.value = val
		return true
	}
	return false
}

// ReadOne advances the iterator and returns the value as a single
// option. This operation IS safe for concurrent use.
//
// ReadOne returns the io.EOF error when the iterator has been
// exhausted, a context expiration error or the underlying error
// produced by the iterator. All errors produced by ReadOne are
// terminal and indicate that no further iteration is possible.
func (i *Iterator[T]) ReadOne(ctx context.Context) (out T, err error) {
	if i.operation == nil || i.closer.state.Load() {
		return out, io.EOF
	} else if err = ctx.Err(); err != nil {
		return out, err
	}

	defer func() { ft.WhenCall(err != nil, i.doClose) }()

	for {
		out, err = i.operation(ctx)
		switch {
		case err == nil:
			return out, nil
		case errors.Is(err, ErrIteratorSkip):
			continue
		case ers.IsTerminating(err):
			return out, err
		default:
			i.AddError(err)
			return out, io.EOF
		}
	}
}

// Filter passes every item in the iterator and, if the check function
// returns true propagates it to the output iterator.  There is no
// buffering, and check functions should return quickly. For more
// advanced use, consider using itertool.Map()
func (i *Iterator[T]) Filter(check func(T) bool) *Iterator[T] {
	return Producer[T](func(ctx context.Context) (out T, _ error) {
		for {
			item, err := i.ReadOne(ctx)
			if err != nil {
				return out, err
			}

			if check(item) {
				return item, nil
			}
		}
	}).Iterator()
}

// Any, as a special case of Transform converts an iterator of any
// type and converts it to an iterator of any (e.g. interface{})
// values.
func (i *Iterator[T]) Any() *Iterator[any] {
	return Converter(func(in T) any { return any(in) }).Process(i)
}

// Reduce processes an iterator with a reducer function. The output
// function is a Producer operation which runs synchronously, and no
// processing happens before producer is called. If the reducer
// function returns, ErrIteratorskip, the output value is ignored, and
// the reducer operation continues. io.EOR errors are not propagated
// to the caller, and in all situations, the last value produced by
// the reducer is returned with an error.
//
// The "previous" value for the first reduce option is the zero value
// for the type T.
func (i *Iterator[T]) Reduce(
	reducer func(T, T) (T, error),
) Producer[T] {
	var value T
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()

		for {
			item, err := i.ReadOne(ctx)
			if err != nil {
				return value, nil
			}

			out, err := reducer(item, value)
			switch {
			case err == nil:
				value = out
				continue
			case errors.Is(err, ErrIteratorSkip):
				continue
			case ers.Is(err, io.EOF, ers.ErrCurrentOpAbort):
				return value, nil
			default:
				return value, err
			}
		}
	}
}

// Count returns the number of items observed by the iterator. Callers
// should still manually call Close on the iterator.
func (i *Iterator[T]) Count(ctx context.Context) int {
	proc := i.Producer()
	var count int
	for {
		if !ft.IsOk(proc.Check(ctx)) {
			break
		}

		count++
	}
	return count
}

// Split produces an arbitrary number of iterators which divide the
// input. The division is lazy and depends on the rate of consumption
// of output iterators, but every item from the input iterator is sent
// to exactly one output iterator, each of which can be safely used
// from a different go routine.
//
// The input iterator is not closed after the output iterators are
// exhausted. There is one background go routine that reads items off
// of the input iterator, which starts when the first output iterator
// is advanced: be aware that canceling this context will effectively
// cancel all iterators.
func (i *Iterator[T]) Split(num int) []*Iterator[T] {
	if num <= 0 {
		return nil
	}

	pipe := Blocking(make(chan T))
	setup := pipe.Processor().
		ReadAll(i.Producer()).
		PostHook(pipe.Close).
		Ignore().Go().Once()

	output := make([]*Iterator[T], num)
	for idx := range output {
		output[idx] = pipe.Producer().PreHook(setup).Iterator()
	}

	return output
}

// Observe processes an iterator calling the handler function for
// every element in the iterator and retruning when the iterator is
// exhausted. Take care to ensure that the handler function does not
// block.
//
// The error returned captures any panics encountered as an error, as
// well as the output of the Close() operation. Observe will not add a
// context cancelation error to its error, though the observed
// iterator may return one in its close method.
func (i *Iterator[T]) Observe(fn Handler[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(i.Close(), err, ers.ParsePanic(recover())) }()
		for {
			item, err := i.ReadOne(ctx)
			switch {
			case err == nil:
				fn(item)
			case ers.Is(err, io.EOF, ers.ErrCurrentOpAbort):
				return nil
			default:
				// It seems like we should try and process
				// ErrIteratorSkip here but ReadOne handles
				// that case for us.
				//
				// this is (realistically) only context
				// cancellation errors, because ReadOne puts
				// all errors into the iterator's
				// Close() method.
				return err
			}
		}
	}
}

// Process provides a function consumes all items in the iterator with
// the provided processor function.
//
// All panics are converted to errors and propagated in the response
// of the worker, and abort the processing. If the processor function
// returns ErrIteratorSkip, processing continues. All other errors
// abort processing and are returned by the worker.
func (i *Iterator[T]) Process(fn Processor[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()

	LOOP:
		for {
			item, err := i.ReadOne(ctx)
			if err != nil {
				goto HANDLE_ERROR
			}

			err = fn(ctx, item)
			if err != nil {
				goto HANDLE_ERROR
			}

		HANDLE_ERROR:
			switch {
			case err == nil || errors.Is(err, ErrIteratorSkip):
				continue LOOP
			case ers.Is(err, io.EOF, ers.ErrCurrentOpAbort):
				return nil
			default:
				return err
			}
		}
	}
}

// Join merges multiple iterators processing and producing their results
// sequentially, and without starting any go routines. Otherwise
// similar to MergeIterators (which processes each iterator in parallel).
func (i *Iterator[T]) Join(iters ...*Iterator[T]) *Iterator[T] {
	proc := i.Producer()
	for idx := range iters {
		proc = proc.Join(iters[idx].ReadOne)
	}
	return proc.Iterator()
}

// Slice converts an iterator to the slice of it's values, and
// closes the iterator at the when the iterator has been exhausted..
//
// In the case of an error in the underlying iterator the output slice
// will have the values encountered before the error.
func (i *Iterator[T]) Slice(ctx context.Context) (out []T, _ error) {
	return out, i.Observe(func(in T) { out = append(out, in) }).Run(ctx)
}

// Channel proides access to the contents of the iterator as a
// channel. The channel is closed when the iterator is exhausted.
func (i *Iterator[T]) Channel(ctx context.Context) <-chan T { return i.BufferedChannel(ctx, 0) }

// BufferedChannel provides access to the content of the iterator with
// a buffered channel that is closed when the iterator is
// exhausted.
func (i *Iterator[T]) BufferedChannel(ctx context.Context, size int) <-chan T {
	out := Blocking(make(chan T, size))

	out.Processor().
		ReadAll(i.Producer()).
		PostHook(out.Close).
		Operation(i.AddError).
		Launch(ctx)

	return out.Channel()
}

// MarshalJSON is useful for implementing json.Marshaler methods
// from iterator-supporting types. Wrapping the standard library's
// json encoding tools.
//
// The contents of the iterator are marshaled as elements in an JSON
// array.
func (i *Iterator[T]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	_ = buf.WriteByte('[')
	first := true

	// decide to capture a context in the
	// iterator or not care
	ctx := context.TODO()

	for val := range i.Seq(ctx) {
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
func (i *Iterator[T]) UnmarshalJSON(in []byte) error {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return err
	}
	var idx int
	i.operation = i.operation.Join(func(_ context.Context) (out T, err error) {
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

// ProcessParallel produces a worker that, when executed, will
// iteratively processes the contents of the iterator. The options
// control the error handling and parallelism semantics of the
// operation.
//
// This is the work-house operation of the package, and can be used as
// the basis of worker pools, even processing, or message dispatching
// for pubsub queues and related systems.
func (i *Iterator[T]) ProcessParallel(
	fn Processor[T],
	optp ...OptionProvider[*WorkerGroupConf],
) Worker {
	return func(ctx context.Context) (err error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		opts := &WorkerGroupConf{}
		err = JoinOptionProviders(optp...).Apply(opts)
		ft.WhenCall(err != nil, cancel)
		if opts.ErrorHandler == nil {
			opts.ErrorHandler = i.ErrorHandler().Lock()
			opts.ErrorResolver = i.Close
		}

		wg := &WaitGroup{}

		operation := fn.WithRecover().WithErrorFilter(func(err error) error {
			return ft.WhenDo(
				!opts.CanContinueOnError(err),
				ft.Wrapper(io.EOF),
			)
		})

		splits := i.Split(opts.NumWorkers)
		for idx := range splits {
			operation.ReadAll(splits[idx].Producer()).
				Operation(func(err error) { ft.WhenCall(ers.Is(err, io.EOF, ers.ErrCurrentOpAbort), cancel) }).
				Add(ctx, wg)
		}

		wg.Operation().Block()
		return opts.ErrorResolver()
	}
}

// Buffer adds a buffer in the queue using a channel as buffer to
// smooth out iteration performance, if the iteration (producer) and the
// consumer both take time, even a small buffer will improve the
// throughput of the system and prevent both components of the system
// from blocking on eachother.
//
// The ordering of elements in the output iterator is the same as the
// order of elements in the input iterator.
func (i *Iterator[T]) Buffer(n int) *Iterator[T] {
	buf := Blocking(make(chan T, n))
	pipe := buf.Send().Consume(i).Operation(i.ErrorHandler().Lock()).PostHook(buf.Close).Once().Go()
	return buf.Producer().PreHook(pipe).IteratorWithHook(func(si *Iterator[T]) { si.AddError(i.Close()) })
}

// ParallelBuffer, like buffer, process the input queue and stores
// those items in a channel; however, unlike Buffer, multiple workers
// consume the input iterator: as a result the order of the elements
// in the output iterator is not the same as the input order.
//
// Otherwise, the two Buffer methods are equivalent and serve the same
// purpose: process the items from an iterator without blocking the
// consumer of the iterator.
func (i *Iterator[T]) ParallelBuffer(n int) *Iterator[T] {
	buf := Blocking(make(chan T, n))
	pipe := i.ProcessParallel(buf.Processor(), WorkerGroupConfNumWorkers(n)).Operation(i.ErrorHandler().Lock()).PostHook(buf.Close).Once().Go()
	return buf.Producer().PreHook(pipe).IteratorWithHook(func(si *Iterator[T]) { si.AddError(i.Close()) })
}

// Seq converts a fun.Iterator into a native go iterator.
func (i *Iterator[T]) Seq(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			item, err := i.ReadOne(ctx)
			if err != nil || !yield(item) {
				return
			}
		}
	}
}
