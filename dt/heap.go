// Package dt provides container type implementations and
// interfaces.
//
// All top level structures in this package can be trivially
// constructed and provide high level interfaces for most common
// operations. These structures are not safe for access from multiple
// concurrent go routines (see the queue/deque in the pubsub package
// as an alternative for these use cases.)
package dt

import (
	"bytes"
	"iter"

	"github.com/tychoish/fun/irt"
)

// Heap provides a priority queue ordered by the CF comparison
// function from lowest to highest. Push operations will panic if CF is
// not set.
//
// Heap is backed by a sorted linked list: Push is O(n) (a linear scan
// for the insertion point), while Pop, Peek, and Len are O(1).
// Iteration and JSON encoding yield a fully sorted sequence. For
// O(log n) Push and Pop, use BinHeap, which does not guarantee sorted
// iteration order.
type Heap[T any] struct {
	CF   func(T, T) int
	data *List[T]
}

func (h *Heap[T]) list() *List[T] {
	if h.CF == nil {
		panic(ErrUninitializedContainer)
	}

	if h.data == nil {
		h.data = &List[T]{}
	}
	return h.data
}

// Push adds an item to the heap.
func (h *Heap[T]) Push(t T) {
	list := h.list()

	if list.Len() == 0 {
		list.PushBack(t)
		return
	}

	for item := list.Back(); item.Ok(); item = item.Previous() {
		if h.CF(t, item.item) < 0 {
			continue
		}

		item.Append(NewElement(t))
		return
	}

	list.PushFront(t)
}

// Len reports the size of the heap. Because the heap tracks its size
// with Push/Pop operations, this is a constant time operation.
func (h *Heap[T]) Len() int { return h.list().Len() }

// Pop removes the element from the underlying list and returns
// it, with an Ok value, which is true when the value returned is valid.
func (h *Heap[T]) Pop() (T, bool) { e := h.list().PopFront(); return e.Value(), e.Ok() }

// Peek returns the minimum element without removing it, with an Ok
// value that is false when the heap is empty.
func (h *Heap[T]) Peek() (T, bool) { e := h.list().Front(); return e.Value(), e.Ok() }

// Iterator provides an iterator to the items in the heap, in sorted order.
func (h *Heap[T]) Iterator() iter.Seq[T] { return h.list().IteratorFront() }

// MarshalJSON encodes the heap as a JSON array in sorted order.
func (h *Heap[T]) MarshalJSON() ([]byte, error) { return irt.MarshalJSON(h.Iterator()) }

// UnmarshalJSON decodes a JSON array and pushes each element onto the heap.
func (h *Heap[T]) UnmarshalJSON(in []byte) error {
	for kv, err := range irt.UnmarshalJSON[T](bytes.NewBuffer(in)) {
		if err != nil {
			return err
		}
		h.Push(kv)
	}
	return nil
}
