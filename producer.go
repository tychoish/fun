package fun

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

// Producer is a function type that is a failrly common
// constructor. It's signature is used to create iterators, as a
// generator, and functions like a Future.
type Producer[T any] func(context.Context) (T, error)

// MakeFuture constructs a producer that blocks to receive one
// item from the specified channel. Subsequent calls to the producer
// will block/yield additional items from the channel. The producer
// will only return an error if the channel is closed (io.EOF) or the
// context has expired.
func MakeFuture[T any](ch <-chan T) Producer[T] { return BlockingReceive(ch).Read }

// BlockingProducer constructs a producer that wraps a similar
// function that does not take a context.
func BlockingProducer[T any](fn func() (T, error)) Producer[T] {
	return func(context.Context) (T, error) { return fn() }
}

// ConsistentProducer constructs a wrapper around a similar function
// type that does not return an error or take a context. The resulting
// function will never error.
func ConsistentProducer[T any](fn func() T) Producer[T] {
	return func(context.Context) (T, error) { return fn(), nil }
}

func StaticProducer[T any](val T, err error) Producer[T] {
	return func(context.Context) (T, error) { return val, err }
}

func ValueProducer[T any](val T) Producer[T] {
	return func(context.Context) (T, error) { return val, nil }
}

// Run executes the producer with a context hat is cacneled after the
// producer returns. It is, itself a producer.
func (pf Producer[T]) Run(ctx context.Context) (T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return pf(ctx)
}

// Background constructs a worker that runs the provided Producer in a
// background thread and passes the produced value to the observe.
//
// The worker function's return value captures the procuder's error,
// and will block until the producer has completed.
func (pf Producer[T]) Background(ctx context.Context, of Observer[T]) Worker {
	return pf.Worker(of).Future(ctx)
}

// Worker passes the produced value to an observer and returns a
// worker that runs the producer, calls the observer, and returns the
// error.
func (pf Producer[T]) Worker(of Observer[T]) Worker {
	return func(ctx context.Context) error { o, e := pf(ctx); of(o); return e }
}

// Safe returns a wrapped producer with a panic handler that converts
// any panic to an error.
func (pf Producer[T]) Safe() Producer[T] {
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Merge(err, ers.ParsePanic(recover())) }()
		return pf(ctx)
	}
}

func (pf Producer[T]) Ignore(ctx context.Context) { _, _ = pf(ctx) }

// Chain runs both producers, with the following logic: if the root
// producer errors, return that error (and a zero value,) run the next
// producer. If the next producer errors, return the result of the
// first producer and the second producer's error. If neither producer
// errors, pass both results to the join function (jf) and return the
// result.
func (pf Producer[T]) Chain(next Producer[T], jf func(T, T) T) Producer[T] {
	return func(ctx context.Context) (T, error) {
		one, err := pf(ctx)
		if err != nil {
			return ZeroOf[T](), err
		}

		two, err := next(ctx)
		if err != nil {
			return one, err
		}

		return jf(one, two), nil
	}
}

func (pf Producer[T]) Join(next Producer[T]) Producer[T] {
	var (
		firstExhausted  bool
		secondExhausted bool
	)

	// This producer is
	return Producer[T](func(ctx context.Context) (out T, _ error) {
		switch {
		case secondExhausted:
			return out, io.EOF
		case !firstExhausted:
			value, err := pf(ctx)
			switch {
			case err == nil:
				return value, nil
			case errors.Is(err, io.EOF):
				firstExhausted = true
			default:
				firstExhausted = true
				return out, err
			}
		}

		if value, err := next(ctx); err != nil {
			secondExhausted = true
			return out, err
		} else {
			return value, nil
		}
	}).Lock()
}

// Must runs the producer returning the constructed value and panicing
// if the producer errors.
func (pf Producer[T]) Must(ctx context.Context) T { return Must(pf(ctx)) }

// Block runs the producer with a context that will ever expire.
func (pf Producer[T]) Block() (T, error) { return pf(internal.BackgroundContext) }

// Force combines the semantics of Must and Block: runs the producer
// with a context that never expires and panics in the case of an
// error.
func (pf Producer[T]) Force() T { return Must(pf.Block()) }

// Wait produces a wait function, using two observers to handle the
// output of the Producer.
func (pf Producer[T]) Wait(of Observer[T], eo Observer[error]) Operation {
	return func(ctx context.Context) { o, e := pf(ctx); of(o); eo(e) }
}

// Check uses the error observer to consume the error from the
// Producer and returns a function that takes a context and returns a value.
func (pf Producer[T]) Check(ctx context.Context) (T, bool) { o, e := pf(ctx); return o, e == nil }
func (pf Producer[T]) ForceCheck() (T, bool)               { return pf.Check(internal.BackgroundContext) }

