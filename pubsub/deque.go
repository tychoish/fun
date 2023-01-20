package pubsub

import (
	"context"
	"errors"
	"sync"

	"github.com/tychoish/fun"
)

// Deque proves a basic double ended queue backed by a doubly linked
// list, with features to support a maximum capacity, burstable
// limits and soft quotas, as well as iterators, that safe for access
// from multiple concurrent go-routines.
//
// Use the NewDeque or NewUnlimitedDeque constructor to instantiate a
// Deque object.
type Deque[T any] struct {
	mtx     sync.Mutex
	nfront  *sync.Cond
	nback   *sync.Cond
	updates *sync.Cond
	root    *element[T]

	tracker queueLimitTracker
	closed  bool
}

type DequeOptions struct {
	Unlimited    bool
	Capacity     int
	QueueOptions *QueueOptions
}

func (opts *DequeOptions) Validate() error {
	if opts.QueueOptions != nil {
		if err := opts.QueueOptions.Validate(); err != nil {
			return err
		}
	} else if opts.Unlimited && opts.Capacity == 0 {
		return nil
	} else if opts.Capacity <= 0 {
		return errors.New("capacity cannot be negative or empty")
	}

	// positive capcity and another valid configuration
	if opts.Unlimited {
		return errors.New("cannot specify with another configuration")
	}
	return nil
}

func NewDeque[T any](opts DequeOptions) (*Deque[T], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	var tracker queueLimitTracker

	if opts.QueueOptions != nil {
		tracker = newQueueLimitTracker(*opts.QueueOptions)
	} else if opts.Capacity > 0 {
		tracker = &queueHardLimitTracker{capacity: opts.Capacity}
	} else if opts.Unlimited {
		tracker = &queueNoLimitTrackerImpl{}
	}

	q := makeDeque[T]()
	q.tracker = tracker

	return q, nil
}

func makeDeque[T any]() *Deque[T] {
	sentinel := &element[T]{}
	q := &Deque[T]{}
	q.updates = sync.NewCond(&q.mtx)
	q.nfront = sync.NewCond(&q.mtx)
	q.nback = sync.NewCond(&q.mtx)
	q.root = sentinel
	sentinel.next = sentinel
	sentinel.prev = sentinel
	sentinel.root = true
	sentinel.list = q

	return q
}

func (dq *Deque[T]) withLock() func() { dq.mtx.Lock(); return dq.mtx.Unlock }

func (dq *Deque[T]) Len() int             { defer dq.withLock()(); return dq.tracker.len() }
func (dq *Deque[T]) Close() error         { defer dq.withLock()(); dq.closed = true; return nil }
func (dq *Deque[T]) PushFront(it T) error { defer dq.withLock()(); return dq.addAfter(it, dq.root) }
func (dq *Deque[T]) PushBack(it T) error  { defer dq.withLock()(); return dq.addAfter(it, dq.root.prev) }
func (dq *Deque[T]) PopFront() (T, bool)  { defer dq.withLock()(); return dq.pop(dq.root.next) }
func (dq *Deque[T]) PopBack() (T, bool)   { defer dq.withLock()(); return dq.pop(dq.root.prev) }

func (dq *Deque[T]) WaitFront(ctx context.Context) (T, error) {
	defer dq.withLock()()
	return dq.waitPop(ctx, dqNext)
}

func (dq *Deque[T]) WaitBack(ctx context.Context) (T, error) {
	defer dq.withLock()()
	return dq.waitPop(ctx, dqPrev)
}

// ForcePushFront is the same as PushFront, except, if the deque is at
// capacity, it removes one item from the back of the deque and then,
// having made room appends the item. Returns an error if the deque is
// closed.
func (dq *Deque[T]) ForcePushFront(it T) error {
	defer dq.withLock()()

	if dq.tracker.cap() == dq.tracker.len() {
		_, _ = dq.pop(dq.root.prev)
	}

	return dq.addAfter(it, dq.root)
}

// ForcePushBack is the same as PushBack, except, if the deque is at
// capacity, it removes one item from the front of the deque and then,
// having made room prepends the item. Returns an error if the deque
// is closed.
func (dq *Deque[T]) ForcePushBack(it T) error {
	defer dq.withLock()()

	if dq.tracker.cap() == dq.tracker.len() {
		_, _ = dq.pop(dq.root.next)
	}

	return dq.addAfter(it, dq.root.prev)
}

func (dq *Deque[T]) Iterator() fun.Iterator[T] {
	defer dq.withLock()()
	return &dqIterator[T]{list: dq, item: dq.root, direction: dqNext}
}

func (dq *Deque[T]) IteratorReverse() fun.Iterator[T] {
	defer dq.withLock()()
	return &dqIterator[T]{list: dq, item: dq.root, direction: dqPrev}
}

// IteratorBlocking starts at the front of the deque, iterates to the
// end and then waits for a new item to be pushed to the back of the
// queue or the context has been canceled.
func (dq *Deque[T]) IteratorBlocking() fun.Iterator[T] {
	defer dq.withLock()()
	return &dqIterator[T]{list: dq, blocking: true, item: dq.root, direction: dqNext}
}

