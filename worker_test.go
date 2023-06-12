package fun

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

func TestWorker(t *testing.T) {
	t.Run("Functions", func(t *testing.T) {
		t.Run("Blocking", func(t *testing.T) {
			start := time.Now()
			err := Worker(func(ctx context.Context) error { time.Sleep(80 * time.Millisecond); return nil }).Block()
			dur := time.Since(start)
			if dur < 10*time.Millisecond {
				t.Error("did not block long enough", dur)
			}
			if dur > 100*time.Millisecond {
				t.Error("blocked too long", dur)
			}
			assert.NotError(t, err)
		})
		t.Run("Background", func(t *testing.T) {
			ctx := testt.Context(t)
			called := &atomic.Int64{}
			expected := errors.New("foo")
			Worker(func(ctx context.Context) error { called.Add(1); panic(expected) }).
				Future(ctx).
				Observe(ctx, func(err error) {
					check.Error(t, err)
					check.ErrorIs(t, err, expected)
					called.Add(1)
				})
			for {
				if called.Load() == 2 {
					return
				}
				time.Sleep(4 * time.Millisecond)
			}
		})
		t.Run("Add", func(t *testing.T) {
			ctx := testt.Context(t)
			wg := &WaitGroup{}
			count := &atomic.Int64{}
			func() {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()

				Worker(func(ctx context.Context) error {
					assert.Equal(t, wg.Num(), 1)
					count.Add(1)
					return nil
				}).Wait(func(err error) {
					assert.Equal(t, wg.Num(), 1)
					count.Add(1)
					assert.NotError(t, err)
				}).Add(ctx, wg)
			}()
			wg.Wait(ctx)
			assert.Equal(t, wg.Num(), 0)
			assert.Equal(t, count.Load(), 2)
		})
		t.Run("Observe", func(t *testing.T) {
			ctx := testt.Context(t)
			t.Run("ObserveNilErrors", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				Worker(func(ctx context.Context) error { called.Store(true); return nil }).Observe(ctx, func(error) { observed.Store(true) })
				assert.True(t, called.Load())
				assert.True(t, observed.Load())
			})
			t.Run("WithError", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				expected := errors.New("hello")
				Worker(func(ctx context.Context) error {
					called.Store(true)
					return expected
				}).Observe(ctx, func(err error) {
					observed.Store(true)
					check.ErrorIs(t, err, expected)
				})
				assert.True(t, called.Load())
				assert.True(t, observed.Load())
			})
			t.Run("AsWait", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				expected := errors.New("hello")
				wf := Worker(func(ctx context.Context) error {
					called.Store(true)
					return expected
				}).Wait(func(err error) {
					observed.Store(true)
					check.ErrorIs(t, err, expected)
				})

				// not called yet
				assert.True(t, !called.Load())
				assert.True(t, !observed.Load())

				wf(ctx)

				// now called
				assert.True(t, called.Load())
				assert.True(t, observed.Load())
			})
			t.Run("Must", func(t *testing.T) {
				expected := errors.New("merlin")
				err := ers.Check(func() {
					var wf Operation //nolint:gosimple
					wf = Worker(func(context.Context) error {
						panic(expected)
					}).Must()
					t.Log(wf)
				})
				// declaration shouldn't call
				assert.NotError(t, err)

				err = ers.Check(func() {
					var wf Operation //nolint:gosimple
					wf = Worker(func(context.Context) error {
						panic(expected)
					}).Must()
					wf(testt.Context(t))
				})
				assert.Error(t, err)
				assert.ErrorIs(t, err, expected)
			})
		})
		t.Run("Signal", func(t *testing.T) {
			ctx := testt.Context(t)
			expected := errors.New("hello")
			wf := Worker(func(ctx context.Context) error { return expected })
			out := wf.Signal(ctx)
			err := <-out
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})
	t.Run("Background", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := make(chan struct{}, 1)

		expected := errors.New("kip")
		var oee Observer[error] = func(err error) { check.ErrorIs(t, err, expected); Blocking(ch).Send().Signal(ctx) }
		var oen Observer[error] = func(err error) { check.NotError(t, err); Blocking(ch).Send().Signal(ctx) }
		var wf Worker
		t.Run("Panic", func(t *testing.T) {
			called := &atomic.Bool{}
			wf = func(context.Context) error { called.Store(true); panic(expected) }
			wf.Background(ctx, oee)
			Blocking(ch).Receive().Ignore(ctx)
			assert.True(t, called.Load())
		})
		t.Run("Nil", func(t *testing.T) {
			called := &atomic.Bool{}
			wf = func(context.Context) error { called.Store(true); return nil }
			wf.Background(ctx, oen)
			Blocking(ch).Receive().Ignore(ctx)
			assert.True(t, called.Load())
		})
		t.Run("Error", func(t *testing.T) {
			called := &atomic.Bool{}
			wf = func(context.Context) error { called.Store(true); return expected }
			wf.Background(ctx, oee)
			Blocking(ch).Receive().Ignore(ctx)
			assert.True(t, called.Load())
		})
	})
	t.Run("WorkerFuture", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		expected := errors.New("cat")
		var ch chan error
		t.Run("NilChannel", func(t *testing.T) {
			assert.NotError(t, WorkerFuture(ch)(ctx))
		})
		t.Run("ClosedChannel", func(t *testing.T) {
			ch = make(chan error)
			close(ch)
			assert.NotError(t, WorkerFuture(ch)(ctx))
		})
		t.Run("ContextCanceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ch = make(chan error)
			err := WorkerFuture(ch)(ctx)
			assert.ErrorIs(t, err, context.Canceled)
		})
		t.Run("Error", func(t *testing.T) {
			ch = make(chan error, 1)
			ch <- expected
			err := WorkerFuture(ch)(ctx)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("NilError", func(t *testing.T) {
			ch = make(chan error, 1)
			ch <- nil
			err := WorkerFuture(ch)(ctx)
			assert.NotError(t, err)
		})
	})
	t.Run("Once ", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var count int
		var wf Worker = func(context.Context) error { count++; return nil }
		wf = wf.Once()
		assert.Equal(t, 0, count)
		check.NotError(t, wf(ctx))
		assert.Equal(t, 1, count)

		for i := 0; i < 100; i++ {
			check.NotError(t, wf(ctx))
			assert.Equal(t, 1, count)
		}
	})
	t.Run("Ignore", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		expected := errors.New("cat")
		count := 0
		var wf Worker = func(context.Context) error { count++; panic(expected) }
		assert.Error(t, wf.Safe()(ctx))
		assert.Equal(t, count, 1)
		assert.ErrorIs(t, wf.Safe()(ctx), expected)
		assert.Equal(t, count, 2)
		assert.Panic(t, func() { _ = wf(ctx) })
		assert.Equal(t, count, 3)
		assert.Panic(t, func() { wf.Ignore()(ctx) })
		assert.Equal(t, count, 4)
		assert.NotPanic(t, func() { wf.Safe().Ignore()(ctx) })
		assert.Equal(t, count, 5)
	})
	t.Run("SerialLimit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var wf Worker = func(context.Context) error { count++; return nil }
		wf = wf.Limit(10)
		for i := 0; i < 100; i++ {
			assert.NotError(t, wf(ctx))
		}
		assert.Equal(t, count, 10)
	})
	t.Run("ParallelLimit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var wf Worker = func(context.Context) error { count++; return nil }
		wf = wf.Limit(10)
		wg := &sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); check.NotError(t, wf(ctx)) }()
		}
		wg.Wait()
		assert.Equal(t, count, 10)
	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("WithoutErrors", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			var wf Worker = func(context.Context) error { count++; return nil }
			wf = wf.Chain(wf)
			check.NotError(t, wf(ctx))
			assert.Equal(t, count, 2)

			cancel()
			check.NotError(t, wf(ctx))
			// first function always runs and this example doesn't
			// respect the context. second one doesn't
			assert.Equal(t, count, 3)
		})
		t.Run("Error", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := errors.New("cat")

			count := 0
			var wf Worker = func(context.Context) error { count++; return expected }
			wf = wf.Chain(func(context.Context) error { count++; return nil })
			err := wf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
			assert.Equal(t, count, 1)
		})
	})
	t.Run("Check", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		expected := errors.New("cat")

		count := 0
		var wf Worker = func(context.Context) error { count++; return expected }
		assert.True(t, !wf.Check(ctx))
		wf = func(context.Context) error { count++; return nil }
		assert.True(t, wf.Check(ctx))
	})
	t.Run("Lock", func(t *testing.T) {
		// this is mostly just tempting the race detecor
		wg := &sync.WaitGroup{}
		count := 0
		var wf Worker = func(context.Context) error {
			defer wg.Done()
			count++
			return nil
		}

		wf = wf.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() { check.NotError(t, wf(ctx)) }()
		}
		wg.Wait()

		assert.Equal(t, count, 64)
	})
	t.Run("TTL", func(t *testing.T) {
		t.Run("Zero", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")

			count := 0
			var wf Worker = func(context.Context) error { count++; return expected }
			wf = wf.TTL(0)
			for i := 0; i < 100; i++ {
				check.ErrorIs(t, wf(ctx), expected)
			}
			check.Equal(t, 100, count)
		})
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")

			count := 0
			var wf Worker = func(context.Context) error { count++; return expected }
			wf = wf.TTL(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				check.ErrorIs(t, wf(ctx), expected)
			}
			check.Equal(t, 1, count)
			time.Sleep(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				check.ErrorIs(t, wf(ctx), expected)
			}
			check.Equal(t, 2, count)
		})
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")
			wg := &sync.WaitGroup{}

			count := 0
			var wf Worker = func(context.Context) error { count++; return expected }
			wf = wf.TTL(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() { defer wg.Done(); check.ErrorIs(t, wf(ctx), expected) }()
			}
			wg.Wait()
			check.Equal(t, 1, count)
			time.Sleep(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() { defer wg.Done(); check.ErrorIs(t, wf(ctx), expected) }()
			}
			wg.Wait()
			check.Equal(t, 2, count)

		})
	})
	t.Run("Delay", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Worker = func(context.Context) error { count.Add(1); return nil }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					check.NotError(t, wf(ctx))
				}()
			}
			check.Equal(t, 100, wg.Num())
			time.Sleep(125 * time.Millisecond)
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 100)
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Worker = func(context.Context) error { count.Add(1); return nil }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			cancel()
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					check.ErrorIs(t, wf(ctx), context.Canceled)
				}()
			}
			time.Sleep(2 * time.Millisecond)
			wg.Wait(context.Background())
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 0)
		})
		t.Run("After", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Worker = func(context.Context) error { count.Add(1); return nil }
			ts := time.Now().Add(100 * time.Millisecond)
			wf = wf.After(ts)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					check.NotError(t, wf(ctx))
				}()
			}
			check.Equal(t, 100, wg.Num())
			time.Sleep(120 * time.Millisecond)
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 100)

		})

	})
	t.Run("Jitter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := &atomic.Int64{}
		var wf Worker = func(context.Context) error { count.Add(1); return nil }
		delay := 100 * time.Millisecond
		wf = wf.Jitter(func() time.Duration { return delay })
		start := time.Now()
		check.NotError(t, wf(ctx))
		dur := time.Since(start)
		testt.Logf(t, "op took %s with delay %s", dur, delay)
		assert.True(t, dur >= 100*time.Millisecond)
		assert.True(t, dur < 200*time.Millisecond)

		delay = time.Millisecond
		start = time.Now()
		check.NotError(t, wf(ctx))
		dur = time.Since(start)
		testt.Logf(t, "op took %s with delay %s", dur, delay)
		assert.True(t, dur >= time.Millisecond)
		assert.True(t, dur < 2*time.Millisecond)
	})
	t.Run("While", func(t *testing.T) {
		t.Parallel()
		t.Run("PRoof", func(t *testing.T) {
			ctx := testt.Context(t)
			eone := errors.New("while errors")
			etwo := errors.New("do errors")

			whichErr := 2
			one := func(context.Context) error { return WhenDo(whichErr == 1, func() error { return eone }) }
			two := func(context.Context) error { return WhenDo(whichErr == 2, func() error { return etwo }) }

			if e, f := one(ctx), two(ctx); e == nil && f == nil {
				t.Error("can't both be nil")
			}

			whichErr = 1
			if e, f := one(ctx), two(ctx); e == nil {
				t.Log(e)
				t.Error(f)
			}
		})
		t.Run("Ordering", func(t *testing.T) {
			ctx := testt.Context(t)

			var (
				ts1 time.Time
				ts2 time.Time
			)

			one := func(context.Context) error { ts1 = time.Now(); time.Sleep(100 * time.Millisecond); return nil }
			two := func(context.Context) error { ts2 = time.Now(); return nil }

			if e, f := one(ctx), two(ctx); e != nil || f != nil {
				t.Error("both should be nil", e, f)
			}

			if ts2.Before(ts1) {
				t.Error("second was before first")
			}
			if dur := ts2.Sub(ts1); dur < 100*time.Millisecond {
				t.Error("not enough delay", dur)
			}
		})
		t.Run("Loop", func(t *testing.T) {
			start := time.Now()
			var wf Worker = func(ctx context.Context) error {
				time.Sleep(time.Millisecond)
				return WhenDo(time.Since(start) > 32*time.Millisecond, func() error { return io.EOF })
			}
			t.Run("Min", func(t *testing.T) {
				assert.MinRuntime(t, 32*time.Millisecond, func() {
					err := wf.While().Run(testt.ContextWithTimeout(t, 100*time.Millisecond))
					assert.ErrorIs(t, err, io.EOF)
				})
			})
			t.Run("Max", func(t *testing.T) {
				assert.MaxRuntime(t, 40*time.Millisecond, func() {
					err := wf.While().Run(testt.ContextWithTimeout(t, 100*time.Millisecond))
					assert.ErrorIs(t, err, io.EOF)
				})
			})
		})

	})
	t.Run("Pipe", func(t *testing.T) {
		ctx := testt.Context(t)
		expected := errors.New("cat")
		errCt := 0
		nopCt := 0
		reset := func() { errCt = 0; nopCt = 0 }
		errProd := func(ctx context.Context) (string, error) { errCt++; return "", expected }
		nopProd := func(ctx context.Context) (string, error) { nopCt++; return "nop", nil }
		errProc := func(ctx context.Context, in string) error { errCt++; return expected }
		nopProc := func(ctx context.Context, in string) error { nopCt++; return nil }

		var worker Worker

		t.Run("FirstFail", func(t *testing.T) {
			worker = Pipe(errProd, nopProc)
			err := worker(ctx)
			assert.ErrorIs(t, err, expected)
			assert.Equal(t, errCt, 1)
			assert.Equal(t, nopCt, 0)

		})
		t.Run("FirstFailCachesError", func(t *testing.T) {
			err := worker(ctx)
			assert.ErrorIs(t, err, expected)
			assert.Equal(t, errCt, 1) // because cached
			assert.Equal(t, nopCt, 0)
		})

		reset()

		t.Run("SecondFails", func(t *testing.T) {
			worker = Pipe(nopProd, errProc)

			err := worker(ctx)
			assert.ErrorIs(t, err, expected)
			assert.Equal(t, errCt, 1)
			assert.Equal(t, nopCt, 1)
		})
		t.Run("SecondFailureCaches", func(t *testing.T) {
			err := worker(ctx)
			assert.ErrorIs(t, err, expected)
			assert.Equal(t, errCt, 1)
			assert.Equal(t, nopCt, 1)
		})
	})
}
