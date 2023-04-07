package fun

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func TestWhen(t *testing.T) {
	t.Run("Do", func(t *testing.T) {
		out := WhenDo(true, func() int { return 100 })
		check.Equal(t, out, 100)

		out = WhenDo(false, func() int { return 100 })
		check.Equal(t, out, 0)
	})
	t.Run("Call", func(t *testing.T) {
		called := false
		WhenCall(true, func() { called = true })
		check.True(t, called)

		called = false
		WhenCall(false, func() { called = true })
		check.True(t, !called)
	})
}
