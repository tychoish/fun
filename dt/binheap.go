package dt

import (
	"bytes"
	"container/heap"
	"iter"
	"slices"

	"github.com/tychoish/fun/irt"
)

// BinHeap provides a priority queue ordered by the CF comparison
// function from lowest to highest, backed by a slice and the standard
// library's container/heap. Push operations will panic if CF is not
// set.
//
// Push and Pop are O(log n); Peek and Len are O(1). In contrast to
// Heap, whose Push is O(n), BinHeap only maintains the heap
// invariant, so Iterator and MarshalJSON yield elements in heap-array
// order, which is not sorted order. Use Heap if sorted iteration or
// serialization matters.
type BinHeap[T any] struct {
	CF   func(T, T) int
	data []T
}

// binHeapAdapter implements heap.Interface over a BinHeap's slice.
type binHeapAdapter[T any] BinHeap[T]

func (*binHeapAdapter[T]) zero() (z T)          { return }
func (a *binHeapAdapter[T]) Len() int           { return len(a.data) }
func (a *binHeapAdapter[T]) Less(i, j int) bool { return a.CF(a.data[i], a.data[j]) < 0 }
func (a *binHeapAdapter[T]) Swap(i, j int)      { a.data[i], a.data[j] = a.data[j], a.data[i] }
func (a *binHeapAdapter[T]) Push(x any)         { a.data = append(a.data, x.(T)) }
func (a *binHeapAdapter[T]) Pop() any {
	last := len(a.data) - 1
	out := a.data[last]
	a.data[last] = a.zero() // release the reference for the GC
	a.data = a.data[:last]
	return out
}

func (*BinHeap[T]) zero() (zero T) { return }

func (h *BinHeap[T]) adapter() *binHeapAdapter[T] {
	if h.CF == nil {
		panic(ErrUninitializedContainer)
	}
	return (*binHeapAdapter[T])(h)
}

// Push adds an item to the heap.
func (h *BinHeap[T]) Push(t T) { heap.Push(h.adapter(), t) }

// Len reports the number of items in the heap.
func (h *BinHeap[T]) Len() int { return len(h.data) }

// Pop removes and returns the minimum element, with an Ok value that
// is false when the heap is empty.
func (h *BinHeap[T]) Pop() (T, bool) {
	if len(h.data) == 0 {
		return h.zero(), false
	}
	return heap.Pop(h.adapter()).(T), true
}

// Peek returns the minimum element without removing it, with an Ok
// value that is false when the heap is empty.
func (h *BinHeap[T]) Peek() (T, bool) {
	if len(h.data) == 0 {
		return h.zero(), false
	}
	return h.data[0], true
}

// Iterator provides a non-destructive iterator over the items in the
// heap in heap-array order: the minimum is first, but the remaining
// elements are NOT sorted. Do not mutate the heap during iteration.
func (h *BinHeap[T]) Iterator() iter.Seq[T] { return slices.Values(h.data) }

// MarshalJSON encodes the heap as a JSON array in heap-array order,
// which is NOT sorted (unlike Heap.MarshalJSON). Decoding the result
// with UnmarshalJSON reproduces an equivalent heap.
func (h *BinHeap[T]) MarshalJSON() ([]byte, error) { return irt.MarshalJSON(h.Iterator()) }

// UnmarshalJSON decodes a JSON array and pushes each element onto the heap.
func (h *BinHeap[T]) UnmarshalJSON(in []byte) error {
	for kv, err := range irt.UnmarshalJSON[T](bytes.NewBuffer(in)) {
		if err != nil {
			return err
		}
		h.Push(kv)
	}
	return nil
}
