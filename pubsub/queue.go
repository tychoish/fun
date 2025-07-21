package pubsub

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
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
	ErrQueueClosed   = ers.ErrContainerClosed
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

// Add adds item to the back of the queue. It reports an error and does not
// enqueue the item if the queue is full or closed, or if it exceeds its soft
// quota and there is not enough burst credit.
func (q *Queue[T]) Add(item T) error {
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

	// for the stream, signal for any updates
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

// Remove removes and returns the frontmost (oldest) item in the queue and
// reports whether an item was available.  If the queue is empty, Remove
// returns nil, false.
func (q *Queue[T]) Remove() (out T, _ bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.tracker.len() == 0 {
		return out, false
	}
	return q.popFront(), true
}

// Wait blocks until q is non-empty or closed, and then returns the frontmost
// (oldest) item from the queue. If ctx ends before an item is available, Wait
// returns a nil value and a context error. If the queue is closed while it is
// still, Wait returns nil, ErrQueueClosed.
//
// Wait is destructive: every item returned is removed from the queue.
func (q *Queue[T]) Wait(ctx context.Context) (out T, _ error) {
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

func (q *Queue[T]) Drain(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.draining = true

	if err := q.waitUntilEmpty(ctx); err != nil {
		return err
	}
	return nil
}

func (q *Queue[T]) waitUntilEmpty(ctx context.Context) error {
	for q.tracker.len() > 0 {
		if err := ctx.Err(); err != nil {
			return ers.Wrapf(err, "Drain() returned early with %d items remaining", q.tracker.len())
		}
		q.nempty.Wait()
	}

	return nil
}

func (q *Queue[T]) waitForNew(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

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

func (q *Queue[T]) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.draining = true

	if err := q.waitUntilEmpty(ctx); err != nil {
		return err
	}

	q.closed = true
	q.draining = false
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

// Distributor creates a object used to process the items in the
// queue: items yielded by the Distributor's stream, are removed
// from the queue.
func (q *Queue[T]) Distributor() Distributor[T] {
	return Distributor[T]{
		push: fun.MakeHandler(q.Add),
		pop: func(ctx context.Context) (_ T, err error) {
			msg, ok := q.Remove()
			if ok {
				return msg, nil
			}
			msg, err = q.Wait(ctx)
			return msg, err
		},
		size: q.tracker.len,
	}
}

// Stream produces a stream implementation that wraps the
// underlying queue linked list. The stream respects the Queue's
// mutex and is safe for concurrent access and current queue
// operations, without additional locking. The stream does not
// modify or remove items from the queue, and will only terminate when
// the queue has been closed via the Close() method.
//
// To create a "consuming" stream, use a Distributor.
func (q *Queue[T]) Stream() *fun.Stream[T] {
	var next *entry[T]
	return fun.NewGenerator(func(ctx context.Context) (o T, _ error) {
		if next == nil {
			q.mu.Lock()
			next = q.front
			q.mu.Unlock()
		}

		q.mu.Lock()
		if next.link == q.front {
			q.mu.Unlock()
			return o, io.EOF
		}

		if next.link != nil {
			next = next.link
			q.mu.Unlock()
		} else if next.link == nil {
			if q.closed {
				q.mu.Unlock()
				return o, io.EOF
			}

			q.mu.Unlock()

			if err := q.waitForNew(ctx); err != nil {
				return o, err
			}

			q.mu.Lock()
			if next.link != q.front {
				next = next.link
			}
			q.mu.Unlock()
		}

		return next.item, nil
	}).Stream()
}
