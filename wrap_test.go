package fun

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/internal"
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
	t.Run("Wrapper", func(t *testing.T) {
		assert.NotError(t, Wrapper[error](nil)())
		assert.Error(t, Wrapper(errors.New("Hello"))())
		assert.Equal(t, Wrapper(1)(), 1)
		assert.Equal(t, Wrapper("hello")(), "hello")
	})
	t.Run("RootUnWrapp", func(t *testing.T) {
		root := errors.New("hello")
		err := fmt.Errorf("bar: %w", fmt.Errorf("foo: %w", root))
		errs := Unwind(err)
		assert.Equal(t, len(errs), 3)
		assert.True(t, root == UnwrapedRoot(err))
	})
	t.Run("IsWrapFalse", func(t *testing.T) {
		l := &wrapTestType{value: 42}
		assert.True(t, !IsWrapped(l))
	})
	t.Run("IsWrap", func(t *testing.T) {
		err := fmt.Errorf("hello: %w", errors.New("world"))
		assert.True(t, IsWrapped(err))
	})
	t.Run("Unwinder", func(t *testing.T) {
		err := fmt.Errorf("hello: %w", errors.New("world"))
		errs1 := Unwind(err)
		errs2 := []error{}

		assert.NotError(t, Observe(internal.BackgroundContext, UnwindIterator(err), func(in error) { errs2 = append(errs2, in) }))
		assert.True(t, IsWrapped(err))
		assert.Equal(t, len(errs1), len(errs2))
		for idx := range errs1 {
			assert.True(t, errs1[idx] == errs2[idx])
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
	t.Run("UnwindErrors", func(t *testing.T) {
		err := errors.New("root")
		wrapped := fmt.Errorf("wrap: %w", err)
		errs := Unwind(wrapped)
		assert.True(t, len(errs) == 2)
		assert.Equal(t, errs[1].Error(), err.Error())
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
