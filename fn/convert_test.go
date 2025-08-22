package fn

import (
	"sync"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestFilter(t *testing.T) {
	// nb: these test cases were written by the robots

	// A simple filter for re-use in tests: doubles an integer.
	double := MakeFilter(func(i int) int { return i * 2 })

	t.Run("Apply", func(t *testing.T) {
		result := double.Apply(5)
		assert.Equal(t, result, 10)
	})

	t.Run("Ptr", func(t *testing.T) {
		val := 5
		double.Ptr(&val)
		assert.Equal(t, val, 10)
	})

	t.Run("WithNext", func(t *testing.T) {
		addOne := MakeFilter(func(i int) int { return i + 1 })
		// Should execute double, then addOne: (5 * 2) + 1
		chained := double.WithNext(addOne)
		result := chained.Apply(5)
		assert.Equal(t, result, 11)
	})
	t.Run("Safe", func(t *testing.T) {
		var zippo Filter[int]
		assert.Panic(t, func() { zippo.WithNext(double).Apply(1) })
		assert.Panic(t, func() { double.WithNext(zippo).Apply(1) })
		assert.Equal(t, 10, zippo.Safe().WithNext(double).Apply(5))
	})

	t.Run("If", func(t *testing.T) {
		// Should apply filter when condition is true
		resultTrue := double.If(true).Apply(5)
		assert.Equal(t, resultTrue, 10)

		// Should not apply filter when condition is false
		resultFalse := double.If(false).Apply(5)
		assert.Equal(t, resultFalse, 5)
	})

	t.Run("Not", func(t *testing.T) {
		// Should not apply filter when Not(true)
		resultTrue := double.Not(true).Apply(5)
		assert.Equal(t, resultTrue, 5)

		// Should apply filter when Not(false)
		resultFalse := double.Not(false).Apply(5)
		assert.Equal(t, resultFalse, 10)
	})

	t.Run("WithLock", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		count := 0
		wlock := double.PostHook(func() { count++ }).WithLock(&sync.Mutex{})
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 100 {
					check.Equal(t, 10, wlock.Apply(5))
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 800, count)
	})

	t.Run("LockerLock", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		count := 0
		wlock := double.PostHook(func() { count++ }).Lock()
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 100 {
					check.Equal(t, 10, wlock.Apply(5))
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 800, count)
	})

	t.Run("Lock", func(t *testing.T) {
		// This test primarily ensures the filter logic still works.
		// To test for race conditions, run `go test -race`.
		lockedFilter := double.Lock()
		result := lockedFilter.Apply(5)
		assert.Equal(t, result, 10)
	})

	t.Run("PreHook", func(t *testing.T) {
		var hookCalled bool
		hookFunc := func() { hookCalled = true }

		filtered := double.PreHook(hookFunc)
		_ = filtered.Apply(5)

		assert.True(t, hookCalled)
	})

	t.Run("PostHook", func(t *testing.T) {
		var hookCalled bool
		hookFunc := func() { hookCalled = true }

		filtered := double.PostHook(hookFunc)
		_ = filtered.Apply(5)

		assert.True(t, hookCalled)
	})

	t.Run("HookOrder", func(t *testing.T) {
		var execOrder []string
		pre := func() { execOrder = append(execOrder, "pre") }
		post := func() { execOrder = append(execOrder, "post") }

		filterWithHooks := MakeFilter(func(i int) int {
			execOrder = append(execOrder, "filter")
			return i
		}).PreHook(pre).PostHook(post)

		_ = filterWithHooks.Apply(1)

		expectedOrder := []string{"pre", "filter", "post"}
		assert.Equal(t, len(execOrder), len(expectedOrder))
		for i := range expectedOrder {
			assert.Equal(t, execOrder[i], expectedOrder[i])
		}
	})

	t.Run("Join", func(t *testing.T) {
		addOne := MakeFilter(func(i int) int { return i + 1 })
		subtractTwo := MakeFilter(func(i int) int { return i - 2 })

		// Should apply in order: double, then addOne, then subtractTwo
		// (((5 * 2) + 1) - 2) = 9
		joined := double.Join(addOne, subtractTwo)
		result := joined.Apply(5)
		assert.Equal(t, result, 9)
	})

	t.Run("JoinNil", func(t *testing.T) {
		addOne := MakeFilter(func(i int) int { return i + 1 })
		// Should skip the nil filter
		// ((5 * 2) + 1) = 11
		joined := double.Join(nil, addOne)
		result := joined.Apply(5)
		assert.Equal(t, result, 11)
	})
}
