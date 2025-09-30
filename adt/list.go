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
	// ErrOperationOnDetachedElements is used as the value for panics when attempting to manipulate add to elements that are not connected to a list.
	ErrOperationOnDetachedElements = ers.Error("impossible operation: detached elements")
	// ErrOperationOnAttachedElements is used as a panic value when attempting to attach elements that already belong to a list to a different list.
	ErrOperationOnAttachedElements = ers.Error("operation impacts elements attached to another list") // probably a bug in
	// ErrOperationOnRootElement is used as a panic when attempting to read the value or performing an
	// operation that is impossible to perform on the root object.
	ErrOperationOnRootElement = ers.Error("operation not permissible on the sentinel node ") // probably a bug in the list implementation
)

// List is a container type for a doublly-linked list that supports appending and prepending elements, as well as inserting
// elements in the middle of the list.  These lists are safe for concurrent access, and operations only require exclusive access
// to, at most 3 nodes in the list. Becaues the list tracks it's length centrally, calls to Len() are efficent and safe.
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

func safeTx[T any, O any](e *element[T], op func() O) O {
	defer WithAll(LocksHeld(e.forTx()))
	return ft.DoUnless(e.root, op)
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
