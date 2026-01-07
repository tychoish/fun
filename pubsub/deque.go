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
// Use the NewDeque constructor to instantiate a Deque object.
type Deque[T any] struct {
	mtx     *sync.Mutex
	nfront  *sync.Cond
	nback   *sync.Cond
	updates *sync.Cond
	root    *element[T]

	tracker  queueLimitTracker
	draining bool
	closed   bool
}

// DequeOptions configure the semantics of the deque. The Validate()
// method ensures that you do not produce a configuration that is
// impossible.
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
	case opts.Capacity <= 0:
		opts.Capacity = 1
	}
	return nil
}

// NewDeque constructs a Deque according to the options, and errors if
// there are any problems with the configuration.
func NewDeque[T any](opts DequeOptions) (*Deque[T], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	q := makeDeque[T]()

	if opts.QueueOptions != nil {
		q.tracker = newQueueLimitTracker(*opts.QueueOptions)
	} else if opts.Capacity > 0 {
		q.tracker = &queueHardLimitTracker{capacity: opts.Capacity}
	} else if opts.Unlimited {
		q.tracker = &queueNoLimitTrackerImpl{}
	}

	return q, nil
}

// NewUnlimitedDeque constructs an unbounded Deque.
func NewUnlimitedDeque[T any]() *Deque[T] {
	return erc.Must(NewDeque[T](DequeOptions{Unlimited: true}))
}

func makeDeque[T any]() *Deque[T] {
	q := &Deque[T]{mtx: &sync.Mutex{}}
	q.updates = sync.NewCond(q.mtx)
	q.nfront = sync.NewCond(q.mtx)
	q.nback = sync.NewCond(q.mtx)
	q.root = &element[T]{root: true, list: q}
	q.root.next = q.root
	q.root.prev = q.root

	return q
}

// Len returns the length of the queue. This is an O(1) operation in
// this implementation.
func (dq *Deque[T]) Len() int { defer adt.With(adt.Lock(dq.mtx)); return dq.tracker.len() }

// Close marks the deque as closed, after which point all blocking
// consumers will stop and no more operations will succeed. The error
// value is not used in the current operation.
func (dq *Deque[T]) Close() error {
	defer adt.With(adt.Lock(dq.mtx))
	dq.doClose()
	return nil
}

// Drain marks the deque as draining so that new items cannot be added, and then blocks until the deque is empty (or its
// context is canceled.) This does not close the deque: when Drain returns the deque is empty, but new work can then be
// added. To Drain and shutdown, use the Shutdown method.
func (dq *Deque[T]) Drain(ctx context.Context) error {
	dq.mtx.Lock()
	defer dq.mtx.Unlock()
	return dq.waitForDrain(ctx)
}

// Shutdown drains the deque, waiting for all items to be removed from the deque and then closes it so no additional work can be
// added to the deque.
func (dq *Deque[T]) Shutdown(ctx context.Context) error {
	dq.mtx.Lock()
	defer dq.mtx.Unlock()

	if err := dq.waitForDrain(ctx); err != nil {
		return err
	}

	dq.doClose()

	return nil
}

func (dq *Deque[T]) waitForDrain(ctx context.Context) error {
	// when the function returns wake all other waiters.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); dq.updates.Broadcast() }()
	defer cancel()
	dq.draining = true
	defer func() { dq.draining = false }()

	// Broadcast to wake up any waiting push operations so they can check draining flag
	dq.updates.Broadcast()

	for dq.tracker.len() > 0 {
		if err := ctx.Err(); err != nil {
			return ers.Wrapf(err, "Drain() returned early with %d items remaining", dq.tracker.len())
		}
		dq.updates.Wait()
	}

	return nil
}

func (dq *Deque[T]) doClose() {
	dq.closed = true
	dq.nfront.Broadcast()
	dq.nback.Broadcast()
	dq.updates.Broadcast()
}

// PushFront adds an item to the front or head of the deque, and
// erroring if the queue is closed, at capacity, or has reached its
// limit.
func (dq *Deque[T]) PushFront(it T) error {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.addAfter(it, dq.root)
}

// PushBack adds an item to the back or end of the deque, and
// erroring if the queue is closed, at capacity, or has reached its
// limit.
func (dq *Deque[T]) PushBack(it T) error {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.addAfter(it, dq.root.prev)
}

// PopFront removes the first (head) item of the queue, with the
// second value being false if the queue is empty or closed.
func (dq *Deque[T]) PopFront() (T, bool) {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.pop(dq.root.next)
}

// PopBack removes the last (tail) item of the queue, with the
// second value being false if the queue is empty or closed.
func (dq *Deque[T]) PopBack() (T, bool) {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.pop(dq.root.prev)
}

// WaitPopFront pops the first (head) item in the deque, and if the queue is
// empty, will block until an item is added, returning an error if the
// context canceled or the queue is closed.
func (dq *Deque[T]) WaitPopFront(ctx context.Context) (v T, err error) {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.waitPop(ctx, dqNext)
}

