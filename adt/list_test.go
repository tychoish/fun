package adt

import (
	"testing"

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
			check.Equal(t, ft.Ref(l.Head().Value()), 2)
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
		l := new(List[int])
		l.PushBack(ft.Ptr(42))
		val, ok := l.Head().Get()
		check.True(t, ok)
		check.Equal(t, 42, ft.Ref(val))
	})
}
