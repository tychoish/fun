package pubsub

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/risky"
)

// Deque proves a basic double ended queue backed by a doubly linked
// list, with features to support a maximum capacity, burstable limits
// and soft quotas, as well as streams, that safe for access from
// multiple concurrent go-routines. Furthermore, the implementation
// safely handles multiple concurrent blocking operations (e.g. Wait,
// streams).
//
// Use the NewDeque constructor to instantiate a Deque object.
type Deque[T any] struct {
	mtx     *sync.Mutex
	nfront  *sync.Cond
	nback   *sync.Cond
	updates *sync.Cond
	root    *element[T]

	tracker queueLimitTracker
	closed  bool
}

// DequeOptions configure the semantics of the deque. The Validate()
// method ensures that you do not produce a configuration that is
// impossible.
//
// Capcaity puts a firm upper cap on the number of items in the deque,
// while the Unlimited options.
type DequeOptions struct {
	Unlimited    bool
	Capacity     int
	QueueOptions *QueueOptions
}

// Validate ensures that the options are consistent. Exported as a
// convenience function. All errors have ErrConfigurationMalformed as
// their root.
func (opts *DequeOptions) Validate() error {
	if opts.QueueOptions != nil {
		if err := opts.QueueOptions.Validate(); err != nil {
			return err
		}
	} else if opts.Unlimited && opts.Capacity == 0 {
		return nil
	} else if opts.Capacity <= 0 {
		opts.Capacity = 1
	}

	if opts.Capacity > 0 && opts.QueueOptions != nil {
		return fmt.Errorf("cannot specify a capcity with queue options: %w", ers.ErrMalformedConfiguration)
	}

	// positive capcity and another valid configuration
	if opts.Unlimited {
		return fmt.Errorf("cannot specify unlimited with another configuration: %w", ers.ErrMalformedConfiguration)
	}
	return nil
}

// NewDeque constructs a Deque according to the options, and errors if
// there are any problems with the configuration.
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

// NewUnlimitedDeque constructs an unbounded Deque.
func NewUnlimitedDeque[T any]() *Deque[T] {
	return risky.Force(NewDeque[T](DequeOptions{Unlimited: true}))
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

// Close marks the deque as closed, after which point all streams
// will stop and no more operations will succeed. The error value is
// not used in the current operation.
func (dq *Deque[T]) Close() error { defer adt.With(adt.Lock(dq.mtx)); dq.closed = true; return nil }

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

// WaitFront pops the first (head) item in the deque, and if the queue is
// empty, will block until an item is added, returning an error if the
// context canceled or the queue is closed.
func (dq *Deque[T]) WaitFront(ctx context.Context) (v T, err error) {
	defer adt.With(adt.Lock(dq.mtx))
	return dq.waitPop(ctx, dqNext)
}

// WaitBack pops the last (tail) item in the deque, and if the queue
// is empty, will block until an item is added, returning an error if
// the context canceled or the queue is closed.
func (dq *Deque[T]) WaitBack(ctx context.Context) (T, error) {
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

// IteratorFront starts at the front of the queue and iterates towards
// the back. When the stream reaches the beginning of the queue it
// ends.
func (dq *Deque[T]) IteratorFront(ctx context.Context) iter.Seq[T] {
	return dq.confFuture(ctx, dqNext, false)
}

// IteratorBack starts at the back of the queue and iterates
// towards the front. When the stream reaches the end of the queue
// it ends.
func (dq *Deque[T]) IteratorBack(ctx context.Context) iter.Seq[T] {
	return dq.confFuture(ctx, dqPrev, false)
}

// IteratorWaitFront exposes the deque to a single-function interface
// for iteration. The future function operation will not modify the
// contents of the Deque, but will produce elements from the deque,
// front to back, and will block for a new element if the deque is
// empty or the future reaches the end, the operation will block
// until another item is added.
func (dq *Deque[T]) IteratorWaitFront(ctx context.Context) iter.Seq[T] {
	return dq.confFuture(ctx, dqNext, true)
}

// IteratorWaitBack exposes the deque to a single-function interface
// for iteration. The future function operation will not modify the
// contents of the Deque, but will produce elements from the deque,
// back to fron, and will block for a new element if the deque is
// empty or the future reaches the end, the operation will block
// until another item is added.
func (dq *Deque[T]) IteratorWaitBack(ctx context.Context) iter.Seq[T] {
	return dq.confFuture(ctx, dqPrev, true)
}

func (dq *Deque[T]) IteratorPopFront(ctx context.Context) iter.Seq[T] {
	return irt.GenerateOk(dq.PopFront)
}

func (dq *Deque[T]) IteratorPopBack(ctx context.Context) iter.Seq[T] {
	return irt.GenerateOk(dq.PopBack)
}

func (*Deque[T]) zero() (z T) { return z }
func (dq *Deque[T]) confFuture(ctx context.Context, direction dqDirection, blocking bool) iter.Seq[T] {
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

// fifoDistImpl produces a fifoDistImpl instance with
// Send/Receive operations that block if Deque is full or empty
// (respectively). Receive operations always remove the element from
// the Deque.
func (dq *Deque[T]) fifoDistImpl() distributor[T] {
	return distributor[T]{
		push: dq.WaitPushBack,
		pop:  dq.WaitFront,
		size: dq.Len,
	}
}

// lifoDistImpl produces a distributor instance that always
// accepts send items: if the deque is full, it removes one element
// from the front of the queue before adding them to the back.
func (dq *Deque[T]) lifoDistImpl() distributor[T] {
	return distributor[T]{
		push: dq.WaitPushBack,
		pop:  dq.WaitBack,
		size: dq.Len,
	}
}

// LIFO returns an iterator that removes items from the queue, with the most recent items removed
// first. The iterator is blocking and will wait for a new item if the Deque is empty.
func (dq *Deque[T]) LIFO(ctx context.Context) iter.Seq[T] {
	dist := dq.lifoDistImpl()
	return irt.GenerateOk(func() (z T, _ bool) {
		if out, err := dist.Read(ctx); err == nil {
			return out, true
		}
		return z, false
	})
}

// LIFO returns an iterator that removes items from the queue in the order they were added. The iterator is blocking and will wait for a new item if the Deque is empty.
func (dq *Deque[T]) FIFO(ctx context.Context) iter.Seq[T] {
	dist := dq.fifoDistImpl()
	return irt.GenerateOk(func() (z T, _ bool) {
		if out, err := dist.Read(ctx); err == nil {
			return out, true
		}
		return z, false
	})
}

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
	defer dq.updates.Broadcast()

	dq.tracker.remove()

	it.prev.next = it.next
	it.next.prev = it.prev

	// don't reset poointers in the item in case we're using this
	// item in a stream
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
