package seq

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
)

type Stack[T any] struct {
	head   *Item[T]
	length int
}

type Item[T any] struct {
	item  T
	ok    bool
	next  *Item[T]
	stack *Stack[T]
}

func NewItem[T any](in T) *Item[T]      { return &Item[T]{item: in, ok: true} }
func (it *Item[T]) String() string      { return fmt.Sprint(it.item) }
func (it *Item[T]) Value() T            { return it.item }
func (it *Item[T]) Ok() bool            { return it.ok }
func (it *Item[T]) Next() *Item[T]      { return it.next }
func (it *Item[T]) In(s *Stack[T]) bool { return it.stack == s }
func (it *Item[T]) Set(v T) bool {
	if it.stack != nil && it.next == nil && !it.ok {
		//  this is the root/head node. don't give it a value
		return false
	}
	it.ok = true
	it.item = v
	return true
}

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

func (it *Item[T]) Detach() *Stack[T] {
	// we're not a valid, what even.
	if !it.ok {
		return &Stack[T]{}
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
	if s.head == nil {
		s.head = &Item[T]{stack: s}
		s.length = 0
	}
}

func (s *Stack[T]) Len() int       { return s.length }
func (s *Stack[T]) Head() *Item[T] { s.lazyInit(); return s.head }
func (s *Stack[T]) Push(it T)      { s.lazyInit(); s.head.Append(NewItem(it)) }

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

func (s *Stack[T]) Iterator() fun.Iterator[*Item[T]]               { return newItemIter(s.head) }
func (s *Stack[T]) IteratorPop() fun.Iterator[*Item[T]]            { return &itemIter[T]{pop: true, next: s.head} }
func newItemIter[T any](it *Item[T]) *itemIter[T]                  { return &itemIter[T]{next: &Item[T]{next: it}} }
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
