package erc

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
)

func TestPanics(t *testing.T) {
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
	t.Run("InvariantViolation", func(t *testing.T) {
		assert.True(t, ers.IsInvariantViolation(ers.ErrInvariantViolation))
		assert.True(t, ers.IsInvariantViolation(errors.Join(io.EOF, ers.Error("hello"), ers.ErrInvariantViolation)))
		assert.True(t, !ers.IsInvariantViolation(nil))
		assert.True(t, !ers.IsInvariantViolation(9001))
		assert.True(t, !ers.IsInvariantViolation(io.EOF))
	})
	t.Run("ExtractErrors", func(t *testing.T) {
		ec := &Collector{}
		input := []any{nil, ers.Error("hi"), 1, true, "   ", "hi"}
		ec.extractErrors(input)
		check.Equal(t, ec.Len(), 2)

		var nerr error
		ec = &Collector{}
		input = []any{nil, ers.Error("hi"), func() error { return nil }, nerr, 2, false, "etc..."}
		ec.extractErrors(input)
		check.Equal(t, ec.Len(), 2)

		ec = &Collector{}
		input = []any{ers.Error("hi"), ers.Error("hi"), ers.Error("hi"), ers.Error("hi")}
		ec.extractErrors(input)
		check.Equal(t, ec.Len(), 4)
	})
	t.Run("InvariantError", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			assert.Equal[error](t, ers.ErrInvariantViolation, NewInvariantError())
			assert.ErrorIs(t, NewInvariantError(), ers.ErrInvariantViolation)
		})
		t.Run("One", func(t *testing.T) {
			vals := []any{"hello", time.Now(), ers.New("hello"), func() error { return errors.New("hello") }}
			for val := range slices.Values(vals) {
				err := NewInvariantError(val)
				check.Error(t, err)
				check.ErrorIs(t, err, ers.ErrInvariantViolation)
				check.Equal(t, 2, len(ers.Unwind(err)))
			}
		})
		t.Run("Many", func(t *testing.T) {
			input := []any{errors.New("hello"), ers.New("hello"), io.EOF}
			err := NewInvariantError(input...)
			check.ErrorIs(t, err, ers.ErrInvariantViolation)
			check.Equal(t, len(input)+1, len(ers.Unwind(err)))
		})
	})
}
