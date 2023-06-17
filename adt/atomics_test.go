package adt

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/testt"
)

type zeroed struct{ Atomic[int] }

func (*zeroed) IsZero() bool { return true }

func TestAtomics(t *testing.T) {
	t.Run("NilIsZero", func(t *testing.T) {
		assert.True(t, IsAtomicZero[int](nil))
	})
	t.Run("IsZeroInterface", func(t *testing.T) {
		atom := &zeroed{}
		assert.True(t, IsAtomicZero[int](atom))
	})

	t.Run("IsAtomicZero", func(t *testing.T) {
		var atom *Atomic[int]
		assert.True(t, IsAtomicZero[int](atom))
		atom = &Atomic[int]{}
		assert.True(t, IsAtomicZero[int](atom))
		atom.Set(0)
		assert.True(t, IsAtomicZero[int](atom))
		atom.Set(100)
		assert.True(t, !IsAtomicZero[int](atom))
		atom = nil
		assert.True(t, IsAtomicZero[int](atom))
	})
	t.Run("IsSynchronizedZero", func(t *testing.T) {
		var atom *Synchronized[int]
		assert.True(t, IsAtomicZero[int](atom))
		atom = &Synchronized[int]{}
		assert.True(t, IsAtomicZero[int](atom))
		atom.Set(0)
		assert.True(t, IsAtomicZero[int](atom))
		atom.Set(100)
		assert.True(t, !IsAtomicZero[int](atom))
	})
	t.Run("SafeSet", func(t *testing.T) {
		atom := &Atomic[int]{}
		SafeSet[int](atom, -1)
		assert.Equal(t, atom.Get(), -1)
		SafeSet[int](atom, 0) // noop
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
		t.Run("Atomic", func(t *testing.T) {
			atom := &Atomic[int]{}
			atom.Set(54)
			swapped := CompareAndSwap[int](atom, 54, 100)
			assert.True(t, swapped)
			assert.Equal(t, 100, atom.Get())
		})
		t.Run("Interface", func(t *testing.T) {
			atom := &swappable{}
			atom.Set(54)
			swapped := CompareAndSwap[int](atom, 54, 100)
			assert.True(t, swapped)
			assert.Equal(t, 100, atom.Get())
		})
		t.Run("Impossible", func(t *testing.T) {
			atom := &Unsafe[int]{}
			atom.Set(54)
			assert.Panic(t, func() {
				_ = CompareAndSwap[int](atom, 54, 100)
			})
		})

	})
	t.Run("Menmeonics", func(t *testing.T) {
		t.Run("Function", func(t *testing.T) {
			t.Parallel()
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			count := &atomic.Int64{}
			mfn := Mnemonize(func() int { count.Add(1); return 42 })
			wg := &fun.WaitGroup{}
			for i := 0; i < 64; i++ {
				wg.Add(1)
				// this function panics rather than
				// asserts because it's very likely to
				// be correct, and to avoid testing.T
				// mutexes.
				go func() {
					defer wg.Done()
					for i := 0; i < 64; i++ {
						if val := mfn(); val != 42 {
							panic(fmt.Errorf("mnemonic function produced %d not 42", val))
						}
					}
				}()
			}
			wg.Wait(ctx)
			assert.Equal(t, count.Load(), 1)
		})
		t.Run("Once", func(t *testing.T) {
			t.Parallel()
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			count := &atomic.Int64{}
			actor := &Once[int]{}
			wg := &fun.WaitGroup{}
			for i := 0; i < 64; i++ {
				wg.Add(1)
				// this function panics rather than
				// asserts because it's very likely to
				// be correct, and to avoid testing.T
				// mutexes.
				go func() {
					defer wg.Done()
					for i := 0; i < 64; i++ {
						val := actor.Do(func() int {
							count.Add(1)
							return 42
						})
						if val != 42 {
							panic(fmt.Errorf("once function produced %d not 42", val))
						}
					}
				}()
			}

			wg.Wait(ctx)
			assert.Equal(t, count.Load(), 1)
		})
	})
}

func TestMnemonize(t *testing.T) {
	const value string = "val"
	counter := 0
	op := func() string { counter++; return value }

	foo := Mnemonize(op)
	if counter != 0 {
		t.Error("should not call yet", counter)
	}
	if out := foo(); out != value {
		t.Error("wrong value", out, value)
	}
	if counter != 1 {
		t.Error("should call only once", counter)
	}
	for i := 0; i < 128; i++ {
		if out := foo(); out != value {
			t.Error("wrong value", out, value)
		}
		if counter != 1 {
			t.Error("should call only once", counter)
		}
	}
}

func TestZero(t *testing.T) {
	t.Run("OrNil", func(t *testing.T) {
		assert.Zero(t, castOrZero[int](any(0)))
		assert.True(t, castOrZero[bool](any(true)))
		assert.True(t, !castOrZero[bool](any(false)))
		assert.Equal(t, "hello world", castOrZero[string](any("hello world")))
		assert.NotZero(t, castOrZero[time.Time](time.Now()))

		var foo = "foo"
		var tt testing.TB
		assert.NotZero(t, castOrZero[*string](&foo))
		assert.Zero(t, castOrZero[*testing.T](tt))
	})
}

func IsOK[T any](t testing.TB) func(T, bool) T {
	return func(in T, ok bool) T {
		t.Helper()
		if !ok {
			t.Errorf("ok value %t for %T and [%v]", ok, in, in)
		}
		return in
	}
}

func IsNotOK[T any](t testing.TB) func(T, bool) T {
	return func(in T, ok bool) T {
		t.Helper()
		if ok {
			t.Errorf("ok value %t for %T and [%v]", ok, in, in)
		}
		return in
	}
}

type swappable struct{ Atomic[int] }

func (s *swappable) CompareAndSwap(a, b int) bool { return CompareAndSwap[int](&s.Atomic, a, b) }

type Unsafe[T any] struct{ val T }

func NewUnsafe[T any](initial T) *Unsafe[T] { a := &Unsafe[T]{}; a.Set(initial); return a }

func (a *Unsafe[T]) Set(in T) { a.val = in }
func (a *Unsafe[T]) Get() T   { return a.val }

func (a *Unsafe[T]) Swap(new T) T {
	out := a.val
	a.val = new
	return out
}
