package dt

import (
	"encoding/json"
	"fmt"
	"iter"
	"slices"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/irt"
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

// IteratorList constructs a doubly-linked list from the elements of a Go standard library iterator.
func IteratorList[T any](in iter.Seq[T]) *List[T] { l := new(List[T]); l.Extend(in); return l }

// Append adds a variadic sequence of items to the end of the list.
func (l *List[T]) Append(items ...T) *List[T] { return l.Extend(irt.Slice(items)) }

// Extend adds all of the items in a slice to the end of the list.
func (l *List[T]) Extend(seq iter.Seq[T]) *List[T] { irt.Apply(seq, l.PushBack); return l }

// Reset removes all members of the list, and releases all references to items in the list.
func (l *List[T]) Reset() {
	// TODO(design) should this return the list for chaining or a count
	// NOTE: remove all items so that they don't pass membership checks
	for range l.IteratorPopFront() {
		continue
	}
}

// Len returns the length of the list. Because the Push/Pop operations
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
// You can use this element to produce a C-style iterator over
// the list, that removes items during the iteration:
//
//	for e := list.PopFront(); e.Ok(); e = input.PopFront() {
//		// do work
//	}
func (l *List[T]) PopFront() *Element[T] { return l.pop(l.Front()) }

// PopBack removes the last element from the list. If the list is
// empty, this returns a detached non-nil value, that will report an
// Ok() false value. You can use this element to produce a C-style iterator
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

// IteratorFront returns an iterator to the items in the list starting
// at the front and moving toward the back of the list.
//
// If you add values to the list during iteration *behind* the current
// position of the iterator, these values will not be present in the
// iterator; however, values added ahead of the current position, will
// be visible.
func (l *List[T]) IteratorFront() iter.Seq[T] { return l.iterator(l.Front, l.elemNext) }

// IteratorBack returns an iterator to the items in the list starting
// at the back and moving toward the front of the list.
//
// If you add values to the list during iteration *behind* the current
// position of the iterator, these values will not be present in the
// iterator; however, values added ahead of the current position, will
// be visible.
func (l *List[T]) IteratorBack() iter.Seq[T] { return l.iterator(l.Back, l.elemPrevious) }

// IteratorPopFront returns a destructive iterator that consumes
// elements from the list as it iterates, moving front-to-back.
//
// If you add values to the list during iteration *behind* the current
// position of the iterator, these values will not be present in the
// iterator; however, values added ahead of the current position, will
// be visible.
func (l *List[T]) IteratorPopFront() iter.Seq[T] { return l.iterator(l.PopFront, l.wrap(l.PopFront)) }

// IteratorPopBack returns a destructive iterator that consumes
// elements from the list as it iterates, moving back-to-front.
//
// If you add values to the list during iteration *behind* the current
// position of the iterator, these values will not be present in the
// iterator; however, values added ahead of the current position, will
// be visible.
func (l *List[T]) IteratorPopBack() iter.Seq[T] { return l.iterator(l.PopBack, l.wrap(l.PopBack)) }

func (*List[T]) elemNext(e *Element[T]) *Element[T]     { return e.Next() }
func (*List[T]) elemPrevious(e *Element[T]) *Element[T] { return e.Previous() }
func (*List[T]) wrap(fn func() *Element[T]) func(*Element[T]) *Element[T] {
	return func(*Element[T]) *Element[T] { return fn() }
}

func (l *List[T]) iterator(first func() *Element[T], next func(*Element[T]) *Element[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for elem := first(); elem.Ok() && yield(elem.Value()); elem = next(elem) {
			continue
		}
	}
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
	erc.Invariant(ers.If(l == nil, ErrUninitializedContainer))

	if l.head == nil {
		l.uncheckedSetup()
	}

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

// MarshalJSON produces a JSON array representing the items in the
// list. By supporting json.Marshaler and json.Unmarshaler, Elements
// and lists can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (l *List[T]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)

	_ = buf.WriteByte('[')

	if l.Len() > 0 {
		for i := l.Front(); i.Ok(); i = i.Next() {
			if i != l.Front() {
				_ = buf.WriteByte(',')
			}
			if err := enc.Encode(i.Value()); err != nil {
				return nil, err
			}
		}
	}

	_ = buf.WriteByte(']')

	return buf.Bytes(), nil
}

// UnmarshalJSON reads json input and adds that to values in the
// list. If there are elements in the list, they are not removed. By
// supporting json.Marshaler and json.Unmarshaler, Elements and lists
// can behave as arrays in larger json objects, and can be as the
// output/input of json.Marshal and json.Unmarshal.
func (l *List[T]) UnmarshalJSON(in []byte) error {
	rv := []json.RawMessage{}

	if err := json.Unmarshal(in, &rv); err != nil {
		return err
	}
	var zero T
	tail := l.Back()
	for idx := range rv {
		elem := NewElement(zero)
		if err := elem.UnmarshalJSON(rv[idx]); err != nil {
			return err
		}
		tail = tail.Append(elem)
	}
	return nil
}

////////////////////////////////////////////////////////////////////////
//
// Element implementation
//
////////////////////////////////////////////////////////////////////////

// Element is the underlying component of a list, provided by
// iterators, the Pop operations, and the Front/Back accesses in the
// list. You can use the methods on this objects to iterate through
// the list, and the Ok() method for validating zero-valued items.
//
// While Elements and Lists are not safe for concurrent access, it's
// safe to combine use of List and Element driven-mutations.
type Element[T any] struct {
	next *Element[T]
	prev *Element[T]
	list *List[T]
	ok   bool
	item T
}

// NewElement produces an unattached Element that you can use with
// Append. Element.Append(NewElement()) is essentially the same as
// List.PushBack().
func NewElement[T any](val T) *Element[T] { return &Element[T]{item: val, ok: true} }

// String returns the string form of the value of the element.
func (e *Element[T]) String() string { return fmt.Sprint(e.Value()) }

// Value accesses the element's value.
func (e *Element[T]) Value() (out T) {
	if e != nil {
		out = e.item
	}
	return
}

// Next produces the next element. This is always non-nil, *unless*
// the element is not a member of a list. At the ends of a list, the
// value is non-nil, but would return false for Ok.
func (e *Element[T]) Next() *Element[T] { return e.next }

// Previous produces the next element. This is always non-nil, *unless*
// the element is not a member of a list. At the ends of a list, the
// value is non-nil, but would return false for Ok.
func (e *Element[T]) Previous() *Element[T] { return e.prev }

// In checks to see if an element is in the specified list. Because
// elements hold a pointer to their list, this is an O(1) operation.
//
// Returns false when the element is nil.
func (e *Element[T]) In(l *List[T]) bool { return e.list.nonNil() && e.list == l }

// Set allows you to change set the value of an item in place. Returns
// true if the operation is successful. The operation fails if the Element is the
// root item in a list or not a member of a list.
//
// Set is safe to call on nil elements.
func (e *Element[T]) Set(v T) bool {
	if e.settable() {
		e.ok = true
		e.item = v
		return true
	}

	return false
}

// Append adds the element 'new' after the element 'e', inserting it
// in the next position in the list. Will return 'e' if 'new' is not
// valid for insertion into this list (e.g. it belongs to another
// list, or is attached to other elements, is already a member of this
// list, or is otherwise invalid.) PushBack and PushFront, are
// implemented in terms of Append.
func (e *Element[T]) Append(val *Element[T]) *Element[T] {
	if e.appendable(val) && val.isDetatched() {
		return e.uncheckedAppend(val)
	}

	return e
}

// Push adds a value to the list, and returns the resulting element.
func (e *Element[T]) Push(v T) *Element[T] { return e.Append(NewElement(v)) }

// Remove removes the elemtn from the list, returning true if the operation
// was successful. Remove returns false when the element is not valid
// to be removed (e.g. is not part of a list, is the root element of
// the list, etc.)
func (e *Element[T]) Remove() bool {
	if e.removable() {
		e.uncheckedRemove()
		return true
	}

	return false
}

// Drop wraps remove, and additionally, if the remove was successful,
// drops the value and sets the Ok value to false.
func (e *Element[T]) Drop() {
	if e.Remove() {
		e.item = e.zero()
		e.ok = false
	}
}

func (*Element[T]) zero() (o T) { return }

// Swap exchanges the location of two elements in a list, returning
// true if the operation was successful, and false if the elements are
// not eligible to be swapped. It is valid/possible to swap the root element of
// the list with another element to "move the head", causing a wrap
// around effect. Swap will not operate if either element is nil, or
// not a member of the same list.
func (e *Element[T]) Swap(with *Element[T]) bool {
	if e.swappable(with) {
		wprev := *with.prev
		with.uncheckedRemove()
		e.prev.uncheckedAppend(with)
		e.uncheckedRemove()
		wprev.uncheckedAppend(e)
		return true
	}

	return false
}

// Ok checks that an element is valid. Invalid elements can be
// produced at the end of iterations (e.g. the list's root object,) or
// if you attempt to Pop an element off of an empty list.
//
// Returns false when the element is nil.
func (e *Element[T]) Ok() bool                        { return e != nil && e.ok && !e.isRoot() }
func (e *Element[T]) appendable(val *Element[T]) bool { return val != nil && val.ok && e.list.nonNil() } // && new.list == nil && new.list != e.list && .next == nil && val.prev == nil
func (e *Element[T]) removable() bool                 { return e.list.nonNil() && !e.isRoot() && e.list.Len() > 0 }
func (e *Element[T]) settable() bool                  { return e != nil && !e.isRoot() }
func (e *Element[T]) isRoot() bool                    { return e.list.nonNil() && e.list.head == e }
func (e *Element[T]) isDetatched() bool               { return e.list == nil }

// make sure we have members of the same list.
func (e *Element[T]) swappable(it *Element[T]) bool {
	return it != nil && e != nil && it != e && e.list.nonNil() && e.list == it.list
}

func (e *Element[T]) uncheckedAppend(val *Element[T]) *Element[T] {
	e.list.meta.length++
	val.list = e.list
	val.prev = e
	val.next = e.next
	val.prev.next = val
	val.next.prev = val
	return val
}

func (e *Element[T]) uncheckedRemove() {
	e.list.meta.length--
	e.prev.next = e.next
	e.next.prev = e.prev
	e.list = nil
	// avoid removing references so iteration and deletes can
	// interleve (ish)
	//
	// e.next = nil
	// e.prev = nil
}

// UnmarshalJSON reads the json value, and sets the value of the
// element to the value in the json, potentially overriding an
// existing value. By supporting json.Marshaler and json.Unmarshaler,
// Elements and lists can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (e *Element[T]) UnmarshalJSON(in []byte) error {
	var val T
	if err := json.Unmarshal(in, &val); err != nil {
		return err
	}
	e.Set(val)
	return nil
}

