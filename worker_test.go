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
	"github.com/tychoish/fun/testt"
)

func TestWorker(t *testing.T) {
	t.Run("Functions", func(t *testing.T) {
		t.Run("Blocking", func(t *testing.T) {
			start := time.Now()
			err := WorkerFunc(func(ctx context.Context) error { time.Sleep(80 * time.Millisecond); return nil }).Block()
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
			called := &atomic.Int64{}
			expected := errors.New("foo")
			WorkerFunc(func(ctx context.Context) error { called.Add(1); panic(expected) }).
				Background(testt.Context(t), func(err error) {
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

				WorkerFunc(func(ctx context.Context) error {
					assert.Equal(t, wg.Num(), 1)
					count.Add(1)
					return nil
				}).Add(ctx, wg, func(err error) {
					assert.Equal(t, wg.Num(), 1)
					count.Add(1)
					assert.NotError(t, err)
				})
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
				WorkerFunc(func(ctx context.Context) error { called.Store(true); return nil }).Observe(ctx, func(error) { observed.Store(true) })
				assert.True(t, called.Load())
				assert.True(t, observed.Load())
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
}
