package dt

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestOptioal(t *testing.T) {
	t.Run("Zero", func(t *testing.T) {
		var opt Optional[string]
		check.True(t, ft.Not(opt.OK()))
		check.Equal(t, opt.Resolve(), "")
		out, ok := opt.Get()
		check.True(t, ft.Not(ok))
		check.Equal(t, out, "")
	})
	t.Run("Constructor", func(t *testing.T) {
		opt := NewOptional("hello")
		check.True(t, opt.OK())
		check.Equal(t, opt.Resolve(), "hello")
		out, ok := opt.Get()
		check.True(t, ok)
		check.Equal(t, out, "hello")
	})
	t.Run("Default", func(t *testing.T) {
		opt := NewOptional("hello")
		opt.Default("world")
		check.Equal(t, opt.Resolve(), "hello")

		opt.Reset()
		assert.True(t, ft.Not(opt.OK()))

		opt.Default("world")
		check.Equal(t, opt.Resolve(), "world")
		opt.Default("hello")
		check.Equal(t, opt.Resolve(), "world")
	})
}
