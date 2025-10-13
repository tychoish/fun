package dt

import (
	"context"
	"io"
	"iter"
	"sort"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/risky"
)

////////////////////////////////////////////////////////////////////////
//
// Double Linked List Implementation
//
////////////////////////////////////////////////////////////////////////

// List provides a doubly linked list. Callers are responsible for
// their own concurrency control and bounds checking, and should
// generally use with the same care as a slice.
//
// The Deque implementation in the pubsub package provides a similar
// implementation with locking and a notification system.
type List[T any] struct {
	head *Element[T]
	meta *struct{ length int }
}

// VariadicList constructs a doubly-linked list from a sequence of arguments passed to the constructor.
func VariadicList[T any](elems ...T) *List[T] { return SliceList(elems) }

// SliceList constructs a doubly-linked list from the elements of a slice.
func SliceList[T any](elems []T) *List[T] { l := new(List[T]); l.Append(elems...); return l }

// MapList constructs a doubly-linked list of Pair objects from the elements of a map.
func MapList[K comparable, V any](mp Map[K, V]) *List[Pair[K, V]] { return StreamList(mp.Stream()) }

// IteratorList constructs a doubly-linked list from the elements of a Go standard library iterator.
func IteratorList[T any](in iter.Seq[T]) *List[T] { l := new(List[T]); l.AppendIterator(in); return l }

// StreamList constructs conscturcts a doubly-linked list from the elements read from a Stream.
func StreamList[T any](st *fun.Stream[T]) *List[T] {
	l := new(List[T])
	l.AppendStream(st).Ignore().Wait()
	return l
}

// AppendStream returns a worker that adds items from the stream to the
// list. Any error returned is either a context cancellation error or
// the result of a panic in the input stream. The close method on
// the input stream is not called.
func (l *List[T]) AppendStream(iter *fun.Stream[T]) fun.Worker {
	return iter.ReadAll(fun.FromHandler(l.PushBack))
}

// Append adds a variadic sequence of items to the end of the list.
func (l *List[T]) Append(items ...T) *List[T] { return l.AppendSlice(items) }

// AppendSlice adds all of the items in a slice to the end of the list.
func (l *List[T]) AppendSlice(sl []T) *List[T] {
	for idx := range sl {
		l.PushBack(sl[idx])
	}
	return l
}

// AppendList removes items from the front of the input list, and appends
// them to the end (back) of the current list.
func (l *List[T]) AppendList(input *List[T]) *List[T] {
	for elem := input.PopFront(); elem.Ok(); elem = input.PopFront() {
		l.Back().Append(elem)
	}
	return l
}

// AppendIterator adds all of the items in the standard Go iterator to end of the list.
func (l *List[T]) AppendIterator(input iter.Seq[T]) *List[T] {
	for val := range input {
		l.PushBack(val)
	}
	return l
}

// Reset removes all members of the list, and releases all references to items in the list.
func (l *List[T]) Reset() {
	// remove all items so that they don't pass membership checks
	l.StreamPopFront().
		ReadAll(fun.FromHandler(fn.NewNoopHandler[T]())).
		PostHook(l.uncheckedSetup). // reset everything...
		Ignore().Wait()
}

// Len returns the length of the list. As the Append/Remove operations
// track the length of the list, this is an O(1) operation.
func (l *List[T]) Len() int {
	if l == nil || l.meta == nil {
		return 0
	}
	return l.meta.length
}

// PushFront creates an element and prepends it to the list. The
// performance of PushFront and PushBack are the same.
func (l *List[T]) PushFront(it T) { l.root().Push(it) }

// PushBack creates an element and appends it to the list. The
// performance of PushFront and PushBack are the same.
func (l *List[T]) PushBack(it T) { l.Back().Push(it) }

// PopFront removes the first element from the list. If the list is
// empty, this returns a nil value, that will report an Ok() false
// You can use this element to produce a C-style stream over
// the list, that removes items during the iteration:
//
//	for e := list.PopFront(); e.Ok(); e = input.PopFront() {
//		// do work
//	}
func (l *List[T]) PopFront() *Element[T] { return l.pop(l.Front()) }

// PopBack removes the last element from the list. If the list is
// empty, this returns a detached non-nil value, that will report an
// Ok() false value. You can use this element to produce a C-style stream
// over the list, that removes items during the iteration:
//
//	for e := list.PopBack(); e.Ok(); e = input.PopBack() {
//		// do work
//	}
func (l *List[T]) PopBack() *Element[T] { return l.pop(l.Back()) }

// Front returns a pointer to the first element of the list. If the
// list is empty, this is also the last element of the list. The
// operation is non-destructive. You can use this pointer to begin a
// c-style iteration over the list:
//
//	for e := list.Front(); e.Ok(); e = e.Next() {
//	       // operate
//	}
func (l *List[T]) Front() *Element[T] { return l.root().next }

// Back returns a pointer to the last element of the list. If the
// list is empty, this is also the first element of the list. The
// operation is non-destructive. You can use this pointer to begin a
// c-style iteration over the list:
//
//	for e := list.Back(); e.Ok(); e = e.Previous() {
//	       // operate
//	}
func (l *List[T]) Back() *Element[T] { return l.root().prev }

