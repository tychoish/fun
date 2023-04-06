package adt

import (
	"context"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
)

func TestAtomics(t *testing.T) {
	t.Run("IsAtomicZero", func(t *testing.T) {
		var atom *Atomic[int]
		assert.True(t, IsAtomicZero(atom))
		atom = &Atomic[int]{}
		assert.True(t, IsAtomicZero(atom))
		atom.Set(0)
		assert.True(t, IsAtomicZero(atom))
		atom.Set(100)
		assert.True(t, !IsAtomicZero(atom))
	})
	t.Run("SafeSet", func(t *testing.T) {
		atom := &Atomic[int]{}
		SafeSet(atom, -1)
		assert.Equal(t, atom.Get(), -1)
		SafeSet(atom, 0) // noop
		assert.Equal(t, atom.Get(), -1)
	})

	t.Run("Default", func(t *testing.T) {
		at := NewAtomic(1000)
		assert.Equal(t, at.Get(), 1000)
	})
	t.Run("Zero", func(t *testing.T) {
		at := &Atomic[int]{}
		assert.Equal(t, at.Get(), 0)
	})
	t.Run("RoundTrip", func(t *testing.T) {
		at := &Atomic[int]{}
		at.Set(42)
		assert.Equal(t, at.Get(), 42)
	})
	t.Run("AtomicSwap", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		atom := &Atomic[int]{}

		wg := &fun.WaitGroup{}
		// this should always pass, but we're mostly tempting
		// the race detector.
		for i := 1; i < 256; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				old := atom.Swap(id)
				assert.True(t, old != id)
			}(i)
		}
		wg.Wait(ctx)
		assert.True(t, atom.Get() != 0)
	})
	t.Run("IsZero", func(t *testing.T) {
		assert.True(t, fun.IsZero[*Atomic[int]](nil))
		var f *Atomic[int]
		assert.True(t, fun.IsZero(f))
		f = &Atomic[int]{}
		// ideally this should be
		// true, but...
		assert.True(t, !fun.IsZero(f))
		// clearly true
		assert.True(t, fun.IsZero(f.Get()))
		f = (*Atomic[int])(NewAtomic(100))
		assert.True(t, !fun.IsZero(f))
	})
	t.Run("CompareAndSwap", func(t *testing.T) {
		atom := &Atomic[int]{}
		atom.Set(54)
		swapped := CompareAndSwap(atom, 54, 100)
		assert.True(t, swapped)
		assert.Equal(t, 100, atom.Get())
	})
}
