package fun

import (
	"context"
	"errors"
	"io"
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

func TestProcess(t *testing.T) {
	t.Parallel()
	t.Run("Risky", func(t *testing.T) {
		called := 0
		pf := MakeProcessor(func(in int) error {
			check.Equal(t, in, 42)
			called++
			return nil
		})
		check.NotError(t, pf.Block(42))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pf.Ignore(ctx, 42)
		pf.Force(42)
		check.Equal(t, called, 3)
	})
	t.Run("WithCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wf, cancel := Processify(func(ctx context.Context, in int) error {
			check.Equal(t, in, 42)
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
			assert.MaxRuntime(t, 100*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				check.NotError(t, wf(ctx, 42))
			})
		})
	})
	t.Run("Add", func(t *testing.T) {
		count := &atomic.Int64{}
		pf := MakeProcessor(func(i int) error { check.Equal(t, i, 54); count.Add(1); return nil })
		wg := &WaitGroup{}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pf.Add(ctx, wg, func(error) {}, 54)
		wg.Operation().Wait()
		check.Equal(t, count.Load(), 1)
	})
	t.Run("If", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		called := 0
		pf := Processify(func(ctx context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		})

		check.NotError(t, pf.If(false).Run(ctx, 42))
		check.Zero(t, called)
		check.NotError(t, pf.If(true).Run(ctx, 42))
		check.Equal(t, 1, called)
		check.NotError(t, pf.If(true).Run(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf.If(false).Run(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf(ctx, 42))
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		pf := Processify(func(ctx context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		})

		check.NotError(t, pf.When(func() bool { return false }).Run(ctx, 42))
		check.Zero(t, called)
		check.NotError(t, pf.When(func() bool { return true }).Run(ctx, 42))
		check.Equal(t, 1, called)
		check.NotError(t, pf.When(func() bool { return true }).Run(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf.When(func() bool { return false }).Run(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf(ctx, 42))
		check.Equal(t, 3, called)
	})
	t.Run("Once", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		pf := Processify(func(ctx context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		}).Once()
		for i := 0; i < 1024; i++ {
			assert.NotError(t, pf(ctx, 42))
		}
		check.Equal(t, called, 1)
	})
	t.Run("Operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		root := ers.New("foo")
		pf := Processify(func(ctx context.Context, n int) error {
			check.Equal(t, 42, n)
			called++
			return root
		})
		of := func(err error) { called++; check.ErrorIs(t, err, root) }
		obv := pf.Operation(of, 42)
		check.Equal(t, called, 0)
		obv(ctx)
		check.Equal(t, called, 2)

	})
	t.Run("Handler", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		root := ers.New("foo")
		pf := Processify(func(ctx context.Context, n int) error {
			check.Equal(t, 42, n)
			called++
			return root
		})
		of := func(err error) { called++; check.ErrorIs(t, err, root) }
		obv := pf.Handler(ctx, of)
		check.Equal(t, called, 0)
		obv(42)
		check.Equal(t, called, 2)
	})
	t.Run("Capture", func(t *testing.T) {
		called := 0
		pf := Processify(func(ctx context.Context, n int) error {
			check.NotNil(t, ctx)
			check.Equal(t, 42, n)
			called++
			return nil
		})
		obv := pf.Capture()
		check.Equal(t, called, 0)
		obv(42)
		check.Equal(t, called, 1)
	})

	t.Run("Worker", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		root := ers.New("foo")
		pf := Processify(func(ctx context.Context, n int) error {
			check.Equal(t, 42, n)
			called++
			return root
		})
		check.Equal(t, called, 0)
		wf := pf.Worker(42)
		check.Equal(t, called, 0)
		check.Error(t, wf(ctx))
		check.Equal(t, called, 1)
	})
	t.Run("Future", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		root := ers.New("foo")
		pf := Processify(func(ctx context.Context, n int) error {
			time.Sleep(250 * time.Millisecond)
			check.Equal(t, 42, n)
			called++
			return root
		})
		check.Equal(t, called, 0)
		wf := pf.Background(ctx, 42)
		check.Equal(t, called, 0)
		check.ErrorIs(t, wf(ctx), root)
		check.Equal(t, called, 1)
	})
	t.Run("Lock", func(t *testing.T) {
		t.Run("NilLockPanics", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := Processify(func(_ context.Context, in int) error {
				count++
				check.Equal(t, in, 42)
				return nil
			})
			check.Panic(t, func() { assert.NotError(t, op.WithLock(nil).Run(ctx, 42)) })
			check.Equal(t, count, 0)
		})
		// the rest of the tests are really just "tempt the
		// race detector"
		t.Run("ManagedLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := Processify(func(_ context.Context, in int) error {
				count++
				check.Equal(t, in, 42)
				return nil
			})

			wg := &WaitGroup{}
			oe := HF.ErrorHandler(func(err error) { Invariant.Must(err) })
			op = op.Lock()

			ft.DoTimes(128, func() { op.Operation(oe, 42).Add(ctx, wg) })
			wg.Operation().Block()
			assert.Equal(t, count, 128)
		})
		t.Run("CustomLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := Processify(func(_ context.Context, in int) error {
				count++
				check.Equal(t, in, 42)
				return nil
			})
			mu := &sync.Mutex{}
			wf := op.WithLock(mu).Worker(42).Group(128)
			assert.NotError(t, wf(ctx))
			assert.Equal(t, count, 128)
		})
	})
	t.Run("WithoutErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var err error
		var pf Processor[int] = func(_ context.Context, in int) error { count++; assert.Equal(t, in, 42); return err }

		err = ers.Error("hello")
		pf = pf.WithoutErrors(io.EOF)
		assert.Equal(t, count, 0)
		assert.Error(t, pf(ctx, 42))
		assert.Equal(t, count, 1)
		err = io.EOF
		assert.NotError(t, pf(ctx, 42))
		assert.Equal(t, count, 2)
		err = context.Canceled
		assert.Error(t, pf(ctx, 42))
		assert.Equal(t, count, 3)
	})
	t.Run("ReadOne", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		nopCt := 0
		var nopProd Producer[string] = func(ctx context.Context) (string, error) { nopCt++; return "nop", nil }
		var nopProc Processor[string] = func(ctx context.Context, in string) error { nopCt++; check.Equal(t, in, "nop"); return nil }
		op := nopProc.ReadOne(nopProd)
		check.Equal(t, nopCt, 0)
		check.NotError(t, op(ctx))
		check.Equal(t, nopCt, 2)
	})
	t.Run("Join", func(t *testing.T) {
		onect, twoct := 0, 0
		reset := func() { onect, twoct = 0, 0 }
		t.Run("Basic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var one Processor[string] = func(ctx context.Context, in string) error { onect++; check.Equal(t, in, t.Name()); return ctx.Err() }
			var two Processor[string] = func(ctx context.Context, in string) error { twoct++; check.Equal(t, in, t.Name()); return ctx.Err() }

			pf := one.Join(two)
			check.NotError(t, pf(ctx, t.Name()))
			check.Equal(t, onect, 1)
			check.Equal(t, twoct, 1)
			reset()

		})
		t.Run("Canceled", func(t *testing.T) {
			var one Processor[string] = func(ctx context.Context, in string) error { onect++; check.Equal(t, in, t.Name()); return ctx.Err() }
			var two Processor[string] = func(ctx context.Context, in string) error { twoct++; check.Equal(t, in, t.Name()); return ctx.Err() }

			pf := one.Join(two)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := pf(ctx, t.Name())
			check.Error(t, err)
			check.ErrorIs(t, err, context.Canceled)
			check.Equal(t, onect, 1)
			check.Equal(t, twoct, 0)
			reset()

		})
		t.Run("CancelBetweenShouldNoopSecond", func(t *testing.T) {
			sig1 := make(chan struct{})
			sig2 := make(chan struct{})
			sig3 := make(chan struct{})
			one := Processor[string](func(ctx context.Context, in string) error {
				defer close(sig1)
				<-sig2
				onect++
				check.Equal(t, in, t.Name())

				return nil
			})
			two := Processor[string](func(ctx context.Context, in string) error {
				twoct++
				check.Equal(t, in, t.Name())
				return nil
			})

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			pf := one.Join(two)
			go func() { defer close(sig3); check.NotError(t, pf(ctx, t.Name())) }()
			close(sig2)
			<-sig1
			check.Equal(t, onect, 1)
			check.Equal(t, twoct, 0)
			<-sig3
		})
	})
	t.Run("Group", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := &atomic.Int64{}
		proc := MakeProcessor(func(i int) error { count.Add(1); check.Equal(t, i, 42); return nil })
		check.Equal(t, 0, count.Load())
		worker := proc.Parallel(42, 42, 42, 42, 42, 42, 42, 42, 42, 42)
		check.Equal(t, 0, count.Load())
		err := worker(ctx)
		check.NotError(t, err)
		check.Equal(t, 10, count.Load())
	})
	t.Run("PreHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Processify(func(ctx context.Context, in int) error {
				check.Equal(t, in, 42)
				assert.Equal(t, count, 1)
				count++
				return nil
			}).PreHook(func(ctx context.Context) { assert.Zero(t, count); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx, 42)
			check.Error(t, err)
			check.ErrorIs(t, err, root)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Processify(func(ctx context.Context, in int) error {
				check.Equal(t, in, 42)
				assert.Equal(t, count, 1)
				count++
				return nil
			}).PreHook(func(ctx context.Context) { assert.Zero(t, count); count++ })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx, 42)
			check.NotError(t, err)
			check.Equal(t, 2, count)
		})
	})
	t.Run("PostHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Processify(func(ctx context.Context, in int) error {
				check.Equal(t, in, 42)
				assert.Zero(t, count)
				count++
				return nil
			}).PostHook(func() { assert.Equal(t, count, 1); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx, 42)
			check.Error(t, err)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Processify(func(ctx context.Context, in int) error {
				check.Equal(t, in, 42)
				assert.Zero(t, count)
				count++
				return nil
			}).PostHook(func() { assert.Equal(t, count, 1); count++ })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx, 42)
			check.NotError(t, err)
			check.Equal(t, 2, count)
		})
	})
	t.Run("Limit", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			var wf Processor[int] = func(ctx context.Context, in int) error { check.Equal(t, in, 42); count++; return nil }
			wf = wf.Limit(10)
			for i := 0; i < 100; i++ {
				check.NotError(t, wf(ctx, 42))
			}
			assert.Equal(t, count, 10)
		})
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Processor[int] = func(ctx context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
			wf = wf.Limit(10)
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); check.NotError(t, wf(ctx, 42)) }()
			}
			wg.Wait()
			assert.Equal(t, count.Load(), 10)
		})
	})
	t.Run("TTL", func(t *testing.T) {
		t.Run("Zero", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")

			count := 0
			var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return expected }
			wf = wf.TTL(0)
			for i := 0; i < 100; i++ {
				check.ErrorIs(t, wf(ctx, 42), expected)
			}
			check.Equal(t, 100, count)
		})
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")

			count := 0
			var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return expected }
			wf = wf.TTL(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				check.ErrorIs(t, wf(ctx, 42), expected)
			}
			check.Equal(t, 1, count)
			time.Sleep(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				check.ErrorIs(t, wf(ctx, 42), expected)
			}
			check.Equal(t, 2, count)
		})
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")
			wg := &sync.WaitGroup{}

			count := 0
			var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return expected }
			wf = wf.TTL(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() { defer wg.Done(); check.ErrorIs(t, wf(ctx, 42), expected) }()
			}
			wg.Wait()
			check.Equal(t, 1, count)
			time.Sleep(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() { defer wg.Done(); check.ErrorIs(t, wf(ctx, 42), expected) }()
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
			var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					check.NotError(t, wf(ctx, 42))
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
			var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			cancel()
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					check.ErrorIs(t, wf(ctx, 42), context.Canceled)
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
			var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
			ts := time.Now().Add(100 * time.Millisecond)
			wf = wf.After(ts)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					check.NotError(t, wf(ctx, 42))
				}()
			}
			check.Equal(t, 100, wg.Num())
			time.Sleep(120 * time.Millisecond)
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 100)
			wg.Operation().Wait()
		})
	})
	t.Run("Jitter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := &atomic.Int64{}
		var wf Processor[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
		delay := 100 * time.Millisecond
		wf = wf.Jitter(func() time.Duration { return delay })
		start := time.Now()
		check.NotError(t, wf(ctx, 42))
		dur := time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= 100*time.Millisecond)
		assert.True(t, dur < 200*time.Millisecond)

		delay = time.Millisecond
		start = time.Now()
		check.NotError(t, wf(ctx, 42))
		dur = time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= time.Millisecond)
		assert.True(t, dur < 2*time.Millisecond)
	})
	t.Run("Retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := &intish.Atomic[int]{}
		err := MakeProcessor(func(in int) error {
			defer count.Add(1)
			assert.Equal(t, in, 42)
			if count.Get() < 8 {
				return ers.ErrCurrentOpSkip
			}

			return nil
		}).Retry(32, 42).Run(ctx)
		assert.NotError(t, err)
	})
	t.Run("Filter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc := MakeProcessor(func(in int) error {
			check.Equal(t, in, 42)
			return nil
		}).Filter(func(in int) bool { return in == 42 })

		for i := 0; i < 100; i++ {
			err := proc(ctx, i)
			switch {
			case err == nil:
				return
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			default:
				assert.Error(t, err)
			}
		}
	})
}
