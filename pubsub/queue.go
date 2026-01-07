package pubsub

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
)

// stolen shamelessly from https://github.com/tendermint/tendermint/tree/master/internal/libs/queue

const (
	// ErrQueueFull is returned by the Add method of a queue when the queue has
	// reached its hard capacity limit.
	ErrQueueFull = ers.Error("queue is full")

	// ErrQueueNoCredit is returned by the Add method of a queue when the queue has
	// exceeded its soft quota and there is insufficient burst credit.
	ErrQueueNoCredit = ers.Error("insufficient burst credit")

	// ErrQueueClosed is returned by the Add method of a closed queue, and by
	// the Wait method of a closed empty queue.
	ErrQueueClosed = ers.ErrContainerClosed
	// ErrQueueDraining is returned by Add methods when the queue is in a draining state before all the work has completed,
	// when the Add method will begin returning ErrQueueClosed.
	ErrQueueDraining = ers.Error("the queue is shutting down")
)

var (
	// Sentinel errors reported by the New constructor.
	errHardLimit   = fmt.Errorf("hard limit must be > 0 and ≥ soft quota: %w", ers.ErrMalformedConfiguration)
	errBurstCredit = fmt.Errorf("burst credit must be non-negative: %w", ers.ErrMalformedConfiguration)
)

// A Queue is a limited-capacity FIFO queue of arbitrary data items.
//
// A queue has a soft quota and a hard limit on the number of items that may be
// contained in the queue. Adding items in excess of the hard limit will fail
// unconditionally.
//
// For items in excess of the soft quota, a credit system applies: Each queue
// maintains a burst credit score. Adding an item ein excess of the soft quota
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
	draining bool
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

// Push adds item to the back of the queue. It reports an error and does not
// enqueue the item if the queue is full or closed, or if it exceeds its soft
// quota and there is not enough burst credit.
func (q *Queue[T]) Push(item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.doAdd(item)
}

// Len returns the number of items in the queue. Because the queue
// tracks its size this is a constant time operation.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.tracker.len()
}

func (q *Queue[T]) doAdd(item T) error {
	if q.draining {
		return ErrQueueDraining
	}

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

// WaitPush attempts to add an item to the queue, as with Add, but
// if the queue is full, blocks until the queue has capacity, is
// closed, or the context is canceled. Returns an error if the context
// is canceled or the queue is closed.
func (q *Queue[T]) WaitPush(ctx context.Context, item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.draining {
		return ErrQueueDraining
	}

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

// Pop removes and returns the frontmost (oldest) item in the queue and
// reports whether an item was available.  If the queue is empty, Pop
// returns nil, false.
func (q *Queue[T]) Pop() (out T, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	switch q.tracker.len() {
	case 0:
		return
	case 1:
		out = q.popFront()
		ok = true
		if q.draining || q.closed {
			q.nempty.Broadcast()
			break
		}
		q.nempty.Signal()
	default:
		out = q.popFront()
		ok = true

		if q.draining || q.closed {
			q.nupdates.Broadcast()
			break
		}
		q.nupdates.Signal()
	}

	return
}

// WaitPop blocks until q is non-empty or closed, and then returns the frontmost
// (oldest) item from the queue. If ctx ends before an item is available, WaitPop
// returns a nil value and a context error. If the queue is closed while it is
// still, WaitPop returns nil, ErrQueueClosed.
//
// WaitPop is destructive: every item returned is removed from the queue.
func (q *Queue[T]) WaitPop(ctx context.Context) (out T, _ error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// If the context terminates, wake the waiter.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); q.nempty.Broadcast() }()
	defer cancel()

	for q.tracker.len() == 0 {
		if q.closed {
			return out, ErrQueueClosed
		}

		if err := ctx.Err(); err != nil {
			return out, ers.Wrapf(err, "Drain() returned early with %d items remaining", q.tracker.len())
		}

		q.nempty.Wait()
	}
	return q.popFront(), nil
}

// Drain marks the queue as draining so that new items cannot be
// added, and then blocks until the queue is empty (or it's context is
// canceled.) This does not close the queue: when Drain returns the
// queue is empty, but new work can then be added. To Drain and
// shutdown, use the Shutdown method.
func (q *Queue[T]) Drain(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.waitForDrain(ctx)
}

func (q *Queue[T]) waitForDrain(ctx context.Context) error {
	// when the function returns wake all other waiters.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); q.nempty.Broadcast() }()
	defer cancel()
	q.draining = true
	defer func() { q.draining = false }()

	for q.tracker.len() > 0 {
		if err := ctx.Err(); err != nil {
			return ers.Wrapf(err, "Drain() returned early with %d items remaining", q.tracker.len())
		}
		q.nempty.Wait()
	}

	return nil
}

func (q *Queue[T]) waitForNew(ctx context.Context) error {
	// when the function returns wake all other waiters.
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); q.nupdates.Broadcast() }()
	defer cancel()

	head := q.back
	for head == q.back && q.back.link != q.front {
		if q.closed {
			return ErrQueueClosed
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		q.nupdates.Wait()
	}

	return nil
}

// Close closes the queue. After closing, any further Add calls will
// report an error, but items that were added to the queue prior to
// closing will still be available for Pop and WaitPop. WaitPop will
// report an error without blocking if it is called on a closed, empty
// queue.
func (q *Queue[T]) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.doClose()

	return nil
}

