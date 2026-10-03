package pubsub

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
)

// Deque proves a basic double ended queue backed by a doubly linked
// list, with features to support a maximum capacity, burstable limits
// and soft quotas, as well as iterators, that safe for access from
// multiple concurrent go-routines. Furthermore, the implementation
// safely handles multiple concurrent blocking operations (e.g. Wait,
// WaitPop IteratorWait, IteratorWaitPop).
//
// Blocking operations (WaitPushFront/Back, WaitPopFront/Back, Drain,
// Shutdown and the Wait iterators) follow one rule, shared with Queue:
// a cancelled ctx wins over a ready item or slot. Under an
// already-cancelled ctx they return the ctx error (or yield nothing)
// and neither consume nor insert anything.
//
// Use the NewDeque constructor to instantiate a Deque object.
type Deque[T any] struct {
	once    sync.Once
	mutex   sync.Mutex
	nfront  *sync.Cond
	nback   *sync.Cond
	updates *sync.Cond
	root    *element[T]

	tracker  queueLimitTracker
	drainers int // number of outstanding Drain calls
	closed   bool
	lo, hi   int // sequence numbers assigned at the front and back
}

func (dq *Deque[T]) mtx() *sync.Mutex { dq.init(); return &dq.mutex }
func (dq *Deque[T]) doInit() {
	dq.updates = sync.NewCond(&dq.mutex)
	dq.nfront = sync.NewCond(&dq.mutex)
	dq.nback = sync.NewCond(&dq.mutex)
	dq.root = &element[T]{root: true, list: dq}
	dq.root.next = dq.root
	dq.root.prev = dq.root
	dq.tracker = &queueNoLimitTrackerImpl{}
}

func (dq *Deque[T]) init() { dq.once.Do(dq.doInit) }

// DequeOptions configure the semantics of the deque. The Validate()
// method ensures that you do not produce a configuration that is
// impossible. A negative Capacity is an error (as is a non-positive
// QueueOptions.HardLimit); a Capacity of zero with no QueueOptions is
// unbounded, like the zero value of Deque and Queue.
type DequeOptions struct {
	Unlimited    bool
	Capacity     int
	QueueOptions *QueueOptions
}

// Validate ensures that the options are consistent. Exported as a
// convenience function. All errors have ErrConfigurationMalformed as
// their root.
func (opts *DequeOptions) Validate() error {
	switch {
	case opts.Unlimited && (opts.Capacity != 0 || opts.QueueOptions != nil):
		return ers.Wrap(ers.ErrMalformedConfiguration, "unlimited deque specified with impossible options")
	case opts.QueueOptions != nil && opts.Capacity != 0:
		return fmt.Errorf("unexpected capacity of %d: %w", opts.Capacity, ers.ErrMalformedConfiguration)
	case opts.QueueOptions != nil:
		return opts.QueueOptions.Validate()
	case opts.Unlimited:
		return nil
	case opts.Capacity < 0:
		return fmt.Errorf("negative capacity of %d: %w", opts.Capacity, ers.ErrMalformedConfiguration)
	}
	return nil
}

// NewDeque constructs a Deque according to the options, and errors if
// there are any problems with the configuration.
func NewDeque[T any](opts DequeOptions) (*Deque[T], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	dq := &Deque[T]{}

	dq.init()

	if opts.QueueOptions != nil {
		dq.tracker = newQueueLimitTracker(*opts.QueueOptions)
	} else if opts.Capacity > 0 {
		dq.tracker = &queueHardLimitTracker{capacity: opts.Capacity}
	}
	return dq, nil
}

// NewUnlimitedDeque constructs an unbounded Deque.
//
// Deprecated: you can use a literal constructor,
// &Deque[T]{}, to get an unlimited dequeue.
func NewUnlimitedDeque[T any]() *Deque[T] {
	return erc.Must(NewDeque[T](DequeOptions{Unlimited: true}))
}

// Len returns the length of the queue. This is an O(1) operation in
// this implementation.
func (dq *Deque[T]) Len() int { defer adt.With(adt.Lock(dq.mtx())); return dq.tracker.len() }

// Close marks the deque as closed, after which point all blocking
// consumers will stop and no more operations will succeed. The error
// value is not used in the current operation.
func (dq *Deque[T]) Close() error {
	defer adt.With(adt.Lock(dq.mtx()))
	dq.doClose()
	return nil
}

