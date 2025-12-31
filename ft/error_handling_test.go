package ft

import (
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestErrorHandlingHelpers(t *testing.T) {
	t.Run("WrapCheck", func(t *testing.T) {
		t.Run("Passthrough", func(t *testing.T) {
			out, ok := Do2(WrapCheck(99, nil))
			check.Equal(t, out, 99)
			check.True(t, ok)
		})
		t.Run("Zero", func(t *testing.T) {
			out, ok := Do2(WrapCheck(99, io.EOF))
			check.Equal(t, out, 0)
			check.True(t, !ok)
		})
	})
	t.Run("InvariantOk", func(t *testing.T) {
		t.Run("TrueCondition", func(t *testing.T) {
			assert.NotPanic(t, func() {
				InvariantOk(true)
			})
		})
		t.Run("FalseCondition", func(t *testing.T) {
			err := WithRecoverCall(func() {
				InvariantOk(false)
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("TrueExpression", func(t *testing.T) {
			assert.NotPanic(t, func() {
				InvariantOk(1 == 1)
			})
		})
		t.Run("FalseExpression", func(t *testing.T) {
			err := WithRecoverCall(func() {
				InvariantOk(1 == 2)
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.True(t, ers.IsInvariantViolation(err))
		})
	})
	t.Run("Invariant", func(t *testing.T) {
		t.Run("NilError", func(t *testing.T) {
			assert.NotPanic(t, func() {
				Invariant(nil)
			})
		})
		t.Run("NonNilError", func(t *testing.T) {
			root := errors.New("test error")
			err := WithRecoverCall(func() {
				Invariant(root)
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, root)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
	})
}
