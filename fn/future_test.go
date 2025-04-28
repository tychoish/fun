package fn

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

func TestFuture(t *testing.T) {
	t.Parallel()
	t.Run("Run", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Resolve(), 42)
		check.Equal(t, count, 1)
		check.Equal(t, thunk.Resolve(), 42)
		check.Equal(t, count, 2)
	})
	t.Run("Once", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 }).Once()
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Resolve(), 42)
		check.Equal(t, count, 1)
		check.Equal(t, thunk.Resolve(), 42)
		check.Equal(t, count, 1)
	})
	t.Run("Ignore", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 }).Ignore()
		check.Equal(t, count, 0)
		check.NotPanic(t, thunk)
		check.Equal(t, count, 1)
		check.NotPanic(t, thunk)
		check.Equal(t, count, 2)
	})
	t.Run("If", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.If(false).Resolve(), 0)
		check.Equal(t, count, 0)
		check.Equal(t, thunk.If(true).Resolve(), 42)
		check.Equal(t, count, 1)
	})
	t.Run("Not", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Not(true).Resolve(), 0)
		check.Equal(t, count, 0)
		check.Equal(t, thunk.Not(false).Resolve(), 42)
		check.Equal(t, count, 1)
	})
	t.Run("When", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		check.Equal(t, thunk.When(AsFuture(true)).Resolve(), 42)
		check.Equal(t, count, 1)
		check.Equal(t, thunk.When(AsFuture(true)).Resolve(), 42)
		check.Equal(t, count, 2)
		check.Equal(t, thunk.When(AsFuture(false)).Resolve(), 0)
		check.Equal(t, count, 2)
	})
	t.Run("PreHook", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { check.Equal(t, count, 1); count++; return 42 }).
			PreHook(func() { check.Equal(t, count, 0); count++ }).
			PostHook(func() { check.Equal(t, count, 2); count++ })
		check.Equal(t, count, 0)
		check.Equal(t, thunk(), 42)
		check.Equal(t, count, 3)
	})
	t.Run("PostHook", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { check.Equal(t, count, 1); count++; panic("expected") }).
			PreHook(func() { check.Equal(t, count, 0); count++ }).
			PostHook(func() { check.Equal(t, count, 2); count++ })
		check.Equal(t, count, 0)
		check.Panic(t, thunk.Ignore())
		check.Equal(t, count, 3)
	})
	t.Run("Reduce", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		val := thunk.Reduce(func(a, b int) int { return a + b }, thunk)
		check.Equal(t, count, 0)
		check.Equal(t, val(), count*42)
		check.Equal(t, count, 2)
	})
	t.Run("Join", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		check.Equal(t, count, 0)
		val := thunk.Join(func(a, b int) int { return a + b }, thunk, thunk, thunk, thunk)
		check.Equal(t, count, 0)
		check.Equal(t, val(), count*42)
		check.Equal(t, count, 5)
	})
	t.Run("Slice", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 }).Slice()
		check.Equal(t, count, 0)
		sl := thunk()
		assert.Equal(t, len(sl), 1)
		assert.Equal(t, sl[0], 42)
		check.Equal(t, count, 1)
	})
	t.Run("Translate", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 })
		think := Translate(thunk, func(in int) string { return fmt.Sprint(in) })
		check.Equal(t, count, 0)
		check.Equal(t, think(), "42")
		check.Equal(t, count, 1)
		check.Equal(t, thunk(), 42)
		check.Equal(t, count, 2)
	})
	t.Run("Limit", func(t *testing.T) {
		count := 0
		thunk := MakeFuture(func() int { count++; return 42 }).Limit(100)
		ft.CallTimes(500, func() { check.Equal(t, 42, thunk()) })
		check.Equal(t, count, 100)
	})
	t.Run("TTL", func(t *testing.T) {
		t.Run("Under", func(t *testing.T) {
			count := 0
			thunk := MakeFuture(func() int { count++; return 42 }).TTL(time.Minute)
			ft.CallTimes(500, func() { check.Equal(t, 42, thunk()) })
			check.Equal(t, count, 1)
		})
		t.Run("Par", func(t *testing.T) {
			count := 0
			thunk := MakeFuture(func() int { count++; return 42 }).TTL(time.Nanosecond)
			ft.CallTimes(500, func() { time.Sleep(10 * time.Nanosecond); check.Equal(t, 42, thunk()) })
			check.Equal(t, count, 500)
		})
		t.Run("Over", func(t *testing.T) {
			count := 0
			thunk := MakeFuture(func() int { count++; return 42 }).TTL(50 * time.Millisecond)
			ft.CallTimes(100, func() { time.Sleep(25 * time.Millisecond); check.Equal(t, 42, thunk()) })

			check.Equal(t, count, 50)
		})
	})
	t.Run("Panics", func(t *testing.T) {
		t.Run("Op", func(t *testing.T) {
			out, err := MakeFuture(func() int { panic("foo") }).RecoverPanic()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.Zero(t, out)

		})
		t.Run("Wrapper", func(t *testing.T) {
			out, err := MakeFuture(func() int { panic("foo") }).Safe()()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.Zero(t, out)

		})

	})
	t.Run("Lock", func(t *testing.T) {
		// this is mostly just tempting the race detecor
		wg := &sync.WaitGroup{}
		count := 0
		var ob Future[int] = func() int {
			defer wg.Done()
			count++
			return 42
		}

		lob := ob.Lock()

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				t.Helper()
				out := lob()
				assert.Equal(t, out, 42)
			}()
		}
		wg.Wait()

		assert.Equal(t, count, 10)
	})
	t.Run("Locker", func(t *testing.T) {
		// this is mostly just tempting the race detecor
		wg := &sync.WaitGroup{}
		count := 0
		var ob Future[int] = func() int {
			defer wg.Done()
			count++
			return 42
		}

		rmtx := &sync.RWMutex{}
		lob := ob.WithLocker(rmtx)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				t.Helper()
				out := lob()
				assert.Equal(t, out, 42)
			}()
		}
		wg.Wait()

		assert.Equal(t, count, 10)
	})

}
