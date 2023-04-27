package fun

import (
	"context"
	"sync"
)

// WaitGroup works like sync.WaitGroup, except that the Wait method
// takes a context (and can be passed as a fun.WaitFunc). The
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
	cond    *sync.Cond
	counter int
}

func (wg *WaitGroup) init() {
	if wg.cond == nil {
		wg.cond = sync.NewCond(&wg.mu)
	}
}

// Add modifies the internal counter. Raises an ErrInvariantViolation
// error if any modification causes the internal coutner to be less
// than 0.
func (wg *WaitGroup) Add(num int) {
	wg.mu.Lock()
	defer wg.mu.Unlock()
	wg.init()

	Invariant(wg.counter+num >= 0, "cannot decrement waitgroup to less than 0: ", wg.counter, " + ", num)

	wg.counter += num

	if wg.counter == 0 {
		wg.cond.Broadcast()
	}
}

// Done marks a single operation as done.
func (wg *WaitGroup) Done() { wg.Add(-1) }

// Num returns the number of pending workers.
func (wg *WaitGroup) Num() int {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	return wg.counter
}

// IsDone returns true if there is pending work, and false otherwise.
func (wg *WaitGroup) IsDone() bool {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	return wg.counter == 0
}

// Wait blocks until either the context is canceled or all items have
// completed.
//
// Wait is pasable or usable as a fun.WaitFunc.
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
// Consider using `fun.WaitFunc(wg.Wait).Block()` if you want blocking
// semantics with the other features of this WaitGroup implementation.
func (wg *WaitGroup) Wait(ctx context.Context) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

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
