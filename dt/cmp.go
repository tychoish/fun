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
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/fn"
)

// Heap provides a min-order heap using the Heap.LT comparison
// operator to sort from lowest to highest. Push operations will panic
// if LT is not set.
type Heap[T any] struct {
	LT   cmp.LessThan[T]
	data *List[T]
}

func (h *Heap[T]) Populate(iter *fun.Stream[T]) fun.Worker { return iter.ReadAll(h.Handler()) }
func (h *Heap[T]) Handler() fn.Handler[T]                  { return h.Push }

func (h *Heap[T]) list() *List[T] {
	if h == nil || h.LT == nil {
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
		if h.LT(t, item.item) {
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

// Stream provides an fun.Stream interface to the heap. The
// stream consumes items from the heap, and will return when the
// heap is empty.
func (h *Heap[T]) Stream() *fun.Stream[T] { ; return h.list().StreamFront() }
