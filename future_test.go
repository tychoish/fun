package fun

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestFuture(t *testing.T) {
	t.Parallel()
	t.Run("Run", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Run(), 42)
		check.Equal(t, count, 1)
		check.Equal(t, thunk.Run(), 42)
		check.Equal(t, count, 2)
	})
	t.Run("Once", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).Once()
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Run(), 42)
		check.Equal(t, count, 1)
		check.Equal(t, thunk.Run(), 42)
		check.Equal(t, count, 1)
	})
	t.Run("Ignore", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).Ignore()
		check.Equal(t, count, 0)
		check.NotPanic(t, thunk)
		check.Equal(t, count, 1)
		check.NotPanic(t, thunk)
		check.Equal(t, count, 2)
	})
	t.Run("If", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.If(false)(), 0)
		check.Equal(t, count, 0)
		check.Equal(t, thunk.If(true)(), 42)
		check.Equal(t, count, 1)
	})
	t.Run("Not", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Not(true)(), 0)
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Not(false)(), 42)
		check.Equal(t, count, 1)
	})
	t.Run("When", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.When(AsFuture(true))(), 42)
		check.Equal(t, count, 1)
		check.Equal(t, thunk.When(AsFuture(true))(), 42)
		check.Equal(t, count, 2)
		check.Equal(t, thunk.When(AsFuture(false))(), 0)
		check.Equal(t, count, 2)
	})
	t.Run("PreHook", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { check.Equal(t, count, 1); count++; return 42 }).
			PreHook(func() { check.Equal(t, count, 0); count++ }).
			PostHook(func() { check.Equal(t, count, 2); count++ })
		check.Equal(t, count, 0)
		check.Equal(t, thunk(), 42)
		check.Equal(t, count, 3)
	})
	t.Run("PostHook", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { check.Equal(t, count, 1); count++; panic("expected") }).
			PreHook(func() { check.Equal(t, count, 0); count++ }).
			PostHook(func() { check.Equal(t, count, 2); count++ })
		check.Equal(t, count, 0)
		check.Panic(t, thunk.Ignore())
		check.Equal(t, count, 3)
	})
	t.Run("Lock", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).Lock()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.DoTimes(ctx, 128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		})
		wg.Wait(ctx)

		check.Equal(t, count, 128)
	})
	t.Run("WithLock", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).WithLock(&sync.Mutex{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.DoTimes(ctx, 128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		})

		wg.Wait(ctx)
		check.Equal(t, count, 128)
	})
	t.Run("Reduce", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		val := thunk.Reduce(func(a, b int) int { return a + b }, thunk)
		check.Equal(t, count, 0)
		check.Equal(t, val(), count*42)
		check.Equal(t, count, 2)
	})
	t.Run("Join", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		val := thunk.Join(func(a, b int) int { return a + b }, thunk, thunk, thunk, thunk)
		check.Equal(t, count, 0)
		check.Equal(t, val(), count*42)
		check.Equal(t, count, 5)
	})
	t.Run("Slice", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).Slice()
		check.Equal(t, count, 0)
		sl := thunk()
		assert.Equal(t, len(sl), 1)
		assert.Equal(t, sl[0], 42)
		check.Equal(t, count, 1)
	})
	t.Run("Producer", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).Producer()
		check.Equal(t, count, 0)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		testFuture, err := thunk(ctx)
		assert.NotError(t, err)
		check.Equal(t, testFuture, 42)
	})
	t.Run("Translate", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 })
		think := Translate(thunk, func(in int) string { return fmt.Sprint(in) })
		check.Equal(t, count, 0)
		check.Equal(t, think(), "42")
		check.Equal(t, count, 1)
		check.Equal(t, thunk(), 42)
		check.Equal(t, count, 2)
	})
	t.Run("Limit", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).Limit(100)
		ft.DoTimes(500, func() { check.Equal(t, 42, thunk()) })
		check.Equal(t, count, 100)
	})
	t.Run("TTL", func(t *testing.T) {
		t.Run("Under", func(t *testing.T) {
			count := 0
			thunk := Futurize(func() int { count++; return 42 }).TTL(time.Minute)
			ft.DoTimes(500, func() { check.Equal(t, 42, thunk()) })
			check.Equal(t, count, 1)
		})
		t.Run("Par", func(t *testing.T) {
			count := 0
			thunk := Futurize(func() int { count++; return 42 }).TTL(time.Nanosecond)
			ft.DoTimes(500, func() { time.Sleep(10 * time.Nanosecond); check.Equal(t, 42, thunk()) })
			check.Equal(t, count, 500)
		})
		t.Run("Over", func(t *testing.T) {
			count := 0
			thunk := Futurize(func() int { count++; return 42 }).TTL(50 * time.Millisecond)
			ft.DoTimes(100, func() { time.Sleep(25 * time.Millisecond); check.Equal(t, 42, thunk()) })

			check.Equal(t, count, 50)
		})
	})
}
