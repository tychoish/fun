// GENERATED FILE FROM ASSERTION PACKAGE
package check_test

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	assert "github.com/tychoish/fun/assert/check"
)

func TestAssertion(t *testing.T) {
	var strVal = "merlin"

	var err error

	t.Run("Passing", func(t *testing.T) {
		t.Parallel()
		assert.True(t, true)
		assert.Equal(t, 1, 1)
		assert.NotEqual(t, 10, 1)
		assert.Zero(t, "")
		assert.NotZero(t, strVal)
		assert.Error(t, errors.New(strVal))
		assert.NotError(t, err)
		assert.ErrorIs(t, fmt.Errorf("end: %w", io.EOF), io.EOF)
		assert.NotErrorIs(t, fmt.Errorf("end"), io.EOF)
		assert.Panic(t, func() { panic(strVal) })
		assert.PanicValue(t, func() { panic(strVal) }, strVal)
		assert.NotPanic(t, func() {})
		assert.EqualItems(t, []int{12, 34, 56}, []int{12, 34, 56})
		assert.NotEqualItems(t, []int{12, 3, 56}, []int{12, 34, 56})
		assert.Contains(t, []int{1, 2, 3}, 3)
		assert.NotContains(t, []int{1, 2, 3}, 43)
		assert.Substring(t, "merlin the cat", strVal)
		assert.NotSubstring(t, "the cat", strVal)
		assert.Type[int](t, 1)
		assert.NotType[string](t, 2)
		assert.MaxRuntime(t, time.Millisecond, func() { time.Sleep(time.Microsecond) })
		assert.MinRuntime(t, time.Microsecond, func() { time.Sleep(time.Millisecond) })
		assert.Runtime(t, time.Millisecond, 100*time.Millisecond, func() { time.Sleep(50 * time.Millisecond) })
	})
	t.Run("Failures", func(t *testing.T) {
		t.Parallel()
		assert.Failing(&testing.B{}, func(b *testing.B) { assert.Failing(b, func(*testing.B) {}) })
		assert.Failing(t, func(t *testing.T) { assert.Failing(t, func(*testing.T) {}) })
		assert.Failing(t, func(t *testing.T) { assert.True(t, false) })
		assert.Failing(t, func(t *testing.T) { assert.Equal(t, 1, 2) })
		assert.Failing(t, func(t *testing.T) { assert.NotEqual(t, 1, 1) })
		assert.Failing(t, func(t *testing.T) { assert.Zero(t, "0") })
		assert.Failing(t, func(t *testing.T) { assert.NotZero(t, 0) })
		assert.Failing(t, func(t *testing.T) { assert.Error(t, nil) })
		assert.Failing(t, func(t *testing.T) { assert.NotError(t, errors.New(strVal)) })
		assert.Failing(t, func(t *testing.T) { assert.ErrorIs(t, errors.New(strVal), io.EOF) })
		assert.Failing(t, func(t *testing.T) { assert.NotErrorIs(t, fmt.Errorf("foo: %w", io.EOF), io.EOF) })
		assert.Failing(t, func(t *testing.T) { assert.Panic(t, func() {}) })
		assert.Failing(t, func(t *testing.T) { assert.NotPanic(t, func() { panic(34) }) })
		assert.Failing(t, func(t *testing.T) { assert.EqualItems(t, []int{1, 7}, []int{2, 8}) })
		assert.Failing(t, func(t *testing.T) { assert.EqualItems(t, []int{1, 7}, []int{2, 9, 1}) })
		assert.Failing(t, func(t *testing.T) { assert.NotEqualItems(t, []int{1, 7}, []int{1, 7}) })
		assert.Failing(t, func(t *testing.T) { assert.PanicValue(t, func() { panic(strVal) }, "woof") })
		assert.Failing(t, func(t *testing.T) { assert.PanicValue(t, func() {}, "woof") })
		assert.Failing(t, func(t *testing.T) { assert.PanicValue(t, func() { panic(53) }, "woof") })
		assert.Failing(t, func(t *testing.T) { assert.Contains(t, []int{1, 2, 3}, 300) })
		assert.Failing(t, func(t *testing.T) { assert.Contains(t, []int{}, 300) })
		assert.Failing(t, func(t *testing.T) { assert.NotContains(t, []int{1, 2, 3}, 1) })
		assert.Failing(t, func(t *testing.T) { assert.Substring(t, "merlin the cat", "woof") })
		assert.Failing(t, func(t *testing.T) { assert.NotSubstring(t, "the cat", "cat") })
		assert.Failing(t, func(t *testing.T) { assert.Type[int](t, "hello") })
		assert.Failing(t, func(t *testing.T) { assert.NotType[int](t, 1) })
		assert.Failing(t, func(t *testing.T) {
			assert.MaxRuntime(t, time.Nanosecond, func() { time.Sleep(time.Millisecond) })
		})
		assert.Failing(t, func(t *testing.T) {
			assert.MinRuntime(t, time.Second, func() { time.Sleep(time.Nanosecond) })
		})
		assert.Failing(t, func(t *testing.T) {
			assert.Runtime(t, time.Millisecond, 100*time.Millisecond, func() { time.Sleep(time.Microsecond) })
		})
		assert.Failing(t, func(t *testing.T) {
			assert.Runtime(t, time.Millisecond, 10*time.Millisecond, func() { time.Sleep(50 * time.Millisecond) })
		})
	})
}