// Drain marks the deque as draining so that new items cannot be added, and then blocks until the deque is empty (or its
// context is canceled.) This does not close the deque: when Drain returns the deque is empty, but new work can then be
// added. To Drain and shutdown, use the Shutdown method. If the deque is closed while items remain, Drain returns
// ErrQueueClosed.
func (dq *Deque[T]) Drain(ctx context.Context) error {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.waitForDrain(ctx)
}

// Shutdown drains the deque, waiting for all items to be removed from the deque and then closes it so no additional work can be
// added to the deque. If the deque is closed while items remain, Shutdown returns ErrQueueClosed.
func (dq *Deque[T]) Shutdown(ctx context.Context) error {
	defer adt.With(adt.Lock(dq.mtx()))

	if err := dq.waitForDrain(ctx); err != nil {
		return err
	}

	dq.doClose()

	return nil
}

func (dq *Deque[T]) waitForDrain(ctx context.Context) error {
	// when the function returns wake all other waiters.
	ctx, cancel := context.WithCancel(ctx)
	stop := wakeOnCancel(ctx, &dq.mutex, dq.updates)
	defer stop()
	defer cancel()
	dq.drainers++
	defer func() { dq.drainers-- }()

	// Broadcast to wake up any waiting push operations so they can check draining flag
	dq.updates.Broadcast()

	for dq.tracker.len() > 0 {
		if dq.closed {
			return ErrQueueClosed
		}
		if err := ctx.Err(); err != nil {
			return ers.Wrapf(err, "Drain() returned early with %d items remaining", dq.tracker.len())
		}
		dq.updates.Wait()
	}

	return nil
}

// notify wakes every blocked waiter so it can re-check its own
// condition. Waiters never re-signal one another, so idle waiters sleep.
func (dq *Deque[T]) notify() {
	dq.nfront.Broadcast()
	dq.nback.Broadcast()
	dq.updates.Broadcast()
}

func (dq *Deque[T]) doClose() {
	dq.closed = true
	dq.notify()
}

// PushFront adds an item to the front or head of the deque, and
// erroring if the queue is closed, at capacity, or has reached its
// limit.
func (dq *Deque[T]) PushFront(it T) error {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.add(it, dqNext)
}

// PushBack adds an item to the back or end of the deque, and
// erroring if the queue is closed, at capacity, or has reached its
// limit.
func (dq *Deque[T]) PushBack(it T) error {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.add(it, dqPrev)
}

// PopFront removes the first (head) item of the queue, with the
// second value being false if the queue is empty or closed. A closed
// deque yields nothing, even if items remain.
func (dq *Deque[T]) PopFront() (T, bool) {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.pop(dq.root.next)
}

// PopBack removes the last (tail) item of the queue, with the
// second value being false if the queue is empty or closed. A closed
// deque yields nothing, even if items remain.
func (dq *Deque[T]) PopBack() (T, bool) {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.pop(dq.root.prev)
}

// WaitPopFront pops the first (head) item in the deque, and if the queue is
// empty, will block until an item is added, returning an error if the
// context canceled or the queue is closed.
func (dq *Deque[T]) WaitPopFront(ctx context.Context) (v T, err error) {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.waitPop(ctx, dqNext)
}

// WaitPopBack pops the last (tail) item in the deque, and if the queue
// is empty, will block until an item is added, returning an error if
// the context canceled or the queue is closed.
func (dq *Deque[T]) WaitPopBack(ctx context.Context) (T, error) {
	defer adt.With(adt.Lock(dq.mtx()))
	return dq.waitPop(ctx, dqPrev)
}

// ForcePushFront is the same as PushFront, except, if the deque is at
// capacity, it removes one item from the back of the deque and then,
// having made room appends the item. Returns an error if the deque is
// closed.
func (dq *Deque[T]) ForcePushFront(it T) error {
	defer adt.With(adt.Lock(dq.mtx()))

	if err := dq.checkOpen(); err != nil {
		return err
	}

	if dq.tracker.cap() == dq.tracker.len() {
		_, _ = dq.pop(dq.root.prev)
	}

	return dq.add(it, dqNext)
}

