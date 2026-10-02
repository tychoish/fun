package pubsub

import (
	"context"
	"testing"
	"time"
)

func waitDequeDraining[T any](t *testing.T, dq *Deque[T]) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu := dq.mtx()
		mu.Lock()
		d := dq.drainers > 0
		mu.Unlock()
		if d {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("deque never started draining")
}

func TestDequeForcePushDoesNotLoseItems(t *testing.T) {
	for name, force := range map[string]func(*Deque[int], int) error{
		"Front": (*Deque[int]).ForcePushFront,
		"Back":  (*Deque[int]).ForcePushBack,
	} {
		t.Run(name+"/Draining", func(t *testing.T) {
			dq, err := NewDeque[int](DequeOptions{Capacity: 2})
			if err != nil {
				t.Fatal(err)
			}
			_ = dq.PushBack(1)
			_ = dq.PushBack(2)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- dq.Drain(ctx) }()
			waitDequeDraining(t, dq)

			if err := force(dq, 3); err != ErrQueueDraining {
				t.Fatalf("expected ErrQueueDraining, got %v", err)
			}
			if dq.Len() != 2 {
				t.Fatalf("item lost while draining: len=%d", dq.Len())
			}
			cancel()
			<-done
		})
		t.Run(name+"/Closed", func(t *testing.T) {
			dq, err := NewDeque[int](DequeOptions{Capacity: 2})
			if err != nil {
				t.Fatal(err)
			}
			_ = dq.PushBack(1)
			_ = dq.PushBack(2)
			_ = dq.Close()
			if err := force(dq, 3); err != ErrQueueClosed {
				t.Fatalf("expected ErrQueueClosed, got %v", err)
			}
			if dq.Len() != 2 {
				t.Fatalf("item lost after close: len=%d", dq.Len())
			}
		})
		t.Run(name+"/Tracker", func(t *testing.T) {
			dq, err := NewDeque[int](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 4, SoftQuota: 2, BurstCredit: 1}})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 10; i++ {
				if err := force(dq, i); err != nil {
					t.Fatalf("force push %d: %v", i, err)
				}
				if dq.Len() == 0 {
					t.Fatal("deque unexpectedly empty")
				}
			}
		})
	}
}
