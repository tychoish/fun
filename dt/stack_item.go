package dt

import (
	"encoding/json"
	"fmt"
)

// Item is a common wrapper for the elements in a stack.
type Item[T any] struct {
	next  *Item[T]
	stack *Stack[T]
	ok    bool
	value T
}

// NewItem produces a valid Item object for the specified value.
func NewItem[T any](in T) *Item[T] { return &Item[T]{value: in, ok: true} }

// String implements fmt.Stringer, and returns the string value of the
// item's value.
func (it *Item[T]) String() string { return fmt.Sprint(it.value) }

// Value returns the Item's underlying value. Use the Ok method to
// check the validity of the zero values.
func (it *Item[T]) Value() T { return it.value }

// Ok return true if the Value() has been set. Returns false for
// incompletely initialized values.
//
// Returns false when the item is nil.
func (it *Item[T]) Ok() bool { return it != nil && it.ok }

// Next returns the following item in the stack.
func (it *Item[T]) Next() *Item[T] { return it.next }

// In reports if an item is a member of a stack. Because item's track
// references to the stack, this is an O(1) operation.
func (it *Item[T]) In(s *Stack[T]) bool { return it.stack == s }

// Set mutates the value of an Item, returning true if the operation
// has been successful. The operation fails if the Item is the
// head item in a stack or not a member of a stack.
func (it *Item[T]) Set(v T) bool {
	if it.stack != nil && (it.stack.head == it || it.next == nil) {
		return false
	}

	it.ok = true
	it.value = v
	return true
}

// Append inserts a new item after the following item in the stack,
// returning the new item, or if the new item is not valid, the item
// itself.
func (it *Item[T]) Append(n *Item[T]) *Item[T] {
	if n == nil || it.stack == nil || it.stack == n.stack || n.stack != nil || !n.ok {
		return it
	}

	n.next = it.stack.root()
	n.stack = it.stack
	n.stack.head = n
	n.stack.length++
	return n
}

func (it *Item[T]) Push(v T) *Item[T] { return it.Append(NewItem(v)) }

// Remove removes the item from the stack, (at some expense, for items
// deeper in the stack.) If the operation isn't successful or possible
// the operation returns false.
func (it *Item[T]) Remove() bool {
	if it == nil || it.stack == nil || !it.ok {
		return false
	}

	// ok, go looking for the detach point in the stack.
	for next := it.stack.head; next.Ok(); next = next.next {
		// the next item is going to be the head of the new stack
		if next == it {
			it.stack.length--
			it.stack = nil
			next.next = it.next
			return true
		}
		if next.next == nil {
			break
		}
	}
	return false
}

// Detach splits a stack into two, using the current Item as the head
// of the new stack. The output is always non-nil: if the item is not
// valid or not the member of a stack Detach creates a new empty
// stack. If this item is currently the head of a stack, Detach
// returns that stack.
func (it *Item[T]) Detach() *Stack[T] {
	// we're not a valid, what even.
	if !it.ok {
		return &Stack[T]{head: &Item[T]{stack: it.stack}}
	}

	// we're not in a stck, make a new stack of one
	if it.stack == nil {
		it.stack = &Stack[T]{
			head:   it,
			length: 1,
		}
		return it.stack
	}

	// if you try and detach the current head, you just want the
	// current stack.
	if it.stack.head == it {
		return it.stack
	}

	// ok, go looking for the detach point in the stack.
	seen := 0
	for next := it.stack.head; next != nil; next = next.next {
		seen++
		// the next item is going to be the head of the new stack
		if next.next == it {
			next.next = &Item[T]{stack: it.stack}
			it.stack.length = seen
			it.stack = &Stack[T]{
				head:   it,
				length: it.stack.length - seen,
			}
			break
		}
	}

	return it.stack
}

// Attach removes items from the back of the stack and appends them to
// the current item. This inverts the order of items in the input
// stack.
func (it *Item[T]) Attach(stack *Stack[T]) bool {
	if stack == nil || stack.Len() == 0 || stack == it.stack {
		return false
	}

	for n := stack.Pop(); n.Ok(); n = stack.Pop() {
		it = it.Append(n)
	}

	return true
}

// MarshalJSON returns the result of json.Marshal on the value of the
// item. By supporting json.Marshaler and json.Unmarshaler,
// Items and stacks can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (it *Item[T]) MarshalJSON() ([]byte, error) { return json.Marshal(it.Value()) }

// UnmarshalJSON reads the json value, and sets the value of the
// item to the value in the json, potentially overriding an
// existing value. By supporting json.Marshaler and json.Unmarshaler,
// Items and stacks can behave as arrays in larger json objects, and
// can be as the output/input of json.Marshal and json.Unmarshal.
func (it *Item[T]) UnmarshalJSON(in []byte) error {
	var val T
	if err := json.Unmarshal(in, &val); err != nil {
		return err
	}
	it.Set(val)
	return nil
}