// Shutdown drains the queue, waiting for all items to be removed from
// the queue and then clsoes it so no additional work can be added to
// the queue.
func (q *Queue[T]) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if err := q.waitForDrain(ctx); err != nil {
		return err
	}

	q.doClose()

	return nil
}

func (q *Queue[T]) doClose() {
	q.closed = true
	q.nupdates.Broadcast()
	q.nempty.Broadcast()
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

// IteratorWait produces an iteratorthat wraps the
// underlying queue linked list. The iterator respects the Queue's
// mutex and is safe for concurrent access and current queue
// operations, without additional locking. The iterator does not
// modify or remove items from the queue, and will only terminate when
// the queue has been closed via the Close() method.
//
// For a consuming stream, use IteratorWaitPop.
func (q *Queue[T]) IteratorWait(ctx context.Context) iter.Seq[T] {
	var next *entry[T]
	op := func() (o T, ok bool) {
		q.mu.Lock()
		defer q.mu.Unlock()

		if next == nil {
			next = q.front
		}

		if next.link == q.front || (next.link == nil && q.closed) || ctx.Err() != nil {
			return o, false
		} else if next.link != nil {
			next = next.link
			return next.item, true
		} else {
			if err := q.waitForNew(ctx); err != nil {
				return o, false
			}

			next, ok = q.advance(next)
			return next.item, ok
		}
	}
	return irt.GenerateOk(op)
}

// IteratorWaitPop returns a consuming iterator that removes items from the
// queue. Blocks waiting for new items when the queue is empty. Iterator
// terminates on context cancellation or queue closure.  Each item returned is
// removed from the queue (destructive read). Safe for concurrent access.
func (q *Queue[T]) IteratorWaitPop(ctx context.Context) iter.Seq[T] {
	return irt.GenerateOk(func() (z T, _ bool) {
		msg, ok := q.Pop() // holds lock
		if ok {
			return msg, true
		}
		if out, err := q.WaitPop(ctx); err == nil {
			return out, true
		}
		return z, false
	})
}

// Iterator returns an iterator for all items in the queue. Does not block.
func (q *Queue[T]) Iterator() iter.Seq[T] {
	return irt.WithMutex(func(yield func(T) bool) {
		for next := q.front.link; !q.closed && next != nil && q.front != q.back && q.front != next && yield(next.item); next = next.link {
			continue
		}
	}, &q.mu)
}

func (q *Queue[T]) advance(next *entry[T]) (_ *entry[T], ok bool) {
	if next.link != q.front && next.link != nil {
		next, ok = next.link, true
	}
	return next, ok
}
