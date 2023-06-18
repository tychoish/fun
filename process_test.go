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

func TestProcess(t *testing.T) {
	t.Run("Risky", func(t *testing.T) {
		called := 0
		pf := BlockingProcessor(func(in int) error {
			check.Equal(t, in, 42)
			called++
			return nil
		})
		check.NotError(t, pf.Block(42))
		pf.Ignore(testt.Context(t), 42)
		pf.Force(42)
		check.Equal(t, called, 3)
	})
	t.Run("Run", func(t *testing.T) {
		called := 0
		pf := BlockingProcessor(func(in int) error {
			check.Equal(t, in, 42)
			called++
			return nil
		})

		//nolint:staticcheck
		check.Panic(t, func() { _ = pf.Run(nil, 42) })
		check.NotPanic(t, func() { check.NotError(t, pf(nil, 42)) })
	})
	t.Run("WithCancel", func(t *testing.T) {
		ctx := testt.Context(t)
		wf, cancel := Processor[int](func(ctx context.Context, in int) error {
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
			assert.MaxRuntime(t, 75*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				check.NotError(t, wf(ctx, 42))
			})
		})
	})
	t.Run("If", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Processor[int](func(ctx context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		})

		check.NotError(t, pf.If(false)(ctx, 42))
		check.Zero(t, called)
		check.NotError(t, pf.If(true)(ctx, 42))
		check.Equal(t, 1, called)
		check.NotError(t, pf.If(true)(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf.If(false)(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf(ctx, 42))
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Processor[int](func(ctx context.Context, n int) error {
			called++
			check.Equal(t, 42, n)
			return nil
		})

		check.NotError(t, pf.When(func() bool { return false })(ctx, 42))
		check.Zero(t, called)
		check.NotError(t, pf.When(func() bool { return true })(ctx, 42))
		check.Equal(t, 1, called)
		check.NotError(t, pf.When(func() bool { return true })(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf.When(func() bool { return false })(ctx, 42))
		check.Equal(t, 2, called)
		check.NotError(t, pf(ctx, 42))
		check.Equal(t, 3, called)
	})
	t.Run("Once", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Processor[int](func(ctx context.Context, n int) error {
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
		ctx := testt.Context(t)
		called := 0
		root := ers.New("foo")
		pf := Processor[int](func(ctx context.Context, n int) error {
			check.Equal(t, 42, n)
			called++
			return root
		})
		of := func(err error) { called++; check.ErrorIs(t, err, root) }
		obv := pf.Operation(42, of)
		check.Equal(t, called, 0)
		obv(ctx)
		check.Equal(t, called, 2)

	})
	t.Run("Observer", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		root := ers.New("foo")
		pf := Processor[int](func(ctx context.Context, n int) error {
			check.Equal(t, 42, n)
			called++
			return root
		})
		of := func(err error) { called++; check.ErrorIs(t, err, root) }
		obv := pf.Observer(ctx, of)
		check.Equal(t, called, 0)
		obv(42)
		check.Equal(t, called, 2)
	})
	t.Run("Worker", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		root := ers.New("foo")
		pf := Processor[int](func(ctx context.Context, n int) error {
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
		ctx := testt.Context(t)
		called := 0
		root := ers.New("foo")
		pf := Processor[int](func(ctx context.Context, n int) error {
			time.Sleep(250 * time.Millisecond)
			check.Equal(t, 42, n)
			called++
			return root
		})
		check.Equal(t, called, 0)
		wf := pf.Future(ctx, 42)
		check.Equal(t, called, 0)
		check.Error(t, wf(ctx))
		check.Equal(t, called, 1)
		check.ErrorIs(t, wf(ctx), root)
		check.Equal(t, called, 1)
	})

}
