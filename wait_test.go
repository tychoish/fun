package fun

import (
	"context"
	"errors"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestWait(t *testing.T) {
	t.Parallel()
	t.Run("WaitGroupEndToEnd", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := &WaitGroup{}

		wg.Add(1)
		check.Equal(t, wg.Num(), 1)

		cancel()
		start := time.Now()
		wg.Wait(ctx)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "canceled cotnext")
		}

		start = time.Now()
		ctxTwo, cancelTwo := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelTwo()
		wg.Wait(ctxTwo)

		if time.Since(start) < 100*time.Millisecond {
			t.Fatal("should have returned after a wait", "timeout", time.Since(start))
		}
		wg.Done()
		check.Equal(t, wg.Num(), 0)
		check.True(t, wg.IsDone())

		start = time.Now()
		ctxThree, cancelThree := context.WithTimeout(context.Background(), time.Second)
		defer cancelThree()
		wg.Wait(ctxThree)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "no pending work")
		}

		wg = &WaitGroup{}
		start = time.Now()
		ctxFour, cancelFour := context.WithTimeout(context.Background(), time.Second)
		defer cancelFour()
		wg.Add(1)
		go func() { time.Sleep(10 * time.Millisecond); wg.Done() }()
		wg.Wait(ctxFour)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 20*time.Millisecond {
			t.Fatal("should have returned after completion", "delayed completion", time.Since(start))
		}
	})
	t.Run("SyncWaitGroupEndToEnd", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		wg.Add(1)
		cancel()
		start := time.Now()
		WaitForGroup(wg)(ctx)
		dur := time.Since(start)
		if dur > time.Millisecond {
			t.Fatal("should have returned instantly", "canceled cotnext", dur)
		}

		start = time.Now()
		ctxTwo, cancelTwo := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelTwo()
		WaitForGroup(wg)(ctxTwo)
		if time.Since(start) < 100*time.Millisecond {
			t.Fatal("should have returned after a wait", "timeout", time.Since(start))
		}
		wg.Done()

		start = time.Now()
		ctxThree, cancelThree := context.WithTimeout(context.Background(), time.Second)
		defer cancelThree()
		WaitForGroup(wg)(ctxThree)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "no pending work")
		}

		wg = &sync.WaitGroup{}
		start = time.Now()
		ctxFour, cancelFour := context.WithTimeout(context.Background(), time.Second)
		defer cancelFour()
		wg.Add(1)
		go func() { time.Sleep(10 * time.Millisecond); wg.Done() }()
		WaitForGroup(wg)(ctxFour)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 20*time.Millisecond {
			t.Fatal("should have returned after completion", "delayed completion", time.Since(start))
		}
	})
	t.Run("WaitObserveAll", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan int, 2)
		ch <- 42
		ch <- 42
		close(ch)
		var sum int

		WaitObserveAll(func(in int) { sum += in }, ch)(ctx)
		if sum != 84 {
			t.Error("unexpected total", sum)
		}
	})
	t.Run("WaitObserveOne", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan int, 2)
		ch <- 42
		ch <- 42
		close(ch)
		var sum int

		WaitObserve(func(in int) { sum += in }, ch)(ctx)
		if sum != 42 {
			t.Error("unexpected total", sum)
		}
	})
	t.Run("WaitContext", func(t *testing.T) {
		t.Run("BaseBlocking", func(t *testing.T) {
			bctx := context.Background()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			WaitContext(bctx)(ctx)
			if time.Since(start) > time.Millisecond {
				t.Error("waited too long")
			}
		})
		t.Run("WaitBlocking", func(t *testing.T) {
			wctx := context.Background()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			WaitContext(ctx)(wctx)
			if time.Since(start) > time.Millisecond {
				t.Error("waited too long")
			}
		})
	})
	t.Run("Block", func(t *testing.T) {
		wf := WaitFunc(func(ctx context.Context) {
			if ctx == context.Background() {
				// block runs through wait, so that
				//  any threads spawned in the
				//  WaitFunc are cleaned up when the
				//  main wait function returns.
				t.Error("background context not expected")
			}
			time.Sleep(10 * time.Millisecond)
		})
		start := time.Now()
		wf.Block()
		if time.Since(start) < 10*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("Merge", func(t *testing.T) {
		wfs := make([]WaitFunc, 100)
		for i := 0; i < 100; i++ {
			wfs[i] = func(context.Context) { time.Sleep(5 * time.Millisecond) }
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		start := time.Now()
		WaitMerge(ctx, internal.NewSliceIter(wfs))(ctx)
		dur := time.Since(start)
		if dur > 10*time.Millisecond || dur < 5*time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("ObserveWait", func(t *testing.T) {
		ops := make([]int, 100)
		for i := 0; i < len(ops); i++ {
			ops[i] = rand.Int()
		}
		seen := make(map[int]struct{})
		counter := 0
		wf := ObserveWait[int](internal.NewSliceIter(ops), func(in int) {
			seen[in] = struct{}{}
			counter++
		})
		if len(seen) != 0 || counter != 0 {
			t.Error("should be lazy execution", counter, seen)
		}

		wf(testt.Context(t))

		if len(seen) != 100 {
			t.Error(len(seen), seen)
		}
		if counter != 100 {
			t.Error(counter)
		}
	})

	t.Run("Blocking", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			start := time.Now()
			WaitBlocking(func() { time.Sleep(10 * time.Millisecond) }).Block()
			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Second {
				t.Error(time.Since(start))
			}
		})
		t.Run("Context", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			start := time.Now()
			WaitBlocking(func() { time.Sleep(10 * time.Millisecond) })(ctx)
			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Second {
				t.Error(time.Since(start))
			}
		})
		t.Run("Observe", func(t *testing.T) {
			var seen int
			start := time.Now()
			WaitBlockingObserve(
				func(in int) { seen = in },
				func() int { time.Sleep(10 * time.Millisecond); return 42 },
			).Block()

			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Second {
				t.Error(time.Since(start))
			}
			if seen != 42 {
				t.Error(seen)
			}
		})
	})
	t.Run("Timeout", func(t *testing.T) {
		wf := WaitFunc(func(ctx context.Context) {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
			case <-timer.C:
			}
		})
		start := time.Now()
		wf.WithTimeout(10 * time.Millisecond)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 15*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("WorkerConverter", func(t *testing.T) {
		called := &atomic.Bool{}
		err := WaitFunc(func(context.Context) { called.Store(true) }).Worker()(testt.Context(t))
		assert.NotError(t, err)
		assert.True(t, called.Load())
		assert.Panic(t, func() {
			var err error //nolint:gosimple
			err = WaitFunc(func(context.Context) { panic("hi") }).Worker()(testt.Context(t))
			check.NotError(t, err)
		})
	})
	t.Run("CheckWorker", func(t *testing.T) {
		assert.NotPanic(t, func() {
			expected := errors.New("hi")
			err := WaitFunc(func(context.Context) { panic(expected) }).CheckWorker()(testt.Context(t))
			check.Error(t, err)
			check.ErrorIs(t, err, expected)
		})
	})
	t.Run("Singal", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("PlainContext", func(t *testing.T) {
			start := time.Now()
			ran := &atomic.Bool{}
			var wf WaitFunc = func(ctx context.Context) {
				time.Sleep(5 * time.Millisecond)
				ran.Store(true)
			}

			sig := wf.Signal(ctx)
			runtime.Gosched()
			<-sig
			dur := time.Since(start)
			if dur < 5*time.Millisecond || dur > 10*time.Millisecond {
				t.Error(dur)
			}
			if !ran.Load() {
				t.Error("did not observe test running")
			}
		})
		t.Run("TimeoutContext", func(t *testing.T) {
			start := time.Now()
			ran := &atomic.Bool{}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
			defer cancel()

			var wf WaitFunc = func(ctx context.Context) {
				timer := testt.Timer(t, 100*time.Millisecond)
				select {
				case <-timer.C:
					ran.Store(true)
				case <-ctx.Done():
					return
				}
			}

			sig := wf.Signal(ctx)
			runtime.Gosched()
			<-sig
			dur := time.Since(start)
			if dur < 5*time.Millisecond || dur > 50*time.Millisecond {
				t.Error(dur)
			}
			if ran.Load() {
				t.Error("should not observe test running")
			}
		})
		t.Run("Timeout", func(t *testing.T) {
			start := time.Now()
			ran := &atomic.Bool{}
			var wf WaitFunc = func(ctx context.Context) {
				timer := testt.Timer(t, 10*time.Millisecond)
				select {
				case <-timer.C:
					ran.Store(true)
				case <-ctx.Done():
					return
				}
			}

			sig := wf.WithTimeoutSignal(5 * time.Millisecond)
			runtime.Gosched()
			<-sig
			dur := time.Since(start)
			if dur < 5*time.Millisecond || dur > 10*time.Millisecond {
				t.Error(dur)
			}

			if ran.Load() {
				t.Error("should not have observed test running")
			}

		})
		t.Run("Block", func(t *testing.T) {
			start := time.Now()
			ran := &atomic.Bool{}
			var wf WaitFunc = func(ctx context.Context) {
				time.Sleep(5 * time.Millisecond)
				ran.Store(true)
			}

			sig := wf.BlockSignal()
			runtime.Gosched()
			<-sig
			dur := time.Since(start)
			if dur < 5*time.Millisecond || dur > 10*time.Millisecond {
				t.Error(dur)
			}
			if !ran.Load() {
				t.Error("did not observe test running")
			}

		})

	})

}
