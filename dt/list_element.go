package dt

import (
	"encoding/json"
	"fmt"
)

////////////////////////////////////////////////////////////////////////
//
// Element implementation
//
////////////////////////////////////////////////////////////////////////

// Element is the underlying component of a list, provided by
// streams, the Pop operations, and the Front/Back accesses in the
// list. You can use the methods on this objects to iterate through
// the list, and the Ok() method for validating zero-valued items.
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

// make sure we have members of the same list
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
