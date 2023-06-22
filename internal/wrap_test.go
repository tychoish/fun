package internal

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestUnwind(t *testing.T) {
	t.Run("Errors", func(t *testing.T) {
		err := errors.New("root")
		wrapped := fmt.Errorf("wrap: %w", err)
		errs := Unwind(wrapped)
		assert.True(t, len(errs) == 2)
		assert.Equal(t, errs[1].Error(), err.Error())
	})

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
		check.Equal(t, 101, len(Unwind(err)))
	})
	t.Run("Slice", func(t *testing.T) {
		var err error = slwrap{out: []error{io.EOF, errors.New("basebase")}}
		errs := Unwind(err)
		check.Equal(t, 2, len(errs))
	})
	t.Run("WithNils", func(t *testing.T) {
		var err error = slwrap{out: []error{io.EOF, nil, errors.New("basebase"), nil}}
		errs := Unwind(err)
		check.Equal(t, 2, len(errs))
		testt.Log(t, errs, err)
	})
	t.Run("NilUnwrap", func(t *testing.T) {
		var err error
		errs := Unwind(err)
		check.True(t, errs == nil)
		check.Equal(t, len(errs), 0)

		err = &oneWrap{}
		errs = Unwind(err)
		check.Equal(t, len(errs), 0)

		err = &oneWrap{out: &oneWrap{}}
		errs = Unwind(err)
		check.Equal(t, len(errs), 1)
	})

}

type wrapTestType struct {
	value int
}

type slwrap struct {
	out []error
}

func (s slwrap) Unwrap() []error { return s.out }
func (s slwrap) Error() string   { return fmt.Sprint("error:", len(s.out), s.out) }

type oneWrap struct {
	out error
}

func (s *oneWrap) Unwrap() error { return s.out }
func (s *oneWrap) Error() string { return fmt.Sprint("error: isnil,", s.out) }
