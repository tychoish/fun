package pubsub

import (
	"context"
	"errors"
	"testing"
)

// A cancelled ctx wins over an already-drained queue: Shutdown must not close it.
func TestCancelledContextWinsOverReadyDrain(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	if err := q.Drain(cancelled()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := q.Shutdown(cancelled()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := q.Push(1); err != nil {
		t.Fatalf("queue closed or draining: %v", err)
	}
	dq := NewUnlimitedDeque[int]()
	if err := dq.Drain(cancelled()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := dq.Shutdown(cancelled()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := dq.PushBack(1); err != nil {
		t.Fatalf("deque closed or draining: %v", err)
	}
}
