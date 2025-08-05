package adt

import (
	"maps"
	"math/rand"
	"slices"
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

func (l *List[T]) init() *element[T] {
	l.size.Store(0)
	head := l.newElem()
	head.next, head.prev = head, head
	head.root = true

	return head
}

func (l *List[T]) Head() *element[T]         { return l.root().Next() }
func (l *List[T]) Tail() *element[T]         { return l.root().Previous() }
func (l *List[T]) PushBack(v *T)             { l.root().prev.append(l.makeElem(v)) }
func (l *List[T]) PushFront(v *T)            { l.root().append(l.makeElem(v)) }
func (l *List[T]) PopFront() *T              { return l.root().next.Pop() }
func (l *List[T]) PopBack() *T               { return l.root().prev.Pop() }
func (l *List[T]) Len() int                  { return int(l.size.Load()) }
func (l *List[T]) valid() bool               { return l != nil }
func (l *List[T]) newElem() *element[T]      { return &element[T]{list: l} }
func (l *List[T]) makeElem(v *T) *element[T] { return l.newElem().Set(v) }
func (l *List[T]) root() *element[T]         { return l.head.Call(l.init) }

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
func (el *element[T]) isNotRoot() bool         { return !el.root }
func (el *element[T]) isNotNil() bool          { return el != nil }
func (el *element[T]) isAttached() bool        { return el.list != nil && el.next != nil && el.prev != nil }
func (el *element[T]) unlessRoot() *element[T] { return ft.Unless(el.root, el) }
func (el *element[T]) Next() *element[T]       { defer With(Lock(el.mtx())); return el.next.unlessRoot() }
func (el *element[T]) Previous() *element[T]   { defer With(Lock(el.mtx())); return el.prev.unlessRoot() }

// func (el *element[T]) isDetatched() bool       { return el.list == nil || el.next == nil || el.prev == nil }

func (el *element[T]) Get() (*T, bool) {
	if el == nil {
		return nil, false
	}
	defer With(Lock(el.mtx()))
	return el.item, el.ok
}

func (el *element[T]) Value() *T {
	if el == nil {
		return nil
	}

	defer With(Lock(el.mtx()))
	return el.item
}

func (el *element[T]) Ok() bool {
	if el == nil {
		return false
	}
	defer With(Lock(el.mtx()))
	return el.ok
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

func (el *element[T]) Set(v *T) *element[T] {
	fun.Invariant.IsFalse(el.root, ErrOperationOnRootElement)
	defer With(Lock(el.mtx()))

	el.item, el.ok = v, true
	return el
}

func (el *element[T]) Pop() (out *T) {
	el.safeOp(func(inner *element[T]) {
		out = inner.item
		inner.item, inner.ok = nil, false
		inner.list.size.Add(-1)
		inner.prev.next = inner.next
		inner.next.prev = inner.prev
		inner.list = nil
	}, el.isNotNil, el.next.isNotNil, el.prev.isNotNil, el.list.valid, el.isNotRoot)
	return
}

func (el *element[T]) Drop() {
	el.safeOp(func(el *element[T]) {
		el.list.size.Add(-1)
		el.prev.next = el.next
		el.next.prev = el.prev
		el.list = nil
	}, el.isNotNil, el.next.isNotNil, el.prev.isNotNil, el.list.valid, el.isNotRoot)
}

func (el *element[T]) safeOp(op func(el *element[T]), preCheks ...func() bool) {
	for _, c := range preCheks {
		if !c() {
			return
		}
	}

	defer WithAll(LocksHeld(el.mtx(), el.next.mtx(), el.prev.mtx()))
	op(el)
}

func (el *element[t]) append(next *element[t]) {
	fun.Invariant.Ok(el != nil, "cannot operate on nil elements")
	fun.Invariant.Ok(el.next != nil, "cannot append to detached elements")
	fun.Invariant.Ok(el.list != nil, "cannot append to items without a list reference")

	defer WithAll(LocksHeld(el.mtx(), el.next.mtx(), next.mtx()))

	fun.Invariant.IsTrue(el.isAttached(), ErrOperationOnAttachedElements)
	fun.Invariant.IsTrue(next.next == nil && next.prev == nil, ErrOperationOnDetachedElements)

	next.list.size.Add(1)
	next.list = el.list
	next.prev = el
	next.next = el.next
	next.prev.next = next
	next.next.prev = next
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

	seen := map[*sync.Mutex]struct{}{}
	for idx := range mtxs {
		seen[mtxs[idx]] = struct{}{}
	}
	if len(seen) < len(mtxs) {
		// we're modifying a single node list don't need to
		// try to get one lock more than once, so we can
		// de-duplicate.
		mtxs = slices.Collect(maps.Keys(seen))
	}

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

		rdur := time.Duration(rand.Int63n(int64(time.Millisecond)))
		time.Sleep(max(
			rdur,
			rdur+(2*time.Microsecond),
		))
		continue ACQUIRE
	}
}
