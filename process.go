package fun

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/tychoish/fun/internal"
)

// Processor are generic functions that take an argument (and a
// context) and return an error. They're the type of function used by
// the itertool.Process/itertool.ParallelForEach and useful in other
// situations as a compliment to fun.Worker and WaitFunc.
//
// In general the implementations of the methods for processing
// functions are wrappers around their similarly named fun.Worker
// analogues.
type Processor[T any] func(context.Context, T) error

func BlockingProcessor[T any](fn func(T) error) Processor[T] {
	return func(_ context.Context, in T) error { return fn(in) }
}

// Run executes the ProcessFunc but creates a context within the
// function (decended from the context provided in the arguments,)
// that is canceled when Run() returns to avoid leaking well behaved
// resources outside of the scope of the function execution. Run can
// also be passed as a Process func.
func (pf Processor[T]) Run(ctx context.Context, in T) error { return pf.Worker(in).Run(ctx) }

// Block runs the ProcessFunc with a context that will never be
// canceled.
func (pf Processor[T]) Block(in T) error { return pf.Worker(in).Block() }

// Wait converts a processor into a worker that will process the input
// provided when executed.
func (pf Processor[T]) Wait(in T, of Observer[error]) WaitFunc { return pf.Worker(in).Wait(of) }

// Safe runs the producer, converted all panics into errors. Safe is
// itself a processor.
func (pf Processor[T]) Safe(ctx context.Context, in T) error { return pf.Worker(in).Safe(ctx) }

// After produces a Processor that will execute after the provided
// timestamp.
func (pf Processor[T]) After(ts time.Time) Processor[T] { return pf.Delay(time.Until(ts)) }

// Delay returns a process
func (pf Processor[T]) Jitter(jf func() time.Duration) Processor[T] {
	return pf.Delay(internal.Max(0, jf()))
}
func (pf Processor[T]) If(c bool) Processor[T] { return pf.When(Wrapper(c)) }

func (pf Processor[T]) Observer(ctx context.Context, oe Observer[error]) Observer[T] {
	return func(in T) { oe(pf(ctx, in)) }
}

func (pf Processor[T]) Worker(in T) Worker {
	return func(ctx context.Context) error { return pf(ctx, in) }
}

func (pf Processor[T]) Once() Processor[T] {
	once := &sync.Once{}
	var err error
	return func(ctx context.Context, in T) error {
		once.Do(func() { err = pf(ctx, in) })
		return err
	}
}

func (pf Processor[T]) When(c func() bool) Processor[T] {
	return func(ctx context.Context, in T) error {
		if c() {
			return pf(ctx, in)
		}
		return nil
	}
}

func (pf Processor[T]) Delay(dur time.Duration) Processor[T] {
	return func(ctx context.Context, in T) error {
		timer := time.NewTimer(dur)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return pf(ctx, in)
		}
	}
}

var (
	ErrLimitExceeded = errors.New("limit exceeded")
	ErrInvalidInput  = errors.New("invalid input")
)

func (pf Processor[T]) Lock() Processor[T] {
	mtx := &sync.Mutex{}
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

func (pf Processor[T]) TTL(dur time.Duration) Processor[T] {
	resolver := ttlExec[error](dur)

	return func(ctx context.Context, arg T) error {
		return resolver(func() error { return pf(ctx, arg) })
	}
}

func limitExec[T any](in int) func(func() T) T {
	Invariant(in > 0, "limit must be greater than zero;", in)
	var (
		counter int
		output  T
	)
	return func(op func() T) T {
		if counter < in {
			output = op()
			counter++
		}

		return output
	}
}

func ttlExec[T any](dur time.Duration) func(op func() T) T {
	Invariant(dur < 0, "ttl must not be negative;", dur)
	Invariant(dur == math.MaxInt64, "ttl must not be max-int64;", dur)

	if dur == 0 {
		// special case
		return func(op func() T) T { return op() }
	}

	var (
		lastAt time.Time
		output T
	)
	mtx := &sync.Mutex{}

	return func(op func() T) (out T) {
		err := func() error {
			mtx.Lock()
			defer mtx.Unlock()

			since := time.Since(lastAt)
			if lastAt.IsZero() || since <= dur {
				lastAt = time.Now().Add(math.MaxInt64)
				return nil
			}
			return ErrLimitExceeded
		}()

		if err != nil {
			return output
		}

		output = op()
		mtx.Lock()
		defer mtx.Unlock()
		lastAt = time.Now()
		return output
	}
}