// IteratorBlockingReverse starts at the back/end of the deque and
// iterates toward the front, waiting at the beginning for a new item
// to be PushedFront.
func (dq *Deque[T]) IteratorBlockingReverse() fun.Iterator[T] {
	defer dq.withLock()()
	return &dqIterator[T]{list: dq, blocking: true, item: dq.root, direction: dqPrev}
}

// TODO: implement destructive iterators

func (dq *Deque[T]) addAfter(value T, after *element[T]) error {
	if dq.closed {
		return ErrQueueClosed
	}

	if err := dq.tracker.add(); err != nil {
		dq.updates.Broadcast()
		return err
	}

	it := &element[T]{item: value, list: dq}
	it.prev = after
	it.next = after.next
	it.prev.next = it
	it.next.prev = it

	if after.isRoot() {
		dq.nfront.Signal()
	}
	if after.next.isRoot() {
		dq.nback.Signal()
	}
	dq.updates.Signal()
	return nil
}

// while this method logically supports removing
// arbitrary elements, this isn't exposed and probably shouldn't be
// because the interface to giving callers access to elements wouldn't
// be ergonomic.
func (dq *Deque[T]) pop(it *element[T]) (T, bool) {
	if dq.closed || it.isRoot() {
		return *new(T), false
	}

	if it.prev.isRoot() {
		defer dq.nfront.Signal()
	}
	if it.next.isRoot() {
		defer dq.nback.Signal()
	}
	defer dq.updates.Broadcast()

	dq.tracker.remove()

	it.prev.next = it.next
	it.next.prev = it.prev
	it.next = nil
	it.prev = nil

	return it.item, true
}

func (dq *Deque[T]) waitPop(ctx context.Context, direction dqDirection) (T, error) {
	if err := dq.root.getNextOrPrevious(direction).wait(ctx, direction); err != nil {
		return *new(T), err
	}

	next := dq.root.getNextOrPrevious(direction)
	if next.isRoot() {
		return *new(T), errors.New("end of iteration")
	}

	it, ok := dq.pop(next)
	if !ok {
		return *new(T), errors.New("end of iteration")
	}
	return it, nil
}

type dqIterator[T any] struct {
	// these fields must be protected by the *list's* mutex
	list      *Deque[T]
	item      *element[T]
	direction dqDirection
	blocking  bool

	// mtx protects only the following fields
	mtx    sync.Mutex
	closed bool
	err    error
	cache  T
}

func (iter *dqIterator[T]) Close(_ context.Context) error {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	iter.closed = true
	return iter.err
}

func (iter *dqIterator[T]) Value() T {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.cache
}

func (iter *dqIterator[T]) isClosed() bool {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.closed
}

func (iter *dqIterator[T]) Next(ctx context.Context) bool {
	iter.list.mtx.Lock()
	defer iter.list.mtx.Unlock()

	if iter.isClosed() || ctx.Err() != nil {
		return false
	}

	next := iter.item.getNextOrPrevious(iter.direction)
	if iter.blocking && next.isRoot() {
		// if it's a blocking iterator we should wait for a
		// new item or someone to close the queue.
		if err := iter.item.wait(ctx, dqNext); err != nil {
			if errors.Is(err, ErrQueueClosed) {
				_ = iter.Close(ctx)
				return false
			}
			// if we get here the context is probably canceled.
			return false
		}
		// otherwise we have a new next item
		next = iter.item.getNextOrPrevious(iter.direction)
	}

	// if the next item is the same as us (should be impossible)
	// or the root item, it means we've reached the end of the
	// iterator.
	if next == iter.item || next.isRoot() {
		// if this happens to a blocking queue, there's been
		// an error.
		if iter.blocking {
			iter.err = errors.New("unrecoverable iteration error")
		}

		return false
	}

	// now we can update the state of the iterator
	iter.mtx.Lock()
	defer iter.mtx.Unlock()
	iter.item = next
	iter.cache = next.item

	return true
}

type dqDirection bool

const (
	dqNext dqDirection = false
	dqPrev dqDirection = true
)

type element[T any] struct {
	item T
	next *element[T]
	prev *element[T]
	root bool
	list *Deque[T]
}

func (it *element[T]) isRoot() bool { return it.root || it == it.list.root }

// this is just to be able to make the wait method generic.
func (it *element[T]) getNextOrPrevious(direction dqDirection) *element[T] {
	if direction == dqPrev {
		return it.prev
	}
	return it.next
}

// callers must hold the *list's* lock
func (it *element[T]) wait(ctx context.Context, direction dqDirection) error {
	var cond *sync.Cond

	// use the cond var for the head or tail if we're waiting for
	// a new item
	switch {
	case (direction == dqPrev) && it.prev.isRoot():
		cond = it.list.nback
	case (direction == dqNext) && it.next.isRoot():
		cond = it.list.nfront
	default:
		cond = it.list.updates
	}

	// If the context terminates, wake the waiter.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); cond.Broadcast() }()
	defer cancel()

	next := it.getNextOrPrevious(direction)
	for next == it.getNextOrPrevious(direction) {
		if it.list.closed {
			return ErrQueueClosed
		}
		cond.Signal()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cond.Wait()
		}
	}

	return nil
}
