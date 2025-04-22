package ers

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestPanics(t *testing.T) {
	t.Run("Recover", func(t *testing.T) {
		var called bool
		ob := func(err error) {
			check.Error(t, err)
			check.ErrorIs(t, err, ErrRecoveredPanic)
			called = true
		}
		assert.NotPanic(t, func() {
			defer Recover(ob)
			assert.True(t, !called)
			panic("hi")
		})
		assert.True(t, called)
	})
	t.Run("ParsePanic", func(t *testing.T) {
		t.Run("NilInput", func(t *testing.T) {
			err := ParsePanic(nil)
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("ErrorSlice", func(t *testing.T) {
			err := ParsePanic([]error{New("one"), WithTime(New("two"))})
			if err == nil {
				t.Fatal("expected error")
			}
			check.Equal(t, 3, len(Unwind(err)))
		})
		t.Run("ArbitraryObject", func(t *testing.T) {
			err := ParsePanic(t)
			if err == nil {
				t.Fatal("expected error")
			}

			check.Substring(t, err.Error(), "testing.T")
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
		})

		t.Run("TwoErrors", func(t *testing.T) {
			err := ParsePanic(io.EOF)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, io.EOF) {
				t.Error("not EOF", err)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
		})
		t.Run("NotErrorObject", func(t *testing.T) {
			err := ParsePanic("EOF")
			if err == nil {
				t.Fatal("expected error")
			}
			if errors.Is(err, io.EOF) {
				t.Error(err)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
			if !strings.Contains(err.Error(), io.EOF.Error()) {
				t.Error(io.EOF.Error(), "NOT IN", err.Error())
			}
			if !strings.Contains(err.Error(), string(ErrRecoveredPanic)) {
				t.Error(ErrRecoveredPanic.Error(), "NOT IN", err.Error())
			}
		})
	})
	t.Run("ExtractErrors", func(t *testing.T) {
		args, errs := ExtractErrors([]any{nil, Error("hi"), 1, true})
		check.Equal(t, len(errs), 1)
		check.Equal(t, len(args), 2)
		var nerr error
		args, errs = ExtractErrors([]any{nil, Error("hi"), func() error { return nil }(), nerr, 2, false})
		check.Equal(t, len(errs), 1)
		check.Equal(t, len(args), 2)
	})

}
