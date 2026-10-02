package pubsub

import (
	"context"
	"testing"
	"time"
)

func TestQueueDrainWakesOnWaitPop(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	for i := 0; i < 3; i++ {
		if err := q.Push(i); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- q.Drain(ctx) }()
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < 3; i++ {
		if _, err := q.WaitPop(ctx); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("drain failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Drain was not woken after WaitPop emptied the queue")
	}
}
