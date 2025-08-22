package ft

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestRecover(t *testing.T) {
	t.Run("WrapRecoverApply", func(t *testing.T) {
		t.Run("NilPanic", func(t *testing.T) {
			ef := WrapRecoverApply(nil, 54)
			err := ef()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("Lazy", func(t *testing.T) {
			ct := 0
			op := func(in int) { ct++; check.Equal(t, in, 42) }
			ef := WrapRecoverApply(op, 42)
			check.Equal(t, ct, 0)
			err := ef()
			check.NotError(t, err)
			check.Equal(t, ct, 1)
		})
		t.Run("LazyPanic", func(t *testing.T) {
			ct := 0
			op := func(in int) { check.Equal(t, in, 42); ct++; panic(11) }
			ef := WrapRecoverApply(op, 42)
			check.Equal(t, ct, 0)
			assert.NotPanic(t, func() {
				err := ef()
				check.Error(t, err)
				check.ErrorIs(t, err, ers.ErrRecoveredPanic)
				check.Equal(t, ct, 1)
				ct++
			})
			check.Equal(t, ct, 2)
		})
	})
	t.Run("Filter", func(t *testing.T) {
		t.Run("NilPanic", func(t *testing.T) {
			v, err := WithRecoverFilter(nil, 54)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.Zero(t, v)
		})
		t.Run("Lazy", func(t *testing.T) {
			ct := 0
			op := func(in int) int { check.Equal(t, in, 42); ct++; return 42 - 1 }
			ef := WrapRecoverFilter(op)
			check.Equal(t, ct, 0)
			out, err := ef(42)
			check.NotError(t, err)
			check.Equal(t, ct, 1)
			check.Equal(t, out, 41)
		})
		t.Run("LazyPanic", func(t *testing.T) {
			ct := 0
			op := func(in int) int { check.Equal(t, in, 42); ct++; panic(11) }
			ef := WrapRecoverFilter(op)
			check.Equal(t, ct, 0)
			assert.NotPanic(t, func() {
				out, err := ef(42)
				check.Error(t, err)
				check.ErrorIs(t, err, ers.ErrRecoveredPanic)
				check.Equal(t, ct, 1)
				check.Equal(t, out, 0)
				ct++
			})

			check.Equal(t, ct, 2)
		})
	})
}
