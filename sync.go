package fun

import (
	"context"
	"sync"
	"sync/atomic"
)

// Atomic is a very simple atomic Get/Set operation, providing a
// generic type-safe implementation wrapping
// sync/atomic.Value.
type Atomic[T any] struct{ atomic.Value }

// NewAtomic creates a new Atomic Get/Set value with the initial value
// already set.
func NewAtomic[T any](initial T) *Atomic[T] { a := &Atomic[T]{}; a.Set(initial); return a }

// Set atomically sets the value of the Atomic.
func (a *Atomic[T]) Set(in T) { a.Value.Store(in) }

// Get resolves the atomic value, returning the zero value of the type
// T if the value is unset.
func (a *Atomic[T]) Get() T { return ZeroWhenNil[T](a.Value.Load()) }

// WaitGroup works like sync.WaitGroup, except that the Wait method
// takes a context (and can be passed as a fun.WaitFunc). The
// implementation is exceptionally simple. The only constraint is that
// you can never modify the value of the internal counter such that it
// is negative, event transiently. The implementation does not require
// background resources aside from Wait, which creates a goroutine
// that lives for the entire time that Wait is running. Multiple
// copies of Wait can be safely called at once, and the WaitGroup is
// reusable more than once.
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

// Wait blocks until either the context is canceled or the
func (wg *WaitGroup) Wait(ctx context.Context) {
	wg.mu.Lock()
	defer wg.mu.Unlock()
	wg.init()

	if wg.counter == 0 || ctx.Err() != nil {
		return
	}
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
