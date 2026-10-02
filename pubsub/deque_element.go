package pubsub

import (
	"context"
	"sync"
)

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

	// seq orders elements within the list (front pushes count down,
	// back pushes count up) so that an iterator holding an element
	// that has since been popped can find its place again.
	seq     int
	removed bool
}

func (it *element[T]) isRoot() bool { return it.root || it == it.list.root }

// release drops everything a popped element references.
func (it *element[T]) release() {
	var zero T
	it.item = zero
	it.next = nil
	it.prev = nil
	it.removed = true
}

// this is just to be able to make the wait method generic.
func (it *element[T]) getNextOrPrevious(direction dqDirection) *element[T] {
	if direction == dqPrev {
		return it.prev
	}
	return it.next
}

// wakeOnCancel broadcasts on cond when ctx ends. The broadcast runs with
// the mutex held, so it cannot fall between a waiter's ctx check and its
// Cond.Wait (which releases the mutex atomically) and be lost. Callers
// must hold mu and call the returned stop function when done.
func wakeOnCancel(ctx context.Context, mu *sync.Mutex, cond *sync.Cond) (stop func()) {
	cancel := context.AfterFunc(ctx, func() {
		mu.Lock()
		defer mu.Unlock()
		cond.Broadcast()
	})
	return func() { cancel() }
}