// Future runs the producer in the background, when function is
// called, and returns a producer which, when called, blocks until the
// original producer returns.
func (pf Producer[T]) Future(ctx context.Context) Producer[T] {
	out := make(chan T, 1)
	var err error
	go func() { defer close(out); o, e := pf.Safe()(ctx); err = e; out <- o }()

	return func(ctx context.Context) (T, error) {
		out, chErr := Blocking(out).Receive().Read(ctx)
		err = ers.Merge(err, chErr)
		return out, err
	}
}

// Once returns a producer that only executes ones, and caches the
// return values, so that subsequent calls to the output producer will
// return the same values.
func (pf Producer[T]) Once() Producer[T] {
	var (
		out T
		err error
	)

	once := &sync.Once{}

	return func(ctx context.Context) (T, error) {
		once.Do(func() { out, err = pf(ctx) })
		return out, err
	}
}

// Iterator creates an iterator that calls the Producer function once
// for every iteration, until it errors. Errors that are not context
// cancellation errors or io.EOF are propgated to the iterators Close
// method.
func (pf Producer[T]) Iterator() *Iterator[T] {
	op, cancel := pf.WithCancel()
	return &Iterator[T]{operation: op, closer: cancel}
}

func (pf Producer[T]) IteratorWithHook(hook func(*Iterator[T])) *Iterator[T] {
	once := &sync.Once{}
	op, cancel := pf.WithCancel()
	iter := &Iterator[T]{operation: op}
	iter.closer = func() { once.Do(func() { hook(iter) }); cancel() }
	return iter
}

// If returns a producer that will execute the root producer only if
// the cond value is true. Otherwise, If will return the zero value
// for T and a nil error.
func (pf Producer[T]) If(cond bool) Producer[T] { return pf.When(Wrapper(cond)) }

// After will return a Producer that will block until the provided
// time is in the past, and then execute normally.
func (pf Producer[T]) After(ts time.Time) Producer[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Producer in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Producer[T]) Delay(d time.Duration) Producer[T] { return pf.Jitter(Wrapper(d)) }

// Jitter wraps a Producer that runs the jitter function (jf) once
// before every execution of the resulting fucntion, and waits for the
// resulting duration before running the Producer.
//
// If the function produces a negative duration, there is no delay.
func (pf Producer[T]) Jitter(jf func() time.Duration) Producer[T] {
	return func(ctx context.Context) (out T, _ error) {
		timer := time.NewTimer(internal.Max(0, jf()))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-timer.C:
			return pf(ctx)
		}
	}
}

// When constructs a producer that will call the cond upon every
// execution, and when true, will run and return the results of the
// root producer. Otherwise When will return the zero value of T and a
// nil error.
func (pf Producer[T]) When(cond func() bool) Producer[T] {
	return func(ctx context.Context) (out T, _ error) {
		if cond() {
			return pf(ctx)

		}
		return out, nil
	}
}

func (pf Producer[T]) WithLock(mtx *sync.Mutex) Producer[T] {
	return func(ctx context.Context) (T, error) {
		mtx.Lock()
		defer mtx.Unlock()
		return pf(ctx)
	}
}

// Lock creates a producer that runs the root mutex as per normal, but
// under the protection of a mutex so that there's only one execution
// of the producer at a time.
func (pf Producer[T]) Lock() Producer[T] { return pf.WithLock(&sync.Mutex{}) }

// WithCancel creates a Producer and a cancel function which will
// terminate the context that the root Producer is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Producer is canceled.)
func (pf Producer[T]) WithCancel() (Producer[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context) (out T, _ error) {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		if err := wctx.Err(); err != nil {
			return out, err
		}
		return pf(ctx)
	}, func() { once.Do(func() {}); WhenCall(cancel != nil, cancel) }
}

// Limit runs the producer a specified number of times, and caches the
// result of the last execution and returns that value for any
// subsequent executions.
func (pf Producer[T]) Limit(in int) Producer[T] {
	resolver := limitExec[tuple[T, error]](in)

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

// TTL runs the producer only one time per specified interval. The
// interval must me greater than 0.
func (pf Producer[T]) TTL(dur time.Duration) Producer[T] {
	resolver := ttlExec[tuple[T, error]](dur)

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

func (pf Producer[T]) PreHook(op func(context.Context)) Producer[T] {
	return func(ctx context.Context) (T, error) { op(ctx); return pf(ctx) }

}
func (pf Producer[T]) PostHook(op func()) Producer[T] {
	return func(ctx context.Context) (o T, e error) {
		o, e = pf(ctx)
		e = ers.Merge(ers.Check(op), e)
		return
	}
}

func (pf Producer[T]) WithoutErrors(errs ...error) Producer[T] {
	return func(ctx context.Context) (_ T, err error) {
		defer func() { err = ers.Filter(err) }()
		return pf(ctx)
	}
}
