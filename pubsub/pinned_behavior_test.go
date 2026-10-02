package pubsub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/ers"
)

// These tests pin current behavior that is open for a decision; see
// the follow-up questions in the review tracker.

func TestDequeCapacityNegativeIsMalformed(t *testing.T) {
	for _, capacity := range []int{-1, -100} {
		dq, err := NewDeque[int](DequeOptions{Capacity: capacity})
		if !errors.Is(err, ers.ErrMalformedConfiguration) || dq != nil {
			t.Fatalf("capacity %d: got %v", capacity, err)
		}
	}
}

func TestDequeCapacityZeroIsUnbounded(t *testing.T) {
	dq, err := NewDeque[int](DequeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 100 {
		if err := dq.PushBack(i); err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
	}
}

func TestDequePopOnClosedNonEmptyReturnsNothing(t *testing.T) {
	dq := &Deque[int]{}
	_ = dq.PushBack(1)
	_ = dq.PushBack(2)
	_ = dq.Close()

	if v, ok := dq.PopFront(); ok {
		t.Fatalf("PopFront on closed non-empty deque returned %d", v)
	}
	if v, ok := dq.PopBack(); ok {
		t.Fatalf("PopBack on closed non-empty deque returned %d", v)
	}
	if _, err := dq.WaitPopFront(t.Context()); err != ErrQueueClosed {
		t.Fatalf("WaitPopFront: %v", err)
	}
	if dq.Len() != 2 {
		t.Fatalf("closed deque lost items: len %d", dq.Len())
	}

}

func TestQueuePopOnClosedNonEmptyReturnsNothing(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	_ = q.Push(1)
	_ = q.Push(2)
	_ = q.Close()

	if v, ok := q.Pop(); ok {
		t.Fatalf("Pop on closed non-empty queue returned %d", v)
	}
	if _, err := q.WaitPop(t.Context()); err != ErrQueueClosed {
		t.Fatalf("WaitPop: %v", err)
	}
	if q.Len() != 2 {
		t.Fatalf("closed queue lost items: len %d", q.Len())
	}
	if err := q.Drain(t.Context()); err != ErrQueueClosed {
		t.Fatalf("Drain: %v", err)
	}
	if err := q.Shutdown(t.Context()); err != ErrQueueClosed {
		t.Fatalf("Shutdown: %v", err)
	}
}

// Iterators, poppers and pushers running together must neither lose
// nor duplicate items nor hang.
func TestDequeIteratorPopInterleave(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	const items = 500
	dq := &Deque[int]{}

	iterCtx, stopIter := context.WithCancel(ctx)
	var iterWG sync.WaitGroup
	for range 2 {
		iterWG.Go(func() {
			prev := -1
			for v := range dq.IteratorWaitFront(iterCtx) {
				if v == prev {
					t.Errorf("iterator yielded %d twice", v)
					return
				}
				prev = v
			}
		})
	}

	var mu sync.Mutex
	seen := map[int]int{}
	var popWG sync.WaitGroup
	for range 3 {
		popWG.Go(func() {
			for v := range dq.IteratorWaitPopFront(ctx) {
				mu.Lock()
				seen[v]++
				n := len(seen)
				mu.Unlock()
				if n == items {
					return
				}
			}
		})
	}

	for i := range items {
		if err := dq.WaitPushBack(ctx, i); err != nil {
			t.Fatal(err)
		}
	}

	done := make(chan struct{})
	go func() { defer close(done); popWG.Wait() }()
	deadline := time.After(15 * time.Second)
	for {
		mu.Lock()
		n := len(seen)
		mu.Unlock()
		if n == items {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only %d of %d items popped", n, items)
		case <-time.After(time.Millisecond):
		}
	}
	_ = dq.Close()
	<-done
	stopIter()
	iterWG.Wait()

	for v, n := range seen {
		if n != 1 {
			t.Fatalf("item %d popped %d times", v, n)
		}
	}
}

func TestQueueIteratorPopInterleave(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	const items = 500
	q := NewUnlimitedQueue[int]()

	var iterWG sync.WaitGroup
	counts := make([]int, 2)
	for i := range counts {
		iterWG.Go(func() {
			prev := -1
			for v := range q.IteratorWait(ctx) {
				if v <= prev {
					t.Errorf("iterator went backwards: %d after %d", v, prev)
					return
				}
				prev = v
				counts[i]++
			}
		})
	}

	popped := make(chan int, items)
	go func() {
		for v := range q.IteratorWaitPop(ctx) {
			popped <- v
		}
	}()

	for i := range items {
		if err := q.Push(i); err != nil {
			t.Fatal(err)
		}
	}
	for want := range items {
		select {
		case got := <-popped:
			if got != want {
				t.Fatalf("popped %d want %d", got, want)
			}
		case <-time.After(15 * time.Second):
			t.Fatalf("stuck after %d pops", want)
		}
	}
	_ = q.Close()
	iterWG.Wait()
}

// Closing a queue under a broker does not drain it: sending reports the
// closed queue, which stops the broker, and the remaining items stay in
// the queue.
func TestQueueBrokerStopsWhenQueueClosedWithItems(t *testing.T) {
	ctx := t.Context()
	q := NewUnlimitedQueue[int]()
	_ = q.Push(1)
	_ = q.Close()
	b := NewQueueBroker(ctx, q, BrokerOptions{})

	if err := b.Send(ctx, 2); !errors.Is(err, ErrBrokerClosed) || !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("Send: %v", err)
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b.Wait(wctx)
	if wctx.Err() != nil {
		t.Fatal("broker did not stop after its queue closed")
	}
	if q.Len() != 1 {
		t.Fatalf("queue len %d", q.Len())
	}
}
