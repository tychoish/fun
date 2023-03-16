package fun

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Functions", func(t *testing.T) {
		t.Run("Blocking", func(t *testing.T) {
			start := time.Now()
			err := WorkerFunc(func(ctx context.Context) error { time.Sleep(10 * time.Millisecond); return nil }).Block()
			dur := time.Since(start)
			if dur < 10*time.Millisecond {
				t.Error("did not block long enough", dur)
			}
			if dur > 12*time.Millisecond {
				t.Error("did not block long enough", dur)
			}
			assert.NotError(t, err)
		})
		t.Run("Observe", func(t *testing.T) {
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
			if dur > 11*time.Millisecond {
				t.Error(dur)
			}
			check.NotError(t, err)
		})
	})
	t.Run("PoolIntegration", func(t *testing.T) {
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
					time.Sleep(time.Millisecond)
					cancel()
				}()

				wait := ObserveWorkerFuncs(ctx, internal.NewSliceIter(workers), func(error) {})

				wait(internal.BackgroundContext)

				check.Equal(t, counter.Load(), 100)
			})
			t.Run("ParallelObserve", func(t *testing.T) {
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

				wait := ObserveWorkerPool(ctx, 5, internal.NewSliceIter(workers), func(error) {})

				wait(internal.BackgroundContext)

				check.Equal(t, counter.Load(), 5)
			})

		})

	})

}
