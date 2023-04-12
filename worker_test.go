package fun

import (
	"context"
	"errors"
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
}
