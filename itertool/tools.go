package itertool

import (
	"context"
	"fmt"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// CollectChannel converts and iterator to a channel.
func CollectChannel[T any](ctx context.Context, iter fun.Iterator[T]) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for iter.Next(ctx) {
			select {
			case <-ctx.Done():
				return
			case out <- iter.Value():
				continue
			}
		}
	}()
	return out
}

// CollectSlice converts an iterator to the slice of it's values.
//
// In the case of an error in the underlying iterator the output slice
// will have the values encountered before the error.
func CollectSlice[T any](ctx context.Context, iter fun.Iterator[T]) ([]T, error) {
	out := []T{}
	err := ForEach(ctx, iter, func(_ context.Context, in T) error { out = append(out, in); return nil })
	return out, err
}

// ForEach passes each item in the iterator through the specified
// handler function, return an error if the handler function errors.
func ForEach[T any](ctx context.Context, iter fun.Iterator[T], fn func(context.Context, T) error) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	catcher := &fun.ErrorCollector{}

	defer func() { err = catcher.Resolve() }()
	defer catcher.Recover()
	defer catcher.CheckCtx(ctx, iter.Close)

	for iter.Next(ctx) {
		if ferr := fn(ctx, iter.Value()); ferr != nil {
			catcher.Add(ferr)
			break
		}
	}

	return
}

// Filter passes all objects in an iterator through the
// specified filter function. If the filter function errors, the
// operation aborts and the error is reported by the returned
// iterator's Close method. If the include boolean is true the result
// of the function is included in the output iterator, otherwise the
// operation is skipped.
//
// The output iterator is produced iteratively as the returned
// iterator is consumed.
func Filter[T any](
	ctx context.Context,
	iter fun.Iterator[T],
	fn func(ctx context.Context, input T) (output T, include bool, err error),
) fun.Iterator[T] {
	out := new(internal.MapIterImpl[T])
	pipe := make(chan T)
	out.Pipe = pipe

	var iterCtx context.Context
	iterCtx, out.Closer = context.WithCancel(ctx)

	out.WG.Add(1)
	go func() {
		catcher := &fun.ErrorCollector{}
		defer out.WG.Done()
		defer func() { out.Error = catcher.Resolve() }()
		defer catcher.Recover()
		defer catcher.CheckCtx(ctx, iter.Close)
		defer close(pipe)

		for iter.Next(iterCtx) {
			o, include, err := fn(iterCtx, iter.Value())
			if err != nil {
				catcher.Add(err)
				break
			}
			if !include {
				continue
			}
			select {
			case <-iterCtx.Done():
			case pipe <- o:
			}
		}
	}()
	return out
}

// MapOptions describes the runtime options to the IteratorMap
// operation. The zero value of this struct provides a usable strict operation.
type MapOptions struct {
	// ContinueOnPanic forces the entire IteratorMap operation to
	// halt when a single map function panics. All panics are
	// converted to errors and propagated to the output iterator's
	// Close() method.
	ContinueOnPanic bool
	// ContinueOnError allows a map function to return an error
	// and allow the work of the IteratorMap operation to
	// continue. Errors are aggregated propagated to the output
	// iterator's Close() method.
	ContinueOnError bool
	// NumWorkers describes the number of parallel workers
	// processing the incoming iterator items and running the map
	// function. All values less than 1 are converted to 1. Any
	// value greater than 1 will result in out-of-sequence results
	// in the output iterator.
	NumWorkers int
}

// Map provides an orthodox functional map implementation
// based around fun.Iterator. Operates in asynchronous/streaming
// manner, so that the output Iterator must be consumed. The zero
// values of IteratorMapOptions provide reasonable defaults for
// abort-on-error and single-threaded map operation.
//
// If the mapper function errors, the result isn't included, but the
// errors would be aggregated and propagated to the `Close()` method
// of the resulting iterator. If there are more than one error (as is
// the case with a panic or with ContinueOnError semantics,) the error
// is an *ErrorStack object. Panics in the map function are converted
// to errors and handled according to the ContinueOnPanic option.
func Map[T any, O any](
	ctx context.Context,
	opts MapOptions,
	iter fun.Iterator[T],
	mapper func(context.Context, T) (O, error),
) fun.Iterator[O] {
	out := new(internal.MapIterImpl[O])
	safeOut := Synchronize[O](out)
	toOutput := make(chan O)
	catcher := &fun.ErrorCollector{}
	out.Pipe = toOutput

	abortCtx, abort := context.WithCancel(ctx)
	var iterCtx context.Context
	iterCtx, out.Closer = context.WithCancel(abortCtx)

	if opts.NumWorkers <= 0 {
		opts.NumWorkers = 1
	}

	fromInput := make(chan T, opts.NumWorkers)

	out.WG.Add(1)
	signal := make(chan struct{})
	go func() {
		defer close(signal)
		defer out.WG.Done()
		defer catcher.Recover()
		defer catcher.CheckCtx(ctx, iter.Close)
		defer close(fromInput)
		defer catcher.Recover()

		for iter.Next(abortCtx) {
			select {
			case <-abortCtx.Done():
				return
			case fromInput <- iter.Value():
				continue
			}
		}
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < opts.NumWorkers; i++ {
		wg.Add(1)
		go mapWorker(iterCtx, catcher, wg, opts, mapper, abort, fromInput, toOutput)
	}

	out.WG.Add(1)
	go func() {
		defer out.WG.Done()
		defer func() { out.Error = catcher.Resolve() }()
		defer catcher.Recover()
		<-signal
		fun.Wait(ctx, wg)
		abort()
		close(toOutput)
	}()

	return safeOut
}

func mapWorker[T any, O any](
	ctx context.Context,
	catcher *fun.ErrorCollector,
	wg *sync.WaitGroup,
	opts MapOptions,
	mapper func(context.Context, T) (O, error),
	abort func(),
	fromInput <-chan T,
	toOutput chan<- O,
) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			catcher.Add(fmt.Errorf("panic: %v", r))
			if opts.ContinueOnPanic {
				wg.Add(1)
				go mapWorker(ctx, catcher, wg, opts, mapper, abort, fromInput, toOutput)
				return
			}
			abort()
			return
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case value, ok := <-fromInput:
			if !ok {
				return
			}
			o, err := mapper(ctx, value)
			if err != nil {
				catcher.Add(err)
				if opts.ContinueOnError {
					continue
				}
				abort()
				return
			}
			select {
			case <-ctx.Done():
				return
			case toOutput <- o:
			}
		}
	}

}

// Reduce processes an input iterator with a reduce function and
// outputs the final value. The initial value may be a zero or nil
// value.
func Reduce[T any, O any](
	ctx context.Context,
	iter fun.Iterator[T],
	reducer func(context.Context, T, O) (O, error),
	initalValue O,
) (value O, err error) {
	value = initalValue
	catcher := &fun.ErrorCollector{}

	defer func() { err = catcher.Resolve() }()
	defer catcher.Recover()
	defer catcher.CheckCtx(ctx, iter.Close)

	for iter.Next(ctx) {
		value, err = reducer(ctx, iter.Value(), value)
		if err != nil {
			catcher.Add(err)
			return
		}
	}

	return
}
