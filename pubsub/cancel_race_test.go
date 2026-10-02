package pubsub

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// A cancel racing with the waiter entering Cond.Wait must still wake it.
func TestCancelVersusWaitStress(t *testing.T) {
	newFull := func() *Deque[int] {
		dq, err := NewDeque[int](DequeOptions{Capacity: 1})
		if err != nil {
			t.Fatal(err)
		}
		if err := dq.PushBack(1); err != nil {
			t.Fatal(err)
		}
		return dq
	}

	cases := map[string]func(context.Context) error{
		"WaitPopFront": func(ctx context.Context) error { _, err := (&Deque[int]{}).WaitPopFront(ctx); return err },
		"WaitPopBack":  func(ctx context.Context) error { _, err := (&Deque[int]{}).WaitPopBack(ctx); return err },
		"WaitPushBack": func(ctx context.Context) error { return newFull().WaitPushBack(ctx, 2) },
		"IterWait": func(ctx context.Context) error {
			for range (&Deque[int]{}).IteratorWaitFront(ctx) {
			}
			return nil
		},
		"QueueWaitPop": func(ctx context.Context) error { _, err := NewUnlimitedQueue[int]().WaitPop(ctx); return err },
		"QueueWaitPush": func(ctx context.Context) error {
			q, _ := NewQueue[int](QueueOptions{HardLimit: 1})
			_ = q.Push(1)
			return q.WaitPush(ctx, 2)
		},
		"QueueIterWait": func(ctx context.Context) error {
			for range NewUnlimitedQueue[int]().IteratorWait(ctx) {
			}
			return nil
		},
	}

	for name, op := range cases {
		t.Run(name, func(t *testing.T) {
			for i := range 1500 {
				ctx, cancel := context.WithCancel(t.Context())
				done := make(chan struct{})
				go func() { defer close(done); _ = op(ctx) }()
				for range i % 7 {
					runtime.Gosched()
				}
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatalf("iteration %d: waiter missed cancellation", i)
				}
			}
		})
	}
}