// WaitPopBack pops the last (tail) item in the deque, and if the queue
// is empty, will block until an item is added, returning an error if
// the context canceled or the queue is closed.
func (dq *Deque[T]) WaitPopBack(ctx context.Context) (T, error) {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.waitPop(ctx, dqPrev)
}

// ForcePushFront is the same as PushFront, except, if the deque is at
// capacity, it removes one item from the back of the deque and then,
// having made room appends the item. Returns an error if the deque is
// closed.
func (dq *Deque[T]) ForcePushFront(it T) error {
	defer adt.With(adt.Lock(dq.mtx))

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
	defer adt.With(adt.Lock(dq.mtx))

	if dq.tracker.cap() == dq.tracker.len() {
		_, _ = dq.pop(dq.root.next)
	}

	return dq.addAfter(it, dq.root.prev)
}

// WaitPushFront performs a blocking add to the deque: if the deque is
// at capacity, this operation blocks until the deque is closed or
// there is capacity to add an item. The new item is added to the
// front of the deque.
func (dq *Deque[T]) WaitPushFront(ctx context.Context, it T) error {
	defer adt.With(adt.Lock(dq.mtx))

	return dq.waitPushAfter(ctx, it, func() *element[T] { return dq.root })
}

// WaitPushBack performs a blocking add to the deque: if the deque is
// at capacity, this operation blocks until the deque is closed or
// there is capacity to add an item. The new item is added to the
// back of the deque.
func (dq *Deque[T]) WaitPushBack(ctx context.Context, it T) error {
	defer adt.With(adt.Lock(dq.mtx))

	return dq.waitPushAfter(ctx, it, func() *element[T] { return dq.root.prev })
}

func (dq *Deque[T]) waitPushAfter(ctx context.Context, it T, afterGetter func() *element[T]) error {
	if dq.draining {
		return ErrQueueDraining
	}

	if dq.tracker.cap() > dq.tracker.len() {
		if dq.tracker.len() == 0 {
			defer dq.updates.Signal()
		}
		return dq.addAfter(it, afterGetter())
	}

	cond := dq.updates
	// If the context terminates, wake the waiter.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); cond.Broadcast() }()
	defer cancel()

	for dq.tracker.cap() <= dq.tracker.len() {
		if dq.draining {
			return ErrQueueDraining
		}
		if dq.closed {
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

	return dq.addAfter(it, afterGetter())
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
func (dq *Deque[T]) IteratorWaitFront(ctx context.Context) iter.Seq[T] { return dq.iterBackWait(ctx) }

// IteratorWaitBack yields items from the back of the Deque to the
// front. When it reaches the first element, it waits for a new element
// to be added. It does not modify the elements in the Deque.
func (dq *Deque[T]) IteratorWaitBack(ctx context.Context) iter.Seq[T] { return dq.iterFrontWait(ctx) }

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
func (dq *Deque[T]) iterBackWait(ctx context.Context) iter.Seq[T]  { return dq.iter(ctx, dqNext, true) }

func (*Deque[T]) zero() (z T) { return z }
func (dq *Deque[T]) iter(ctx context.Context, direction dqDirection, blocking bool) iter.Seq[T] {
	var current *element[T]

	op := func() (T, bool) {
		defer adt.With(adt.Lock(dq.mtx))
		if current == nil {
			current = dq.root
		}
		if current.getNextOrPrevious(direction) == dq.root && blocking {
			if err := current.wait(ctx, direction); err != nil {
				return dq.zero(), false
			}
		}
		next := current.getNextOrPrevious(direction)
		if next == nil || next == dq.root {
			return dq.zero(), false
		}

		current = next
		return current.item, true
	}
	return irt.GenerateOk(op)
}

func (dq *Deque[T]) addAfter(value T, after *element[T]) error {
	if dq.draining {
		return ErrQueueDraining
	}

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
	if after.prev.isRoot() {
		dq.nback.Signal()
	}
	dq.updates.Signal()
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

	if it.prev.isRoot() {
		defer dq.nfront.Signal()
	}
	if it.next.isRoot() {
		defer dq.nback.Signal()
	}

	// If draining or closed, broadcast to wake all waiters
	if dq.draining || dq.closed {
		defer dq.updates.Broadcast()
	} else {
		defer dq.updates.Signal()
	}

	dq.tracker.remove()

	it.prev.next = it.next
	it.next.prev = it.prev

	// don't reset poointers in the item in case we're using this
	// item in an iterator
	//
	// it.next = nil
	// it.prev = nil

	return it.item, true
}

func (dq *Deque[T]) waitPop(ctx context.Context, direction dqDirection) (out T, _ error) {
	for {
		if err := dq.root.getNextOrPrevious(direction).wait(ctx, direction); err != nil {
			return out, err
		}

		it, ok := dq.pop(dq.root.getNextOrPrevious(direction))
		if ok {
			return it, nil
		}
	}
}
