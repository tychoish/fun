// Package fun is a zero-dependency collection of tools and idoms that
// takes advantage of generics. Iterators, error handling, a
// native-feeling Set type, and a simple pub-sub framework for
// distributing messages in fan-out patterns.
package fun

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/internal"
)

// Iterable provides a safe, context-respecting iterator paradigm for
// iterable objects, along with a set of consumer functions and basic
// implementations.
//
// The itertool package provides a number of tools and paradigms for
// creating and processing Iterable objects, including Generators, Map
// and Reduce as well as Split and Merge to combine or divide
// iterators.
//
// In general, Iterators cannot be safe for access from multiple
// concurrent goroutines, because it is impossible to synchronize
// calls to Next() and Value(); however, itertool.Range() and
// itertool.Split() provide support for these workloads.
type Iterable[T any] interface {
	Next(context.Context) bool
	Close() error
	Value() T
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
	defer func() { err = mergeWithRecover(i.Close(), recover()) }()
	proc := i.Producer()
	for {
		item, ok := proc.Check(ctx)
		if !ok {
			return nil // channel closed or complete.
		}

		fn(item)
	}
}

// IterateOne, like ReadOne reads one value from the iterator, and
// returns it. The error values are either a context cancelation error
// if the context is canceled, or io.EOF if there are no elements in
// the iterator. The semantics of fun.IterateOne and fun.ReadOne are
// the same.
//
// IterateOne does not provide atomic exclusion if multiple calls to
// the iterator or IterateOne happen concurrently; however, the
// adt.NewIterator wrapper, and most of the iterator implementations
// provided by the fun package, provide a special case which *does*
// allow for safe and atomic concurrent use with fun.IterateOne.
func IterateOne[T any](ctx context.Context, iter Iterable[T]) (T, error) {
	if si, ok := iter.(interface {
		ReadOne(ctx context.Context) (T, error)
	}); ok {
		out, err := si.ReadOne(ctx)
		return out, err
	}

	if iter.Next(ctx) {
		return iter.Value(), nil
	}

	if e := ctx.Err(); e != nil {
		return ZeroOf[T](), e
	}

	return ZeroOf[T](), io.EOF
}

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
func Generator[T any](op Producer[T]) *Iterator[T] { return op.Generator() }

// MapIterator converts a map into an iterator of fun.Pair objects. The
// iterator is panic-safe, and uses one go routine to track the
// progress through the map. As a result you should always, either
// exhaust the iterator, cancel the context that you pass to the
// iterator OR call iterator.Close().
//
// To use this iterator the items in the map are not copied, and the
// iteration order is randomized following the convention in go.
//
// Use in combination with other iterator processing tools
// (generators, observers, transformers, etc.) to limit the number of
// times a collection of data must be coppied.
func MapIterator[K comparable, V any](in map[K]V) Iterable[Pair[K, V]] { return Mapify(in).Iterator() }
func SliceIterator[T any](in []T) *Iterator[T]                         { return Sliceify(in).Iterator() }
func VariadicIterator[T any](in ...T) *Iterator[T]                     { return SliceIterator(in) }
func FromIterable[T any](in Iterable[T]) *Iterator[T]                  { return IterableProducer(in).Generator() }
func ChannelIterator[T any](ch <-chan T) *Iterator[T]                  { return BlockingReceive(ch).Iterator() }

func (i *Iterator[T]) doClose()         { i.closeOnce.Do(func() { i.closed.Store(true); i.closer() }) }
func (i *Iterator[T]) Close() error     { i.doClose(); return i.err }
func (i *Iterator[T]) AddError(e error) { i.err = internal.MergeErrors(e, i.err) }

func (i *Iterator[T]) ErrorObserver() Observer[error] { return i.AddError }
func (i *Iterator[T]) Producer() Producer[T]          { return i.ReadOne }
func (i *Iterator[T]) Value() T                       { return i.value }

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
	if i.operation == nil || i.closed.Load() || ctx.Err() != nil {
		return out, io.EOF
	}

	defer func() { WhenCall(err != nil, i.doClose) }()

	out, err = i.operation(ctx)
	switch {
	case err == nil:
		return out, nil
	case internal.IsTerminatingError(err):
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
	return Producer[T](func(ctx context.Context) (T, error) {
		for {
			item, err := i.ReadOne(ctx)
			if err != nil {
				return ZeroOf[T](), err
			}

			if check(item) {
				return item, nil
			}
		}
	}).Generator()
}

// Any, as a special case of Transform converts an iterator of any
// type and converts it to an iterator of any (e.g. interface{})
// values.
func (i *Iterator[T]) Any() *Iterator[any] {
	return Producer[any](func(ctx context.Context) (any, error) {
		item, err := i.ReadOne(ctx)
		if err != nil {
			return nil, err
		}

		return any(item), nil
	}).Generator()
}

// Transform processes the input iterator of type I into an output
// iterator of type O. It's implementation uses the Generator, will
// continue producing values as long as the input iterator produces
// values, the context isn't canceled, or
func Transform[I, O any](iter *Iterator[I], op func(in I) (O, error)) *Iterator[O] {
	return Generator(func(ctx context.Context) (O, error) {
		item, err := iter.ReadOne(ctx)
		if err != nil {
			return ZeroOf[O](), err
		}

		return op(item)
	})
}

// Count returns the number of items observed by the iterator. Callers
// should still manually call Close on the iterator.
func (i *Iterator[T]) Count(ctx context.Context) int {
	proc := i.Producer()
	var count int
	for {
		if !IsOk(proc.Check(ctx)) {
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

	pipe := make(chan T)
	setup := WaitFunc(func(ctx context.Context) {
		defer close(pipe)
		proc := i.Producer()
		for {
			if value, ok := proc.Check(ctx); !ok || !Blocking(pipe).Send().Check(ctx, value) {
				return
			}
		}
	}).Launch().Once()

	output := make([]*Iterator[T], num)

	for idx := range output {
		output[idx] = Producer[T](func(ctx context.Context) (T, error) {
			setup(ctx)
			return Blocking(pipe).Receive().Read(ctx)
		}).Generator()

	}

	return output
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
		send := out.Send()
		proc := i.Producer()
		for {
			if item, ok := proc.Check(ctx); !ok || !send.Check(ctx, item) {
				return
			}

		}
	}()
	return out.Channel()
}
