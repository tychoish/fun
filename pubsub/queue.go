package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/tychoish/fun"
)

// stolen shamelessly from https://github.com/tendermint/tendermint/tree/master/internal/libs/queue

// queue implements a dynamic FIFO queue with a fixed upper bound
// and a flexible quota mechanism to handle bursty load.
var (
	// ErrQueueFull is returned by the Add method of a queue when the queue has
	// reached its hard capacity limit.
	ErrQueueFull = errors.New("queue is full")

	// ErrQueueNoCredit is returned by the Add method of a queue when the queue has
	// exceeded its soft quota and there is insufficient burst credit.
	ErrQueueNoCredit = errors.New("insufficient burst credit")

	// ErrQueueClosed is returned by the Add method of a closed queue, and by
	// the Wait method of a closed empty queue.
	ErrQueueClosed = errors.New("queue is closed")

	// Sentinel errors reported by the New constructor.
	errHardLimit   = fmt.Errorf("hard limit must be > 0 and ≥ soft quota: %w", ErrConfigurationMalformed)
	errBurstCredit = fmt.Errorf("burst credit must be non-negative: %w", ErrConfigurationMalformed)
)

// A Queue is a limited-capacity FIFO queue of arbitrary data items.
//
// A queue has a soft quota and a hard limit on the number of items that may be
// contained in the queue. Adding items in excess of the hard limit will fail
// unconditionally.
//
// For items in excess of the soft quota, a credit system applies: Each queue
// maintains a burst credit score. Adding an item in excess of the soft quota
// costs 1 unit of burst credit. If there is not enough burst credit, the add
// will fail.
//
// The initial burst credit is assigned when the queue is constructed. Removing
// items from the queue adds additional credit if the resulting queue length is
// less than the current soft quota. Burst credit is capped by the hard limit.
//
// A Queue is safe for concurrent use by multiple goroutines.
type Queue[T any] struct {
	mu       sync.Mutex // protects the fields below
	tracker  queueLimitTracker
	closed   bool
	nempty   *sync.Cond
	nupdates *sync.Cond

	// The queue is singly-linked. Front points to the sentinel and back points
	// to the newest entry. The oldest entry is front.link if it exists.
	back  *entry[T]
	front *entry[T]
}

// NewQueue constructs a new empty queue with the specified options.
// It reports an error if any of the option values are invalid.
func NewQueue[T any](opts QueueOptions) (*Queue[T], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	return makeQueue[T](newQueueLimitTracker(opts)), nil
}

// NewUnlimitedQueue produces an unbounded queue.
func NewUnlimitedQueue[T any]() *Queue[T] {
	return makeQueue[T](&queueNoLimitTrackerImpl{})
}

func makeQueue[T any](tracker queueLimitTracker) *Queue[T] {
	sentinel := new(entry[T])
	q := &Queue[T]{
		back:    sentinel,
		front:   sentinel,
		tracker: tracker,
	}
	q.nempty = sync.NewCond(&q.mu)
	q.nupdates = sync.NewCond(&q.mu)
	return q
}

// Add adds item to the back of the queue. It reports an error and does not
// enqueue the item if the queue is full or closed, or if it exceeds its soft
// quota and there is not enough burst credit.
func (q *Queue[T]) Add(item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.doAdd(item)
}

func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.tracker.len()
}

func (q *Queue[T]) doAdd(item T) error {
	if q.closed {
		return ErrQueueClosed
	}

	if err := q.tracker.add(); err != nil {
		return err
	}

	e := &entry[T]{item: item}
	q.back.link = e
	q.back = e
	if q.tracker.len() == 1 { // was empty
		q.nempty.Signal()
	}

	// for the iterator, signal for any updates
	q.nupdates.Signal()

	return nil
}

// BlockingAdd attempts to add an item to the queue, as with Add, but
// if the queue is full, blocks until the queue has capacity, is
// closed, or the context is canceled. Returns an error if the context
// is canceled or the queue is closed.
func (q *Queue[T]) BlockingAdd(ctx context.Context, item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	if q.tracker.cap() > q.tracker.len() {
		return q.doAdd(item)
	}

	cond := q.nupdates

	// If the context terminates, wake the waiter.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); cond.Broadcast() }()
	defer cancel()

	for q.tracker.cap() <= q.tracker.len() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			cond.Wait()
		}
	}
	return q.doAdd(item)
}

// Remove removes and returns the frontmost (oldest) item in the queue and
// reports whether an item was available.  If the queue is empty, Remove
// returns nil, false.
func (q *Queue[T]) Remove() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.tracker.len() == 0 {
		return fun.ZeroOf[T](), false
	}
	return q.popFront(), true
}

// Wait blocks until q is non-empty or closed, and then returns the frontmost
// (oldest) item from the queue. If ctx ends before an item is available, Wait
// returns a nil value and a context error. If the queue is closed while it is
// still empty, Wait returns nil, ErrQueueClosed.
func (q *Queue[T]) Wait(ctx context.Context) (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if err := q.unsafeWaitWhileEmpty(ctx); err != nil {
		return fun.ZeroOf[T](), err
	}

	return q.popFront(), nil
}