// Slice exports the contents of the list to a slice.
func (l *List[T]) Slice() Slice[T] { return risky.BlockForceIgnore(l.StreamFront().Slice) }

// Iterator returns a native go stream function for the items in a list.
func (l *List[T]) Iterator() iter.Seq[T] { return risky.BlockForce(l.StreamFront().Iterator) }

// StreamFront returns a stream over the values in the list in
// front-to-back order. The Stream is not synchronized with the
// values in the list, and will be exhausted when you reach the end of
// the list.
//
// If you add values to the list during iteration *behind* where the
// stream is, these values will not be present in the stream;
// however, values added ahead of the stream, will be visible.
func (l *List[T]) StreamFront() *fun.Stream[T] {
	current := l.root()
	return fun.MakeStream(func(context.Context) (o T, _ error) {
		current = current.Next()
		if !current.Ok() || current.isDetatched() {
			return o, io.EOF
		}
		return current.Value(), nil
	})
}

// StreamBack returns a stream that produces elements from the list,
// from the back to the front. The stream is not
// synchronized with the values in the list, and will be exhausted
// when you reach the front of the list.
//
// If you add values to the list during iteration *behind* where the
// stream is, these values will not be present in the stream;
// however, values added ahead of the stream, will be visible.
func (l *List[T]) StreamBack() *fun.Stream[T] {
	current := l.root()
	return fun.MakeStream(func(context.Context) (o T, _ error) {
		current = current.Previous()
		if !current.Ok() || current.isDetatched() {
			return o, io.EOF
		}
		return current.Value(), nil
	})
}

// StreamPopFront produces a stream that consumes elements from the
// list as it iterates, moving front-to-back.
//
// If you add values to the list during iteration *behind* where the
// stream is, these values will not be present in the stream;
// however, values added ahead of the stream, will be visible.
//
// In most cases, for destructive iteration, use the pubsub.Queue,
// pubsub.Deque, or one of the pubsub.Distributor implementations.
func (l *List[T]) StreamPopFront() *fun.Stream[T] {
	var current *Element[T]
	return fun.MakeStream(func(context.Context) (o T, _ error) {
		current = l.PopFront()

		if !current.Ok() {
			return o, io.EOF
		}

		return current.item, nil
	})
}

// StreamPopBack produces a stream that consumes elements from the
// list as it iterates, moving back-to-front.
//
// If you add values to the list during iteration *behind* where the
// stream is, these values will not be present in the stream;
// however, values added ahead of the stream, will be visible.
func (l *List[T]) StreamPopBack() *fun.Stream[T] {
	var current *Element[T]
	return fun.MakeStream(func(context.Context) (o T, _ error) {
		current = l.PopBack()
		if !current.Ok() {
			return o, io.EOF
		}
		return current.item, nil
	})
}

// Copy duplicates the list. The element objects in the list are
// distinct, though if the Values are themselves references, the
// values of both lists would be shared.
func (l *List[T]) Copy() *List[T] {
	out := &List[T]{}

	for elem := l.Front(); elem.Ok(); elem = elem.Next() {
		out.Back().Push(elem.Value())
	}

	return out
}

func (l *List[T]) nonNil() bool { return l != nil && l.head != nil && l.meta != nil }

func (l *List[T]) root() *Element[T] {
	fun.Invariant.Ok(l != nil, ErrUninitializedContainer)

	ft.CallWhen(l.head == nil, l.uncheckedSetup)

	return l.head
}

func (l *List[T]) uncheckedSetup() {
	l.meta = &struct{ length int }{}
	l.head = &Element[T]{}
	l.head.next = l.head
	l.head.prev = l.head
	l.head.list = l
	l.head.ok = false
}

func (l *List[T]) pop(it *Element[T]) *Element[T] {
	if !it.removable() || it.list == nil || l.head == nil || it.list.head != l.head {
		return &Element[T]{}
	}

	it.uncheckedRemove()

	return it
}

////////////////////////////////////////////////////////////////////////
//
// sorting implementation
//
////////////////////////////////////////////////////////////////////////

// IsSorted reports if the list is sorted from low to high, according
// to the LessThan function.
func (l *List[T]) IsSorted(lt cmp.LessThan[T]) bool {
	if l == nil || l.Len() <= 1 {
		return true
	}

	for item := l.Front(); item.Next().Ok(); item = item.Next() {
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
	for list.Len() > total/2 {
		out.Back().Append(list.PopFront())
	}
	return out
}

func merge[T any](lt cmp.LessThan[T], a, b *List[T]) *List[T] {
	out := &List[T]{}
	for a.Len() != 0 && b.Len() != 0 {
		if lt(a.Front().Value(), b.Front().Value()) {
			out.Back().Append(a.PopFront())
		} else {
			out.Back().Append(b.PopFront())
		}
	}
	out.AppendList(a)
	out.AppendList(b)

	return out
}
