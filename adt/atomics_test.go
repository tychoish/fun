package adt

import (
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
)

func TestAtomics(t *testing.T) {
	t.Run("IsAtomicZero", func(t *testing.T) {
		var atom *fun.Atomic[int]
		assert.True(t, IsAtomicZero(atom))
		atom = &fun.Atomic[int]{}
		assert.True(t, IsAtomicZero(atom))
		atom.Set(0)
		assert.True(t, IsAtomicZero(atom))
		atom.Set(100)
		assert.True(t, !IsAtomicZero(atom))
	})
	t.Run("SafeSet", func(t *testing.T) {
		atom := &fun.Atomic[int]{}
		SafeSet(atom, -1)
		assert.Equal(t, atom.Get(), -1)
		SafeSet(atom, 0) // noop
		assert.Equal(t, atom.Get(), -1)
	})

}
