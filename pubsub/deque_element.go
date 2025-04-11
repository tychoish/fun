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
