package ft

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestConditionals(t *testing.T) {
	t.Run("Lispisms", func(t *testing.T) {
		t.Run("When", func(t *testing.T) {
			check.Equal(t, 42, When(true, 42))
			check.Equal(t, 0, When(false, 42))
		})
		t.Run("Unless", func(t *testing.T) {
			check.Equal(t, 0, Unless(true, 42))
			check.Equal(t, 42, Unless(false, 42))
		})
	})
	t.Run("Filter", func(t *testing.T) {
		t.Run("IfElse", func(t *testing.T) {
			filterTrue := func(in int) int { return in + 1 }
			filterFalse := func(in int) int { return in - 1 }
			out := FilterIfElse(true, filterTrue, filterFalse, 99)
			assert.Equal(t, out, 100)
			out = FilterIfElse(false, filterTrue, filterFalse, 101)
			assert.Equal(t, out, 100)
			out = FilterIfElse(false, filterTrue, filterFalse, 43)
			assert.Equal(t, out, 42)
		})
		t.Run("When", func(t *testing.T) {
			filter := func(in int) int { return in - 1 }
			assert.Equal(t, 43, FilterWhen(true, filter, 44))
			assert.Equal(t, 44, FilterWhen(false, filter, 44))
		})
		t.Run("Unless", func(t *testing.T) {
			filter := func(in int) int { return in - 1 }
			assert.Equal(t, 44, FilterUnless(true, filter, 44))
			assert.Equal(t, 43, FilterUnless(false, filter, 44))
		})
	})
}
