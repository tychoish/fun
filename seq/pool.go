package seq

import (
	"fmt"
	"runtime"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
)

// this is a some what experimental pool for element objects.
//
// it doesn't particularly improve performance in the common case (as
// the element objects themselves are pretty small,) and by default
// list/stack don't use them for Push operations. (the
// NewElement/NewItem) methods do, just to prevent bitrot.

var elemPools *adt.Map[string, any]
var itemPools *adt.Map[string, any]

func init() {
	itemPools = &adt.Map[string, any]{}
	elemPools = &adt.Map[string, any]{}
}

func getElementPool[T any](val T) *adt.Pool[*Element[T]] {
	// beause types aren't comparble/values you can't use them as
	// keys in maps, this is just a trick to get a string that's usable.
	key := fmt.Sprintf("%T", val)
	if out, ok := elemPools.Load(key); ok {
		return out.(*adt.Pool[*Element[T]])
	}

	return elemPools.EnsureDefault(key, func() any {
		pool := &adt.Pool[*Element[T]]{}
		pool.SetConstructor(func() *Element[T] {
			e := &Element[T]{}
			runtime.SetFinalizer(e, func(elem *Element[T]) {
				elem.ok = false
				elem.item = fun.ZeroOf[T]()
				go pool.Put(elem)
			})
			return e
		})

		return pool
	}).(*adt.Pool[*Element[T]])
}

func makeElement[T any](val T) *Element[T] { return getElement(getElementPool(val), val) }

func getElement[T any](pool *adt.Pool[*Element[T]], val T) *Element[T] {
	elem := pool.Get()
	elem.item = val
	elem.ok = true
	return elem
}

func getItemPool[T any](val T) *adt.Pool[*Item[T]] {
	key := fmt.Sprintf("%T", val)
	if out, ok := itemPools.Load(key); ok {
		return out.(*adt.Pool[*Item[T]])
	}

	return itemPools.EnsureDefault(key, func() any {
		pool := &adt.Pool[*Item[T]]{}
		pool.SetConstructor(func() *Item[T] {
			i := &Item[T]{}
			runtime.SetFinalizer(i, func(item *Item[T]) {
				item.ok = false
				item.value = fun.ZeroOf[T]()
				go pool.Put(item)
			})
			return i
		})
		return pool
	}).(*adt.Pool[*Item[T]])
}

func makeItem[T any](val T) *Item[T] { return getItem(getItemPool(val), val) }

func getItem[T any](pool *adt.Pool[*Item[T]], val T) *Item[T] {
	item := pool.Get()
	item.value = val
	item.ok = true

	return item
}
