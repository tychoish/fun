package dt

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

type slwrap struct {
	out []error
}

func (s slwrap) Unwrap() []error { return s.out }
func (s slwrap) Error() string   { return fmt.Sprint("error:", len(s.out), s.out) }

func TestWrap(t *testing.T) {
	t.Run("MergedSlice", func(t *testing.T) {
		err := ers.Join(io.EOF, slwrap{out: []error{io.EOF, errors.New("basebase")}})

		errs := Unwind(err)

		check.Equal(t, 3, len(errs))
	})
	t.Run("Wrap", func(t *testing.T) {
		l := &wrapTestType{value: 42}
		nl := Unwrap(l)
		if nl != nil {
			t.Fatal("should be nil")
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

type wrapTestType struct {
	value int
}
