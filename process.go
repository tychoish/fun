package fun

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Processor are generic functions that take an argument (and a
// context) and return an error. They're the type of function used by
// the itertool.Process/itertool.ParallelForEach and useful in other
// situations as a compliment to fun.Worker and Operation.
//
// In general the implementations of the methods for processing
// functions are wrappers around their similarly named fun.Worker
// analogues.
type Processor[T any] func(context.Context, T) error

// BlockingProcessor converts a function with the Processor signature
// (minus the context, and adds a noop context,) for easy conversion.
func BlockingProcessor[T any](fn func(T) error) Processor[T] {
	return func(_ context.Context, in T) error { return fn(in) }
}

// Run executes the ProcessFunc but creates a context within the
// function (decended from the context provided in the arguments,)
// that is canceled when Run() returns to avoid leaking well behaved
// resources outside of the scope of the function execution. Run can
// also be passed as a Processor func.
func (pf Processor[T]) Run(ctx context.Context, in T) error { return pf.Worker(in).Run(ctx) }

// Block runs the ProcessFunc with a context that will never be
// canceled.
func (pf Processor[T]) Block(in T) error { return pf.Worker(in).Block() }

// Ignore runs the process function and discards the error.
func (pf Processor[T]) Ignore(ctx context.Context, in T) { _ = pf(ctx, in) }

func (pf Processor[T]) Check(ctx context.Context, in T) bool { return pf(ctx, in) == nil }

func (pf Processor[T]) Force(in T) { pf.Worker(in).Ignore().Block() }

// Operation converts a processor into a worker that will process the input
// provided when executed.
func (pf Processor[T]) Operation(in T, of Observer[error]) Operation {
	return pf.Worker(in).Operation(of)
}

// Safe runs the producer, converted all panics into errors. Safe is
// itself a processor.
func (pf Processor[T]) Safe() Processor[T] {
	return func(ctx context.Context, in T) error { return pf.Worker(in).Safe()(ctx) }
}

// After produces a Processor that will execute after the provided
// timestamp.
func (pf Processor[T]) After(ts time.Time) Processor[T] { return pf.Delay(time.Until(ts)) }

// Delay wraps a Processor in a function that will always wait for the
// specified duration before running.
//
// If the value is negative, then there is always zero delay.
func (pf Processor[T]) Delay(dur time.Duration) Processor[T] { return pf.Jitter(ft.Wrapper(dur)) }

// Jitter wraps a Processor that runs the jitter function (jf) once
// before every execution of the resulting fucntion, and waits for the
// resulting duration before running the processor.
//
// If the function produces a negative duration, there is no delay.
func (pf Processor[T]) Jitter(jf func() time.Duration) Processor[T] {
	return func(ctx context.Context, in T) error {
		timer := time.NewTimer(internal.Max(0, jf()))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return pf(ctx, in)
		}
	}
}

// If runs the processor function if, and only if the condition is
// true. Otherwise the function does not run and the processor returns
// nil.
//
// The resulting processor can be used more than once.
func (pf Processor[T]) If(c bool) Processor[T] { return pf.When(ft.Wrapper(c)) }

// When returns a processor function that runs if the conditional function
// returns true, and does not run otherwise. The conditional function
// is evaluated every time the returned processor is run.
func (pf Processor[T]) When(c func() bool) Processor[T] {
	return func(ctx context.Context, in T) error {
		if c() {
			return pf(ctx, in)
		}
		return nil
	}
}

// Join wraps the processor and executes both the root processor and
// then the next processor, assuming the root processor is not
// canceled and the context has not expired.
func (pf Processor[T]) Join(next Processor[T]) Processor[T] {
	return func(ctx context.Context, in T) error {
		if err := pf(ctx, in); err != nil {
			return err
		}
		return next.If(ctx.Err() == nil)(ctx, in)
	}
}

// Iterator creates a worker function that processes the iterator with
// the processor function, merging/collecting all errors, and
// respecting the worker's context. The processing does not begin
// until the worker is called.
func (pf Processor[T]) Iterator(iter *Iterator[T]) Worker {
	return func(ctx context.Context) error {
		return iter.Process(ctx, pf)
	}
}

// Observer converts a processor into an observer, handling the error
// with the error observer and using the provided context.
func (pf Processor[T]) Observer(ctx context.Context, oe Observer[error]) Observer[T] {
	return func(in T) { oe(pf(ctx, in)) }
}

// Worker converts the processor into a worker, passing the provide
// input into the root processor function. The Processor is not run
// until the worker is called.
func (pf Processor[T]) Worker(in T) Worker {
	return func(ctx context.Context) error { return pf(ctx, in) }
}

