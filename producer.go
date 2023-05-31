package fun

import (
	"context"
	"sync"
	"time"

	"github.com/tychoish/fun/internal"
)

type Producer[T any] func(context.Context) (T, error)

func MakeFuture[T any](ch <-chan T) Producer[T] {
	return func(ctx context.Context) (T, error) {
		return BlockingReceive(ch).Read(ctx)
	}
}

func BlockingProducer[T any](fn func() (T, error)) Producer[T] {
	return func(context.Context) (T, error) { return fn() }
}

func ConsistentProducer[T any](fn func() T) Producer[T] {
	return func(context.Context) (T, error) { return fn(), nil }
}

func (pf Producer[T]) Run(ctx context.Context) (T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	return pf(ctx)
}

func (pf Producer[T]) Background(ctx context.Context, of Observer[T]) Worker {
	return pf.Worker(of).Future(ctx)
}

func (pf Producer[T]) Worker(of Observer[T]) Worker {
	return func(ctx context.Context) error { o, e := pf(ctx); of(o); return e }
}

func (pf Producer[T]) Safe(ctx context.Context) (_ T, err error) {
	defer func() { err = mergeWithRecover(err, recover()) }()
	return pf(ctx)
}

func (pf Producer[T]) Must(ctx context.Context) T { return Must(pf(ctx)) }

func (pf Producer[T]) Force() T { return Must(pf.Block()) }

func (pf Producer[T]) Block() (T, error) { return pf(internal.BackgroundContext) }

func (pf Producer[T]) Wait(of Observer[T], eo Observer[error]) WaitFunc {
	return func(ctx context.Context) { o, e := pf(ctx); of(o); eo(e) }
}

func (pf Producer[T]) Check(of Observer[error]) func(context.Context) T {
	return func(ctx context.Context) T { o, e := pf(ctx); of(e); return o }
}

func (pf Producer[T]) Future(ctx context.Context) Producer[T] {
	out := make(chan T, 1)
	spf := Producer[T](pf.Safe)
	var err error
	go func() { defer close(out); o, e := spf(ctx); err = e; out <- o }()

	return func(ctx context.Context) (T, error) {
		out, chErr := Blocking(out).Receive().Read(ctx)
		err = internal.MergeErrors(err, chErr)
		return out, err
	}
}

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

func (pf Producer[T]) Generator() Iterator[T]         { return Generator(pf) }
func (pf Producer[T]) If(c bool) Producer[T]          { return pf.When(Wrapper(c)) }
func (pf Producer[T]) After(ts time.Time) Producer[T] { return pf.Delay(time.Until(ts)) }
func (pf Producer[T]) Jitter(jf func() time.Duration) Producer[T] {
	return pf.Delay(internal.Max(0, jf()))
}

func (pf Producer[T]) When(c func() bool) Producer[T] {
	return func(ctx context.Context) (out T, _ error) {
		if c() {
			return pf(ctx)

		}
		return out, nil
	}
}

func (pf Producer[T]) Delay(d time.Duration) Producer[T] {
	return func(ctx context.Context) (out T, _ error) {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-timer.C:
			return pf(ctx)
		}
	}
}

func (pf Producer[T]) Lock() Producer[T] {
	mtx := &sync.Mutex{}
	return func(ctx context.Context) (T, error) {
		mtx.Lock()
		defer mtx.Unlock()
		return pf(ctx)
	}
}

type tuple[T, U any] struct {
	One T
	Two U
}

func (pf Producer[T]) Limit(in int) Worker {
	resolver := limitExec[tuple[T, error]](in)

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}

func (pf Producer[T]) TTL(dur time.Duration) Worker {
	resolver := ttlExec[tuple[T, error]](dur)

	return func(ctx context.Context) (T, error) {
		out := resolver(func() (val tuple[T, error]) {
			val.One, val.Two = pf(ctx)
			return
		})
		return out.One, out.Two
	}
}
