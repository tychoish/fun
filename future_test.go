package fun

import (
	"context"
	"sync"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
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

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.DoTimes(testt.Context(t), 128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		})

		check.Equal(t, count, 128)
	})
	t.Run("WithLock", func(t *testing.T) {
		count := 0
		thunk := Futurize(func() int { count++; return 42 }).WithLock(&sync.Mutex{})

		// tempt the race detector.
		wg := &WaitGroup{}
		wg.DoTimes(testt.Context(t), 128, func(context.Context) {
			check.Equal(t, thunk(), 42)
		})

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
		testFuture := testt.Must(thunk(testt.Context(t)))
		check.Equal(t, testFuture(t), 42)
	})
}
