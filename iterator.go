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
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

var ErrIteratorSkip = errors.New("skip-iteration")

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
// iteraction paradim: they combine the Next() and value operations
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

	closed atomic.Bool
	value  T
	err    error

	// the once protects the closer
	closeOnce sync.Once
	closer    context.CancelFunc
}

// Generator creates an iterator that produces new values, using the
// generator function provided. This implementation does not create
// any background go routines, and the iterator will produce values
// until the function returns an error or the Close() method is
// called. Any non-nil error returned by the generator function is
// propagated to the close method, as long as it is not a context
// cancellation error or an io.EOF error.
func Generator[T any](op Producer[T]) *Iterator[T]    { return op.Iterator() }
func VariadicIterator[T any](in ...T) *Iterator[T]    { return SliceIterator(in) }
func ChannelIterator[T any](ch <-chan T) *Iterator[T] { return BlockingReceive(ch).Iterator() }
func SliceIterator[T any](in []T) *Iterator[T] {
	s := in
	var idx int = -1
	return Producer[T](func(ctx context.Context) (out T, _ error) {
		if len(s) <= idx+1 {
			return out, io.EOF
		}
		idx++
		return s[idx], ctx.Err()
	}).Iterator()
}

func MergeIterators[T any](iters ...*Iterator[T]) *Iterator[T] {
	pipe := Blocking(make(chan T))

	init := Operation(func(ctx context.Context) {
		wg := &WaitGroup{}
		wctx, cancel := context.WithCancel(ctx)

		// start a go routine for every iterator, to read from
		// the incoming iterator and push it to the pipe

		send := pipe.Send()
		for idx := range iters {
			send.Consume(iters[idx]).Ignore().Add(wctx, wg)
		}

		wg.Operation().PostHook(func() { cancel(); pipe.Close() }).Go(ctx)
	}).Once()

	return pipe.Receive().Producer().PreHook(init).Iterator()
}

func (i *Iterator[T]) doClose() {
	i.closeOnce.Do(func() { i.closed.Store(true); ft.SafeCall(i.closer) })
}
func (i *Iterator[T]) Close() error                   { i.doClose(); return i.err }
func (i *Iterator[T]) AddError(e error)               { i.err = ers.Join(e, i.err) }
func (i *Iterator[T]) ErrorObserver() Observer[error] { return i.AddError }

func (i *Iterator[T]) Producer() Producer[T] { return i.ReadOne }
func (i *Iterator[T]) Value() T              { return i.value }

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
	if i.operation == nil || i.closed.Load() || ctx.Err() != nil {
		return false
	}

	val, err := i.ReadOne(ctx)
	if err == nil {
		i.value = val
		return true
	}
	return false
}

func (i *Iterator[T]) ReadOne(ctx context.Context) (out T, err error) {
	if i.operation == nil || i.closed.Load() {
		return out, io.EOF
	} else if err = ctx.Err(); err != nil {
		return out, err
	}

	defer func() { ft.WhenCall(err != nil, i.doClose) }()

	out, err = i.operation(ctx)
	switch {
	case err == nil:
		return out, nil
	case errors.Is(err, io.EOF):
		return out, err
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return out, err
	default:
		i.AddError(err)
		return out, io.EOF
	}
}

// Filter passes every item in the iterator and, if the check function
// returns true propogates it to the output iterator.  There is no
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
	return Transform(i, func(in T) (any, error) { return any(in), nil })
}

// Transform processes the input iterator of type I into an output
// iterator of type O. It's implementation uses the Generator, will
// continue producing values as long as the input iterator produces
// values, the context isn't canceled, or
func Transform[I, O any](iter *Iterator[I], op func(in I) (O, error)) *Iterator[O] {
	var zero O
	return Producer[O](func(ctx context.Context) (out O, _ error) {
		for {
			if item, err := iter.ReadOne(ctx); err == nil {
				if out, err = op(item); err == nil {
					return out, nil
				} else if errors.Is(err, ErrIteratorSkip) {
					continue
				}

				return zero, err
			} else if !errors.Is(err, ErrIteratorSkip) {
				return zero, err
			}
		}
	}).Iterator()
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
		Ignore().Launch().Once()

	output := make([]*Iterator[T], num)
	for idx := range output {
		output[idx] = pipe.Producer().PreHook(setup).Iterator()
	}

	return output
}

