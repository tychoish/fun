package fun

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
)

type wrapTestType struct {
	value int
}

func TestWrap(t *testing.T) {
	t.Run("Wrap", func(t *testing.T) {
		l := &wrapTestType{value: 42}
		nl := Unwrap(l)
		if nl != nil {
			t.Fatal("should be nil")
		}
	})
	t.Run("Is", func(t *testing.T) {
		if Is[*testing.T](5) {
			t.Error("Is should return false when types do not match ")
		}
		if !Is[int](100) {
			t.Error("Is should return true when types match")
		}
	})
	t.Run("Errors", func(t *testing.T) {
		err := errors.New("root")
		wrapped := fmt.Errorf("wrap: %w", err)
		unwrapped := Unwrap(wrapped)
		if unwrapped != err {
			t.Fatal("unexpected unrwapping")
		}
	})
}
func TestZeroHelpers(t *testing.T) {
	t.Run("Zero", func(t *testing.T) {
		assert.Zero(t, Zero(100))
		assert.Zero(t, Zero(true))
		assert.Zero(t, Zero("hello world"))
	})
	t.Run("Predicate", func(t *testing.T) {
		assert.True(t, !IsZero(100))
		assert.True(t, !IsZero(true))
		assert.True(t, !IsZero("hello world"))
		assert.True(t, IsZero(0))
		assert.True(t, IsZero(false))
		assert.True(t, IsZero(""))
		assert.True(t, IsZero(time.Time{}))
	})
	t.Run("OrNil", func(t *testing.T) {
		assert.Zero(t, ZeroWhenNil[int](any(0)))
		assert.True(t, ZeroWhenNil[bool](any(true)))
		assert.True(t, !ZeroWhenNil[bool](any(false)))
		assert.Equal(t, "hello world", ZeroWhenNil[string](any("hello world")))
		assert.NotZero(t, ZeroWhenNil[time.Time](time.Now()))

		var foo = "foo"
		var tt testing.TB
		assert.NotZero(t, ZeroWhenNil[*string](&foo))
		assert.Zero(t, ZeroWhenNil[*testing.T](tt))
	})
}
