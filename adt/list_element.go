package adt

import (
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
)

// Element is a container for an item in a doubly linked list.
type Element[T any] struct {
	mutx sync.Mutex
	next *Element[T]
	prev *Element[T]
	list *List[T]
	item *T
	ok   bool
	root bool
}

func (el *Element[T]) mtx() *sync.Mutex        { return &el.mutx }
func (el *Element[T]) isAttached() bool        { return el.list != nil && el.next != nil && el.prev != nil }
func (el *Element[T]) unlessRoot() *Element[T] { return ft.Unless(el.root, el) }
func (el *Element[T]) mustNotRoot()            { fun.Invariant.Ok(!el.root, ErrOperationOnRootElement) }
func (el *Element[T]) innerElem() *Element[T]  { defer With(Lock(el.mtx())); return el.unlessRoot() }
func (el *Element[T]) innerOk() bool           { defer With(Lock(el.mtx())); return el.ok }
func (el *Element[T]) innerValue() *T          { defer With(Lock(el.mtx())); return el.item }

// Ref returns the value stored in this element by dereferencing the pointer.
// Panics if the element is nil or contains no value.
func (el *Element[T]) Ref() T { return ft.Ref(el.Value()) }

// RefOk returns the value stored in this element and a boolean indicating success.
// Returns the zero value and false if the element is nil or contains no value.
func (el *Element[T]) RefOk() (T, bool) { return ft.RefOk(el.Value()) }

// Ok returns true if this element contains a valid value, false otherwise.
// Safe to call on nil elements, which will return false.
func (el *Element[T]) Ok() bool { return ft.DoWhen(el != nil, el.innerOk) }

// Value returns a pointer to the value stored in this element.
// Returns nil if the element is nil or contains no value.
func (el *Element[T]) Value() *T { return ft.DoWhen(el != nil, el.innerValue) }

// Next returns the next element in the list, or nil if this is the last element.
// Safe to call on nil elements, which will return nil.
func (el *Element[T]) Next() *Element[T] { return ft.DoWhen(el != nil, el.next.innerElem) }

// Previous returns the previous element in the list, or nil if this is the first element.
// Safe to call on nil elements, which will return nil.
func (el *Element[T]) Previous() *Element[T] { return ft.DoWhen(el != nil, el.prev.innerElem) }

// Pop removes this element from its list and returns the value it contained.
// Returns nil if the element was already detached or contained no value.
// This operation is safe for concurrent access.
func (el *Element[T]) Pop() *T { return safeTx(el, el.innerPop) }

// Drop removes this element from its list, discarding the value it contained.
// This operation is safe for concurrent access and is equivalent to calling Pop() and ignoring the result.
func (el *Element[T]) Drop() { safeTx(el, el.innerPop) }

// Set stores the given value in this element and returns the element for method chaining.
// Panics if called on a root/sentinel element. This operation is safe for concurrent access.
func (el *Element[T]) Set(v *T) *Element[T] { el.mustNotRoot(); return el.doSet(v) }

// Get returns the value stored in this element and a boolean indicating whether the element contains a valid value.
// Returns (nil, false) if the element is nil or contains no value. Safe for concurrent access.
func (el *Element[T]) Get() (*T, bool) {
	if el == nil {
		return nil, false
	}
	defer With(Lock(el.mtx()))
	return el.item, el.ok
}

// Unset removes the value from this element and returns the previous value and success status.
// Returns (nil, false) if the element was nil or already contained no value. Safe for concurrent access.
func (el *Element[T]) Unset() (out *T, ok bool) {
	if el == nil {
		return nil, false
	}

	defer With(Lock(el.mtx()))
	out, ok = el.item, el.ok
	el.item, el.ok = nil, false
	return
}

func (el *Element[T]) doSet(v *T) *Element[T] {
	defer With(Lock(el.mtx()))
	el.item, el.ok = v, true
	return el
}

func (el *Element[T]) innerPop() (out *T) {
	out = el.item
	el.item, el.ok = nil, false
	el.list.size.Add(-1)
	el.prev.next = el.next
	el.next.prev = el.prev
	el.list = nil
	return out
}

func (el *Element[T]) forTx() []*sync.Mutex {
	if el == nil || el.next == nil || el.prev == nil {
		return nil
	}
	return ft.Slice(el.mtx(), el.next.mtx(), el.prev.mtx())
}

func (el *Element[t]) append(next *Element[t]) {
	defer WithAll(LocksHeld(el.forTx()))

	fun.Invariant.IsTrue(el.isAttached(), ErrOperationOnAttachedElements)
	fun.Invariant.IsTrue(next.next == nil && next.prev == nil, ErrOperationOnDetachedElements)

	next.list.size.Add(1)
	next.list = el.list
	next.prev = el
	next.next = el.next
	next.prev.next = next
	next.next.prev = next
}
