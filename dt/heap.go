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

// Heap provides a min-order heap using the Heap.LT comparison
// operator to sort from lowest to highest. Push operations will panic
// if LT is not set.
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

// Iterator provides an iterator to the items in the heap.
func (h *Heap[T]) Iterator() iter.Seq[T]        { ; return h.list().IteratorFront() }
func (h *Heap[T]) MarshalJSON() ([]byte, error) { return irt.MarshalJSON(h.Iterator()) }
func (h *Heap[T]) UnmarshalJSON(in []byte) error {
	for kv, err := range irt.UnmarshalJSON[T](bytes.NewBuffer(in)) {
		if err != nil {
			return err
		}
		h.Push(kv)
	}
	return nil
}
