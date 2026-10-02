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
	once     sync.Once
	tracker  queueLimitTracker
	drainers int // number of outstanding Drain calls
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
	q := &Queue[T]{tracker: tracker}
	q.init()
	return q
}

func (q *Queue[T]) mtx() *sync.Mutex   { return &q.mu }
func (*Queue[T]) with(mtx *sync.Mutex) { mtx.Unlock() }
func (q *Queue[T]) lock() *sync.Mutex  { mtx := q.mtx(); mtx.Lock(); q.init(); return mtx }
func (q *Queue[T]) init()              { q.once.Do(q.doInit) }
func (q *Queue[T]) doInit() {
	sentinel := new(entry[T])
	q.back = sentinel
	q.front = sentinel
	q.nempty = sync.NewCond(&q.mu)
	q.nupdates = sync.NewCond(&q.mu)
	if q.tracker == nil {
		q.tracker = &queueNoLimitTrackerImpl{}
	}
}

// Push adds item to the back of the queue. It reports an error and does not
// enqueue the item if the queue is full or closed, or if it exceeds its soft
// quota and there is not enough burst credit.
func (q *Queue[T]) Push(item T) error {
	defer q.with(q.lock())

	return q.doAdd(item)
}

// Len returns the number of items in the queue. Because the queue
// tracks its size this is a constant time operation.
func (q *Queue[T]) Len() int {
	defer q.with(q.lock())
	return q.tracker.len()
}

func (q *Queue[T]) doAdd(item T) error {
	if q.drainers > 0 {
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

	// for the iterators and WaitPush callers, which may be many
	q.nupdates.Broadcast()

	return nil
}

// WaitPush attempts to add an item to the queue, as with Add, but
// if the queue is full, blocks until the queue has capacity, is
// closed, or the context is canceled. Returns an error if the context
// is canceled or the queue is closed. If the queue is closed or starts
// draining while WaitPush is blocked, it returns ErrQueueClosed or
// ErrQueueDraining rather than waiting out the context.
func (q *Queue[T]) WaitPush(ctx context.Context, item T) error {
	defer q.with(q.lock())
	if q.drainers > 0 {
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
	defer wakeOnCancel(ctx, &q.mu, cond)()

	for q.tracker.cap() <= q.tracker.len() {
		if q.drainers > 0 {
			return ErrQueueDraining
		}
		if q.closed {
			return ErrQueueClosed
		}

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
// reports whether an item was available.  If the queue is empty or
// closed, Pop returns T<zero>, false.
func (q *Queue[T]) Pop() (out T, ok bool) {
	defer q.with(q.lock())

	if q.closed {
		return
	}

	switch q.tracker.len() {
	case 0:
		return
	case 1:
		out = q.popFront()
		ok = true
		if q.drainers > 0 {
			q.nempty.Broadcast()
			break
		}
		q.nempty.Signal()
	default:
		out = q.popFront()
		ok = true
		if q.drainers > 0 {
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
// A closed queue yields nothing, even if items remain.
func (q *Queue[T]) WaitPop(ctx context.Context) (out T, _ error) {
	defer q.with(q.lock())

	// If the context terminates, wake the waiter.
	defer wakeOnCancel(ctx, &q.mu, q.nempty)()

	for q.tracker.len() == 0 || q.closed {
		if q.closed {
			return out, ErrQueueClosed
		}

		if err := ctx.Err(); err != nil {
			return out, ers.Wrap(err, "WaitPop() canceled while waiting for an item")
		}

		q.nempty.Wait()
	}
	return q.popFront(), nil
}

// Drain marks the queue as draining so that new items cannot be
// added, and then blocks until the queue is empty (or it's context is
// canceled.) This does not close the queue: when Drain returns the
// queue is empty, but new work can then be added. To Drain and
// shutdown, use the Shutdown method. If the queue is closed while
// items remain, Drain returns ErrQueueClosed.
func (q *Queue[T]) Drain(ctx context.Context) error {
	defer q.with(q.lock())

	return q.waitForDrain(ctx)
}

func (q *Queue[T]) waitForDrain(ctx context.Context) error {
	// when the function returns wake all other waiters.
	ctx, cancel := context.WithCancel(ctx)
	defer wakeOnCancel(ctx, &q.mu, q.nempty)()
	defer cancel()
	q.drainers++
	defer func() { q.drainers-- }()

	// wake blocked pushers so they notice the drain.
	q.nupdates.Broadcast()

	for q.tracker.len() > 0 {
		if q.closed {
			return ErrQueueClosed
		}
		if err := ctx.Err(); err != nil {
			return ers.Wrapf(err, "Drain() returned early with %d items remaining", q.tracker.len())
		}
		q.nempty.Wait()
	}

	return nil
}

// Close closes the queue immediately, without draining it: further
// pushes report ErrQueueClosed, Pop returns nothing and WaitPop
// returns ErrQueueClosed, even if items remain. To get the items out
// before closing, use Drain or Shutdown.
func (q *Queue[T]) Close() error {
	defer q.with(q.lock())

	q.doClose()

	return nil
}

// Shutdown drains the queue, waiting for all items to be removed from
// the queue and then clsoes it so no additional work can be added to
// the queue. If the queue is closed while items remain, Shutdown
// returns ErrQueueClosed.
func (q *Queue[T]) Shutdown(ctx context.Context) error {
	defer q.with(q.lock())
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
	item := e.item
	*e = entry[T]{popped: true}

	q.tracker.remove()
	q.nupdates.Broadcast()

	// Drain waits on nempty: wake it whenever any consumer empties the
	// queue, rather than relying on the caller to do so.
	if q.tracker.len() == 0 {
		q.nempty.Broadcast()
	}

	return item
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
	item   T
	link   *entry[T]
	popped bool
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
	var cursor *entry[T]
	op := func() (o T, _ bool) {
		defer q.with(q.lock())

		if cursor == nil {
			cursor = q.front
		}

		defer wakeOnCancel(ctx, &q.mu, q.nupdates)()
		for {
			if ctx.Err() != nil {
				return o, false
			}
			if next := q.after(cursor); next != nil {
				cursor = next
				return next.item, true
			}
			if q.closed {
				return o, false
			}
			q.nupdates.Wait()
		}
	}
	return irt.GenerateOk(op)
}

// after returns the entry following the cursor, or nil if the cursor is
// at the end of the queue. Items only leave from the front, so every
// live entry is newer than a popped cursor and the oldest live entry
// is the one that follows it.
func (q *Queue[T]) after(cursor *entry[T]) *entry[T] {
	if cursor.popped {
		return q.front.link
	}
	return cursor.link
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
	}, q.mtx())
}
