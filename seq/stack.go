package seq

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
)

// Stack provides a generic singly linked list, with an interface that
// is broadly similar to seq.List.
type Stack[T any] struct {
	head   *Item[T]
	length int
}

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
func (it *Item[T]) Ok() bool { return it.ok }

// Next returns the following item in the stack.
func (it *Item[T]) Next() *Item[T] { return it.next }

// In reports if an item is a member of a stack. Because item's track
// references to the stack, this is an O(1) operation.
func (it *Item[T]) In(s *Stack[T]) bool { return it.stack == s }

// Set mutates the value of an Item, returning true if the operation
// has been successful. The operation fails if the Item is the
// head item in a stack or not a member of a list.
func (it *Item[T]) Set(v T) bool {
	if it.stack != nil && it.next == nil {
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
	it.stack.lazyInit()
	n.next = it.stack.head
	n.stack = it.stack
	n.stack.head = n
	n.stack.length++
	return n
}

// Remove removes the item from the list, (at some expense, for items
// deeper in the stack.) If the operation isn't successful or possible
// the operation returns false.
func (it *Item[T]) Remove() bool {
	if it.stack == nil || !it.ok {
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

func (s *Stack[T]) lazyInit() {
	if s == nil {
		panic(ErrUninitialized)
	}

	if s.head == nil {
		s.head = &Item[T]{stack: s}
		s.length = 0
	}
}

// Len returns the length of the stack. Because stack's track their
// own size, this is an O(1) operation.
func (s *Stack[T]) Len() int { return s.length }

// Push appends an item to the stack.
func (s *Stack[T]) Push(it T) { s.lazyInit(); s.head.Append(NewItem(it)) }

// Head returns the item at the top of this stack. This is a non
// destructive operation.
func (s *Stack[T]) Head() *Item[T] { s.lazyInit(); return s.head }

// Pop removes the item on the top of the stack, and returns it. If
// the stack is empty, this will return, but not detach, the root item
// of the stack, which will report a false Ok() value.
func (s *Stack[T]) Pop() *Item[T] {
	if s.head == nil {
		s.head = &Item[T]{}
		return s.head
	}
	if s.length == 0 {
		return s.head
	}
	s.length--
	out := s.head
	out.stack = nil
	s.head = s.head.next
	return out
}

// Iterator returns a non-destructive iterator over the Items in a
// stack. Iterator will not observe new items added to the stack
// during iteration.
func (s *Stack[T]) Iterator() fun.Iterator[*Item[T]] { return newItemIter(s.head) }

// IteratorPop returns a destructive iterator over the Items in a
// stack. IteratorPop will not observe new items added to the
// stack during iteration.
func (s *Stack[T]) IteratorPop() fun.Iterator[*Item[T]] { return &itemIter[T]{pop: true, next: s.head} }
func newItemIter[T any](it *Item[T]) *itemIter[T]       { return &itemIter[T]{next: &Item[T]{next: it}} }

// StackValues converts an iterator of stack Items to an iterator of
// their values. The conversion does not have additional overhead
// relative to the original iterator, and only overides the behavior
// of the Value() method of the iterator.
func StackValues[T any](in fun.Iterator[*Item[T]]) fun.Iterator[T] { return &valIter[T, *Item[T]]{in} }

type itemIter[T any] struct {
	next   *Item[T]
	pop    bool
	closed bool
}

func (iter *itemIter[T]) Value() *Item[T]                 { return iter.next }
func (iter *itemIter[T]) Close(ctx context.Context) error { iter.closed = true; return nil }
func (iter *itemIter[T]) Next(ctx context.Context) bool {
	if iter.closed || ctx.Err() != nil || iter.next == nil {
		return false
	}
	next := iter.next.next

	if iter.pop {
		if iter.next.Remove() {
			iter.next = next
			return true
		}

		return false
	}
	if next == nil || !next.ok {
		return false
	}

	iter.next = next
	return true
}