// Future begins processing the input immediately and returns a worker
// function that returns the processor's error, and will block until
// the processor returns.
func (pf Processor[T]) Future(ctx context.Context, in T) Worker {
	return pf.Worker(in).Future(ctx)
}

// Once make a processor that can only run once. Subsequent calls to
// the processor return the cached error of the original run.
func (pf Processor[T]) Once() Processor[T] {
	once := &sync.Once{}
	var err error
	return func(ctx context.Context, in T) error {
		once.Do(func() { err = pf(ctx, in) })
		return err
	}
}

// Lock wraps the Processor and protects its execution with a mutex.
func (pf Processor[T]) Lock() Processor[T] { return pf.WithLock(&sync.Mutex{}) }

func (pf Processor[T]) WithLock(mtx *sync.Mutex) Processor[T] {
	return func(ctx context.Context, arg T) error {
		mtx.Lock()
		defer mtx.Unlock()
		return pf(ctx, arg)
	}
}

func (pf Processor[T]) Limit(in int) Processor[T] {
	resolver := limitExec[error](in)

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

// TTL returns a Processor that runs once in the specified window, and
// returns the error from the last run in between this interval. While
// the executions of the underlying function happen in isolation,
// in between, the processor is concurrently accessible.
func (pf Processor[T]) TTL(dur time.Duration) Processor[T] {
	resolver := ttlExec[error](dur)

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

// WithCancel creates a Processor and a cancel function which will
// terminate the context that the root proccessor is running
// with. This context isn't canceled *unless* the cancel function is
// called (or the context passed to the Processor is canceled.)
func (pf Processor[T]) WithCancel() (Processor[T], context.CancelFunc) {
	var wctx context.Context
	var cancel context.CancelFunc
	once := &sync.Once{}

	return func(ctx context.Context, in T) error {
		once.Do(func() { wctx, cancel = context.WithCancel(ctx) })
		Invariant.IsFalse(wctx == nil, "must start the operation before calling cancel")
		return pf(wctx, in)
	}, func() { once.Do(func() {}); ft.SafeCall(cancel) }
}

func (pf Processor[T]) PreHook(op Operation) Processor[T] {
	return func(ctx context.Context, in T) error {
		return ers.Join(ers.Check(func() { op(ctx) }), pf(ctx, in))
	}
}

func (pf Processor[T]) PostHook(op func()) Processor[T] {
	return func(ctx context.Context, in T) error {
		return ers.Join(ft.Flip(pf(ctx, in), ers.Check(op)))
	}
}

func (pf Processor[T]) FilterErrors(ef ers.Filter) Processor[T] {
	return func(ctx context.Context, in T) error { return ef(pf(ctx, in)) }
}

func (pf Processor[T]) WithoutErrors(errs ...error) Processor[T] {
	return pf.FilterErrors(ers.FilterRemove(errs...))
}

func (pf Processor[T]) ReadOne(prod Producer[T]) Worker { return Pipe(prod, pf) }

func (pf Processor[T]) ReadAll(prod Producer[T]) Worker {
	return func(ctx context.Context) error {
		for {
			out, err := prod(ctx)
			switch {
			case err == nil:
				if err := pf(ctx, out); err != nil {
					return err
				}
			case errors.Is(err, ErrIteratorSkip):
				continue
			default:
				return nil
			}
		}
	}
}

////////////////////////////////////////////////////////////////////////

type tuple[T, U any] struct {
	One T
	Two U
}

func limitExec[T any](in int) func(func() T) T {
	Invariant.IsTrue(in > 0, "limit must be greater than zero;", in)
	counter := &atomic.Int64{}
	mtx := &sync.Mutex{}

	var output T
	return func(op func() T) T {
		if counter.CompareAndSwap(int64(in), int64(in)) {
			return output
		}

		mtx.Lock()
		defer mtx.Unlock()
		num := counter.Load()

		if num < int64(in) {
			output = op()
			counter.CompareAndSwap(num, internal.Min(int64(in), num+1))
		}

		return output
	}
}

func ttlExec[T any](dur time.Duration) func(op func() T) T {
	Invariant.IsTrue(dur >= 0, "ttl must not be negative;", dur)

	if dur == 0 {
		return func(op func() T) T { return op() }
	}

	var (
		lastAt time.Time
		output T
	)
	mtx := &sync.RWMutex{}

	return func(op func() T) (out T) {
		if out, ok := func() (T, bool) {
			mtx.RLock()
			defer mtx.RUnlock()
			return output, !lastAt.IsZero() && time.Since(lastAt) < dur
		}(); ok {
			return out
		}

		mtx.Lock()
		defer mtx.Unlock()

		since := time.Since(lastAt)
		if lastAt.IsZero() || since >= dur {
			output = op()
			lastAt = time.Now()
		}

		return output
	}
}
