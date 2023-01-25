package seq

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/fun"
)

var ErrUninitialized = errors.New("initialized container")

// List provides a doubly linked list. Callers are responsible for
// their own concurrency control and bounds checking, and should
// generally use with the same care as a slice.
//
// The Deque implementation in the pubsub package provides a similar
// implementation with locking and a notification system.
type List[T any] struct {
	root   *Element[T]
	length int
}

type Element[T any] struct {
	item T
	next *Element[T]
	prev *Element[T]
	root bool
	ok   bool
	list *List[T]
}

func NewElement[T any](val T) *Element[T]   { return &Element[T]{item: val, ok: true} }
func (e *Element[T]) String() string        { return fmt.Sprint(e.item) }
func (e *Element[T]) Value() T              { return e.item }
func (e *Element[T]) Ok() bool              { return e.ok }
func (e *Element[T]) Next() *Element[T]     { return e.next }
func (e *Element[T]) Previous() *Element[T] { return e.prev }
func (e *Element[T]) In(l *List[T]) bool    { return e.list == l }
func (e *Element[T]) Set(v T) bool {
	if e.root {
		return false
	}

	e.ok = true
	e.item = v
	return true
}
func (e *Element[T]) Append(new *Element[T]) *Element[T] {
	if !e.appendable(new) {
		return e
	}

	e.uncheckedAppend(new)
	return new
}

func (e *Element[T]) Remove() bool {
	if !e.removable() {
		return false
	}
	e.uncheckedRemove()
	return true
}

func (e *Element[T]) Drop() {
	if !e.Remove() {
		return
	}
	e.item = *new(T)
	e.ok = false
}

func (e *Element[T]) Swap(with *Element[T]) bool {
	// make sure we have members of the same list, and not the
	// same element.
	if with == nil || e.list == nil || e.list != with.list || e == with {
		return false
	}

	// we want to flip one with it's neighbor,
	// this is
	// much more straightforward.
	if !with.Remove() {
		return false
	}

	e.prev.Append(with)
	return true
}

func (e *Element[T]) removable() bool {
	return !e.root && e.list != nil && e.list.root != e && e.list.length > 0
}

func (e *Element[T]) appendable(new *Element[T]) bool {
	return new != nil && e.list != nil && new.list != e.list && new.list == nil && new.ok
}

func (e *Element[T]) uncheckedAppend(new *Element[T]) {
	e.list.length++
	new.list = e.list
	new.prev = e
	new.next = e.next
	new.prev.next = new
	new.next.prev = new
}

func (e *Element[T]) uncheckedRemove() {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.list.length--
	e.list = nil
	e.next = nil
	e.prev = nil
}

func (l *List[T]) Len() int                            { return l.length }
func (l *List[T]) PushFront(it T)                      { l.lazySetup(); l.root.Append(NewElement(it)) }
func (l *List[T]) PushBack(it T)                       { l.lazySetup(); l.root.prev.Append(NewElement(it)) }
func (l *List[T]) PopFront() *Element[T]               { l.lazySetup(); return l.pop(l.root.next) }
func (l *List[T]) PopBack() *Element[T]                { l.lazySetup(); return l.pop(l.root.prev) }
func (l *List[T]) Front() *Element[T]                  { l.lazySetup(); return l.root.next }
func (l *List[T]) Back() *Element[T]                   { l.lazySetup(); return l.root.prev }
func (l *List[T]) Iterator() fun.Iterator[*Element[T]] { return &elemIter[T]{elem: l.root, dir: next} }
func (l *List[T]) Reverse() fun.Iterator[*Element[T]]  { return &elemIter[T]{elem: l.root, dir: prev} }
func (l *List[T]) IteratorPop() fun.Iterator[*Element[T]] {
	return &elemIter[T]{elem: l.root, dir: next, pop: true}
}
func (l *List[T]) ReversePop() fun.Iterator[*Element[T]] {
	return &elemIter[T]{elem: l.root, dir: prev, pop: true}
}

func (l *List[T]) Extend(input *List[T]) {
	for elem := input.PopFront(); elem.Ok(); elem = input.PopFront() {
		l.Back().Append(elem)
	}
}

func (l *List[T]) lazySetup() {
	if l == nil {
		panic(ErrUninitialized)
	}

	if l.root == nil {
		l.root = &Element[T]{root: true, list: l}
		l.root.next = l.root
		l.root.prev = l.root
		l.root.list = l
	}

}

func (l *List[T]) pop(it *Element[T]) *Element[T] {
	if !it.removable() || it.list != l {
		return &Element[T]{}
	}

	it.uncheckedRemove()

	return it
}

type direction bool

const (
	next = true
	prev = false
)

type valIter[T any, I interface{ Value() T }] struct{ fun.Iterator[I] }

func (iter *valIter[T, I]) Value() T { return iter.Iterator.Value().Value() }

func ListValues[T any](in fun.Iterator[*Element[T]]) fun.Iterator[T] {
	return &valIter[T, *Element[T]]{Iterator: in}
}

type elemIter[T any] struct {
	dir    direction
	elem   *Element[T]
	list   *List[T]
	closed bool
	pop    bool
}

func (iter *elemIter[T]) getNext(next direction) *Element[T] {
	var elem *Element[T]
	if next {
		if iter.pop {
			elem = iter.list.PopFront()
		} else {
			elem = iter.elem.next
		}
	} else {
		if iter.pop {
			elem = iter.list.PopBack()
		} else {
			elem = iter.elem.prev
		}
	}

	return elem
}

func (iter *elemIter[T]) Close(_ context.Context) error { iter.closed = true; return nil }
func (iter *elemIter[T]) Value() *Element[T]            { return iter.elem }
func (iter *elemIter[T]) Next(ctx context.Context) bool {
	if iter.list == nil {
		iter.list = iter.elem.list
	}

	if iter.closed || ctx.Err() != nil {
		return false
	}

	next := iter.getNext(iter.dir)
	if next == nil || next.root || !next.ok {
		return false
	}

	iter.elem = next
	return true
}
