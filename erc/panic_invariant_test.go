package erc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

func TestPanics(t *testing.T) {
	t.Run("Invariant", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := erc.NewInvariantViolation("math is a construct")
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.True(t, erc.IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := erc.NewInvariantViolation(err)

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := erc.NewInvariantViolation(42, err)
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
			err := erc.NewInvariantViolation(42)
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("CheckError", func(t *testing.T) {
			if erc.IsInvariantViolation(nil) {
				t.Error("nil error shouldn't read as invariant")
			}
			if erc.IsInvariantViolation(errors.New("foo")) {
				t.Error("arbitrary errors are not invariants")
			}
		})
		t.Run("ZeroFilterCases", func(t *testing.T) {
			err := erc.NewInvariantViolation("", nil, nil, "")
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrInvariantViolation)
			es := ers.Unwind(err)
			check.Equal(t, len(es), 1)
		})
		t.Run("Future", func(t *testing.T) {
			t.Run("Single", func(t *testing.T) {
				count := 0
				op := func() error { count++; return ers.ErrInvalidInput }
				err := erc.NewInvariantViolation(op)
				check.Equal(t, count, 1)
				check.ErrorIs(t, err, ers.ErrInvalidInput)
			})
			t.Run("Multi", func(t *testing.T) {
				count := 0
				op := func() error { count++; return ers.ErrInvalidInput }
				err := erc.NewInvariantViolation(op, op, op, op, op, op, op, op)
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
			assert.Error(t, erc.NewInvariantViolation())
			assert.ErrorIs(t, erc.NewInvariantViolation(), ers.ErrInvariantViolation)
		})
		t.Run("LongInvariant", func(t *testing.T) {
			err := erc.NewInvariantViolation("math is a construct", "1 == 2")
			if err == nil {
				t.FailNow()
			}
			if !strings.Contains(err.Error(), "construct 1 == 2") {
				t.Error(err)
			}
		})
	})
}
