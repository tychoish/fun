package seq

import (
	"fmt"
	"runtime"
	"sync"
)

// this is a some what experimental pool for element objects.
//
// it doesn't particularly improve performance in the common case (as
// the element objects themselves are pretty small,) and by default
// list/stack don't use them for Push operations. (the
// NewElement/NewItem) methods do, just to prevent bitrot.

var elemPoolsMtx *sync.Mutex
var elemPools map[any]*sync.Pool

var itemPoolsMtx *sync.Mutex
var itemPools map[any]*sync.Pool

func init() {
	itemPoolsMtx = &sync.Mutex{}
	itemPools = map[any]*sync.Pool{}

	elemPoolsMtx = &sync.Mutex{}
	elemPools = map[any]*sync.Pool{}
}

func getElementPool[T any](val T) *sync.Pool {
	// beause types aren't comparble/values you can't use them as
	// keys in maps, this is just a trick to get a string that's usable.
	key := fmt.Sprintf("%T", val)

	elemPoolsMtx.Lock()
	defer elemPoolsMtx.Unlock()

	pool, ok := elemPools[key]
	if ok {
		return pool
	}

	zero := *new(T)
	pool = &sync.Pool{
		New: func() any {
			e := &Element[T]{}
			runtime.SetFinalizer(e, func(elem *Element[T]) {
				elem.ok = false
				elem.item = zero
				go pool.Put(elem)
			})
			return e
		},
	}

	elemPools[key] = pool
	return pool
}

func makeElement[T any](val T) *Element[T] { return getElement(getElementPool(val), val) }

func getElement[T any](pool *sync.Pool, val T) *Element[T] {
	elem := pool.Get().(*Element[T])
	elem.item = val
	elem.ok = true
	return elem
}

func getItemPool[T any](val T) *sync.Pool {
	key := fmt.Sprintf("%T", val)

	var pool *sync.Pool
	var ok bool

	itemPoolsMtx.Lock()
	defer itemPoolsMtx.Unlock()

	pool, ok = itemPools[key]
	if ok {
		return pool
	}

	zero := *new(T)
	pool = &sync.Pool{
		New: func() any {
			i := &Item[T]{}
			runtime.SetFinalizer(i, func(item *Item[T]) {
				item.ok = false
				item.value = zero
				go pool.Put(item)
			})
			return i
		},
	}
	itemPools[key] = pool
	return pool
}

func makeItem[T any](val T) *Item[T] { return getItem(getItemPool(val), val) }

func getItem[T any](pool *sync.Pool, val T) *Item[T] {
	item := pool.Get().(*Item[T])
	item.value = val
	item.ok = true

	return item
}
