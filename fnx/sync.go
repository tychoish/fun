package fnx

import (
	"context"
	"sync"

	"github.com/tychoish/fun/irt"
)

// WaitGroup works like sync.WaitGroup, except that the Wait method
// takes a context (and can be passed as a fnx.Operation). The
// implementation is exceptionally simple. The only constraint, like
// sync.WaitGroup, is that you can never modify the value of the
// internal counter such that it is negative, event transiently. The
// implementation does not require background resources aside from
// Wait, which creates a single goroutine that lives for the entire
// time that Wait is running, but no other background resources are
// created. Multiple copies of Wait can be safely called at once, and
// the WaitGroup is reusable more than once.
//
// This implementation is about 50% slower than sync.WaitGroup after
// informal testing. It provides a little extra flexiblity and
// introspection, with similar semantics, that may be worth the
// additional performance hit.
type WaitGroup struct {
	mu      sync.Mutex
	once    sync.Once
	cond    *sync.Cond
	counter int
}

func (wg *WaitGroup) doInit()                         { wg.cond = sync.NewCond(&wg.mu) }
func (wg *WaitGroup) init()                           { wg.once.Do(wg.doInit) }
func (wg *WaitGroup) mtx() *sync.Mutex                { return &wg.mu }
func (*WaitGroup) with(mu *sync.Mutex)                { mu.Unlock() }
func (wg *WaitGroup) lock(mu *sync.Mutex) *sync.Mutex { mu.Lock(); return mu }

// Add modifies the internal counter. Raises an ErrInvariantViolation
// error if any modification causes the internal coutner to be less
// than 0.
func (wg *WaitGroup) Add(num int) {
	defer wg.with(wg.lock(wg.mtx()))
	wg.init()

	invariant(wg.counter+num >= 0, "cannot decrement waitgroup to less than 0: ", wg.counter, " + ", num)

	wg.counter += num

	if wg.counter == 0 {
		wg.cond.Broadcast()
	}
}

// Done marks a single operation as done.
func (wg *WaitGroup) Done() { wg.Add(-1) }

// Inc adds one item to the wait group.
func (wg *WaitGroup) Inc() { wg.Add(1) }

// Num returns the number of pending workers.
func (wg *WaitGroup) Num() int {
	defer wg.with(wg.lock(wg.mtx()))
	return wg.counter
}

// IsDone returns true if there is pending work, and false otherwise.
func (wg *WaitGroup) IsDone() bool {
	defer wg.with(wg.lock(wg.mtx()))
	return wg.counter == 0
}

// Operation returns with WaitGroups Wait method as a Operation.
func (wg *WaitGroup) Operation() Operation { return wg.Wait }

// Worker returns a worker that will block on the wait group
// returning and return the conbext's error if one exits.
func (wg *WaitGroup) Worker() Worker {
	return func(ctx context.Context) error { wg.Wait(ctx); return ctx.Err() }
}

// Group returns an operation that, when executed, starts <n> copies
// of the operation and blocks until all have finished.
func (wg *WaitGroup) Group(n int, op Operation) Operation {
	return func(ctx context.Context) { wg.StartGroup(ctx, n, op).Run(ctx) }
}

// StartGroup starts <n> copies of the operation in separate threads
// and returns an operation that waits on the wait group.
func (wg *WaitGroup) StartGroup(ctx context.Context, n int, op Operation) Operation {
	_ = irt.Count(irt.GenerateN(n, func() bool { wg.Launch(ctx, op); return true }))
	return wg.Wait
}

// Launch increments the WaitGroup and starts the operation in a go
// routine.
func (wg *WaitGroup) Launch(ctx context.Context, op Operation) {
	wg.Inc()
	go func(ctx context.Context) {
		defer wg.Done()
		op(ctx)
	}(ctx)
}

// Wait blocks until either the context is canceled or all items have
// completed.
//
// Wait is passable or usable as a fnx.Operation.
//
// In many cases, callers should not rely on the Wait operation
// returning after the context expires: If Done() calls are used in
// situations that respect a context cancellation, aborting the Wait
// on a context cancellation, particularly when Wait gets a context
// that has the same lifecycle as the operations its waiting on, the
// result is that worker routines will leak. Nevertheless, in some
// situations, when workers may take a long time to respond to a
// context cancellation, being able to set a second deadline on
// Waiting may be useful.
//
// Consider using `fnx.Operation(wg.Wait).Block()` if you want blocking
// semantics with the other features of this WaitGroup implementation.
func (wg *WaitGroup) Wait(ctx context.Context) {
	defer wg.with(wg.lock(wg.mtx()))

	if wg.counter == 0 || ctx.Err() != nil {
		return
	}

	wg.init()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	// need this to wake up any waiters in the case that the
	// context has been canceled, to avoid having many
	// theads/waiters blocking.
	go func() { <-ctx.Done(); wg.cond.Broadcast() }()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// block until the context is canceled or we
			// are signaled.
			wg.cond.Wait()

			if wg.counter == 0 {
				return
			}
		}
	}
}
