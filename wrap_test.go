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

func TestIsZero(t *testing.T) {
	assert.True(t, !IsZero(100))
	assert.True(t, !IsZero(true))
	assert.True(t, !IsZero("hello world"))
	assert.True(t, !IsZero(time.Now()))
	assert.True(t, IsZero(0))
	assert.True(t, IsZero(false))
	assert.True(t, IsZero(""))
	assert.True(t, IsZero(time.Time{}))
}
