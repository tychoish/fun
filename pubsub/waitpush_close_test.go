package pubsub

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type pushWaiter interface {
	WaitPush(context.Context, int) error
	Close() error
}

type dequeAdapter struct{ *Deque[int] }

func (d dequeAdapter) WaitPush(ctx context.Context, v int) error { return d.WaitPushBack(ctx, v) }

func fullPushers(t *testing.T) map[string]pushWaiter {
	t.Helper()
	q, err := NewQueue[int](QueueOptions{HardLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	dq, err := NewDeque[int](DequeOptions{Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Push(1); err != nil {
		t.Fatal(err)
	}
	if err := dq.PushBack(1); err != nil {
		t.Fatal(err)
	}
	return map[string]pushWaiter{"Queue": q, "Deque": dequeAdapter{dq}}
}

func TestWaitPushCloseWhileBlocked(t *testing.T) {
	for name, c := range fullPushers(t) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			errs := make(chan error, 4)
			var started sync.WaitGroup
			for range 4 {
				started.Add(1)
				go func() { started.Done(); errs <- c.WaitPush(ctx, 2) }()
			}
			started.Wait()
			time.Sleep(20 * time.Millisecond)
			_ = c.Close()
			for range 4 {
				if err := <-errs; !errors.Is(err, ErrQueueClosed) {
					t.Fatalf("want ErrQueueClosed, got %v", err)
				}
			}
		})
	}
}

func TestWaitPushCancelStorm(t *testing.T) {
	for name, c := range fullPushers(t) {
		t.Run(name, func(t *testing.T) {
			const waiters = 64
			ctx, cancel := context.WithCancel(t.Context())
			errs := make(chan error, waiters)
			for range waiters {
				go func() { errs <- c.WaitPush(ctx, 2) }()
			}
			time.Sleep(20 * time.Millisecond)
			cancel()
			for range waiters {
				select {
				case err := <-errs:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("want context.Canceled, got %v", err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("waiter stuck after cancel")
				}
			}
		})
	}
}

func TestQueueWaitPushDrainWhileBlocked(t *testing.T) {
	q, _ := NewQueue[int](QueueOptions{HardLimit: 1})
	_ = q.Push(1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	errs := make(chan error, 1)
	go func() { errs <- q.WaitPush(ctx, 2) }()
	time.Sleep(20 * time.Millisecond)
	dctx, dcancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer dcancel()
	_ = q.Drain(dctx) // times out: nothing consumes
	if err := <-errs; !errors.Is(err, ErrQueueDraining) {
		t.Fatalf("want ErrQueueDraining, got %v", err)
	}
}
