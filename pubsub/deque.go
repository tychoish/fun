package pubsub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/risky"
)

// Deque proves a basic double ended queue backed by a doubly linked
// list, with features to support a maximum capacity, burstable limits
// and soft quotas, as well as iterators, that safe for access from
// multiple concurrent go-routines. Furthermore, the implementation
// safely handles multiple concurrent blocking operations (e.g. Wait,
// iterators).
//
// Use the NewDeque constructor to instantiate a Deque object.
type Deque[T any] struct {
	mtx     sync.Mutex
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
// while the Unlimited options
type DequeOptions struct {
	Unlimited    bool
	Capacity     int
	QueueOptions *QueueOptions
}

// ErrConfigurationError is the returned by the queue and deque
// constructors if their configurations are malformed.
var ErrConfigurationMalformed = errors.New("configuration error")

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
		return fmt.Errorf("cannot specify a capcity with queue options: %w", ErrConfigurationMalformed)
	}

	// positive capcity and another valid configuration
	if opts.Unlimited {
		return fmt.Errorf("cannot specify unlimited with another configuration: %w", ErrConfigurationMalformed)
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
	q := &Deque[T]{}
	q.updates = sync.NewCond(&q.mtx)
	q.nfront = sync.NewCond(&q.mtx)
	q.nback = sync.NewCond(&q.mtx)
	q.root = &element[T]{root: true, list: q}
	q.root.next = q.root
	q.root.prev = q.root

	return q
}

// Len returns the length of the queue. This is an O(1) operation in
// this implementation.
func (dq *Deque[T]) Len() int { defer adt.With(adt.Lock(&dq.mtx)); return dq.tracker.len() }

// Close marks the deque as closed, after which point all iterators
// will stop and no more operations will succeed. The error value is
// not used in the current operation.
func (dq *Deque[T]) Close() error { defer adt.With(adt.Lock(&dq.mtx)); dq.closed = true; return nil }

// PushFront adds an item to the front or head of the deque, and
// erroring if the queue is closed, at capacity, or has reached its
// limit.
func (dq *Deque[T]) PushFront(it T) error {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.addAfter(it, dq.root)
}

// PushFront adds an item to the back or end of the deque, and
// erroring if the queue is closed, at capacity, or has reached its
// limit.
func (dq *Deque[T]) PushBack(it T) error {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.addAfter(it, dq.root.prev)
}

// PopFront removes the first (head) item of the queue, with the
// second value being false if the queue is empty or closed.
func (dq *Deque[T]) PopFront() (T, bool) {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.pop(dq.root.next)
}

// PopBack removes the last (tail) item of the queue, with the
// second value being false if the queue is empty or closed.
func (dq *Deque[T]) PopBack() (T, bool) {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.pop(dq.root.prev)
}

// WaitFront pops the first (head) item in the deque, and if the queue is
// empty, will block until an item is added, returning an error if the
// context canceled or the queue is closed.
func (dq *Deque[T]) WaitFront(ctx context.Context) (v T, err error) {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.waitPop(ctx, dqNext)
}

// WaitBack pops the last (tail) item in the deque, and if the queue
// is empty, will block until an item is added, returning an error if
// the context canceled or the queue is closed.
func (dq *Deque[T]) WaitBack(ctx context.Context) (T, error) {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.waitPop(ctx, dqPrev)
}

// ForcePushFront is the same as PushFront, except, if the deque is at
// capacity, it removes one item from the back of the deque and then,
// having made room appends the item. Returns an error if the deque is
// closed.
func (dq *Deque[T]) ForcePushFront(it T) error {
	defer adt.With(adt.Lock(&dq.mtx))

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
	defer adt.With(adt.Lock(&dq.mtx))

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
	defer adt.With(adt.Lock(&dq.mtx))

	return dq.waitPushAfter(ctx, it, func() *element[T] { return dq.root })
}

// WaitPushBack performs a blocking add to the deque: if the deque is
// at capacity, this operation blocks until the deque is closed or
// there is capacity to add an item. The new item is added to the
// back of the deque.
func (dq *Deque[T]) WaitPushBack(ctx context.Context, it T) error {
	defer adt.With(adt.Lock(&dq.mtx))

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

// IteratorReverse starts at the back of the queue and iterates
// towards the front. When the iterator reaches the beginning of the queue
// it ends.
func (dq *Deque[T]) Iterator() *fun.Iterator[T] {
	return dq.Producer().Iterator()
}

// IteratorReverse starts at the back of the queue and iterates
// towards the front. When the iterator reaches the end of the queue
// it ends.
func (dq *Deque[T]) IteratorReverse() *fun.Iterator[T] {
	return dq.ProducerReverse().Iterator()
}

func (dq *Deque[T]) Producer() fun.Producer[T] {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.confProducer(dqNext, false).WithLock(&dq.mtx)
}

func (dq *Deque[T]) ProducerBlocking() fun.Producer[T] {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.confProducer(dqNext, true).WithLock(&dq.mtx)
}

func (dq *Deque[T]) ProducerReverse() fun.Producer[T] {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.confProducer(dqPrev, false).WithLock(&dq.mtx)
}

func (dq *Deque[T]) ProducerReverseBlocking() fun.Producer[T] {
	defer adt.With(adt.Lock(&dq.mtx))
	return dq.confProducer(dqPrev, true).WithLock(&dq.mtx)
}

func (dq *Deque[T]) confProducer(direction dqDirection, blocking bool) fun.Producer[T] {
	var current *element[T]
	return func(ctx context.Context) (out T, _ error) {
		if current == nil {
			current = dq.root
		}

		if current.getNextOrPrevious(direction) == dq.root && blocking {
			if err := current.wait(ctx, direction); err != nil {
				return out, err
			}
		}
		next := current.getNextOrPrevious(direction)
		if next == nil || next == dq.root {
			return out, io.EOF
		}

		current = next
		return current.item, nil
	}
}

// Distributor produces a consuming Distributor implementation that
// always accepts new Push operations by removing the oldest element
// in the queue, with Pop operations returning the oldest elements
// first (FIFO).
//
// If the deque is full, the write operations block.
func (dq *Deque[T]) Distributor() Distributor[T] {
	return Distributor[T]{
		push: dq.WaitPushBack,
		pop:  dq.WaitFront,
		size: dq.Len,
	}
}

func (dq *Deque[T]) DistributorNonBlocking() Distributor[T] {
	return Distributor[T]{
		push: fun.BlockingProcessor(dq.ForcePushBack),
		pop:  dq.WaitFront,
		size: dq.Len,
	}
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
	// item in an iterator
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
	// a new item. The addAfter method signals both forward and
	// reverse waiters when the queue is empty.
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