// ForcePushBack is the same as PushBack, except, if the deque is at
// capacity, it removes one item from the front of the deque and then,
// having made room prepends the item. Returns an error if the deque
// is closed.
func (dq *Deque[T]) ForcePushBack(it T) error {
	defer adt.With(adt.Lock(dq.mtx()))

	if err := dq.checkOpen(); err != nil {
		return err
	}

	if dq.tracker.cap() == dq.tracker.len() {
		_, _ = dq.pop(dq.root.next)
	}

	return dq.add(it, dqPrev)
}

// WaitPushFront performs a blocking add to the deque: if the deque is
// at capacity, this operation blocks until the deque is closed or
// there is capacity to add an item. The new item is added to the
// front of the deque. If the deque is closed or starts draining while
// blocked, it returns ErrQueueClosed or ErrQueueDraining.
func (dq *Deque[T]) WaitPushFront(ctx context.Context, it T) error {
	defer adt.With(adt.Lock(dq.mtx()))

	return dq.waitPushAfter(ctx, it, dqNext)
}

// WaitPushBack performs a blocking add to the deque: if the deque is
// at capacity, this operation blocks until the deque is closed or
// there is capacity to add an item. The new item is added to the
// back of the deque. If the deque is closed or starts draining while
// blocked, it returns ErrQueueClosed or ErrQueueDraining.
func (dq *Deque[T]) WaitPushBack(ctx context.Context, it T) error {
	defer adt.With(adt.Lock(dq.mtx()))

	return dq.waitPushAfter(ctx, it, dqPrev)
}

func (dq *Deque[T]) waitPushAfter(ctx context.Context, it T, side dqDirection) error {
	// If the context terminates, wake the waiter.
	defer wakeOnCancel(ctx, &dq.mutex, dq.updates)()

	// check ctx before capacity: a cancelled context must not insert.
	for {
		if dq.drainers > 0 {
			return ErrQueueDraining
		}
		if dq.closed {
			return ErrQueueClosed
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if dq.tracker.cap() > dq.tracker.len() {
			return dq.add(it, side)
		}
		dq.updates.Wait()
	}
}

// IteratorFront starts at the front of the Deque and iterates towards
// the back. When the iterator reaches the end (back) of the deque
// iteration halts.
func (dq *Deque[T]) IteratorFront(ctx context.Context) iter.Seq[T] { return dq.iterFrontEnd(ctx) }

// IteratorBack starts at the back of the Deque and iterates towards
// the front. When the iterator reaches the end (front) of the queue
// iteration halts.
func (dq *Deque[T]) IteratorBack(ctx context.Context) iter.Seq[T] { return dq.iterBackEnd(ctx) }

// IteratorWaitFront yields items from the front of the Deque to the
// back. When it reaches the last element, it waits for a new element
// to be added. It does not modify the elements in the Deque.
func (dq *Deque[T]) IteratorWaitFront(ctx context.Context) iter.Seq[T] { return dq.iterFrontWait(ctx) }

// IteratorWaitBack yields items from the back of the Deque to the
// front. When it reaches the first element, it waits for a new element
// to be added at the front (PushFront); items added to the back after
// the iterator has started are behind it and are not yielded. It does
// not modify the elements in the Deque.
func (dq *Deque[T]) IteratorWaitBack(ctx context.Context) iter.Seq[T] { return dq.iterBackWait(ctx) }

// IteratorWaitPopFront returns a sequence that removes
// and returns objects from the front of the deque.
// When the Deque is empty, iteration ends.
func (dq *Deque[T]) IteratorWaitPopFront(ctx context.Context) iter.Seq[T] {
	return irt.GenerateOk(dq.wrapsrc(ctx, dq.WaitPopFront))
}

// IteratorWaitPopBack returns a sequence that removes
// and returns objects from the back of the deque.
// When the Deque is empty, iteration ends.
func (dq *Deque[T]) IteratorWaitPopBack(ctx context.Context) iter.Seq[T] {
	return irt.GenerateOk(dq.wrapsrc(ctx, dq.WaitPopBack))
}

func (*Deque[T]) wrapsrc(ctx context.Context, op func(ctx context.Context) (T, error)) func() (T, bool) {
	return func() (zero T, _ bool) {
		value, err := op(ctx)
		if err != nil {
			return zero, false
		}
		return value, true
	}
}
func (dq *Deque[T]) iterFrontEnd(ctx context.Context) iter.Seq[T]  { return dq.iter(ctx, dqNext, false) }
func (dq *Deque[T]) iterBackEnd(ctx context.Context) iter.Seq[T]   { return dq.iter(ctx, dqPrev, false) }
func (dq *Deque[T]) iterFrontWait(ctx context.Context) iter.Seq[T] { return dq.iter(ctx, dqNext, true) }
func (dq *Deque[T]) iterBackWait(ctx context.Context) iter.Seq[T]  { return dq.iter(ctx, dqPrev, true) }

func (*Deque[T]) zero() (z T) { return z }
func (dq *Deque[T]) iter(ctx context.Context, direction dqDirection, blocking bool) iter.Seq[T] {
	var current *element[T]

	op := func() (T, bool) {
		defer adt.With(adt.Lock(dq.mtx()))
		if current == nil {
			current = dq.root
		}

		if blocking {
			err := dq.await(ctx, direction, func() bool {
				return dq.closed || dq.neighbor(current, direction) != dq.root
			})
			if err != nil {
				return dq.zero(), false
			}
		}

		next := dq.neighbor(current, direction)
		if next == dq.root {
			return dq.zero(), false
		}

		current = next
		return current.item, true
	}
	return irt.GenerateOk(op)
}

// neighbor returns the live element after (or, for dqPrev, before) the
// one given, or the root at the end of the list. Popped elements hold no
// links, so their position is recovered from their sequence number.
func (dq *Deque[T]) neighbor(from *element[T], direction dqDirection) *element[T] {
	if !from.removed {
		return from.getNextOrPrevious(direction)
	}
	if direction == dqPrev {
		n := dq.root.prev
		for n != dq.root && n.seq >= from.seq {
			n = n.prev
		}
		return n
	}
	n := dq.root.next
	for n != dq.root && n.seq <= from.seq {
		n = n.next
	}
	return n
}

// await blocks (with the lock held, as a Cond does) until ready reports
// true or ctx ends. Callers' ready funcs must account for closure.
func (dq *Deque[T]) await(ctx context.Context, direction dqDirection, ready func() bool) error {
	cond := dq.nfront
	if direction == dqPrev {
		cond = dq.nback
	}
	defer wakeOnCancel(ctx, &dq.mutex, cond)()

	// check ctx before ready: a cancelled context must not take items.
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ready() {
			return nil
		}
		cond.Wait()
	}
}

