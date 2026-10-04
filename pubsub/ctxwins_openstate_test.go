package pubsub

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// These tests pin the part of the error-precedence rule that is
// unchanged by the uniform closed/draining-before-ctx decision
// (see errorprecedence_queue_test.go and errorprecedence_deque_test.go):
// on an OPEN queue or deque, a cancelled ctx still wins over a ready
// item or slot. The former cancelfirst_pop_test.go and
// cancelfirst_push_test.go covered this before the precedence rule
// changed; their open-state cases are preserved here under a name that
// no longer implies ctx always wins.

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// A cancelled ctx wins over a ready item on an open queue/deque:
// nothing is consumed.
func TestCancelledContextWinsOverReadyItemPop(t *testing.T) {
	t.Run("QueueWaitPop", func(t *testing.T) {
		q := NewUnlimitedQueue[int]()
		_ = q.Push(1)
		if v, err := q.WaitPop(cancelled()); !errors.Is(err, context.Canceled) || v != 0 {
			t.Fatalf("got %d, %v", v, err)
		}
		if q.Len() != 1 {
			t.Fatal("item consumed")
		}
	})
	t.Run("DequeWaitPop", func(t *testing.T) {
		for name, op := range map[string]func(*Deque[int]) (int, error){
			"front": func(d *Deque[int]) (int, error) { return d.WaitPopFront(cancelled()) },
			"back":  func(d *Deque[int]) (int, error) { return d.WaitPopBack(cancelled()) },
		} {
			dq := NewUnlimitedDeque[int]()
			_ = dq.PushBack(1)
			if v, err := op(dq); !errors.Is(err, context.Canceled) || v != 0 {
				t.Fatalf("%s: got %d, %v", name, v, err)
			}
			if dq.Len() != 1 {
				t.Fatalf("%s: item consumed", name)
			}
		}
	})
	t.Run("Iterators", func(t *testing.T) {
		q := NewUnlimitedQueue[int]()
		_ = q.Push(1)
		if got := slices.Collect(q.IteratorWait(cancelled())); len(got) != 0 {
			t.Fatal(got)
		}
		if got := slices.Collect(q.IteratorWaitPop(cancelled())); len(got) != 0 || q.Len() != 1 {
			t.Fatal(got)
		}
		dq := NewUnlimitedDeque[int]()
		_ = dq.PushBack(1)
		for name, it := range map[string]func(context.Context) []int{
			"waitfront":    func(c context.Context) []int { return slices.Collect(dq.IteratorWaitFront(c)) },
			"waitback":     func(c context.Context) []int { return slices.Collect(dq.IteratorWaitBack(c)) },
			"waitpopfront": func(c context.Context) []int { return slices.Collect(dq.IteratorWaitPopFront(c)) },
			"waitpopback":  func(c context.Context) []int { return slices.Collect(dq.IteratorWaitPopBack(c)) },
		} {
			if got := it(cancelled()); len(got) != 0 || dq.Len() != 1 {
				t.Fatalf("%s: %v", name, got)
			}
		}
	})
}

// A cancelled ctx wins over a ready slot on an open queue/deque:
// nothing is inserted.
func TestCancelledContextWinsOverReadySlotPush(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	if err := q.WaitPush(cancelled(), 1); !errors.Is(err, context.Canceled) || q.Len() != 0 {
		t.Fatalf("queue: %v len=%d", err, q.Len())
	}
	for name, op := range map[string]func(*Deque[int]) error{
		"front": func(d *Deque[int]) error { return d.WaitPushFront(cancelled(), 1) },
		"back":  func(d *Deque[int]) error { return d.WaitPushBack(cancelled(), 1) },
	} {
		dq := NewUnlimitedDeque[int]()
		if err := op(dq); !errors.Is(err, context.Canceled) || dq.Len() != 0 {
			t.Fatalf("%s: %v len=%d", name, err, dq.Len())
		}
	}
}