// MarshalJSON satisfies the json.Marshaler interface. Nil and unset
// Element values are marshaled as nil.
func (e *Element[T]) MarshalJSON() ([]byte, error) {
	if !e.Ok() {
		return json.Marshal(nil)
	}

	return json.Marshal(e.Value())
}

////////////////////////////////////////////////////////////////////////
//
// sorting implementation
//
////////////////////////////////////////////////////////////////////////

// IsSorted reports if the list is sorted from low to high, according
// to the LessThan function.
func (l *List[T]) IsSorted(cf func(T, T) int) bool {
	if l == nil || l.Len() <= 1 {
		return true
	}

	comp := l.compareFunc(cf)

	for item := l.Front(); item.Next().Ok(); item = item.Next() {
		if comp(item, item.Previous()) < 0 {
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
func (l *List[T]) SortMerge(cf func(T, T) int) { *l = *mergeSort(l, cf) }

func (*List[T]) forCompare(x, y *Element[T]) (T, T) { return x.item, y.item }
func (l *List[T]) compareFunc(cf func(T, T) int) func(x, y *Element[T]) int {
	return func(x, y *Element[T]) int { return cf(l.forCompare(x, y)) }
}

// SortQuick sorts the list, by removing the elements, adding them
// to a slice, and then using sort.SliceStable(). In many cases this
// performs better than the merge sort implementation.
func (l *List[T]) SortQuick(cf func(T, T) int) {
	elems := make([]*Element[T], 0, l.Len())

	for l.Len() > 0 {
		elems = append(elems, l.PopFront())
	}

	slices.SortStableFunc(elems, l.compareFunc(cf))

	for idx := range elems {
		l.Back().Append(elems[idx])
	}
}

func mergeSort[T any](head *List[T], cf func(T, T) int) *List[T] {
	if head.Len() < 2 {
		return head
	}

	tail := split(head)

	head = mergeSort(head, cf)
	tail = mergeSort(tail, cf)

	return merge(cf, head, tail)
}

func split[T any](list *List[T]) *List[T] {
	total := list.Len()
	out := &List[T]{}
	for list.Len() > total/2 {
		out.Back().Append(list.PopFront())
	}
	return out
}

func merge[T any](cf func(T, T) int, a, b *List[T]) *List[T] {
	out := &List[T]{}
	for a.Len() != 0 && b.Len() != 0 {
		if cf(a.Front().Value(), b.Front().Value()) < 0 {
			out.Back().Append(a.PopFront())
		} else {
			out.Back().Append(b.PopFront())
		}
	}
	out.Extend(a.IteratorFront())
	out.Extend(b.IteratorFront())

	return out
}