// Observe processes an iterator calling the observer function for
// every element in the iterator and retruning when the iterator is
// exhausted. Take care to ensure that the Observe function does not
// block.
//
// The error returned captures any panics encountered as an error, as
// well as the output of the Close() operation. Observe will not add a
// context cancelation error to its error, though the observed
// iterator may return one in its close method.
func (i *Iterator[T]) Observe(ctx context.Context, fn Observer[T]) (err error) {
	defer func() { err = ers.Join(i.Close(), err, ers.ParsePanic(recover())) }()
	proc := i.Producer()
	for {
		item, err := proc(ctx)
		switch {
		case err == nil:
			fn(item)
		case errors.Is(err, io.EOF):
			return nil
		default:
			// this is (realistically) only context
			// cancelation errors
			return err
		}
	}
}

func (i *Iterator[T]) Process(ctx context.Context, fn Processor[T]) (err error) {
	defer func() { err = ers.Join(i.Close(), err, ers.ParsePanic(recover())) }()

	for i.Next(ctx) {
		item := i.Value()

		operr := fn(ctx, item)
		switch {
		case operr == nil || errors.Is(operr, ErrIteratorSkip):
			continue
		case errors.Is(operr, io.EOF):
			return nil
		case ers.ContextExpired(operr):
			return operr
		default:
			i.AddError(operr)
		}
	}

	// the close error is added in the defer
	return err
}

func (i *Iterator[T]) Chain(iters ...*Iterator[T]) *Iterator[T] {
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
func (i *Iterator[T]) Slice(ctx context.Context) ([]T, error) {
	out := []T{}
	return out, i.Observe(ctx, func(in T) { out = append(out, in) })
}

func (i *Iterator[T]) Channel(ctx context.Context) <-chan T { return i.BufferedChannel(ctx, 0) }
func (i *Iterator[T]) BufferedChannel(ctx context.Context, size int) <-chan T {
	out := Blocking(make(chan T, size))
	go func() {
		defer out.Close()
		pipe := Pipe(i.Producer(), out.Send().Processor())
		for {
			if !pipe.Check(ctx) {
				return
			}
		}
	}()
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
	_, _ = buf.Write([]byte("["))
	first := true

	// decide to capture a context in the
	// iterator or not care
	ctx := context.TODO()

	for {
		val, err := i.ReadOne(ctx)
		if err != nil {
			break
		}
		if first {
			first = false
		} else {
			_, _ = buf.Write([]byte(","))
		}

		if err := enc.Encode(val); err != nil {
			return nil, err
		}
	}

	_, _ = buf.Write([]byte("]"))

	return buf.Bytes(), nil
}

func (i *Iterator[T]) UnmarshalJSON(in []byte) error {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return err
	}
	var idx int
	i.operation = i.operation.Chain(func(ctx context.Context) (out T, err error) {
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

func (i *Iterator[T]) ProcessParallel(
	ctx context.Context,
	fn Processor[T],
	optp ...OptionProvider[*WorkerGroupOptions],
) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	opts := &WorkerGroupOptions{}
	err = ApplyOptions(opts, optp...)
	ft.WhenCall(err != nil, cancel)

	if opts.ErrorObserver == nil {
		opts.ErrorObserver = i.ErrorObserver().Lock()
		opts.ErrorResolver = i.Close
	}

	wg := &WaitGroup{}

	operation := fn.Safe().FilterErrors(func(err error) error {
		return ft.WhenDo(
			!opts.CanContinueOnError(err),
			ft.Wrapper(io.EOF),
		)
	})

	splits := i.Split(opts.NumWorkers)
	for idx := range splits {
		operation.ReadAll(splits[idx].Producer()).
			Operation(func(err error) { ft.WhenCall(errors.Is(err, io.EOF), cancel) }).
			Add(ctx, wg)
	}

	wg.Operation().Block()
	return opts.ErrorResolver()
}

func (i *Iterator[T]) ProcessFuture(fn Processor[T], optp ...OptionProvider[*WorkerGroupOptions]) Worker {
	return func(ctx context.Context) error { return i.ProcessParallel(ctx, fn, optp...) }
}

func (i *Iterator[T]) Buffer(n int) *Iterator[T] {
	buf := Blocking(make(chan T, n))
	pipe := buf.Send().Consume(i).Operation(i.ErrorObserver()).PostHook(buf.Close).Once().Launch()
	return buf.Producer().PreHook(pipe).IteratorWithHook(func(si *Iterator[T]) { si.AddError(i.Close()) })
}

func (i *Iterator[T]) ParallelBuffer(n int) *Iterator[T] {
	buf := Blocking(make(chan T, n))
	pipe := i.ProcessFuture(buf.Processor(), NumWorkers(n)).Operation(i.ErrorObserver()).PostHook(buf.Close).Once().Launch()
	return buf.Producer().PreHook(pipe).IteratorWithHook(func(si *Iterator[T]) { si.AddError(i.Close()) })

}
