// package dt provides container type implementations and
// interfaces.
//
// All top level structures in this package can be trivially
// constructed and provide high level interfaces for most common
// operations. These structures are not safe for access from multiple
// concurrent go routines (see the queue/deque in the pubsub package
// as an alternative for these use cases.)
package dt

import (
	"context"
	"sort"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
)

// Heap provides a min-order heap using the Heap.LT comparison
// operator to sort from lowest to highest. Push operations will panic
// if LT is not set.
type Heap[T any] struct {
	LT   cmp.LessThan[T]
	list *List[T]
}

// NewHeapFromIterator constructs and populates a heap using the input
// iterator and corresponding comparison function. Will return the
// input iterators Close() error if one exists, and otherwise will
// return the populated heap.
func NewHeapFromIterator[T any](ctx context.Context, cmp cmp.LessThan[T], iter *fun.Iterator[T]) (*Heap[T], error) {
	out := &Heap[T]{LT: cmp}
	out.lazySetup()
	return out, iter.Observe(ctx, func(in T) { out.Push(in) })
}

func (h *Heap[T]) lazySetup() {
	if h == nil || h.LT == nil {
		panic(ErrUninitializedContainer)
	}

	if h.list == nil {
		h.list = &List[T]{}
		h.list.lazySetup()
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
func (h *Heap[T]) Iterator() *fun.Iterator[T] { h.lazySetup(); return h.list.Iterator() }

// IsSorted reports if the list is sorted from low to high, according
// to the LessThan function.
func (l *List[T]) IsSorted(lt cmp.LessThan[T]) bool {
	if l == nil || l.Len() <= 1 {
		return true
	}

	for item := l.root.Next(); item.next.Ok(); item = item.Next() {
		if lt(item.Value(), item.Previous().Value()) {
			return false
		}
	}
	return true
}

// SortMerge sorts the list, using the provided comparison
// function and a Merge Sort operation. This is something of a novelty
// in most cases, as removing the elements from the list, adding to a
// slice and then using sort.Slice() from the standard library, and
// then re-adding those elements to the list, will perform better.
//
// The operation will modify the input list, replacing it with an new
// list operation.
func (l *List[T]) SortMerge(lt cmp.LessThan[T]) { *l = *mergeSort(l, lt) }

// SortQuick sorts the list, by removing the elements, adding them
// to a slice, and then using sort.SliceStable(). In many cases this
// performs better than the merge sort implementation.
func (l *List[T]) SortQuick(lt cmp.LessThan[T]) {
	elems := make([]*Element[T], 0, l.Len())

	for l.Len() > 0 {
		elems = append(elems, l.PopFront())
	}
	sort.SliceStable(elems, func(i, j int) bool { return lt(elems[i].item, elems[j].item) })
	for idx := range elems {
		l.Back().Append(elems[idx])
	}
}

func mergeSort[T any](head *List[T], lt cmp.LessThan[T]) *List[T] {
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
	out.lazySetup()
	for list.Len() > total/2 {
		out.Back().Append(list.PopFront())
	}
	return out
}

func merge[T any](lt cmp.LessThan[T], a, b *List[T]) *List[T] {
	out := &List[T]{}
	out.lazySetup()
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
