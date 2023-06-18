package fun

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

func TestProducer(t *testing.T) {
	t.Run("WithCancel", func(t *testing.T) {
		ctx := testt.Context(t)
		wf, cancel := Producer[int](func(ctx context.Context) (int, error) {
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
			assert.MaxRuntime(t, 75*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				out, err := wf(ctx)
				check.Equal(t, out, 42)
				check.NotError(t, err)
			})
		})
	})
	t.Run("Once", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Producer[int](func(ctx context.Context) (int, error) {
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
		ctx := testt.Context(t)
		called := 0
		pf := Producer[int](func(ctx context.Context) (int, error) {
			called++
			return 42, nil
		})

		check.Equal(t, 0, pf.If(false).Must(ctx))
		check.Equal(t, 0, called)
		check.Equal(t, 42, pf.If(true).Must(ctx))
		check.Equal(t, 1, called)
		check.Equal(t, 42, pf.If(true).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 0, pf.If(false).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 42, pf.Must(ctx))
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Producer[int](func(ctx context.Context) (int, error) {
			called++
			return 42, nil
		})

		check.Equal(t, 0, pf.When(func() bool { return false }).Must(ctx))
		check.Equal(t, 0, called)
		check.Equal(t, 42, pf.When(func() bool { return true }).Must(ctx))
		check.Equal(t, 1, called)
		check.Equal(t, 42, pf.When(func() bool { return true }).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 0, pf.When(func() bool { return false }).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 42, pf.Must(ctx))
		check.Equal(t, 3, called)
	})
	t.Run("Constructor", func(t *testing.T) {
		t.Run("Value", func(t *testing.T) {
			ctx := testt.Context(t)
			pf := ValueProducer(42)
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.NotError(t, err)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Static", func(t *testing.T) {
			ctx := testt.Context(t)
			root := ers.Error(t.Name())
			pf := StaticProducer(42, root)
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.ErrorIs(t, err, root)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Future", func(t *testing.T) {
			ctx := testt.Context(t)
			ch := make(chan *string, 2)
			prod := MakeFuture(ch)
			ch <- Ptr("hi")
			ch <- Ptr("hi")
			check.NotZero(t, prod.Must(ctx))
		})
		t.Run("Blocking", func(t *testing.T) {
			ctx := testt.Context(t)
			root := ers.Error(t.Name())
			pf := BlockingProducer(func() (int, error) { return 42, root })
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.ErrorIs(t, err, root)
				assert.Equal(t, v, 42)
			}
		})

		t.Run("Consistent", func(t *testing.T) {
			ctx := testt.Context(t)
			pf := ConsistentProducer(func() int { return 42 })
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.Equal(t, v, 42)
				assert.NotError(t, err)
			}
		})
	})
}
