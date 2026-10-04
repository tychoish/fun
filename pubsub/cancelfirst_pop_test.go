package pubsub

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// A cancelled ctx wins over a ready item: nothing is consumed.
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
