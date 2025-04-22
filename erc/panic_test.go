package erc

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

func TestPanics(t *testing.T) {
	t.Run("Recover", func(t *testing.T) {
		var called bool
		ob := func(err error) {
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrRecoveredPanic)
			called = true
		}
		assert.NotPanic(t, func() {
			defer Recover(ob)
			assert.True(t, !called)
			panic("hi")
		})
		assert.True(t, called)
	})

	t.Run("NilInput", func(t *testing.T) {
		err := ParsePanic(nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ErrorSlice", func(t *testing.T) {
		err := ParsePanic([]error{ers.New("one"), ers.WithTime(ers.New("two"))})
		if err == nil {
			t.Fatal("expected error")
		}
		check.Equal(t, 3, len(internal.Unwind(err)))
	})
	t.Run("ArbitraryObject", func(t *testing.T) {
		err := ParsePanic(t)
		if err == nil {
			t.Fatal("expected error")
		}

		check.Substring(t, err.Error(), "testing.T")
		if !errors.Is(err, ers.ErrRecoveredPanic) {
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
		if !errors.Is(err, ers.ErrRecoveredPanic) {
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
		if !errors.Is(err, ers.ErrRecoveredPanic) {
			t.Error("not wrapped", err)
		}
		if !strings.Contains(err.Error(), io.EOF.Error()) {
			t.Error(io.EOF.Error(), "NOT IN", err.Error())
		}
		if !strings.Contains(err.Error(), string(ers.ErrRecoveredPanic)) {
			t.Error(ers.ErrRecoveredPanic.Error(), "NOT IN", err.Error())
		}
	})
	t.Run("ExtractErrors", func(t *testing.T) {
		args, errs := extractErrors([]any{nil, ers.Error("hi"), 1, true})
		check.Equal(t, len(errs), 1)
		check.Equal(t, len(args), 2)
		var nerr error
		args, errs = extractErrors([]any{nil, ers.Error("hi"), func() error { return nil }(), nerr, 2, false})
		check.Equal(t, len(errs), 1)
		check.Equal(t, len(args), 2)
	})
	t.Run("InvariantViolation", func(t *testing.T) {
		assert.True(t, IsInvariantViolation(ers.ErrInvariantViolation))
		assert.True(t, IsInvariantViolation(errors.Join(io.EOF, ers.Error("hello"), ers.ErrInvariantViolation)))
		assert.True(t, !IsInvariantViolation(nil))
		assert.True(t, !IsInvariantViolation(9001))
		assert.True(t, !IsInvariantViolation(io.EOF))
	})

}
