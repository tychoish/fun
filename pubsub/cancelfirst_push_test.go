package pubsub

import (
	"context"
	"errors"
	"testing"
)

// A cancelled ctx wins over a ready slot: nothing is inserted.
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
