package dt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/risky"
)

// List provides a doubly linked list. Callers are responsible for
// their own concurrency control and bounds checking, and should
// generally use with the same care as a slice.
//
// The Deque implementation in the pubsub package provides a similar
// implementation with locking and a notification system.
type List[T any] struct {
	head   *Element[T]
	length int
}

// NewListFromIterator builds a list from the elements in the
// iterator. Any error returned is either a context cancellation error
// or the result of a panic in the input iterator. The close method on
// the input iterator is not called.
func NewListFromIterator[T any](ctx context.Context, iter *fun.Iterator[T]) (*List[T], error) {
	out := &List[T]{}
	if err := out.Populate(iter).Run(ctx); err != nil {
		return nil, err
	}

	return out, nil
}

// Append adds a variadic sequence of items to the end of the list.
func (l *List[T]) Append(items ...T) {
	for idx := range items {
		l.PushBack(items[idx])
	}
}

// Populate returns a worker that adds items from the iterator to the
// list. Any error returned is either a context cancellation error or
// the result of a panic in the input iterator. The close method on
// the input iterator is not called.
func (l *List[T]) Populate(iter *fun.Iterator[T]) fun.Worker {
	return iter.Process(fun.Handle(l.PushBack).Processor())
}

// Element is the underlying component of a list, provided by
// iterators, the Pop operations, and the Front/Back accesses in the
// list. You can use the methods on this objects to iterate through
// the list, and the Ok() method for validating zero-valued items.
type Element[T any] struct {
	next *Element[T]
	prev *Element[T]
	list *List[T]
	ok   bool
	item T
}

func (e *Element[T]) isRoot() bool { return e.list != nil && e.list.head != nil && e.list.head == e }

// NewElement produces an unattached Element that you can use with
// Append. Element.Append(NewElement()) is essentially the same as
// List.PushBack().
func NewElement[T any](val T) *Element[T] { return &Element[T]{item: val, ok: true} }

// String returns the string form of the value of the element.
func (e *Element[T]) String() string { return fmt.Sprint(e.item) }

// Value accesses the element's value.
func (e *Element[T]) Value() T { return e.item }

// Ok checks that an element is valid. Invalid elements can be
// produced at the end of iterations (e.g. the list's root object,) or
// if you attempt to Pop an element off of an empty list.
//
// Returns false when the element is nil.
func (e *Element[T]) Ok() bool { return e != nil && e.ok }

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
func (e *Element[T]) In(l *List[T]) bool { return e.list != nil && e.list == l }

