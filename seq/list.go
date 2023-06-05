package seq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/tychoish/fun"
)

// ErrUninitialized is the content of the panic produced when you
// attempt to perform an operation on an uninitialized sequence.
var ErrUninitialized = errors.New("initialized container")

// List provides a doubly linked list. Callers are responsible for
// their own concurrency control and bounds checking, and should
// generally use with the same care as a slice.
//
// The Deque implementation in the pubsub package provides a similar
// implementation with locking and a notification system.
type List[T any] struct {
	root           *Element[T]
	length         int
	elementCreator func(val T) *Element[T]
}

// NewListFromIterator builds a list from the elements in the
// iterator. In general, any error would be the result of the input
// iterators's close method
func NewListFromIterator[T any](ctx context.Context, iter *fun.Iterator[T]) (*List[T], error) {
	out := &List[T]{}
	return out, iter.Observe(ctx, func(in T) { out.PushBack(in) })
}

// Append adds a variadic sequence of items to the end of the list.
func (l *List[T]) Append(items ...T) {
	for idx := range items {
		l.PushBack(items[idx])
	}
}

// Element is the underlying component of a list, provided by
// iterators, the Pop operations, and the Front/Back accesses in the
// list. You can use the methods on this objects to iterate through
// the list, and the Ok() method for validating zero-valued items.
type Element[T any] struct {
	ok      bool
	deleted bool
	next    *Element[T]
	prev    *Element[T]
	list    *List[T]
	item    T
}

// NewElement produces an unattached Element that you can use with
// Append. Element.Append(NewElement()) is essentially the same as
// List.PushBack().
func NewElement[T any](val T) *Element[T] { return makeElement(val) }

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
func (e *Element[T]) In(l *List[T]) bool { return !e.deleted && e.list != nil && e.list == l }

// Set allows you to change set the value of an item in place. Returns
// true if the operation is successful. The operation fails if the Element is the
// root item in a list or not a member of a list.
//
// Set is safe to call on nil elements.
func (e *Element[T]) Set(v T) bool {
	if e == nil || (e.list != nil && e.list.root == e) {
		return false
	}

	e.ok = true
	e.item = v
	return true
}

// MarshalJSON returns the result of json.Marshal on the value of the
// element. By supporting json.Marshaler and json.Unmarshaler,
// Elements and lists can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (e *Element[T]) MarshalJSON() ([]byte, error) { return json.Marshal(e.Value()) }

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
	buf := &bytes.Buffer{}
	_, _ = buf.Write([]byte("["))
	for i := l.Front(); i.Ok(); i = i.Next() {
		if i != l.Front() {
			_, _ = buf.Write([]byte(","))
		}
		e, err := i.MarshalJSON()
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(e)
	}

	_, _ = buf.Write([]byte("]"))

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
	zero := fun.ZeroOf[T]()
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

func (e *Element[T]) appendable(new *Element[T]) bool {
	return new != nil && new.ok && e.list != nil // && new.list == nil && new.list != e.list //&& new.next == nil && new.prev == nil
}

// Append adds the element 'new' after the element 'e', inserting it
// in the next position in the list. Will return 'e' if 'new' is not
// valid for insertion into this list (e.g. it belongs to another
// list, or is attached to other elements, is already a member of this
// list, or is otherwise invalid.) PushBack and PushFront, are
// implemented in terms of Append.
func (e *Element[T]) Append(new *Element[T]) *Element[T] {
	if !e.appendable(new) {
		return e
	}

	e.uncheckedAppend(new)
	return new
}

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
	e.item = fun.ZeroOf[T]()
	e.ok = false
}

// Swap exchanges the location of two elements in a list, returning
// true if the operation was successful, and false if the elements are
// not eligable to be swapped. It is valid/possible to swap the root element of
// the list with another element to "move the head", causing a wrap
// around effect. Swap will not operate if either element is nil, or
// not a member of the same list.
func (e *Element[T]) Swap(with *Element[T]) bool {
	// make sure we have members of the same list, and not the
	// same element.
	if with == nil || e == nil || e.list == nil || e.list != with.list || e == with {
		return false
	}
	wprev := *with.prev
	with.uncheckedRemove()
	e.prev.uncheckedAppend(with)
	e.uncheckedRemove()
	wprev.uncheckedAppend(e)
	return true
}

func (e *Element[T]) removable() bool {
	return e.list != nil && e.list.root != e && e.list.length > 0
}

func (e *Element[T]) uncheckedAppend(new *Element[T]) {
	e.list.length++
	new.list = e.list
	new.prev = e
	new.next = e.next
	new.prev.next = new
	new.next.prev = new
	new.deleted = false
}

func (e *Element[T]) uncheckedRemove() {
	e.list.length--
	e.prev.next = e.next
	e.next.prev = e.prev
	e.deleted = true
	// avoid removing references so iteration and deletes can
	// interleve (ish)
	//
	// e.list = nil
	// e.next = nil
	// e.prev = nil
}

// Len returns the length of the list. As the Append/Remove operations
// track the length of the list, this is an O(1) operation.
func (l *List[T]) Len() int { return l.length }

// PushFront creates an element and prepends it to the list. The
// performance of PushFront and PushBack are the same.
func (l *List[T]) PushFront(it T) { l.lazySetup(); l.root.Append(l.elementCreator(it)) }

