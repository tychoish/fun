//go:build go1.20

package adt

import (
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestSwap(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		mp := &Map[string, int]{}
		mp.Store("foo", 100)

		out, ok := mp.Swap("foo", 200)
		assert.True(t, ok)
		assert.Equal(t, 100, out)

		out, ok = mp.Load("foo")
		assert.True(t, ok)
		assert.Equal(t, 200, out)
	})
	t.Run("Unknown", func(t *testing.T) {
		mp := &Map[string, int]{}

		out, ok := mp.Swap("foo", 200)
		assert.True(t, !ok)
		assert.Equal(t, 0, out)

		out, ok = mp.Load("foo")
		assert.True(t, ok)
		assert.Equal(t, 200, out)
	})
}