// Set allows you to change set the value of an item in place. Returns
// true if the operation is successful. The operation fails if the Element is the
// root item in a list or not a member of a list.
//
// Set is safe to call on nil elements.
func (e *Element[T]) Set(v T) bool {
	if e == nil || (e.list != nil && e.list.head == e) {
		return false
	}

	e.ok = true
	e.item = v
	return true
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

// Append adds the element 'new' after the element 'e', inserting it
// in the next position in the list. Will return 'e' if 'new' is not
// valid for insertion into this list (e.g. it belongs to another
// list, or is attached to other elements, is already a member of this
// list, or is otherwise invalid.) PushBack and PushFront, are
// implemented in terms of Append.
func (e *Element[T]) Append(val *Element[T]) *Element[T] {
	if !e.appendable(val) {
		return e
	}

	e.uncheckedAppend(val)
	return val
}

func (e *Element[T]) Push(v T) *Element[T] { return e.Append(makeElem(v)) }

// Remove removes the elemtn from the list, returning true if the operation
// was successful. Remove returns false when the element is not valid
// to be removed (e.g. is not part of a list, is the root element of
// the list, etc.)
func (e *Element[T]) Remove() bool {
	if !e.removable() {
		return false
	}
	e.uncheckedRemove()
	return true
}

// Drop wraps remove, and additionally, if the remove was successful,
// drops the value and sets the Ok value to false.
func (e *Element[T]) Drop() {
	if !e.Remove() {
		return
	}
	e.item = e.zero()
	e.ok = false
}

func (*Element[T]) zero() (o T) { return }
func (*List[T]) zero() (o T)    { return }

// Swap exchanges the location of two elements in a list, returning
// true if the operation was successful, and false if the elements are
// not eligible to be swapped. It is valid/possible to swap the root element of
// the list with another element to "move the head", causing a wrap
// around effect. Swap will not operate if either element is nil, or
// not a member of the same list.
func (e *Element[T]) Swap(with *Element[T]) bool {
	if !e.swapable(with) {
		return false
	}
	wprev := *with.prev
	with.uncheckedRemove()
	e.prev.uncheckedAppend(with)
	e.uncheckedRemove()
	wprev.uncheckedAppend(e)
	return true
}

// make sure we have members of the same list, and not the same element.
func (e *Element[T]) swapable(with *Element[T]) bool {
	return with != nil && e != nil && e.list != nil && e.list == with.list && e != with
}
func (e *Element[T]) appendable(val *Element[T]) bool { return val != nil && val.ok && e.list != nil } // && new.list == nil && new.list != e.list && .next == nil && val.prev == nil
func (e *Element[T]) removable() bool                 { return e.list != nil && e.list.head != e && e.list.length > 0 }

func (e *Element[T]) uncheckedAppend(val *Element[T]) {
	e.list.length++
	val.list = e.list
	val.prev = e
	val.next = e.next
	val.prev.next = val
	val.next.prev = val
}

func (e *Element[T]) uncheckedRemove() {
	e.list.length--
	e.prev.next = e.next
	e.next.prev = e.prev
	e.list = nil
	// avoid removing references so iteration and deletes can
	// interleve (ish)
	//
	// e.next = nil
	// e.prev = nil
}

// Len returns the length of the list. As the Append/Remove operations
// track the length of the list, this is an O(1) operation.
func (l *List[T]) Len() int {
	if l == nil {
		return 0
	}
	return l.length
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
func (l *List[T]) PopFront() *Element[T] { return l.pop(l.root().next) }

// PopBack removes the last element from the list. If the list is
// empty, this returns a detached non-nil value, that will report an
// Ok() false value. You can use this element to produce a C-style iterator
// over the list, that removes items during the iteration:
//
//	for e := list.PopBack(); e.Ok(); e = input.PopBack() {
//		// do work
//	}
func (l *List[T]) PopBack() *Element[T] { return l.pop(l.root().prev) }

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

// Producer provides a producer function that iterates through the
// contents of the list. When the producer reaches has fully iterated
// through the list, or iterates to an item that has been removed
// (likely due to concurrent access,) the producer returns io.EOF.
//
// The Producer starts at the front of the list and iterates in order.
func (l *List[T]) Producer() fun.Producer[T] {
	current := l.root()
	return func(_ context.Context) (o T, _ error) {
		current = current.Next()
		if !current.Ok() || current.isRoot() {
			return o, io.EOF
		}
		return current.Value(), nil
	}
}

// ProducerPop provides a producer function that iterates through the
// contents of the list, removing each item from the list as it
// encounters it. When the producer reaches has fully iterated
// through the list, or iterates to an item that has been removed
// (likely due to concurrent access,) the producer returns io.EOF.
//
// In most cases, for destructive iteration, use the pubsub.Queue,
// pubsub.Deque, or one of the pubsub.Distributor implementations.
//
// The Producer starts at the front of the list and iterates in order.
func (l *List[T]) ProducerPop() fun.Producer[T] {
	var current *Element[T]
	return func(_ context.Context) (o T, _ error) {
		current = l.PopFront()

		if !current.Ok() || current.isRoot() {
			return o, io.EOF
		}

		return current.item, nil
	}
}

// ProducerReverse provides the same semantics and operation as the
// Producer operation, but starts at the end/tail of the list and
// works forward.
func (l *List[T]) ProducerReverse() fun.Producer[T] {
	current := l.root()
	return func(_ context.Context) (o T, _ error) {
		current = current.Previous()
		if !current.Ok() || current.isRoot() {
			return o, io.EOF
		}
		return current.Value(), nil
	}
}

// ProducerReverse provides the same semantics and operation as the
// ProducerPop operation, but starts at the end/tail of the list and
// works forward.
func (l *List[T]) ProducerReversePop() fun.Producer[T] {
	var current *Element[T]
	return func(_ context.Context) (o T, _ error) {
		current = l.PopBack()
		if !current.Ok() || current.isRoot() {
			return o, io.EOF
		}
		return current.item, nil
	}
}

// Iterator returns an iterator over the values in the list in
// front-to-back order. The Iterator is not synchronized with the
// values in the list, and will be exhausted when you reach the end of
// the list.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visible.
func (l *List[T]) Iterator() *fun.Iterator[T] { return l.Producer().Iterator() }

// Reverse returns an iterator that produces elements from the list,
// from the back to the front. The iterator is not
// synchronized with the values in the list, and will be exhausted
// when you reach the front of the list.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visible.
func (l *List[T]) Reverse() *fun.Iterator[T] { return l.ProducerReverse().Iterator() }

// PopIterator produces an iterator that consumes elements from the
// list as it iterates, moving front-to-back.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visible.
func (l *List[T]) PopIterator() *fun.Iterator[T] { return l.ProducerPop().Iterator() }

// PopReverse produces an iterator that consumes elements from the
// list as it iterates, moving back-to-front. To access an iterator of
// the *values* in the list, use PopReverseValues() for an
// iterator over the values.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visible.
func (l *List[T]) PopReverse() *fun.Iterator[T] { return l.ProducerReversePop().Iterator() }

// Extend removes items from the front of the input list, and appends
// them to the end (back) of the current list.
func (l *List[T]) Extend(input *List[T]) {
	if input.Len() == 0 {
		return
	}

	for elem := input.PopFront(); elem.Ok(); elem = input.PopFront() {
		l.Back().Append(elem)
	}
}

// Copy duplicates the list. The element objects in the list are
// distinct, though if the Values are themselves references, the
// values of both lists would be shared.
func (l *List[T]) Copy() *List[T] {
	out := &List[T]{}
	if l.Len() == 0 {
		return &List[T]{}
	}

	for elem := l.Front(); elem.Ok(); elem = elem.Next() {
		out.PushBack(elem.Value())
	}

	return out
}

// Slice exports the contents of the list to a slice.
func (l *List[T]) Slice() Slice[T] { return fun.NewProducer(l.Iterator().Slice).Force().Resolve() }

// Seq returns a native go iterator function for the items in a list.
func (l *List[T]) Seq() iter.Seq[T] { return risky.Block(l.Iterator().Seq) }

func (l *List[T]) root() *Element[T] {
	fun.Invariant.Ok(l != nil, ErrUninitializedContainer)

	ft.WhenCall(l.head == nil, l.uncheckedSetup)

	return l.head
}

func (l *List[T]) uncheckedSetup() {
	l.head = &Element[T]{}
	l.head.next = l.head
	l.head.prev = l.head
	l.head.list = l
	l.head.ok = false
}

func makeElem[T any](val T) *Element[T] { return &Element[T]{item: val, ok: true} }

func (l *List[T]) pop(it *Element[T]) *Element[T] {
	if !it.removable() || it.list == nil || l.head == nil || it.list.head != l.head {
		return &Element[T]{}
	}

	it.uncheckedRemove()

	return it
}
