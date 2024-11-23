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
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
)

func TestWorker(t *testing.T) {
	t.Parallel()
	t.Run("Functions", func(t *testing.T) {
		t.Run("Blocking", func(t *testing.T) {
			start := time.Now()
			err := Worker(func(_ context.Context) error { time.Sleep(80 * time.Millisecond); return nil }).Block()
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := &atomic.Int64{}
			expected := errors.New("foo")
			Worker(func(_ context.Context) error { called.Add(1); panic(expected) }).WithRecover().
				Launch(ctx).
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wg := &WaitGroup{}
			count := &atomic.Int64{}
			func() {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()

				Worker(func(_ context.Context) error {
					assert.Equal(t, wg.Num(), 1)
					count.Add(1)
					return nil
				}).Operation(func(err error) {
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			t.Run("ObserveNilErrors", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				Worker(func(_ context.Context) error { called.Store(true); return nil }).Observe(ctx, func(error) { observed.Store(true) })
				assert.True(t, called.Load())
				assert.True(t, observed.Load())
			})
			t.Run("WithError", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				expected := errors.New("hello")
				Worker(func(_ context.Context) error {
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
				wf := Worker(func(_ context.Context) error {
					called.Store(true)
					return expected
				}).Operation(func(err error) {
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
				err := ers.WithRecoverCall(func() {
					var wf Operation //nolint:gosimple
					wf = Worker(func(context.Context) error {
						panic(expected)
					}).Must()
					t.Log(wf)
				})
				// declaration shouldn't call
				assert.NotError(t, err)

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				err = ers.WithRecoverCall(func() {
					var wf Operation //nolint:gosimple
					wf = Worker(func(context.Context) error {
						panic(expected)
					}).Must()
					wf(ctx)
				})
				assert.Error(t, err)
				assert.ErrorIs(t, err, expected)
			})
		})
		t.Run("Signal", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := errors.New("hello")
			wf := MakeWorker(func() error { return expected })
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
		var oee Handler[error] = func(err error) { check.ErrorIs(t, err, expected); Blocking(ch).Send().Signal(ctx) }
		var oen Handler[error] = func(err error) { check.NotError(t, err); Blocking(ch).Send().Signal(ctx) }
		var wf Worker
		t.Run("Panic", func(t *testing.T) {
			called := &atomic.Bool{}
			wf = func(context.Context) error { called.Store(true); panic(expected) }
			wf.WithRecover().Background(ctx, oee).Run(ctx)
			Blocking(ch).Receive().Ignore(ctx)
			assert.True(t, called.Load())
		})
		t.Run("Nil", func(t *testing.T) {
			called := &atomic.Bool{}
			wf = func(context.Context) error { called.Store(true); return nil }
			wf.Background(ctx, oen).Run(ctx)
			Blocking(ch).Receive().Ignore(ctx)
			assert.True(t, called.Load())
		})
		t.Run("Error", func(t *testing.T) {
			called := &atomic.Bool{}
			wf = func(context.Context) error { called.Store(true); return expected }
			wf.Background(ctx, oee).Run(ctx)
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
			assert.NotError(t, WorkerFuture(ch).Run(ctx))
		})
		t.Run("ClosedChannel", func(t *testing.T) {
			ch = make(chan error)
			close(ch)
			assert.NotError(t, WorkerFuture(ch).Run(ctx))
		})
		t.Run("ContextCanceled", func(t *testing.T) {
			nctx, cancel := context.WithCancel(context.Background())
			cancel()
			ch = make(chan error)
			err := WorkerFuture(ch).Run(nctx)
			assert.ErrorIs(t, err, context.Canceled)
		})
		t.Run("Error", func(t *testing.T) {
			ch = make(chan error, 1)
			ch <- expected
			err := WorkerFuture(ch).Run(ctx)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("NilError", func(t *testing.T) {
			ch = make(chan error, 1)
			ch <- nil
			err := WorkerFuture(ch).Run(ctx)
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
		assert.Error(t, wf.WithRecover().Run(ctx))
		assert.Equal(t, count, 1)
		assert.ErrorIs(t, wf.WithRecover().Run(ctx), expected)
		assert.Equal(t, count, 2)
		assert.Panic(t, func() { _ = wf(ctx) })
		assert.Equal(t, count, 3)
		assert.Panic(t, func() { wf.Ignore().Run(ctx) })
		assert.Equal(t, count, 4)
		assert.NotPanic(t, func() { wf.WithRecover().Ignore().Run(ctx) })
		assert.Equal(t, count, 5)
	})
	t.Run("Limit", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
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
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := &atomic.Int64{}

			var wf Worker = func(context.Context) error { count.Add(1); return nil }
			wf = wf.Limit(10)
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); check.NotError(t, wf(ctx)) }()
			}
			wg.Wait()
			assert.Equal(t, count.Load(), 10)
		})
	})
	t.Run("Interval", func(t *testing.T) {
		t.Run("NoEarlyAbort", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := &atomic.Int64{}
			var wf Worker = func(context.Context) error { count.Add(1); runtime.Gosched(); return nil }
			wf = wf.Interval(25 * time.Millisecond)
			check.Equal(t, 0, count.Load())
			sig := wf.Signal(ctx)
			time.Sleep(150 * time.Millisecond)
			runtime.Gosched()
			cancel()
			<-sig

			check.True(t, count.Load() >= 6)
			if t.Failed() {
				t.Log("iters", count.Load())
			}
		})
		t.Run("Half", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := &atomic.Int64{}
			var wf Worker = func(context.Context) error {
				if count.Add(1) > 3 {
					return errors.New(t.Name())
				}
				runtime.Gosched()
				return nil
			}
			wf = wf.Interval(25 * time.Millisecond)
			check.Equal(t, 0, count.Load())
			start := time.Now()
			sig := wf.Signal(ctx)
			<-sig
			dur := time.Since(start)
			check.True(t, dur >= 100)
			check.Equal(t, 4, count.Load())
		})

	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("Join", func(t *testing.T) {
			t.Run("WithoutErrors", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				count := 0
				var wf Worker = func(context.Context) error { count++; return nil }
				wf = wf.Join(wf)
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
				wf = wf.Join(func(context.Context) error { count++; return nil })
				err := wf(ctx)
				assert.Error(t, err)
				assert.ErrorIs(t, err, expected)
				assert.Equal(t, count, 1)
			})
			t.Run("Expired", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				count := 0
				var wf Worker = func(context.Context) error { count++; return nil }
				wf = wf.Join(
					func(context.Context) error { count++; cancel(); return nil },
					func(context.Context) error { count++; return nil },
					func(context.Context) error { count++; return nil },
				)
				err := wf(ctx)
				check.NotError(t, err)
				assert.Equal(t, count, 2)
			})
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
	t.Run("ErrorCheck", func(t *testing.T) {
		t.Run("ShortCircut", func(t *testing.T) {
			err := errors.New("test error")
			var hf Future[error] = func() error { return err }
			called := 0
			wf := Worker(func(_ context.Context) error { called++; return nil })
			ecpf := wf.WithErrorCheck(hf)
			e := ecpf.Wait()
			check.Error(t, e)
			check.ErrorIs(t, e, err)
			check.Equal(t, 0, called)

		})
		t.Run("Noop", func(t *testing.T) {
			var hf Future[error] = func() error { return nil }
			called := 0
			wf := Worker(func(_ context.Context) error { called++; return nil })
			ecpf := wf.WithErrorCheck(hf)
			e := ecpf.Wait()
			check.NotError(t, e)
			check.Equal(t, 1, called)
		})
		t.Run("Multi", func(t *testing.T) {
			called := 0
			hfcall := 0
			var hf Future[error] = func() error {
				hfcall++
				switch hfcall {
				case 1, 2, 3:
					return nil
				case 4, 5:
					return ers.ErrCurrentOpAbort
				}
				return errors.New("unexpected error")
			}
			wf := Worker(func(_ context.Context) error {
				called++
				if hfcall == 3 {
					return io.EOF
				}
				return nil
			})
			ecpf := wf.WithErrorCheck(hf)
			e := ecpf.Wait()
			check.NotError(t, e)
			check.Equal(t, 1, called)
			check.Equal(t, 2, hfcall)

			e = ecpf.Wait()
			check.Error(t, e)
			check.Equal(t, 2, called)
			check.Equal(t, 4, hfcall)
			check.ErrorIs(t, e, ers.ErrCurrentOpAbort)
			check.ErrorIs(t, e, io.EOF)

			e = ecpf.Wait()
			check.Error(t, e)
			check.Equal(t, 2, called)
			check.Equal(t, 5, hfcall)
			check.ErrorIs(t, e, ers.ErrCurrentOpAbort)
			check.NotErrorIs(t, e, io.EOF)
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
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		delay := (100 * time.Millisecond)
		timing := func(in *time.Duration) time.Duration {
			return *in
		}
		ptr := func(in time.Duration) *time.Duration { return &in }

		count := &atomic.Int64{}
		var wf Worker = func(context.Context) error { count.Add(1); return nil }
		wf = wf.Jitter(func() time.Duration { return timing(ptr(delay)) })
		start := time.Now()
		check.NotError(t, wf(ctx))
		dur := time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= 100*time.Millisecond)
		assert.True(t, dur < 200*time.Millisecond)

		delay = time.Millisecond
		start = time.Now()
		check.NotError(t, wf(ctx))
		dur = time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= time.Millisecond)
		assert.True(t, dur < 2*time.Millisecond)
	})
	t.Run("While", func(t *testing.T) {
		t.Parallel()
		t.Run("PRoof", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			eone := errors.New("while errors")
			etwo := errors.New("do errors")

			whichErr := 2
			one := func(context.Context) error { return ft.WhenDo(whichErr == 1, func() error { return eone }) }
			two := func(context.Context) error { return ft.WhenDo(whichErr == 2, func() error { return etwo }) }

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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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
			var wf Worker = func(_ context.Context) error {
				time.Sleep(5 * time.Millisecond)
				return ft.WhenDo(time.Since(start) > 50*time.Millisecond, func() error { return io.EOF })
			}
			t.Run("Min", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				assert.MinRuntime(t, 50*time.Millisecond, func() {
					err := wf.While().Run(ctx)
					assert.ErrorIs(t, err, io.EOF)
				})
			})
			t.Run("Max", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
				assert.MaxRuntime(t, 40*time.Millisecond, func() {
					err := wf.While().Run(ctx)
					assert.ErrorIs(t, err, io.EOF)
				})
			})
		})

	})
	t.Run("Pipe", func(t *testing.T) {
		t.Run("EndToEnd", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := errors.New("cat")
			errCt := 0
			nopCt := 0
			reset := func() { errCt = 0; nopCt = 0 }
			errProd := func(_ context.Context) (string, error) { errCt++; return "", expected }
			nopProd := func(_ context.Context) (string, error) { nopCt++; return "nop", nil }
			errProc := func(_ context.Context, _ string) error { errCt++; return expected }
			nopProc := func(_ context.Context, _ string) error { nopCt++; return nil }

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
		t.Run("RespectsSkips", func(t *testing.T) {
			prodCt := 0
			procCt := 0
			prod := func(_ context.Context) (string, error) {
				prodCt++
				if prodCt <= 5 {
					return "", ErrIteratorSkip
				}

				return "", io.EOF
			}
			proc := func(_ context.Context, _ string) error { procCt++; return nil }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			wk := Pipe(prod, proc).While()
			err := wk(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, io.EOF)
			assert.Equal(t, 6, prodCt)
			assert.Equal(t, 0, procCt)
		})
	})
	t.Run("FilterError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wf Worker = func(context.Context) error {
			return io.EOF
		}
		assert.ErrorIs(t, wf(ctx), io.EOF)
		fwf := wf.WithErrorFilter(func(error) error { return nil })
		assert.NotError(t, fwf(ctx))
		assert.ErrorIs(t, wf(ctx), io.EOF)
		fwf = wf.WithErrorFilter(func(error) error { return context.Canceled })
		assert.ErrorIs(t, fwf(ctx), context.Canceled)
	})
	t.Run("WithoutError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var wf Worker = func(context.Context) error {
			return io.EOF
		}
		assert.ErrorIs(t, wf(ctx), io.EOF)
		fwf := wf.WithoutErrors(io.EOF, context.Canceled)
		assert.NotError(t, fwf(ctx))
		assert.ErrorIs(t, wf(ctx), io.EOF)
		wf = func(context.Context) error { return nil }
		fwf = wf.WithoutErrors(io.EOF, context.Canceled)
		assert.NotError(t, fwf(ctx))
	})
	t.Run("WithCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf, cancel := Worker(func(ctx context.Context) error {
			timer := time.NewTimer(time.Hour)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				check.ErrorIs(t, ctx.Err(), context.Canceled)
				return nil
			case <-timer.C:
				t.Error("should not have reached this timeout")
			}
			return ers.Error("unreachable")
		}).WithCancel()
		assert.MinRuntime(t, 40*time.Millisecond, func() {
			assert.MaxRuntime(t, 75*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				check.NotError(t, wf(ctx))
			})
		})
	})
	t.Run("WorkerFuture", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := make(chan error, 1)
		wf := WorkerFuture(ch)
		root := ers.New("will-be-cached")
		ch <- root
		err := wf(ctx)
		check.ErrorIs(t, err, root)
		close(ch)
		check.NotError(t, wf(ctx))
		check.NotError(t, wf(ctx))
		check.NotError(t, wf(ctx))
	})
	t.Run("PreHook", func(t *testing.T) {
		t.Run("Chain", func(t *testing.T) {
			count := 0
			rctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wf Worker = func(ctx context.Context) error {
				check.True(t, count == 1 || count == 4)
				check.Equal(t, rctx, ctx)
				count++
				return nil
			}

			wf = wf.PreHook(func(ctx context.Context) {
				check.True(t, count == 0 || count == 3)
				check.Equal(t, rctx, ctx)
				count++
			})
			check.NotError(t, wf(rctx))
			check.Equal(t, 2, count)
			wf = wf.PreHook(func(ctx context.Context) {
				check.Equal(t, count, 2)
				check.Equal(t, rctx, ctx)
				count++
			})
			check.NotError(t, wf(rctx))
			check.Equal(t, 5, count)
		})

		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Worker(func(_ context.Context) error {
				assert.Equal(t, count, 1)
				count++
				return nil
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, root)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Worker(func(_ context.Context) error {
				assert.Equal(t, count, 1)
				count++
				return nil
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++ })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx)
			check.NotError(t, err)
			check.Equal(t, 2, count)
		})
	})
	t.Run("PostHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Worker(func(_ context.Context) error {
				assert.Zero(t, count)
				count++
				return nil
			}).PostHook(func() { assert.Equal(t, count, 1); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx)
			check.Error(t, err)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Worker(func(_ context.Context) error {
				assert.Zero(t, count)
				count++
				return nil
			}).PostHook(func() { assert.Equal(t, count, 1); count++ })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx)
			check.NotError(t, err)
			check.Equal(t, 2, count)
		})
	})
	t.Run("Retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("Skip", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			err := MakeWorker(func() error {
				defer count.Add(1)
				if count.Get() == 0 {
					return ers.ErrCurrentOpSkip
				}
				return nil
			}).Retry(5).Run(ctx)
			assert.Equal(t, count.Get(), 2)
			assert.NotError(t, err)
		})
		t.Run("FirstTry", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			err := MakeWorker(func() error {
				defer count.Add(1)
				return nil
			}).Retry(10).Run(ctx)
			assert.Equal(t, count.Get(), 1)
			assert.NotError(t, err)
		})
		t.Run("ArbitraryError", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			err := MakeWorker(func() error {
				defer count.Add(1)
				if count.Get() < 3 {
					return errors.New("why not")
				}
				return nil
			}).Retry(10).Run(ctx)
			assert.Equal(t, count.Get(), 4)
			assert.NotError(t, err)
		})
		t.Run("DoesFail", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			exp := errors.New("why not")
			err := MakeWorker(func() error {
				defer count.Add(1)
				return exp
			}).Retry(16).Run(ctx)
			assert.Equal(t, count.Get(), 16)
			assert.Error(t, err)
			errs := ers.Unwind(err)
			assert.Equal(t, len(errs), 16)
			for _, err := range errs {
				assert.Equal(t, err, exp)
				assert.ErrorIs(t, err, exp)
			}
		})
		t.Run("Terminating", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			exp := errors.New("why not")
			err := MakeWorker(func() error {
				defer count.Add(1)
				if count.Load() == 11 {
					return ers.Join(exp, ers.ErrCurrentOpAbort)
				}
				return exp
			}).Retry(16).Run(ctx)
			assert.Equal(t, count.Get(), 12)
			assert.NotError(t, err)
		})
		t.Run("Canceled", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			exp := errors.New("why not")
			err := MakeWorker(func() error {
				defer count.Add(1)
				if count.Load() == 11 {
					return ers.Join(exp, context.Canceled)
				}
				return exp
			}).Retry(16).Run(ctx)
			assert.Equal(t, count.Get(), 12)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.ErrorIs(t, err, exp)
			errs := ers.Unwind(err)
			assert.Equal(t, len(errs), 13)

		})
	})
	t.Run("WorkerPool", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			counter := &atomic.Int64{}
			wfs := make([]Worker, 100)
			for i := 0; i < 100; i++ {
				wfs[i] = func(context.Context) error {
					counter.Add(1)
					time.Sleep(10 * time.Millisecond)
					counter.Add(1)
					return nil
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			start := time.Now()
			assert.Equal(t, counter.Load(), 0)
			err := HF.WorkerPool(SliceIterator(wfs)).Run(ctx)
			dur := time.Since(start)
			if dur > 50*time.Millisecond || dur < 10*time.Millisecond {
				t.Error(dur)
			}
			assert.NotError(t, err)
			assert.Equal(t, counter.Load(), 200)
		})
		t.Run("Errors", func(t *testing.T) {
			const experr ers.Error = "expected error"

			counter := &atomic.Int64{}
			wfs := make([]Worker, 100)
			for i := 0; i < 100; i++ {
				wfs[i] = func(context.Context) error { counter.Add(1); time.Sleep(10 * time.Millisecond); return experr }
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			start := time.Now()
			assert.Equal(t, counter.Load(), 0)

			err := HF.WorkerPool(SliceIterator(wfs)).Run(ctx)
			dur := time.Since(start)
			if dur > 50*time.Millisecond || dur < 10*time.Millisecond {
				t.Error(dur)
			}

			assert.Error(t, err)
			errs := ers.Unwind(err)

			assert.Equal(t, len(errs), 100)
			assert.Equal(t, counter.Load(), 100)

			for _, e := range errs {
				assert.ErrorIs(t, e, experr)
			}
		})
	})
}
