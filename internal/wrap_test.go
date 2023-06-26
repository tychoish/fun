package internal

import (
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestUnwind(t *testing.T) {
	t.Run("Errors", func(t *testing.T) {
		err := errors.New("root")
		wrapped := fmt.Errorf("wrap: %w", err)
		errs := Unwind(wrapped)
		if !(len(errs) == 2) {
			t.Fatal("assertion failure")
		}
		if errs[1].Error() != err.Error() {
			t.Fatal(errs[1], err)
		}
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
		if l := len(Unwind(err)); l != 101 {
			t.Error(l, "not 101")
		}
	})
	t.Run("Slice", func(t *testing.T) {
		var err error = slwrap{out: []error{io.EOF, errors.New("basebase")}}
		errs := Unwind(err)
		if len(errs) != 2 {
			t.Error(len(errs), 2, errs)
		}
	})
	t.Run("WithNils", func(t *testing.T) {
		var err error = slwrap{out: []error{io.EOF, nil, errors.New("basebase"), nil}}
		errs := Unwind(err)
		if len(errs) != 2 {
			t.Error(len(errs), 2, errs)
		}
	})
	t.Run("NilUnwrap", func(t *testing.T) {
		var err error
		errs := Unwind(err)
		if errs != nil || len(errs) != 0 {
			t.Error(len(errs), errs)
		}

		err = &oneWrap{}
		errs = Unwind(err)

		if len(errs) != 0 {
			t.Error(len(errs), errs)
		}

		err = &oneWrap{out: &oneWrap{}}
		errs = Unwind(err)
		if len(errs) != 1 {
			t.Error(len(errs), errs)
		}
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
