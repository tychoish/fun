package fn

import (
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestFilter(t *testing.T) {
	// nb: these test cases were written by the robots (gemini)

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

	t.Run("If", func(t *testing.T) {
		// Should apply filter when condition is true
		resultTrue := double.If(true).Apply(5)
		assert.Equal(t, resultTrue, 10)

		// Should not apply filter when condition is false
		resultFalse := double.If(false).Apply(5)
		assert.Equal(t, resultFalse, 5)
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

func TestConverter(t *testing.T) {
	// nb: these test cases were written by the robots (copilot+claude-3.5)
	t.Run("Convert", func(t *testing.T) {
		c := MakeConverter(func(_ int) string { return "ok" })
		assert.Equal(t, c.Convert(1), "ok")

		n := MakeConverter(func(i int) int { return i * 2 })
		assert.Equal(t, n.Convert(2), 4)
	})
	t.Run("If", func(t *testing.T) {
		c := MakeConverter(func(_ int) string { return "ok" })
		assert.Equal(t, c.If(false).Convert(1), "")
		assert.Equal(t, c.If(true).Convert(1), "ok")

		n := MakeConverter(func(i int) int { return i * 2 })
		assert.Equal(t, n.If(false).Convert(2), 0)
		assert.Equal(t, n.If(true).Convert(2), 4)
	})
	t.Run("When", func(t *testing.T) {
		flag := false
		c := MakeConverter(func(_ int) string { return "ok" })
		when := c.When(func() bool { return flag })
		assert.Equal(t, when.Convert(1), "")
		flag = true
		assert.Equal(t, when.Convert(1), "ok")
	})
	t.Run("Hooks", func(t *testing.T) {
		called := false
		c := MakeConverter(func(_ int) string { return "ok" })
		pre := c.PreHook(func() { called = true })
		assert.Equal(t, pre.Convert(1), "ok")
		assert.True(t, called)

		called = false
		post := c.PostHook(func() { called = true })
		assert.Equal(t, post.Convert(1), "ok")
		assert.True(t, called)
	})
	t.Run("Filters", func(t *testing.T) {
		c := MakeConverter(func(_ int) string { return "ok" })
		pre := c.PreFilter(func(i int) int { return i + 1 })
		assert.Equal(t, pre.Convert(1), "ok")

		post := c.PostFilter(func(s string) string { return s + "!" })
		assert.Equal(t, post.Convert(1), "ok!")

		n := MakeConverter(func(i int) int { return i * 2 })
		npre := n.PreFilter(func(i int) int { return i + 1 })
		assert.Equal(t, npre.Convert(2), 6) // (2+1)*2

		npost := n.PostFilter(func(i int) int { return i + 1 })
		assert.Equal(t, npost.Convert(2), 5) // (2*2)+1
	})
	t.Run("Concurrent", func(t *testing.T) {
		counter := 0
		mu := &sync.Mutex{}

		c := MakeConverter(func(_ int) int {
			time.Sleep(time.Millisecond)
			counter++
			return counter
		})

		locked := c.WithLock(mu)

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result := locked.Convert(1)
				assert.True(t, result > 0 && result <= 10)
			}()
		}
		wg.Wait()
		assert.Equal(t, counter, 10)

		counter = 0
		locked = c.Lock()

		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result := locked.Convert(1)
				assert.True(t, result > 0 && result <= 10)
			}()
		}
		wg.Wait()
		assert.Equal(t, counter, 10)
	})
	t.Run("WithLocker", func(t *testing.T) {
		counter := 0
		mu := &sync.Mutex{}

		c := MakeConverter(func(_ int) int {
			time.Sleep(time.Millisecond)
			counter++
			return counter
		})

		locked := c.WithLocker(mu)

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result := locked.Convert(1)
				assert.True(t, result > 0 && result <= 10)
			}()
		}
		wg.Wait()
		assert.Equal(t, counter, 10)
	})
}
