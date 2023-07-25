package adt

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/testt"
)

func TestAtomics(t *testing.T) {
	t.Run("SafeSet", func(t *testing.T) {
		t.Run("AtomicInt", func(t *testing.T) {
			atom := &Atomic[int]{}
			SafeSet[int](atom, -1)
			assert.Equal(t, atom.Get(), -1)
			SafeSet[int](atom, 0) // noop
			assert.Equal(t, atom.Get(), -1)
		})
		t.Run("Int64", func(t *testing.T) {
			atom := &atomic.Int64{}
			SafeSet[int64](atom, -1)
			assert.Equal(t, atom.Load(), -1)
			SafeSet[int64](atom, 0) // noop
			assert.Equal(t, atom.Load(), -1)
		})
		t.Run("AtomicInt", func(t *testing.T) {
			atom := &Synchronized[int8]{}
			SafeSet[int8](atom, -1)
			assert.Equal(t, atom.Get(), -1)
			SafeSet[int8](atom, 0) // noop
			assert.Equal(t, atom.Get(), -1)
		})
	})
	t.Run("Default", func(t *testing.T) {
		at := NewAtomic(1000)
		assert.Equal(t, at.Get(), 1000)
	})
	t.Run("Reset", func(t *testing.T) {
		t.Run("Atomic", func(t *testing.T) {
			atom := &Atomic[int]{}
			atom.Store(100)
			assert.Equal(t, Reset[int](atom), 100)
			assert.Equal(t, atom.Load(), 0)
		})
		t.Run("Int64", func(t *testing.T) {
			atom := &atomic.Int64{}
			atom.Store(100)
			assert.Equal(t, Reset[int64](atom), 100)
			assert.Equal(t, atom.Load(), 0)
		})
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
		assert.True(t, ft.IsZero[*Atomic[int]](nil))
		var f *Atomic[int]
		assert.True(t, ft.IsZero(f))
		f = &Atomic[int]{}
		// ideally this should be
		// true, but...
		assert.True(t, !ft.IsZero(f))
		// clearly true
		assert.True(t, ft.IsZero(f.Get()))
		f = NewAtomic(100)
		assert.True(t, !ft.IsZero(f))
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
			atom.Store(54)
			assert.Panic(t, func() {
				_ = CompareAndSwap[int](atom, 54, 100)
			})
		})

	})
	t.Run("Once", func(t *testing.T) {
		t.Run("Do", func(t *testing.T) {
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
						actor.Do(func() int {
							count.Add(1)
							return 42
						})
						val := actor.Resolve()
						if val != 42 {
							panic(fmt.Errorf("once function produced %d not 42", val))
						}
					}
				}()
			}

			wg.Wait(ctx)
			assert.Equal(t, count.Load(), 1)

		})
		t.Run("Resolve", func(t *testing.T) {
			t.Parallel()
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			count := &atomic.Int64{}
			actor := NewOnce(func() int {
				count.Add(1)
				return 42
			})
			check.True(t, actor.Defined())
			check.True(t, !actor.Called())
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
						val := actor.Resolve()
						check.True(t, actor.Called())
						if val != 42 {
							panic(fmt.Errorf("once function produced %d not 42", val))
						}
					}
				}()
			}

			wg.Wait(ctx)
			assert.Equal(t, count.Load(), 1)
		})
		t.Run("Zero", func(t *testing.T) {
			count := &atomic.Int64{}
			actor := &Once[int]{}
			assert.True(t, !actor.Defined())
			assert.Zero(t, actor.Resolve())
			actor.Do(func() int {
				count.Add(1)
				return 42
			})
			assert.True(t, !actor.Defined())
			assert.Zero(t, actor.Resolve())
			assert.True(t, !actor.Defined())
		})
		t.Run("SetWorflow", func(t *testing.T) {
			count := &atomic.Int64{}
			actor := &Once[int]{}
			actor.Set(func() int {
				count.Add(1)
				return 42
			})
			assert.True(t, actor.Defined())
			assert.Equal(t, 42, actor.Resolve())
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

func NewUnsafe[T any](initial T) *Unsafe[T] { a := &Unsafe[T]{}; a.Store(initial); return a }

func (a *Unsafe[T]) Store(in T) { a.val = in }
func (a *Unsafe[T]) Load() T    { return a.val }

func (a *Unsafe[T]) Swap(new T) T {
	out := a.val
	a.val = new
	return out
}
