// Package seq provides single and double linked-list implementations
// and tools (e.g. heap, sorting, iterators) for using these
// structures.
//
// All top level structures in this package can be trivially
// constructed and provide high level interfaces for most common
// operations. These structures are not safe for access from multiple
// concurrent go routines (see the queue/deque in the pubsub package
// as an alternative for these use cases.)
package seq

import (
	"sort"
	"time"

	"github.com/tychoish/fun"
)

// Orderable describes all native types which (currently) support the
// < operator. To order custom types, use the OrderableUser interface.
type Orderable interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~string
}

// OrderableUser allows users to define a method on their types which
// implement a method to provide a LessThan operation.
type OrderableUser[T any] interface{ LessThan(T) bool }

// LessThan describes a less than operation, typically provided by one
// of the following operations.
type LessThan[T any] func(a, b T) bool

// LessThanNative provides a wrapper around the < operator for types
// that support it, and can be used for sorting lists of compatible
// types.
func LessThanNative[T Orderable](a, b T) bool { return a < b }

// LessThanCustom converts types that implement OrderableUser
// interface.
func LessThanCustom[T OrderableUser[T]](a, b T) bool { return a.LessThan(b) }

// LessThanTime compares time using the time.Time.Before() method.
func LessThanTime(a, b time.Time) bool { return a.Before(b) }

// Reverse wraps an existing LessThan operator and reverses it's
// direction.
func Reverse[T any](fn LessThan[T]) LessThan[T] { return func(a, b T) bool { return !fn(a, b) } }

// Heap provides a min-order heap using the Heap.LT comparison
// operator to sort from lowest to highest. Push operations will panic
// if LT is not set.
type Heap[T any] struct {
	LT   LessThan[T]
	list *List[T]
}

func (h *Heap[T]) lazySetup() {
	if h == nil || h.LT == nil {
		panic(ErrUninitialized)
	}

	if h.list == nil {
		h.list = &List[T]{}
	}
}

// Push adds an item to the heap.
func (h *Heap[T]) Push(t T) {
	h.lazySetup()
	if h.list.length == 0 {
		h.list.PushBack(t)
		return
	}

	for item := h.list.Back(); item.Ok(); item = item.Previous() {
		if h.LT(t, item.item) {
			continue
		}

		item.Append(NewElement(t))
		return
	}

	h.list.PushFront(t)
}

// Len reports the size of the heap. Because the heap tracks its size
// with Push/Pop operations, this is a constant time operation.
func (h *Heap[T]) Len() int {
	if h.list == nil {
		return 0
	}

	return h.list.Len()
}

// Pop removes the element from the underlying list and returns
// it, with an OK value, which is true when the value returned is valid.
func (h *Heap[T]) Pop() (T, bool) { h.lazySetup(); e := h.list.PopFront(); return e.Value(), e.Ok() }

// Iterator provides an fun.Iterator interface to the heap. The
// iterator consumes items from the heap, and will return when the
// heap is empty.
func (h *Heap[T]) Iterator() fun.Iterator[T] { h.lazySetup(); return ListValues(h.list.IteratorPop()) }

// IsSorted reports if the list is sorted from low to high, according
// to the LessThan function.
func IsSorted[T any](list *List[T], lt LessThan[T]) bool {
	if list == nil || list.Len() <= 1 {
		return true
	}

	for item := list.root.next; item.next.ok; item = item.next {
		if lt(item.item, item.prev.item) {
			return false
		}
	}
	return true
}

// SortListMerge sorts the list, using the provided comparison
// function and a Merge Sort operation. This is something of a novelty
// in most cases, as removing the elements from the list, adding to a
// slice and then using sort.Slice() from the standard library, and
// then re-adding those elements to the list, will perform better.
//
// The operation will modify the input list, replacing it with an new
// list operation.
func SortListMerge[T any](list *List[T], lt LessThan[T]) { *list = *mergeSort(list, lt) }

// SortListQuick sorts the list, by removing the elements, adding them
// to a slice, and then using sort.SliceStable(). In many cases this
// performs better than the merge sort implementation.
func SortListQuick[T any](list *List[T], lt LessThan[T]) {
	elems := make([]*Element[T], 0, list.Len())

	for list.Len() > 0 {
		elems = append(elems, list.PopFront())
	}
	sort.SliceStable(elems, func(i, j int) bool { return lt(elems[i].item, elems[j].item) })
	for idx := range elems {
		list.Back().Append(elems[idx])
	}
}

func mergeSort[T any](head *List[T], lt LessThan[T]) *List[T] {
	if head.Len() < 2 {
		return head
	}

	tail := split(head)

	head = mergeSort(head, lt)
	tail = mergeSort(tail, lt)

	return merge(lt, head, tail)
}

func split[T any](list *List[T]) *List[T] {
	total := list.Len()
	out := &List[T]{}
	for list.Len() > total/2 {
		out.Back().Append(list.PopFront())
	}
	return out
}

func merge[T any](lt LessThan[T], a, b *List[T]) *List[T] {
	out := &List[T]{}
	for a.Len() != 0 && b.Len() != 0 {
		if lt(a.Front().Value(), b.Front().Value()) {
			out.Back().Append(a.PopFront())
		} else {
			out.Back().Append(b.PopFront())
		}
	}
	out.Extend(a)
	out.Extend(b)

	return out
}
