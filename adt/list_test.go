package adt

import (
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestAtomicList(t *testing.T) {
	t.Run("BasicSmoke", func(t *testing.T) {
		t.Run("Back", func(t *testing.T) {
			l := new(List[int])
			check.Equal(t, l.Len(), 0)
			l.PushBack(ft.Ptr(1))
			check.Equal(t, l.Len(), 1)
			l.PushBack(ft.Ptr(2))
			check.Equal(t, l.Len(), 2)
			// recall
			check.Equal(t, ft.Ref(l.Tail().Value()), 2)
			check.Equal(t, l.Len(), 2)
			check.NotNil(t, l.Head())
			check.NotNil(t, l.Tail())
			check.Equal(t, ft.Ref(l.PopBack()), 2)
			check.Equal(t, l.Len(), 1)
			check.NotNil(t, l.PopBack())
			check.Equal(t, l.Len(), 0)
		})
		t.Run("", func(t *testing.T) {
			l := new(List[int])
			check.Equal(t, l.Len(), 0)
			l.PushFront(ft.Ptr(1))
			check.Equal(t, l.Len(), 1)
			l.PushFront(ft.Ptr(2))
			check.Equal(t, l.Len(), 2)
			// recall
			check.Equal(t, l.Head().Ref(), 2)
			check.Equal(t, l.Len(), 2)
			check.NotNil(t, l.Head())
			check.NotNil(t, l.Tail())
			check.Equal(t, ft.Ref(l.PopFront()), 2)
			check.Equal(t, l.Len(), 1)
			check.NotNil(t, l.PopFront())
			check.Equal(t, l.Len(), 0)
		})
	})
	t.Run("PopEmptyList", func(t *testing.T) {
		t.Run("Back", func(t *testing.T) {
			l := new(List[int])
			check.Equal(t, l.Len(), 0)
			check.NotPanic(t, func() { check.Nil(t, l.PopBack()) })
			check.Equal(t, l.Len(), 0)
			check.NotPanic(t, func() { check.Nil(t, l.PopBack()) })
		})
		t.Run("Front", func(t *testing.T) {
			l := new(List[int])
			check.Equal(t, l.Len(), 0)
			check.NotPanic(t, func() { check.Nil(t, l.PopFront()) })
			check.Equal(t, l.Len(), 0)
			check.NotPanic(t, func() { check.Nil(t, l.PopFront()) })
		})
		t.Run("ValueAccessorIsNilSafe", func(t *testing.T) {
			l := new(List[int])
			elem := l.Tail()
			assert.Nil(t, elem)
			assert.Nil(t, elem.Value())
			assert.True(t, ft.Not(elem.Ok()))
		})
	})
	t.Run("Access", func(t *testing.T) {
		t.Run("Get", func(t *testing.T) {
			t.Run("NonNil", func(t *testing.T) {
				l := new(List[int])
				l.PushBack(ft.Ptr(42))
				val, ok := l.Head().Get()
				check.True(t, ok)
				check.Equal(t, 42, ft.Ref(val))
				vv, ok := l.Head().RefOk()
				check.Equal(t, vv, 42)
				check.True(t, ok)
			})
			t.Run("Nil", func(t *testing.T) {
				l := new(List[int])
				check.Equal(t, l.Len(), 0)
				elem := l.Head()
				check.Nil(t, elem)
				check.Equal(t, l.Len(), 0)
				v, ok := elem.Get()
				check.Nil(t, v)
				check.True(t, ft.Not(ok))
				vv, ok := elem.RefOk()
				check.Zero(t, vv)
				check.True(t, ft.Not(ok))
			})
		})
		t.Run("Unset", func(t *testing.T) {
			l := new(List[int])
			l.PushBack(ft.Ptr(42))
			elem := l.Head()
			check.True(t, elem.Ok())
			v, ok := elem.Unset()
			check.True(t, ok)
			check.NotNil(t, v)
			check.Equal(t, ft.Ref(v), 42)
			v, ok = elem.Unset()
			check.True(t, ft.Not(ok))
			check.Nil(t, v)
		})
		t.Run("UnsetNil", func(t *testing.T) {
			l := new(List[int])
			check.Equal(t, l.Len(), 0)
			nada := l.Tail()
			check.Nil(t, nada)
			check.Nil(t, nada.Value())
			check.True(t, ft.Not(nada.Ok()))
			v, ok := nada.Unset()
			check.Nil(t, v)
			check.True(t, ft.Not(ok))
		})

		t.Run("Drop", func(t *testing.T) {
			l := new(List[int])
			l.PushBack(ft.Ptr(42))
			check.NotNil(t, l.Head())
			check.Equal(t, l.Len(), 1)
			check.NotNil(t, l.Head())
			check.Equal(t, l.Len(), 1)
			l.Head().Drop()
			check.Equal(t, l.Len(), 0)
			check.Panic(t, func() {
				l.Head().Drop()
			})
		})
	})
	t.Run("LockHandling", func(t *testing.T) {
		t.Run("Smoke", func(t *testing.T) {
			l := new(List[int])
			l.PushBack(ft.Ptr(4))
			l.PushBack(ft.Ptr(8))
			l.PushBack(ft.Ptr(16))
			l.PushBack(ft.Ptr(32))
			l.PushBack(ft.Ptr(64))
			mid := l.Head().Next().Next()
			check.Equal(t, ft.Ref(mid.Value()), 16)
			mid.mtx().Lock()
			sig := make(chan struct{})
			startSig := make(chan struct{})
			called := &atomic.Int64{}
			started := &atomic.Int64{}
			go func() {
				started.Add(1)
				close(startSig)
				defer close(sig)
				defer called.Add(1)
				l.Head().Next().append(l.makeElem(ft.Ptr(12)))
			}()
			runtime.Gosched()
			<-startSig
			time.Sleep(3 * time.Millisecond)
			check.Equal(t, started.Load(), 1)
			check.Equal(t, called.Load(), 0)
			check.Equal(t, l.Len(), 5)
			l.PushBack(ft.Ptr(128))
			check.Equal(t, l.Len(), 6)
			check.Equal(t, started.Load(), 1)
			check.Equal(t, called.Load(), 0)
			mid.mtx().Unlock()
			<-sig
			check.Equal(t, called.Load(), 1)
			check.Equal(t, l.Len(), 7)
		})
		t.Run("Panic", func(t *testing.T) {
			check.Panic(t, func() { doPanic(nil) })
			check.Panic(t, func() { doPanic("hello") })
			check.Panic(t, func() { ft.ApplyWhen(true, doPanic, 1) })
			check.Panic(t, func() { ft.ApplyWhen(true, doPanic, nil) })
			check.NotPanic(t, func() { ft.ApplyWhen(false, doPanic, 1) })
			check.NotPanic(t, func() { ft.ApplyWhen(false, doPanic, nil) })
		})
	})
	t.Run("AddingElements", func(t *testing.T) {
		l := new(List[int])
		wg := &sync.WaitGroup{}
		wg.Add(32)
		for range 32 {
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int64N(10 * int64(time.Millisecond))))
				l.Append(rand.Int())
			}()
		}
	})
}
