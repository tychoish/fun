package assert_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestAssertion(t *testing.T) {
	var strVal string = "merlin"

	var err error

	t.Run("Passing", func(t *testing.T) {
		assert.True(t, true)
		assert.Equal(t, 1, 1)
		assert.NotEqual(t, 10, 1)
		assert.Zero(t, "")
		assert.NotZero(t, strVal)
		assert.Error(t, errors.New(strVal))
		assert.NotError(t, err)
		assert.ErrorIs(t, fmt.Errorf("end: %w", io.EOF), io.EOF)
		assert.Panic(t, func() { panic(strVal) })
		assert.PanicValue(t, func() { panic(strVal) }, strVal)
		assert.NotPanic(t, func() {})
		assert.Contains(t, []int{1, 2, 3}, 3)
		assert.NotContains(t, []int{1, 2, 3}, 43)
		assert.Substring(t, "merlin the cat", strVal)
		assert.NotSubstring(t, "the cat", strVal)
	})
	t.Run("Failures", func(t *testing.T) {
		assert.Failing(t, func(t *testing.T) { assert.Failing(t, func(*testing.T) {}) })
		assert.Failing(t, func(t *testing.T) { assert.True(t, false) })
		assert.Failing(t, func(t *testing.T) { assert.Equal(t, 1, 2) })
		assert.Failing(t, func(t *testing.T) { assert.NotEqual(t, 1, 1) })
		assert.Failing(t, func(t *testing.T) { assert.Zero(t, "0") })
		assert.Failing(t, func(t *testing.T) { assert.NotZero(t, 0) })
		assert.Failing(t, func(t *testing.T) { assert.Error(t, nil) })
		assert.Failing(t, func(t *testing.T) { assert.NotError(t, errors.New(strVal)) })
		assert.Failing(t, func(t *testing.T) { assert.ErrorIs(t, errors.New(strVal), io.EOF) })
		assert.Failing(t, func(t *testing.T) { assert.Panic(t, func() {}) })
		assert.Failing(t, func(t *testing.T) { assert.NotPanic(t, func() { panic(34) }) })
		assert.Failing(t, func(t *testing.T) { assert.PanicValue(t, func() { panic(strVal) }, "woof") })
		assert.Failing(t, func(t *testing.T) { assert.PanicValue(t, func() {}, "woof") })
		assert.Failing(t, func(t *testing.T) { assert.PanicValue(t, func() { panic(53) }, "woof") })
		assert.Failing(t, func(t *testing.T) { assert.Contains(t, []int{1, 2, 3}, 300) })
		assert.Failing(t, func(t *testing.T) { assert.Contains(t, []int{}, 300) })
		assert.Failing(t, func(t *testing.T) { assert.NotContains(t, []int{1, 2, 3}, 1) })
		assert.Failing(t, func(t *testing.T) { assert.Substring(t, "merlin the cat", "woof") })
		assert.Failing(t, func(t *testing.T) { assert.NotSubstring(t, "the cat", "cat") })
	})
}
