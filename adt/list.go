package adt

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

const (
	ErrOperationOnDetachedElements = ers.Error("impossible operation detached elements")              // probably a bug in calling code
	ErrOperationOnAttachedElements = ers.Error("operation impacts elements attached to another list") // probably a bug in calling code
	ErrOperationOnRootElement      = ers.Error("operation not permissible on the sentinel node ")     // probably a bug in the list implementation
)

type List[T any] struct {
	size atomic.Int64
	head Once[*element[T]]
}

func (l *List[T]) Head() *element[T] { return l.root().next }
func (l *List[T]) Tail() *element[T] { return l.root().prev }
func (l *List[T]) PushBack(v *T)     { l.root().prev.append(l.makeElem(v)) }
func (l *List[T]) PushFront(v *T)    { l.root().append(l.makeElem(v)) }
func (l *List[T]) PopFront() *T      { return l.root().next.Pop() }
func (l *List[T]) PopBack() *T       { return l.root().prev.Pop() }

type element[T any] struct {
	mutx sync.Mutex
	next *element[T]
	prev *element[T]
	list *List[T]
	item *T
	ok   bool
}

func (el *element[T]) Get() (*T, bool) { defer With(Lock(el.mtx())); return el.item, el.ok }
func (el *element[T]) Value() *T       { defer With(Lock(el.mtx())); return el.item }
func (el *element[T]) Ok() bool        { defer With(Lock(el.mtx())); return el.ok }
func (el *element[T]) Attached() bool  { defer With(Lock(el.mtx())); return el.isAttached() }
func (el *element[T]) Detached() bool  { defer With(Lock(el.mtx())); return el.isDetatched() }

func (el *element[T]) Next() *element[T] {
	defer With(Lock(el.mtx()))
	return ft.Unless(el.next.isRoot() && el.isDetatched(), el.next)
}

func (el *element[T]) Previous() *element[T] {
	defer With(Lock(el.mtx()))
	return ft.Unless(el.prev.isRoot() && el.isDetatched(), el.prev)
}

func (l *List[T]) init() *element[T] {
	l.size.Store(0)
	head := l.newElem()
	head.next, head.prev = head, head
	return head
}

func (l *List[T]) valid() bool               { return l != nil && l.head.Called() }
func (l *List[T]) newElem() *element[T]      { return &element[T]{list: l} }
func (l *List[T]) makeElem(v *T) *element[T] { return l.newElem().Set(v) }
func (l *List[T]) root() *element[T]         { return l.head.Call(l.init) }

func (el *element[T]) mtx() *sync.Mutex          { return &el.mutx }
func (el *element[T]) with(op func(*element[T])) { defer With(Lock(el.mtx())); op(el) }

func (el *element[T]) isRoot() bool      { return el.list.valid() && el.list.root() == el }
func (el *element[T]) isDetatched() bool { return el.list == nil && el.next == nil && el.prev == nil }
func (el *element[T]) isAttached() bool  { return el.list != nil && el.next != nil && el.prev != nil }
func (el *element[T]) isCorrupt() bool   { return el.isAttached() == el.isDetatched() }
func (el *element[T]) isValid() bool     { return el.isAttached() != el.isDetatched() }

func (el *element[T]) Unset() (out *T, ok bool) {
	defer With(Lock(el.mtx()))

	out, ok = el.item, el.ok
	el.item, el.ok = nil, false
	return
}

func (el *element[T]) Set(v *T) *element[T] {
	fun.Invariant.IsFalse(el.isRoot(), ErrOperationOnRootElement)
	defer With(Lock(el.mtx()))

	el.item, el.ok = v, true
	return el
}

func (el *element[T]) Pop() (out *T) {
	el.safeOp(func(el *element[T]) {
		out = el.item
		el.item, el.ok = nil, false
		el.list.size.Add(-1)
		el.prev.next = el.next
		el.next.prev = el.prev
		el.list = nil
	}, el.isRoot)

	return nil
}

func (el *element[T]) Drop() {
	el.safeOp(func(el *element[T]) {
		el.list.size.Add(-1)
		el.prev.next = el.next
		el.next.prev = el.prev
		el.list = nil
	})
}

func (el *element[T]) safeOp(op func(el *element[T]), preCheks ...func() bool) {
	for _, c := range preCheks {
		if c() {
			return
		}
	}

	if el.Detached() {
		return
	}

	defer WithAll(LocksHeld(el.mtx(), el.next.mtx(), el.prev.mtx()))
	op(el)
}

func (el *element[T]) append(next *element[T]) {
	fun.Invariant.IsFalse(el.Detached(), ErrOperationOnDetachedElements)
	fun.Invariant.IsFalse(next.Attached(), ErrOperationOnAttachedElements)

	defer WithAll(LocksHeld(el.mtx(), el.next.mtx(), next.mtx()))

	fun.Invariant.IsTrue(el.isAttached(), ErrOperationOnAttachedElements)
	fun.Invariant.IsTrue(next.isDetatched(), ErrOperationOnDetachedElements)

	next.prev, next.next = el, el.next
	next.prev.prev, next.next.prev = el, el
	el.list.size.Add(1)
}

func WithAll(locks []*sync.Mutex) { released(locks) }

func LocksHeld(mtxs ...*sync.Mutex) []*sync.Mutex { return lockMany(mtxs) }

func released(locks []*sync.Mutex) {
	for idx, mtx := range locks {
		if mtx != nil {
			mtx.Unlock()
			locks[idx] = nil
		}
	}
}

func lockMany(mtxs []*sync.Mutex) []*sync.Mutex {

	locked := make([]*sync.Mutex, len(mtxs))

	defer func() {
		if p := recover(); p != nil {
			defer panic(p)
			released(locked)
		}
	}()

ACQUIRE:
	for {
		rand.Shuffle(len(mtxs), func(i, j int) { mtxs[i], mtxs[j] = mtxs[j], mtxs[i] })

		for idx := range mtxs {
			if !mtxs[idx].TryLock() {
				goto RELEASE_ATTEMPT
			}
			locked[idx] = mtxs[idx]
		}

		return locked

	RELEASE_ATTEMPT:
		for idx := range locked {
			if locked[idx] == nil {
				break RELEASE_ATTEMPT
			}
			locked[idx].Unlock()
			locked[idx] = nil
		}

		rdur := time.Duration(rand.Int63n(int64(2 * time.Millisecond)))
		time.Sleep(max(
			rdur,
			rdur+(2*time.Microsecond),
		))
		continue ACQUIRE
	}
}