// PushBack creates an element and appends it to the list. The
// performance of PushFront and PushBack are the same.
func (l *List[T]) PushBack(it T) { l.lazySetup(); l.root.prev.Append(l.elementCreator(it)) }

// PopFront removes the first element from the list. If the list is
// empty, this returns a nil value, that will report an Ok() false
// You can use this element to produce a C-style iterator over
// the list, that removes items during the iteration:
//
//	for e := list.PopFront(); e.Ok(); e = input.PopFront() {
//		// do work
//	}
func (l *List[T]) PopFront() *Element[T] { l.lazySetup(); return l.pop(l.root.next) }

// PopBack removes the last element from the list. If the list is
// empty, this returns a detached non-nil value, that will report an
// Ok() false value. You can use this element to produce a C-style iterator
// over the list, that removes items during the iteration:
//
//	for e := list.PopBack(); e.Ok(); e = input.PopBack() {
//		// do work
//	}
func (l *List[T]) PopBack() *Element[T] { l.lazySetup(); return l.pop(l.root.prev) }

// Front returns a pointer to the first element of the list. If the
// list is empty, this is also the last element of the list. The
// operation is non-destructive. You can use this pointer to begin a
// c-style iteration over the list:
//
//	for e := list.Front(); e.Ok(); e = e.Next() {
//	       // operate
//	}
func (l *List[T]) Front() *Element[T] { l.lazySetup(); return l.root.next }

// Back returns a pointer to the last element of the list. If the
// list is empty, this is also the first element of the list. The
// operation is non-destructive. You can use this pointer to begin a
// c-style iteration over the list:
//
//	for e := list.Back(); e.Ok(); e = e.Previous() {
//	       // operate
//	}
func (l *List[T]) Back() *Element[T] { l.lazySetup(); return l.root.prev }

func (l *List[T]) Producer() fun.Producer[T] {
	l.lazySetup()
	current := l.root
	return func(ctx context.Context) (o T, _ error) {
		current = current.Next()
		if !current.Ok() || current == l.root {
			return o, io.EOF
		}
		return current.Value(), nil
	}
}
func (l *List[T]) ProducerPop() fun.Producer[T] {
	l.lazySetup()
	var current *Element[T]
	return func(ctx context.Context) (o T, _ error) {
		current = l.PopFront()

		if !current.Ok() || current == l.root {
			return o, io.EOF
		}

		return current.item, nil
	}
}

func (l *List[T]) ProducerReverse() fun.Producer[T] {
	l.lazySetup()
	current := l.root
	return func(ctx context.Context) (o T, _ error) {
		current = current.Previous()
		if !current.Ok() || current == l.root {
			return o, io.EOF
		}
		return current.Value(), nil
	}
}

func (l *List[T]) ProducerReversePop() fun.Producer[T] {
	l.lazySetup()
	var current *Element[T]
	return func(ctx context.Context) (o T, _ error) {
		current = l.PopBack()
		if !current.Ok() || current == l.root {
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
// however, values added ahead of the iterator, will be visable.
func (l *List[T]) Iterator() *fun.Iterator[T] { return l.Producer().Generator() }

// Reverse returns an iterator that produces elements from the list,
// from the back to the front. The iterator is not
// synchronized with the values in the list, and will be exhausted
// when you reach the front of the list.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visable.
func (l *List[T]) Reverse() *fun.Iterator[T] { return l.ProducerReverse().Generator() }

// PopIterator produces an iterator that consumes elements from the
// list as it iterates, moving front-to-back.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visible.
func (l *List[T]) PopIterator() *fun.Iterator[T] { return l.ProducerPop().Generator() }

// PopReverse produces an iterator that consumes elements from the
// list as it iterates, moving back-to-front. To access an iterator of
// the *values* in the list, use PopReverseValues() for an
// iterator over the values.
//
// If you add values to the list during iteration *behind* where the
// iterator is, these values will not be present in the iterator;
// however, values added ahead of the iterator, will be visible.
func (l *List[T]) PopReverse() *fun.Iterator[T] { return l.ProducerReversePop().Generator() }

// Extend removes items from the front of the input list, and appends
// them to the end of the current list.
func (l *List[T]) Extend(input *List[T]) {
	back := l.Back()
	for elem := input.PopFront(); elem.Ok(); elem = input.PopFront() {
		back = back.Append(elem)
	}
}

// Copy duplicates the list. The element objects in the list are
// distinct, though if the Values are themsevles references, the
// values of both lists would be shared.
func (l *List[T]) Copy() *List[T] {
	out := &List[T]{}
	for elem := l.Front(); elem.Ok(); elem = elem.Next() {
		out.PushBack(elem.Value())
	}
	return out
}

func (l *List[T]) lazySetup() {
	if l == nil {
		panic(ErrUninitialized)
	}

	if l.root == nil {
		val := fun.ZeroOf[T]()
		l.elementCreator = func(val T) *Element[T] { return &Element[T]{item: val, ok: true} }
		l.root = l.elementCreator(val)
		l.root.next = l.root
		l.root.prev = l.root
		l.root.list = l
		l.root.ok = false
	}
}

func (l *List[T]) pop(it *Element[T]) *Element[T] {
	if !it.removable() || it.list != l {
		return &Element[T]{}
	}

	it.uncheckedRemove()

	return it
}
