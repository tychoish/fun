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
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
)

func TestGenerator(t *testing.T) {
	t.Parallel()
	t.Run("WithCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wf, cancel := NewGenerator(func(ctx context.Context) (int, error) {
			timer := time.NewTimer(time.Hour)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				check.ErrorIs(t, ctx.Err(), context.Canceled)
				return 42, nil
			case <-timer.C:
				t.Error("should not have reached this timeout")
			}
			return -1, ers.Error("unreachable")
		}).WithCancel()
		assert.MinRuntime(t, 40*time.Millisecond, func() {
			assert.MaxRuntime(t, 100*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				out, err := wf(ctx)
				check.Equal(t, out, 42)
				check.NotError(t, err)
			})
		})
	})
	t.Run("Once", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		called := 0
		pf := Generator[int](func(_ context.Context) (int, error) {
			called++
			return 42, nil
		}).Once()
		for i := 0; i < 1024; i++ {
			val, err := pf(ctx)
			assert.NotError(t, err)
			assert.Equal(t, val, 42)
		}
		check.Equal(t, called, 1)
	})
	t.Run("If", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		pf := Generator[int](func(_ context.Context) (int, error) {
			called++
			return 42, nil
		})

		check.Equal(t, 0, pf.If(false).Must(ctx).Resolve())
		check.Equal(t, 0, called)
		check.Equal(t, 42, pf.If(true).Must(ctx).Resolve())
		check.Equal(t, 1, called)
		check.Equal(t, 42, pf.If(true).Must(ctx).Resolve())
		check.Equal(t, 2, called)
		check.Equal(t, 0, pf.If(false).Must(ctx).Resolve())
		check.Equal(t, 2, called)
		check.Equal(t, 42, pf.Must(ctx).Resolve())
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		called := 0
		pf := Generator[int](func(_ context.Context) (int, error) {
			called++
			return 42, nil
		})

		check.Equal(t, 0, pf.When(func() bool { return false }).Must(ctx).Resolve())
		check.Equal(t, 0, called)
		check.Equal(t, 42, pf.When(func() bool { return true }).Must(ctx).Resolve())
		check.Equal(t, 1, called)
		check.Equal(t, 42, pf.When(func() bool { return true }).Must(ctx).Resolve())
		check.Equal(t, 2, called)
		check.Equal(t, 0, pf.When(func() bool { return false }).Must(ctx).Resolve())
		check.Equal(t, 2, called)
		check.Equal(t, 42, pf.Must(ctx).Resolve())
		check.Equal(t, 3, called)
	})
	t.Run("Constructor", func(t *testing.T) {
		t.Run("Value", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			pf := ValueGenerator(42)
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.NotError(t, err)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Ptr", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				pf := PtrGenerator(func() *int { return nil })
				ctx := t.Context()

				for i := 0; i < 1024; i++ {
					v, err := pf(ctx)
					assert.ErrorIs(t, err, io.EOF)
					assert.Zero(t, v)
				}
			})
			t.Run("Always", func(t *testing.T) {
				pf := PtrGenerator(func() *int { return ft.Ptr(42) })
				ctx := t.Context()

				for i := 0; i < 1024; i++ {
					v, err := pf(ctx)
					assert.NotError(t, err)
					assert.Equal(t, v, 42)
				}
			})
			t.Run("Few", func(t *testing.T) {
				count := 0
				pf := PtrGenerator(func() *int {
					if count < 512 {
						count++
						return ft.Ptr(42)
					}
					return nil
				})
				ctx := t.Context()

				fortyTwos := 0
				errs := 0
				for i := 0; i < 1024; i++ {
					v, err := pf(ctx)
					if err != nil {
						errs++
						assert.Zero(t, v)

						continue
					}
					fortyTwos++
					assert.Equal(t, v, 42)
				}
				assert.Equal(t, fortyTwos, 512)
				assert.Equal(t, errs, 512)
			})
		})
		t.Run("Static", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			root := ers.Error(t.Name())
			pf := StaticGenerator(42, root)
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.ErrorIs(t, err, root)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Blocking", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			root := ers.Error(t.Name())
			pf := MakeGenerator(func() (int, error) { return 42, root })
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.ErrorIs(t, err, root)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Future", func(t *testing.T) {
			callCount := 0
			errCount := 0
			root := ers.Error(t.Name())

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			pf := MakeGenerator(func() (int, error) { callCount++; return 42, root })
			ff := pf.Future(ctx, func(err error) { errCount++; assert.ErrorIs(t, err, root) })
			for i := 0; i < 1024; i++ {
				v := ff()
				assert.Equal(t, v, 42)
			}
			check.Equal(t, callCount, 1024)
			check.Equal(t, errCount, 1024)
		})

		t.Run("Consistent", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			pf := FutureGenerator(func() int { return 42 })
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.Equal(t, v, 42)
				assert.NotError(t, err)
			}
		})
	})
	t.Run("Lock", func(t *testing.T) {
		t.Run("NilLockPanics", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := Generator[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			})
			check.Panic(t, func() {
				v, err := op.WithLock(nil).Read(ctx)
				check.Equal(t, v, 0)
				check.Error(t, err)
			})
			check.Equal(t, count, 0)
		})
		// the rest of the tests are really just "tempt the
		// race detector"
		t.Run("ManagedLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := Generator[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			})

			opct := &atomic.Int64{}
			wg := &WaitGroup{}
			future := op.Lock()

			wg.Group(128, func(ctx context.Context) {
				out, err := future(ctx)
				opct.Add(1)
				check.Equal(t, out, 42)
				assert.NotError(t, err)
			}).Run(ctx)

			wg.Wait(ctx)

			check.Equal(t, 128, opct.Load())
			assert.Equal(t, count, 128)

		})
		t.Run("Locker", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			op := Generator[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			})

			opct := &atomic.Int64{}
			mtx := &sync.RWMutex{}
			future := op.WithLocker(mtx)
			wg := &WaitGroup{}

			wg.Group(128, func(ctx context.Context) {
				out, err := future(ctx)
				opct.Add(1)
				check.Equal(t, out, 42)
				assert.NotError(t, err)
			}).Run(ctx)
			wg.Wait(ctx)

			check.Equal(t, 128, opct.Load())
			assert.Equal(t, count, 128)
		})
		t.Run("CustomLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mu := &sync.Mutex{}
			opct := &atomic.Int64{}
			wg := &WaitGroup{}

			count := 0
			op := Generator[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			}).WithLock(mu)

			wg.Group(128, func(ctx context.Context) {
				out, err := op(ctx)
				opct.Add(1)
				check.Equal(t, out, 42)
				opct.Add(1)
				check.NotError(t, err)
			}).Run(ctx)

			wg.Wait(ctx)

			check.Equal(t, 2*128, opct.Load())
			assert.Equal(t, count, 128)
		})
	})
	t.Run("WithoutErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var err error
		var pf Generator[int] = func(_ context.Context) (int, error) { count++; return 42, err }

		checkOutput := func(in int, err error) error { check.Equal(t, 42, in); return err }

		err = ers.Error("hello")
		pf = pf.WithoutErrors(io.EOF)
		assert.Equal(t, count, 0)
		assert.Error(t, checkOutput(pf(ctx)))
		assert.Equal(t, count, 1)
		err = io.EOF
		assert.NotError(t, checkOutput(pf(ctx)))
		assert.Equal(t, count, 2)
		err = context.Canceled
		assert.Error(t, checkOutput(pf(ctx)))
		assert.Equal(t, count, 3)
	})
	t.Run("Force", func(t *testing.T) {
		count := 0
		var err = io.EOF
		var pf Generator[int] = func(_ context.Context) (int, error) { count++; return 42, err }
		var out int

		assert.NotPanic(t, func() { out = pf.Force().Resolve() })
		assert.Equal(t, out, 42)
		assert.Equal(t, count, 1)
		err = nil
		assert.Equal(t, 42, pf.Force().Resolve())
	})
	t.Run("Capture", func(t *testing.T) {
		count := 0
		var err = io.EOF
		var pf = MakeGenerator(func() (int, error) { count++; return 42, err })
		var out int

		assert.NotPanic(t, func() { out = pf.Capture().Resolve() })
		assert.Equal(t, out, 42)
		assert.Equal(t, count, 1)
		err = nil
		assert.Equal(t, 42, pf.Capture().Resolve())

	})
	t.Run("Ignore", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var err = io.EOF
		var pf Generator[int] = func(_ context.Context) (int, error) { count++; return 42, err }
		var out int

		assert.NotPanic(t, func() { out = pf.Ignore(ctx).Resolve() })
		assert.Equal(t, out, 42)
		assert.Equal(t, count, 1)
		err = nil
		assert.Equal(t, 42, pf.Force().Resolve())
	})
	t.Run("Must", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var err = io.EOF
		var pf Generator[int] = func(_ context.Context) (int, error) { count++; return 42, err }
		var out int

		assert.Panic(t, func() { out = pf.Must(ctx).Resolve() })
		assert.Equal(t, out, 0)
		assert.Equal(t, count, 1)
		err = nil
		assert.Equal(t, 42, pf.Force().Resolve())
	})
	t.Run("Block", func(t *testing.T) {
		count := 0
		var pf Generator[int] = func(_ context.Context) (int, error) { count++; return 42, io.EOF }
		out, err := pf.Wait()
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, out, 42)
		assert.Equal(t, count, 1)
	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("Exhausted", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			var pf Generator[int] = func(_ context.Context) (int, error) { count++; return -1, io.EOF }
			pf = pf.Join(pf)
			out, err := pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
			assert.Equal(t, 0, out)

			assert.Equal(t, 2, count)

			out, err = pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
			assert.Equal(t, 0, out)
		})
		t.Run("FirstContinues", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			counter := &atomic.Int64{}

			pf := producerContinuesOnce(42, counter)
			pf = pf.Join(producerContinuesOnce(42, counter))
			check.Equal(t, counter.Load(), 0)

			val, err := pf(ctx)
			assert.NotError(t, err)
			assert.Equal(t, val, 0)
			check.Equal(t, counter.Load(), 2)

			val, err = pf(ctx)
			assert.Error(t, err)
			assert.Equal(t, val, 0)
			check.Equal(t, counter.Load(), 4)
		})
		t.Run("SecondContinues", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			counter := &atomic.Int64{}
			pf := Generator[int](func(_ context.Context) (int, error) {
				return -1, io.EOF
			}).Join(producerContinuesOnce(42, counter))
			out, err := pf(ctx)
			assert.NotError(t, err)
			assert.Zero(t, out)
			assert.Equal(t, counter.Load(), 2)
		})
		t.Run("ErrorFirstCanceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			count := 0
			pf := Generator[int](func(_ context.Context) (int, error) {
				return -1, context.Canceled
			}).Join(func(_ context.Context) (int, error) { count++; return 300, nil })
			out, err := pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Zero(t, count)

			// should repeat the second time
			out, err = pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Zero(t, count)
		})
		t.Run("SecondCancled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			counter := 0
			pf := Generator[int](func(_ context.Context) (int, error) {
				return -1, io.EOF
			}).Join(func(_ context.Context) (int, error) { counter++; return 300, context.Canceled })
			out, err := pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Equal(t, counter, 1)

			// should repeat the second time
			out, err = pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Equal(t, counter, 1)
		})
	})
	t.Run("PreHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Generator[int](func(_ context.Context) (int, error) {
				assert.Equal(t, count, 1)
				count++
				return 42, nil
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			val, err := pf(ctx)
			check.Error(t, err)
			check.Equal(t, val, 42)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Generator[int](func(_ context.Context) (int, error) {
				assert.Equal(t, count, 1)
				count++
				return 42, nil
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++ })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			val, err := pf(ctx)
			check.NotError(t, err)
			check.Equal(t, val, 42)
			check.Equal(t, 2, count)
		})
	})
	t.Run("PostHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Generator[int](func(_ context.Context) (int, error) {
				assert.Zero(t, count)
				count++
				return 42, nil
			}).PostHook(func() { assert.Equal(t, count, 1); count++; panic(root) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			val, err := pf(ctx)
			check.Error(t, err)
			check.Equal(t, val, 42)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Generator[int](func(_ context.Context) (int, error) {
				assert.Zero(t, count)
				count++
				return 42, nil
			}).PostHook(func() { assert.Equal(t, count, 1); count++ })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			val, err := pf(ctx)
			check.NotError(t, err)
			check.Equal(t, val, 42)
			check.Equal(t, 2, count)
		})
	})
	t.Run("ErrorCheck", func(t *testing.T) {
		t.Run("ShortCircut", func(t *testing.T) {
			err := errors.New("test error")
			var hf fn.Future[error] = func() error { return err }
			called := 0
			pf := MakeGenerator(func() (int, error) { called++; return 42, nil })
			ecpf := pf.WithErrorCheck(hf)
			out, e := ecpf.Wait()
			check.Error(t, e)
			check.ErrorIs(t, e, err)
			check.Zero(t, out)
			check.Equal(t, 0, called)

		})
		t.Run("Noop", func(t *testing.T) {
			var hf fn.Future[error] = func() error { return nil }
			called := 0
			pf := MakeGenerator(func() (int, error) { called++; return 42, nil })
			ecpf := pf.WithErrorCheck(hf)
			out, e := ecpf.Wait()
			check.NotError(t, e)
			check.Equal(t, 42, out)
			check.Equal(t, 1, called)
		})
		t.Run("Multi", func(t *testing.T) {
			called := 0
			hfcall := 0
			var hf fn.Future[error] = func() error {
				hfcall++
				switch hfcall {
				case 1, 2, 3:
					return nil
				case 4, 5:
					return ers.ErrCurrentOpAbort
				}
				return errors.New("unexpected error")
			}
			wf := MakeGenerator(func() (int, error) {
				called++
				if hfcall == 3 {
					return 0, io.EOF
				}
				return 42, nil
			})
			ecpf := wf.WithErrorCheck(hf)
			out, e := ecpf.Wait()
			check.NotError(t, e)
			check.Equal(t, out, 42)
			check.Equal(t, 1, called)
			check.Equal(t, 2, hfcall)

			out, e = ecpf.Wait()
			check.Equal(t, out, 0)
			check.Error(t, e)
			check.Equal(t, 2, called)
			check.Equal(t, 4, hfcall)
			check.ErrorIs(t, e, ers.ErrCurrentOpAbort)
			check.ErrorIs(t, e, io.EOF)

			out, e = ecpf.Wait()
			check.Error(t, e)
			check.Equal(t, out, 0)
			check.Equal(t, 2, called)
			check.Equal(t, 5, hfcall)
			check.ErrorIs(t, e, ers.ErrCurrentOpAbort)
			check.NotErrorIs(t, e, io.EOF)
		})

	})
	t.Run("Limit", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			var wf Generator[int] = func(_ context.Context) (int, error) { count++; return 42, nil }
			wf = wf.Limit(10)
			for i := 0; i < 100; i++ {
				out, err := wf(ctx)
				check.Equal(t, 42, out)
				check.NotError(t, err)
			}
			assert.Equal(t, count, 10)
		})
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Generator[int] = func(_ context.Context) (int, error) { count.Add(1); return 42, nil }
			wf = wf.Limit(10)
			wg := &sync.WaitGroup{}
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); out, err := wf(ctx); check.Equal(t, 42, out); check.NotError(t, err) }()
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

			count := &atomic.Int64{}
			var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
			wf = wf.TTL(0)
			for i := 0; i < 100; i++ {
				out, err := wf(ctx)
				check.ErrorIs(t, err, expected)
				check.Equal(t, out, 42)
			}
			check.Equal(t, 100, count.Load())
		})
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")

			count := &atomic.Int64{}
			var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
			wf = wf.TTL(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				out, err := wf(ctx)
				check.ErrorIs(t, err, expected)
				check.Equal(t, out, 42)
			}
			check.Equal(t, 1, count.Load())
			time.Sleep(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				out, err := wf(ctx)
				check.ErrorIs(t, err, expected)
				check.Equal(t, out, 42)
			}
			check.Equal(t, 2, count.Load())
		})
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			expected := errors.New("cat")
			wg := &sync.WaitGroup{}

			count := &atomic.Int64{}
			var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
			wf = wf.TTL(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					out, err := wf(ctx)
					check.ErrorIs(t, err, expected)
					check.Equal(t, out, 42)
				}()
			}
			wg.Wait()
			check.Equal(t, 1, count.Load())
			time.Sleep(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					out, err := wf(ctx)
					check.ErrorIs(t, err, expected)
					check.Equal(t, out, 42)
				}()
			}
			wg.Wait()
			check.Equal(t, 2, count.Load())

		})
	})
	t.Run("Delay", func(t *testing.T) {
		expected := errors.New("cat")
		t.Run("Basic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					out, err := wf(ctx)
					check.ErrorIs(t, err, expected)
					check.Equal(t, out, 42)
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
			var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			cancel()
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					out, err := wf(ctx)
					check.ErrorIs(t, err, context.Canceled)
					check.Equal(t, out, 0)
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
			var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
			ts := time.Now().Add(100 * time.Millisecond)
			wf = wf.After(ts)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					out, err := wf(ctx)
					check.ErrorIs(t, err, expected)
					check.Equal(t, out, 42)
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

		expected := errors.New("cat")

		count := &atomic.Int64{}
		var wf Generator[int] = func(context.Context) (int, error) { count.Add(1); return 42, expected }
		delay := 100 * time.Millisecond
		wf = wf.Jitter(func() time.Duration { return delay })
		start := time.Now()
		out, err := wf(ctx)
		check.ErrorIs(t, err, expected)
		check.Equal(t, out, 42)
		dur := time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= 100*time.Millisecond)
		assert.True(t, dur < 200*time.Millisecond)

		delay = 10 * time.Millisecond
		start = time.Now()

		out, err = wf(ctx)
		check.ErrorIs(t, err, expected)
		check.Equal(t, out, 42)

		dur = time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= 10*time.Millisecond)
		assert.True(t, dur < 20*time.Millisecond)
	})
	t.Run("CheckGenerator", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ss := &erc.Stack{}
		ss.Add(ers.ErrInvariantViolation, ers.ErrRecoveredPanic, context.Canceled, io.EOF, ErrNonBlockingChannelOperationSkipped)
		stack := &erc.Stack{}
		assert.True(t, errors.As(ss.Resolve(), &stack))
		errs := ft.Must(CheckedGenerator(stack.Generator()).Stream().Slice(ctx))
		assert.Equal(t, 5, len(errs))
	})
	t.Run("Retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("Skip", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			val, err := MakeGenerator(func() (int, error) {
				defer count.Add(1)
				if count.Get() == 0 {
					return 100, ers.ErrCurrentOpSkip
				}
				return 42, nil
			}).Retry(5).Read(ctx)
			assert.Equal(t, count.Get(), 2)
			assert.NotError(t, err)
			assert.Equal(t, val, 42)
			assert.NotEqual(t, val, 100)
		})
		t.Run("FirstTry", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			val, err := MakeGenerator(func() (int, error) {
				defer count.Add(1)
				return 42, nil
			}).Retry(10).Read(ctx)
			assert.Equal(t, count.Get(), 1)
			assert.NotError(t, err)
			assert.Equal(t, val, 42)
			assert.NotEqual(t, val, 100)
		})
		t.Run("ArbitraryError", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			val, err := MakeGenerator(func() (int, error) {
				defer count.Add(1)
				if count.Get() < 3 {
					return 100, errors.New("why not")
				}
				return 42, nil
			}).Retry(10).Read(ctx)
			assert.Equal(t, count.Get(), 4)
			assert.NotError(t, err)
			assert.Equal(t, val, 42)
			assert.NotEqual(t, val, 100)
		})
		t.Run("DoesFail", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			exp := errors.New("why not")
			val, err := MakeGenerator(func() (int, error) {
				defer count.Add(1)
				return 100, exp
			}).Retry(16).Read(ctx)
			assert.Equal(t, count.Get(), 16)
			assert.Error(t, err)
			assert.Equal(t, val, 0)
			errs := ers.Unwind(err)
			assert.Equal(t, len(errs), 16)
			for _, err := range errs {
				assert.Equal(t, err, exp)
				assert.ErrorIs(t, err, exp)
			}
			assert.NotEqual(t, val, 100)

		})
		t.Run("Terminating", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			exp := errors.New("why not")
			val, err := MakeGenerator(func() (int, error) {
				defer count.Add(1)
				if count.Load() == 11 {
					return 100, ers.Join(exp, ers.ErrCurrentOpAbort)
				}
				return 100, exp
			}).Retry(16).Read(ctx)
			assert.Equal(t, count.Get(), 12)
			assert.Error(t, err)
			assert.ErrorIs(t, err, exp)
			assert.ErrorIs(t, err, ers.ErrCurrentOpAbort)
			assert.Equal(t, val, 0)
			assert.NotEqual(t, val, 100)
		})
		t.Run("Canceled", func(t *testing.T) {
			count := &intish.Atomic[int]{}
			exp := errors.New("why not")
			val, err := MakeGenerator(func() (int, error) {
				defer count.Add(1)
				if count.Load() == 11 {
					return 100, ers.Join(exp, context.Canceled)
				}
				return 100, exp
			}).Retry(16).Read(ctx)
			assert.Equal(t, count.Get(), 12)
			assert.Error(t, err)
			assert.ErrorIs(t, err, exp)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, val, 0)
			assert.NotEqual(t, val, 100)
		})
	})
	t.Run("ErrorHanldingOptions", func(t *testing.T) {
		g := MakeGenerator(func() (int, error) { panic(42) })
		g = g.Parallel(WorkerGroupConfWithErrorCollector(nil))
		out, err := g.Read(t.Context())
		assert.Zero(t, out)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
	})
	t.Run("WithErrorHandler", func(t *testing.T) {
		const (
			ErrUnseen ers.Error = "unseen"
			ErrSeen   ers.Error = "seen"
		)

		eh, er := func(error) {}, func() error { return ErrSeen }
		out, err := MakeGenerator(func() (int, error) { return 0, ErrUnseen }).WithErrorHandler(eh, er).Wait()
		assert.Error(t, err)
		assert.Zero(t, out)
		assert.ErrorIs(t, err, ErrSeen)
	})

}

func producerContinuesOnce[T any](out T, counter *atomic.Int64) Generator[T] {
	once := &sync.Once{}
	var zero T
	return func(_ context.Context) (_ T, err error) {
		once.Do(func() {
			out = zero
			err = ErrStreamContinue
		})
		if counter.Add(1) > 2 {
			return zero, io.EOF
		}

		return out, err
	}

}
