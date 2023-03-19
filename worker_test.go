package fun

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestWorker(t *testing.T) {
	t.Run("Functions", func(t *testing.T) {
		t.Run("Blocking", func(t *testing.T) {
			start := time.Now()
			err := WorkerFunc(func(ctx context.Context) error { time.Sleep(10 * time.Millisecond); return nil }).Block()
			dur := time.Since(start)
			if dur < 10*time.Millisecond {
				t.Error("did not block long enough", dur)
			}
			if dur > 15*time.Millisecond {
				t.Error("blocked too long", dur)
			}
			assert.NotError(t, err)
		})
		t.Run("Observe", func(t *testing.T) {
			ctx := testt.Context(t)
			t.Run("WithoutError", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				WorkerFunc(func(ctx context.Context) error { called.Store(true); return nil }).Observe(ctx, func(error) { observed.Store(true) })
				assert.True(t, called.Load())
				assert.True(t, !observed.Load())
			})
			t.Run("WithError", func(t *testing.T) {
				called := &atomic.Bool{}
				observed := &atomic.Bool{}
				expected := errors.New("hello")
				WorkerFunc(func(ctx context.Context) error {
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
				wf := WorkerFunc(func(ctx context.Context) error {
					called.Store(true)
					return expected
				}).ObserveWait(func(err error) {
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
				err := Check(func() {
					var wf WaitFunc //nolint:gosimple
					wf = WorkerFunc(func(context.Context) error {
						panic(expected)
					}).MustWait()
					t.Log(wf)
				})
				// declaration shouldn't call
				assert.NotError(t, err)

				err = Check(func() {
					var wf WaitFunc //nolint:gosimple
					wf = WorkerFunc(func(context.Context) error {
						panic(expected)
					}).MustWait()
					wf(testt.Context(t))
				})
				assert.Error(t, err)
				assert.ErrorIs(t, err, expected)
			})
		})
		t.Run("Timeout", func(t *testing.T) {
			timer := testt.Timer(t, time.Hour)
			start := time.Now()
			err := WorkerFunc(func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return nil
				case <-timer.C:
					t.Fatal("timer must not fail")
				}
				return errors.New("should not exist")
			}).WithTimeout(10 * time.Millisecond)
			dur := time.Since(start)
			if dur > 20*time.Millisecond {
				t.Error(dur)
			}
			check.NotError(t, err)
		})
		t.Run("Signal", func(t *testing.T) {
			ctx := testt.Context(t)
			expected := errors.New("hello")
			wf := WorkerFunc(func(ctx context.Context) error { return expected })
			out := wf.Singal(ctx)
			err := <-out
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})
	t.Run("PoolIntegration", func(t *testing.T) {
		ctx := testt.Context(t)
		t.Run("Unbounded", func(t *testing.T) {
			const num = 100
			called := &atomic.Int64{}
			workers := make([]WorkerFunc, num)
			for i := range workers {
				workers[i] = func(context.Context) error { called.Add(1); return nil }
			}

			wait := ObserveWorkerFuncs(ctx, internal.NewSliceIter(workers), func(error) {})

			wait(ctx)

			assert.Equal(t, called.Load(), num)
		})
		t.Run("ParallelObserve", func(t *testing.T) {
			const num = 100

			called := &atomic.Int64{}
			workers := make([]WorkerFunc, num)
			for i := range workers {
				workers[i] = func(context.Context) error { called.Add(1); return nil }
			}

			wait := ObserveWorkerPool(ctx, 5, internal.NewSliceIter(workers), func(error) {})

			wait(ctx)

			assert.Equal(t, called.Load(), num)
		})
		t.Run("MinimumWorkerPoolSizeIsOne", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			const num = 100
			workers := make([]WorkerFunc, num)
			counter := &atomic.Int64{}
			for i := range workers {
				workers[i] = func(context.Context) error { time.Sleep(100 * time.Millisecond); counter.Add(1); return nil }
			}
			go func() {
				time.Sleep(time.Millisecond)
				cancel()
			}()

			wait := ObserveWorkerPool(ctx, 0, internal.NewSliceIter(workers), func(error) {})

			wait(internal.BackgroundContext)

			check.Equal(t, counter.Load(), 1)
		})
		t.Run("ObserverPoolErrorCheck", func(t *testing.T) {
			t.Parallel()
			called := &atomic.Bool{}
			mock := &mockErrIter[int]{err: errors.New("kip")}
			wf := ObservePool(testt.Context(t), 1,
				Iterator[int](mock),
				func(in int) { called.Store(true) },
			)

			if called.Load() {
				t.Error("should not observe any")
			}

			err := wf(ctx)
			if mock.nextCalls.Load() != 1 {
				t.Error("pool should execute when worker function is called")
			}

			if err == nil {
				t.Error("should have errored")
			}
			t.Log(err)
			if err.Error() != "kip" {
				t.Error("unexpected error", err)
			}
		})
		t.Run("ObserverPoolNoError", func(t *testing.T) {
			t.Parallel()
			called := &atomic.Bool{}
			mock := &mockNoErrIter[int]{}
			wf := ObservePool(testt.Context(t), 1,
				Iterator[int](mock),
				func(in int) { called.Store(true) },
			)

			if called.Load() {
				t.Error("should not observe any")
			}

			err := wf(ctx)

			if mock.nextCalls.Load() != 1 {
				t.Error("pool should execute when worker function is called")
			}

			if err != nil {
				t.Errorf("should not have errored, %T", err)
			}
		})
		t.Run("ContextCacelation", func(t *testing.T) {
			t.Run("Unbounded", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				const num = 100
				workers := make([]WorkerFunc, num)
				counter := &atomic.Int64{}
				for i := range workers {
					workers[i] = func(context.Context) error { time.Sleep(100 * time.Millisecond); counter.Add(1); return nil }
				}

				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()

				wait := ObserveWorkerFuncs(ctx, internal.NewSliceIter(workers), func(error) {})

				wait(internal.BackgroundContext)

				check.Equal(t, counter.Load(), 100)
			})
			t.Run("ParallelObserve", func(t *testing.T) {
				ctx, cancel := context.WithCancel(testt.Context(t))
				timer := testt.Timer(t, time.Hour)
				const num = 100
				workers := make([]WorkerFunc, num)
				counter := &atomic.Int64{}
				for i := range workers {
					workers[i] = func(context.Context) error {
						select {
						case <-timer.C:
							t.Error("timer should never fire")
						case <-ctx.Done():
							counter.Add(1)
						}
						return nil
					}
				}

				wait := ObserveWorkerPool(ctx, 5, internal.NewSliceIter(workers), func(error) {})

				runtime.Gosched()

				time.Sleep(100 * time.Millisecond)
				cancel()

				wait(testt.ContextWithTimeout(t, time.Second))

				check.Equal(t, counter.Load(), 5)
			})
		})
	})
}

type mockErrIter[T any] struct {
	nextCalls atomic.Int64
	err       error
}

func (e *mockErrIter[T]) Close() error              { return e.err }
func (e *mockErrIter[T]) Next(context.Context) bool { e.nextCalls.Add(1); return false }
func (e *mockErrIter[T]) Value() T                  { return ZeroOf[T]() }

type mockNoErrIter[T any] struct{ nextCalls atomic.Int64 }

func (e *mockNoErrIter[T]) Close() error              { return nil }
func (e *mockNoErrIter[T]) Next(context.Context) bool { e.nextCalls.Add(1); return false }
func (e *mockNoErrIter[T]) Value() T                  { return ZeroOf[T]() }