// caller must hold the lock, but this implements the wait behavior
// without modifying the queue for use in the iterator.
func (q *Queue[T]) unsafeWaitWhileEmpty(ctx context.Context) error {
	// If the context terminates, wake the waiter.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); q.nempty.Broadcast() }()
	defer cancel()

	for q.tracker.len() == 0 {
		if q.closed {
			return ErrQueueClosed
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			q.nempty.Wait()
		}
	}
	return nil
}

func (q *Queue[T]) unsafeWaitForNew(ctx context.Context) error {
	// when the function returns wake all other waiters.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); q.nupdates.Broadcast() }()
	defer cancel()

	head := q.back
	for head == q.back {
		if q.closed {
			return ErrQueueClosed
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			q.nupdates.Wait()
		}
	}

	return nil
}

// Close closes the queue. After closing, any further Add calls will report an
// error, but items that were added to the queue prior to closing will still be
// available for Remove and Wait. Wait will report an error without blocking if
// it is called on a closed, empty queue.
func (q *Queue[T]) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.nupdates.Broadcast()
	q.nempty.Broadcast()
	return nil
}

// popFront removes the frontmost item of q and returns its value after
// updating quota and credit settings.
//
// Preconditions: The caller holds q.mu and q is not empty.
func (q *Queue[T]) popFront() T {
	e := q.front.link
	q.front.link = e.link
	if e == q.back {
		q.back = q.front
	}

	q.tracker.remove()
	q.nupdates.Broadcast()

	return e.item
}

// QueueOptions are the initial settings for a Queue or Deque.
type QueueOptions struct {
	// The maximum number of items the queue will ever be
	// permitted to hold. This value must be positive, and greater
	// than or equal to SoftQuota. The hard limit is fixed and
	// does not change as the queue is used.
	//
	// The hard limit should be chosen to exceed the largest burst
	// size expected under normal operating conditions.
	HardLimit int

	// The initial expected maximum number of items the queue
	// should contain on an average workload. If this value is
	// zero, it is initialized to the hard limit. The soft quota
	// is adjusted from the initial value dynamically as the queue
	// is used.
	SoftQuota int

	// The initial burst credit score.  This value must be greater
	// than or equal to zero. If it is zero, the soft quota is
	// used.
	BurstCredit float64
}

// Validate ensures that the options are consistent. Exported as a
// convenience function. All errors have ErrConfigurationMalformed as
// their root.
func (opts *QueueOptions) Validate() error {
	if opts.HardLimit <= 0 || opts.HardLimit < opts.SoftQuota {
		return errHardLimit
	}
	if opts.BurstCredit < 0 {
		return errBurstCredit
	}
	if opts.SoftQuota <= 0 {
		opts.SoftQuota = opts.HardLimit
	}
	if opts.BurstCredit == 0 {
		opts.BurstCredit = float64(opts.SoftQuota)
	}
	return nil

}

type entry[T any] struct {
	item T
	link *entry[T]
}

// internal implementation of the queue iterator, operations must hold
// the queue's mutex.
type queueIterImpl[T any] struct {
	queue  *Queue[T]
	item   *entry[T]
	closed bool
}

// Iterator produces an iterator implementation that wraps the
// underlying queue linked list. The iterator respects the Queue's
// mutex and is safe for concurrent access and current queue
// operations, without additional locking. The iterator does not
// modify the contents of the queue, and will only terminate when the
// queue has been closed via the Close() method.
func (q *Queue[T]) Iterator() fun.Iterator[T] {
	iter := &queueIterImpl[T]{
		queue: q,
	}

	return iter
}

func (iter *queueIterImpl[T]) Next(ctx context.Context) bool {
	iter.queue.mu.Lock()
	defer iter.queue.mu.Unlock()

	if ctx.Err() != nil {
		return false
	}

	if iter.item == nil {
		iter.item = iter.queue.front
		// the sentinal is always non-nil, but the link has to
		// be non-nil and so we bump here as a special case
		if iter.item.link != nil {
			iter.item = iter.item.link
			return true
		}
	}

	if iter.item.link == nil {
		if iter.closed {
			return false
		}
		if err := iter.queue.unsafeWaitForNew(ctx); err != nil {
			if errors.Is(err, ErrQueueClosed) {
				iter.closed = true
				return false
			}
			return false
		}
	}

	iter.item = iter.item.link
	return true
}

func (iter *queueIterImpl[T]) Close() error {
	iter.queue.mu.Lock()
	defer iter.queue.mu.Unlock()
	iter.closed = true
	return nil
}

func (iter *queueIterImpl[T]) Value() T {
	iter.queue.mu.Lock()
	defer iter.queue.mu.Unlock()

	return iter.item.item
}
