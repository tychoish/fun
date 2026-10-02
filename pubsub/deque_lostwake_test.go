package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Items must never sit in the deque while every popper sleeps.
func TestDequeLostWakeupStress(t *testing.T) {
	for _, front := range []bool{true, false} {
		for iter := range 200 {
			dq := &Deque[int]{}
			ctx, cancel := context.WithCancel(t.Context())
			const poppers, items = 3, 20
			var got atomic.Int64
			var wg sync.WaitGroup
			for range poppers {
				wg.Go(func() {
					for {
						var err error
						if front {
							_, err = dq.WaitPopFront(ctx)
						} else {
							_, err = dq.WaitPopBack(ctx)
						}
						if err != nil {
							return
						}
						got.Add(1)
					}
				})
			}
			for i := range items {
				if err := dq.PushBack(i); err != nil {
					t.Fatal(err)
				}
				if i%3 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
			deadline := time.Now().Add(5 * time.Second)
			for got.Load() < items && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if got.Load() != items {
				t.Fatalf("iter %d front=%v: popped %d of %d, deque len %d (lost wakeup)", iter, front, got.Load(), items, dq.Len())
			}
			cancel()
			wg.Wait()
		}
	}
}

// Poppers arriving after items were pushed must take them immediately.
func TestDequeWaitPopWithItemsPresent(t *testing.T) {
	dq := &Deque[int]{}
	for i := range 3 {
		if err := dq.PushBack(i); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	for want := range 3 {
		got, err := dq.WaitPopFront(ctx)
		if err != nil || got != want {
			t.Fatalf("got %d, %v want %d", got, err, want)
		}
	}
}
