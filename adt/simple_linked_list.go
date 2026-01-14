package adt

import (
	"iter"
	"slices"
	"sync"
)

type list[T any] struct {
	once sync.Once
	head *elem[T]
	size int
}

type elem[T any] struct {
	next *elem[T]
	prev *elem[T]
	list *list[T]
	item T
	root bool
}

func (e *elem[T]) PushBack(n *elem[T]) *elem[T]  { n.prev, n.next = e, e.next; return e.own(n).append() }
func (e *elem[T]) PushFront(p *elem[T]) *elem[T] { return e.prev.PushBack(p) }
func (e *elem[T]) Pop() *elem[T]                 { _ = (e.dec() && e.remove() && e.unhook()); return e }
func (e *elem[T]) Value() T                      { return e.item }
func (e *elem[T]) Set(v T) *elem[T]              { e.item = v; return e }
func (e *elem[T]) Next() *elem[T]                { return e.next }
func (e *elem[T]) Previous() *elem[T]            { return e.prev }
func (e *elem[T]) Ok() bool                      { return e.list.In(e) }
func (e *elem[T]) append() *elem[T]              { e.prev.next, e.next.prev = e, e; return e }
func (e *elem[T]) own(n *elem[T]) *elem[T]       { n.list = e.list.inc(); return n }
func (e *elem[T]) dec() bool                     { return e.Ok() && e.list.dec() > 0 }
func (e *elem[T]) unhook() bool                  { e.list = nil; return true }
func (e *elem[T]) remove() bool                  { return e.list.whenRemove(e) }
func (l *list[T]) init()                         { e := l.newElem(); e.root = true; l.head = e; e.next, e.prev = e, e }
func (l *list[T]) root() *elem[T]                { l.once.Do(l.init); return l.head }
func (l *list[T]) newElem() *elem[T]             { e := new(elem[T]); e.list = l; return e }
func (l *list[T]) inc() *list[T]                 { l.size++; return l }
func (l *list[T]) dec() int                      { p := l.size; l.size = max(0, l.size-1); return p }
func (l *list[T]) remove(e *elem[T]) bool        { e.prev.next = e.next; e.next.prev = e.prev; return true }
func (l *list[T]) whenRemove(e *elem[T]) bool    { return l != nil && e != nil && l.remove(e) }
func (l *list[T]) toValue(e *elem[T]) T          { return e.item }
func (l *list[T]) Len() int                      { return l.size }
func (l *list[T]) In(e *elem[T]) bool            { return l != nil && e != nil && e.list == l && !e.root }
func (l *list[T]) Front() *elem[T]               { return l.root().Next() }
func (l *list[T]) Back() *elem[T]                { return l.root().Previous() }
func (l *list[T]) PushBack(it T) *list[T]        { l.Back().PushBack(l.newElem().Set(it)); return l }
func (l *list[T]) PushFront(it T) *list[T]       { l.root().PushBack(l.newElem().Set(it)); return l }
func (l *list[T]) PopBack() T                    { return l.Back().Pop().Value() }
func (l *list[T]) PopFront() T                   { return l.Front().Pop().Value() }
func (l *list[T]) IteratorFront() iter.Seq[T]    { return l.unwrapIter(l.nextFrom(l.Front())) }
func (l *list[T]) IteratorBack() iter.Seq[T]     { return l.unwrapIter(l.prevFrom(l.Back())) }

func (l *list[T]) Extend(seq iter.Seq[T]) {
	for v := range seq {
		l.PushBack(v)
	}
}

func (l *list[T]) nextFrom(e *elem[T]) iter.Seq[*elem[T]] {
	return func(yield func(*elem[T]) bool) {
		for it := e; l.In(it) && yield(it); it = it.Next() {
			continue
		}
	}
}

func (l *list[T]) prevFrom(e *elem[T]) iter.Seq[*elem[T]] {
	return func(yield func(*elem[T]) bool) {
		for it := e; l.In(it) && yield(it); it = it.Previous() {
			continue
		}
	}
}

func (l *list[T]) unwrapIter(seq iter.Seq[*elem[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			if !yield(l.toValue(value)) {
				return
			}
		}
	}
}

// SortQuick sorts the list by extracting elements to a slice, sorting
// with slices.SortFunc, and re-adding them to the list.
func (l *list[T]) SortQuick(cmp func(T, T) int) {
	if l.size <= 1 {
		return
	}

	// Extract all elements to a slice
	items := make([]*elem[T], 0, l.Len())
	for l.Len() > 0 {
		items = append(items, l.Front().Pop())
	}

	// Sort the slice
	slices.SortFunc(items, l.cmp(cmp))

	// Re-add elements to the list
	for _, item := range items {
		l.root().PushFront(item)
	}
}

func (*list[T]) cmp(cmpfn func(l, r T) int) func(*elem[T], *elem[T]) int {
	return func(lh, rh *elem[T]) int { return cmpfn(lh.item, rh.item) }
}

// SortMerge sorts the list in-place using merge sort.
func (l *list[T]) SortMerge(cmp func(T, T) int) {
	if l.size <= 1 {
		return
	}

	// Split list into two halves
	left := &list[T]{}
	right := &list[T]{}

	for range l.Len() / 2 {
		left.root().PushBack(l.Front().Pop())
	}
	for l.Len() > 0 {
		right.root().PushBack(l.Front().Pop())
	}

	// Recursively sort each half
	left.SortMerge(cmp)
	right.SortMerge(cmp)

	// Merge the sorted halves back into l
	for left.Len() > 0 && right.Len() > 0 {
		if cmp(left.Front().Value(), right.Front().Value()) <= 0 {
			l.Back().PushBack(left.Front().Pop())
		} else {
			l.Back().PushBack(right.Front().Pop())
		}
	}

	// Append remaining elements
	l.ExtendElements(left.IteratorPopFront())
	l.ExtendElements(right.IteratorPopFront())
}

func (l *list[T]) IteratorPopFront() iter.Seq[*elem[T]] { return l.popIter(l.nextFrom(l.Front())) }
func (l *list[T]) IteratorPopBack() iter.Seq[*elem[T]]  { return l.popIter(l.prevFrom(l.Back())) }

func (l *list[T]) ExtendElements(seq iter.Seq[*elem[T]]) {
	for v := range seq {
		l.Back().PushBack(v)
	}
}

func (l *list[T]) popIter(seq iter.Seq[*elem[T]]) iter.Seq[*elem[T]] {
	return func(yield func(*elem[T]) bool) {
		var last *elem[T]
		for value := range seq {
			if last != nil && !yield(last.Pop()) {
				return
			}
			last = value
		}
		if last != nil {
			yield(last.Pop())
		}
	}
}
