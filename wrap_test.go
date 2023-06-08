package fun

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
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

	t.Run("Unwind", func(t *testing.T) {
		t.Run("NoErrors", func(t *testing.T) {
			errs := Unwind[error](nil)
			if errs != nil {
				t.Fail()
			}
			if len(errs) != 0 {
				t.Fail()
			}
		})
		t.Run("OneError", func(t *testing.T) {
			err := errors.New("42")
			errs := Unwind(err)
			if len(errs) != 1 {
				t.Fatal(len(errs))
			}
		})
		t.Run("Wrapped", func(t *testing.T) {
			err := errors.New("base")
			for i := 0; i < 100; i++ {
				err = fmt.Errorf("wrap %d: %w", i, err)
			}

			errs := Unwind(err)
			if len(errs) != 101 {
				t.Error(len(errs))
			}
			if errs[100].Error() != "base" {
				t.Error(errs[100])
			}
		})
		t.Run("Slice", func(t *testing.T) {
			var err error = slwrap{out: []error{io.EOF, errors.New("basebase")}}
			errs := Unwind(err)
			check.Equal(t, 3, len(errs))
		})
		t.Run("MergedSlice", func(t *testing.T) {
			err := internal.MergeErrors(io.EOF, slwrap{out: []error{io.EOF, errors.New("basebase")}})

			errs := Unwind(err)
			check.Equal(t, 4, len(errs))
		})
		t.Run("WithNils", func(t *testing.T) {
			var err error = slwrap{out: []error{io.EOF, nil, errors.New("basebase"), nil}}
			errs := Unwind(err)
			check.Equal(t, 3, len(errs))
			testt.Log(t, errs, err)
		})
		t.Run("NilUnwrap", func(t *testing.T) {
			var err error
			errs := Unwind(err)
			check.True(t, errs == nil)
			check.Equal(t, len(errs), 0)

			err = &oneWrap{}
			errs = Unwind(err)
			check.Equal(t, len(errs), 1)

			err = &oneWrap{out: &oneWrap{}}
			errs = Unwind(err)
			check.Equal(t, len(errs), 2)
		})

	})
}

type oneWrap struct {
	out error
}

func (s *oneWrap) Unwrap() error { return s.out }
func (s *oneWrap) Error() string { return fmt.Sprint("error: isnil,", s.out) }

type slwrap struct {
	out []error
}

func (s slwrap) Unwrap() []error { return s.out }
func (s slwrap) Error() string   { return fmt.Sprint("error:", len(s.out), s.out) }

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
