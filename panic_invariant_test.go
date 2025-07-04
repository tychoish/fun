package fun_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestPanics(t *testing.T) {
	t.Run("Invariant", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := fun.Invariant.New("math is a construct")
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.True(t, ers.IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := fun.Invariant.New(err)

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := fun.Invariant.New(42, err)
			if !errors.Is(se, err) {
				t.Log("se", se)
				t.Log("err", err)
				t.FailNow()
			}
			if !strings.Contains(se.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("NoError", func(t *testing.T) {
			err := fun.Invariant.New(42)
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("CheckError", func(t *testing.T) {
			if ers.IsInvariantViolation(nil) {
				t.Error("nil error shouldn't read as invariant")
			}
			if ers.IsInvariantViolation(errors.New("foo")) {
				t.Error("arbitrary errors are not invariants")
			}
		})
		t.Run("ZeroFilterCases", func(t *testing.T) {
			err := fun.Invariant.New("", nil, nil, "")
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrInvariantViolation)
			es := ers.Unwind(err)
			t.Log(es)
			check.Equal(t, len(es), 1)
		})
		t.Run("Future", func(t *testing.T) {
			t.Run("Single", func(t *testing.T) {
				count := 0
				op := func() error { count++; return ers.ErrInvalidInput }
				err := fun.Invariant.New(op)
				check.Equal(t, count, 1)
				check.ErrorIs(t, err, ers.ErrInvalidInput)
			})
			t.Run("Multi", func(t *testing.T) {
				count := 0
				op := func() error { count++; return ers.ErrInvalidInput }
				err := fun.Invariant.New(op, op, op, op, op, op, op, op)
				check.Equal(t, count, 8)
				check.ErrorIs(t, err, ers.ErrInvalidInput)

				errs := ers.Unwind(err)
				check.Equal(t, len(errs), 9)
				for idx := range errs {
					t.Log(idx, "/", len(errs), errs[idx])
				}
			})
		})
		t.Run("AlwaysErrors", func(t *testing.T) {
			assert.Error(t, fun.Invariant.New())
			assert.ErrorIs(t, fun.Invariant.New(), ers.ErrInvariantViolation)
		})
		t.Run("LongInvariant", func(t *testing.T) {
			err := fun.Invariant.New("math is a construct", "1 == 2")
			if err == nil {
				t.FailNow()
			}
			if !strings.Contains(err.Error(), "construct 1 == 2") {
				t.Error(err)
			}
		})
	})
}