// checkOpen reports why the deque cannot accept new items, if it can't.
func (dq *Deque[T]) checkOpen() error {
	if dq.drainers > 0 {
		return ErrQueueDraining
	}

	if dq.closed {
		return ErrQueueClosed
	}
	return nil
}

// add inserts at the front (dqNext) or back (dqPrev) of the list.
func (dq *Deque[T]) add(value T, side dqDirection) error {
	if err := dq.checkOpen(); err != nil {
		return err
	}

	if err := dq.tracker.add(); err != nil {
		dq.updates.Broadcast()
		return err
	}

	it := &element[T]{item: value, list: dq}
	after := dq.root
	if side == dqPrev {
		after = dq.root.prev
		dq.hi++
		it.seq = dq.hi
	} else {
		dq.lo--
		it.seq = dq.lo
	}
	it.prev = after
	it.next = after.next
	it.prev.next = it
	it.next.prev = it

	dq.notify()
	return nil
}

// while this method logically supports removing
// arbitrary elements, this isn't exposed and probably shouldn't be
// because the interface to giving callers access to elements wouldn't
// be ergonomic.
func (dq *Deque[T]) pop(it *element[T]) (out T, _ bool) {
	if dq.closed || it.isRoot() {
		return out, false
	}

	defer dq.notify()

	dq.tracker.remove()

	it.prev.next = it.next
	it.next.prev = it.prev

	out = it.item
	it.release()

	return out, true
}

func (dq *Deque[T]) waitPop(ctx context.Context, direction dqDirection) (out T, _ error) {
	err := dq.await(ctx, direction, func() bool {
		return dq.closed || !dq.root.getNextOrPrevious(direction).isRoot()
	})
	if err != nil {
		return out, err
	}
	if dq.closed {
		return out, ErrQueueClosed
	}

	out, _ = dq.pop(dq.root.getNextOrPrevious(direction))
	return out, nil
}
