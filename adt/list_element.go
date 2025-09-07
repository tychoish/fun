package adt

import (
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
)

type element[T any] struct {
	mutx sync.Mutex
	next *element[T]
	prev *element[T]
	list *List[T]
	item *T
	ok   bool
	root bool
}

func (el *element[T]) mtx() *sync.Mutex        { return &el.mutx }
func (el *element[T]) isAttached() bool        { return el.list != nil && el.next != nil && el.prev != nil }
func (el *element[T]) unlessRoot() *element[T] { return ft.Unless(el.root, el) }
func (el *element[T]) mustNotRoot()            { fun.Invariant.Ok(!el.root, ErrOperationOnRootElement) }
func (el *element[T]) innerElem() *element[T]  { defer With(Lock(el.mtx())); return el.unlessRoot() }
func (el *element[T]) innerOk() bool           { defer With(Lock(el.mtx())); return el.ok }
func (el *element[T]) innerValue() *T          { defer With(Lock(el.mtx())); return el.item }
func (el *element[T]) Ref() T                  { return ft.Ref(el.Value()) }
func (el *element[T]) RefOk() (T, bool)        { return ft.RefOk(el.Value()) }
func (el *element[T]) Ok() bool                { return ft.DoWhen(el != nil, el.innerOk) }
func (el *element[T]) Value() *T               { return ft.DoWhen(el != nil, el.innerValue) }
func (el *element[T]) Next() *element[T]       { return ft.DoWhen(el != nil, el.next.innerElem) }
func (el *element[T]) Previous() *element[T]   { return ft.DoWhen(el != nil, el.prev.innerElem) }
func (el *element[T]) Pop() *T                 { return safeTx(el, el.innerPop) }
func (el *element[T]) Drop()                   { safeTx(el, el.innerPop) }
func (el *element[T]) Set(v *T) *element[T]    { el.mustNotRoot(); return el.doSet(v) }

func (el *element[T]) Get() (*T, bool) {
	if el == nil {
		return nil, false
	}
	defer With(Lock(el.mtx()))
	return el.item, el.ok
}

func (el *element[T]) Unset() (out *T, ok bool) {
	if el == nil {
		return nil, false
	}

	defer With(Lock(el.mtx()))
	out, ok = el.item, el.ok
	el.item, el.ok = nil, false
	return
}

func (el *element[T]) doSet(v *T) *element[T] {
	defer With(Lock(el.mtx()))
	el.item, el.ok = v, true
	return el
}

func (el *element[T]) innerPop() (out *T) {
	out = el.item
	el.item, el.ok = nil, false
	el.list.size.Add(-1)
	el.prev.next = el.next
	el.next.prev = el.prev
	el.list = nil
	return out
}

func (el *element[T]) forTx() []*sync.Mutex {
	if el == nil || el.next == nil || el.prev == nil {
		return nil
	}
	return ft.Slice(el.mtx(), el.next.mtx(), el.prev.mtx())
}

func (el *element[t]) append(next *element[t]) {
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
