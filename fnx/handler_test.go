package fnx

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
)

func TestProcess(t *testing.T) {
	t.Parallel()
	t.Run("Risky", func(t *testing.T) {
		called := 0
		pf := MakeHandler(func(in int) error {
			check.Equal(t, in, 42)
			called++
			return nil
		})
		check.NotError(t, pf.Wait(42))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pf.Ignore(ctx, 42)
		pf.Force(42)
		check.Equal(t, called, 3)
	})
	t.Run("WithCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wf, cancel := NewHandler(func(ctx context.Context, in int) error {
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
	t.Run("If", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		called := 0
		pf := NewHandler(func(_ context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		})

		check.NotError(t, pf.If(false).Read(ctx, 42))
		check.Zero(t, called)
		check.NotError(t, pf.If(true).Read(ctx, 42))
		check.Equal(t, 1, called)
		check.NotError(t, pf.If(true).Read(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf.If(false).Read(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf(ctx, 42))
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		pf := NewHandler(func(_ context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		})

		check.NotError(t, pf.When(func() bool { return false }).Read(ctx, 42))
		check.Zero(t, called)
		check.NotError(t, pf.When(func() bool { return true }).Read(ctx, 42))
		check.Equal(t, 1, called)
		check.NotError(t, pf.When(func() bool { return true }).Read(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf.When(func() bool { return false }).Read(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf(ctx, 42))
		check.Equal(t, 3, called)
	})
	t.Run("Once", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		pf := NewHandler(func(_ context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		}).Once()
		for i := 0; i < 1024; i++ {
			assert.NotError(t, pf(ctx, 42))
		}
		check.Equal(t, called, 1)
	})
	t.Run("Handler", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		root := ers.New("foo")
		pf := NewHandler(func(_ context.Context, n int) error {
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
		pf := NewHandler(func(ctx context.Context, n int) error {
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
	t.Run("Lock", func(t *testing.T) {
		t.Run("NilLockPanics", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := NewHandler(func(_ context.Context, in int) error {
				count++
				check.Equal(t, in, 42)
				return nil
			})
			check.Panic(t, func() { assert.NotError(t, op.WithLock(nil).Read(ctx, 42)) })
			check.Equal(t, count, 0)
		})
		// the rest of the tests are really just "tempt the
		// race detector"
		t.Run("ManagedLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := NewHandler(func(_ context.Context, in int) error {
				count++
				check.Equal(t, in, 42)
				return nil
			})

			wg := &WaitGroup{}
			oe := FromHandler(func(err error) { must(err) })
			op = op.Lock()

			ft.CallTimes(128, func() { oe(ctx, op(ctx, 42)) })
			wg.Wait(ctx)
			assert.Equal(t, count, 128)
		})
		t.Run("CustomLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := NewHandler(func(_ context.Context, in int) error {
				count++
				check.Equal(t, in, 42)
				return nil
			})
			mu := &sync.Mutex{}
			wf := op.WithLock(mu)
			wg := &WaitGroup{}
			wg.Group(128, func(context.Context) {
				assert.NotError(t, wf(ctx, 42))
			}).Run(ctx)
			wg.Wait(ctx)

			assert.Equal(t, count, 128)
		})
		t.Run("Locker", func(t *testing.T) {
			t.Run("Mutex", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				count := 0
				op := NewHandler(func(_ context.Context, in int) error {
					count++
					check.Equal(t, in, 42)
					return nil
				})
				mu := &sync.Mutex{}
				wf := op.WithLocker(mu)
				wg := &WaitGroup{}
				wg.Group(128, func(context.Context) {
					assert.NotError(t, wf(ctx, 42))
				}).Run(ctx)
				wg.Wait(ctx)

				assert.Equal(t, count, 128)
			})
			t.Run("RWMutex", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				count := 0
				op := NewHandler(func(_ context.Context, in int) error {
					count++
					check.Equal(t, in, 42)
					return nil
				})
				mu := &sync.RWMutex{}
				wf := op.WithLocker(mu)
				wg := &WaitGroup{}
				wg.Group(128, func(context.Context) {
					assert.NotError(t, wf(ctx, 42))
				}).Run(ctx)
				wg.Wait(ctx)

				assert.Equal(t, count, 128)
			})
		})
	})
	t.Run("WithoutErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var err error
		var pf Handler[int] = func(_ context.Context, in int) error { count++; assert.Equal(t, in, 42); return err }

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
	t.Run("Join", func(t *testing.T) {
		onect, twoct := 0, 0
		reset := func() { onect, twoct = 0, 0 }
		t.Run("Basic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var one Handler[string] = func(ctx context.Context, in string) error { onect++; check.Equal(t, in, t.Name()); return ctx.Err() }
			var two Handler[string] = func(ctx context.Context, in string) error { twoct++; check.Equal(t, in, t.Name()); return ctx.Err() }

			pf := one.Join(two)
			check.NotError(t, pf(ctx, t.Name()))
			check.Equal(t, onect, 1)
			check.Equal(t, twoct, 1)
			reset()
		})
		t.Run("Canceled", func(t *testing.T) {
			var one Handler[string] = func(ctx context.Context, in string) error { onect++; check.Equal(t, in, t.Name()); return ctx.Err() }
			var two Handler[string] = func(ctx context.Context, in string) error { twoct++; check.Equal(t, in, t.Name()); return ctx.Err() }

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
			one := Handler[string](func(_ context.Context, in string) error {
				defer close(sig1)
				<-sig2
				onect++
				check.Equal(t, in, t.Name())

				return nil
			})
			two := Handler[string](func(_ context.Context, in string) error {
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
	t.Run("JoinedConstructor", func(t *testing.T) {
		counter := 0
		reset := func() { counter = 0 }
		pf := MakeHandler(func(in int) error { counter++; assert.Equal(t, in, 42); return nil })
		t.Run("All", func(t *testing.T) {
			defer reset()
			pg := JoinHandlers(pf, pf, pf, pf, pf, pf, pf, pf)
			assert.NotError(t, pg.Wait(42))
			assert.Equal(t, counter, 8)
		})
		t.Run("WithNils", func(t *testing.T) {
			defer reset()
			pg := JoinHandlers(pf, nil, pf, pf, nil, pf, pf, pf, pf, pf, nil)
			assert.NotError(t, pg.Wait(42))
			assert.Equal(t, counter, 8)
		})
		t.Run("Empty", func(t *testing.T) {
			pg := JoinHandlers[int]()
			assert.Nil(t, pg)
		})
	})
	t.Run("PreHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := NewHandler(func(_ context.Context, in int) error {
				check.Equal(t, in, 42)
				assert.Equal(t, count, 1)
				count++
				return nil
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err := pf(ctx, 42)
			check.Error(t, err)
			check.ErrorIs(t, err, root)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := NewHandler(func(_ context.Context, in int) error {
				check.Equal(t, in, 42)
				assert.Equal(t, count, 1)
				count++
				return nil
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++ })
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
			pf := NewHandler(func(_ context.Context, in int) error {
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
			pf := NewHandler(func(_ context.Context, in int) error {
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return nil }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return expected }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return expected }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count++; return expected }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
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
			var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
			ts := time.Now().Add(100 * time.Millisecond)
			wf = wf.After(ts)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					check.True(t, wf.Check(ctx, 42))
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
		var wf Handler[int] = func(_ context.Context, in int) error { check.Equal(t, in, 42); count.Add(1); return nil }
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

		t.Log(dur)
		assert.True(t, dur >= time.Millisecond)
		assert.True(t, dur < 5*time.Millisecond)
	})
	t.Run("Filter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc := MakeHandler(func(in int) error {
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
	t.Run("Group", func(t *testing.T) {
		t.Run("Noop", func(t *testing.T) {
			check.Nil(t, JoinHandlers[int](nil, nil))
			check.Nil(t, JoinHandlers[int]())
		})
		t.Run("Single", func(t *testing.T) {
			var count int
			pf := MakeHandler(func(in int) error { count++; check.Equal(t, in, 42); return nil })
			op := JoinHandlers(pf)
			check.Equal(t, 0, count)
			assert.NotError(t, op.Wait(42))
			check.Equal(t, 1, count)
		})
		t.Run("Multi", func(t *testing.T) {
			var count int
			pf := MakeHandler(func(in int) error { count++; check.Equal(t, in, 42); return nil })
			op := JoinHandlers(pf, pf, pf, pf)

			check.Equal(t, 0, count)
			assert.NotError(t, op.Wait(42))
			check.Equal(t, 4, count)
		})
	})
	t.Run("Must", func(t *testing.T) {
		called := 0
		assert.NotPanic(t, func() {
			MakeHandler(func(in int) error {
				called++
				check.Equal(t, in, 42)
				return nil
			}).Must(t.Context(), 42)
		})
		assert.Equal(t, called, 1)

		assert.Panic(t, func() {
			MakeHandler(func(in int) error {
				called++
				check.Equal(t, in, 42)
				return errors.New("beep")
			}).Must(t.Context(), 42)
		})
		assert.Equal(t, called, 2)

		err := ft.WithRecoverCall(func() {
			MakeHandler(func(in int) error {
				called++
				check.Equal(t, in, 42)
				return errors.New("beep")
			}).Must(t.Context(), 42)
		})
		assert.Equal(t, called, 3)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
	})
	t.Run("WithRecover", func(t *testing.T) {
		phf := MakeHandler(func(int) error { panic(42) })
		t.Run("ValidateFixture", func(t *testing.T) {
			assert.Panic(t, func() { _ = phf.Wait(42) })
		})

		t.Run("ConvertsToError", func(t *testing.T) {
			var err error
			assert.NotPanic(t, func() { err = phf.WithRecover().Wait(42) })
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
	})
}
