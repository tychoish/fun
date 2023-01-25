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

func LessThanNative[T Orderable](a, b T) bool        { return a < b }
func LessThanCustom[T OrderableUser[T]](a, b T) bool { return a.LessThan(b) }
func LessThanTime(a, b time.Time) bool               { return a.Before(b) }

// Reverse wraps an existing LessThan operator and reverses it's
// direction.
func Reverse[T any](fn LessThan[T]) LessThan[T] { return func(a, b T) bool { return !fn(a, b) } }

type Heap[T any] struct {
	LT   LessThan[T]
	list *List[T]
}

func (h *Heap[T]) lazySetup() {
	if h.list == nil {
		h.list = &List[T]{}
	}
}

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

func (h *Heap[T]) Len() int {
	if h.list == nil {
		return 0
	}

	return h.list.Len()
}

// Pop removes the element from the underlying list and returns
// it, with an OK value, which is true when the value returned is valid.
func (h *Heap[T]) Pop() (T, bool)            { h.lazySetup(); e := h.list.PopFront(); return e.Value(), e.Ok() }
func (h *Heap[T]) Iterator() fun.Iterator[T] { h.lazySetup(); return ListValues(h.list.IteratorPop()) }

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

func Sort[T any](list *List[T], lt LessThan[T]) { *list = *mergeSort(list, lt) }

func mergeSort[T any](head *List[T], lt LessThan[T]) *List[T] {
	if head.Len() < 2 {
		return head
	}

	tail := split(head)

	head = mergeSort(head, lt)
	tail = mergeSort(tail, lt)

	return func(a, b *List[T]) *List[T] {
		out := &List[T]{}
		for a.Front().Ok() && b.Front().Ok() {
			if lt(a.Front().Value(), b.Front().Value()) {
				unsafeRemoveAndPush(a.Front(), out.Back())
			} else {
				unsafeRemoveAndPush(b.Front(), out.Back())
			}
		}
		for e := a.Front(); e.Ok(); e = a.Front() {
			unsafeRemoveAndPush(e, out.Back())
		}
		for e := b.Front(); e.Ok(); e = b.Front() {
			unsafeRemoveAndPush(e, out.Back())
		}

		return out

	}(head, tail)
}

func split[T any](list *List[T]) *List[T] {
	total := list.Len()
	out := &List[T]{}
	for list.Len() > total/2 {
		out.Back().Append(list.PopFront())
	}
	return out
}

func unsafeRemoveAndPush[T any](a, b *Element[T]) {
	a.Remove()
	b.Append(a)
}
