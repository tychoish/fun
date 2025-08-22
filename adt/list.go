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
	n := l.newElem()
	n.next, n.prev = n, n
	n.root = true
	return n
}

func (l *List[T]) Head() *element[T]         { return l.root().Next() }
func (l *List[T]) Tail() *element[T]         { return l.root().Previous() }
func (l *List[T]) PushBack(v *T)             { l.root().prev.append(l.makeElem(v)) }
func (l *List[T]) PushFront(v *T)            { l.root().append(l.makeElem(v)) }
func (l *List[T]) PopFront() *T              { return l.root().next.Pop() }
func (l *List[T]) PopBack() *T               { return l.root().prev.Pop() }
func (l *List[T]) Len() int                  { return int(l.size.Load()) }
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

func safeTx[T any, O any](e *element[T], op func() O) O {
	defer WithAll(LocksHeld(e.forTx()))
	return ft.DoUnless(e.root, op)
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

func WithAll(locks []*sync.Mutex) { released(locks) }

func released(locks []*sync.Mutex) {
	for idx, mtx := range locks {
		if mtx != nil {
			mtx.Unlock()
			locks[idx] = nil
		}
	}
}

func doPanic(p any) { panic(p) }

func LocksHeld(mtxs []*sync.Mutex) []*sync.Mutex {
	fun.Invariant.Ok(mtxs != nil, "must hold the correct set of locks for the operation")

	seen := map[*sync.Mutex]struct{}{}

	for idx := range mtxs {
		seen[mtxs[idx]] = struct{}{}
	}
	numDupes := len(mtxs) - len(seen)
	if numDupes > 0 {
		// we're modifying a single node list don't need to
		// try to get one lock more than once, so we can
		// de-duplicate.
		mtxs = slices.Collect(maps.Keys(seen))
	}

	delete(seen, nil)
	numNils := len(mtxs) - len(seen)
	fun.Invariant.Ok(numNils == 0, "cannot lock with missing locks:", numNils)

	locked := make([]*sync.Mutex, len(mtxs))

	defer func() {
		p := recover()
		released(locked)
		defer ft.ApplyWhen(p != nil, doPanic, p)
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
				continue
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
