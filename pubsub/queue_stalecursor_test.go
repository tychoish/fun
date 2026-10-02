package pubsub

import (
	"context"
	"testing"
	"time"
)

func TestQueueIteratorWaitSurvivesPop(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	t.Run("TailPopped", func(t *testing.T) {
		q := NewUnlimitedQueue[int]()
		_ = q.Push(1)
		next, stop := pullIter(q.IteratorWait(ctx))
		defer stop()
		pullN(t, next, 1)
		if v, ok := q.Pop(); !ok || v != 1 {
			t.Fatal(v, ok)
		}
		_ = q.Push(2)
		if got := pullN(t, next, 1); got[0] != 2 {
			t.Fatalf("got %v want [2]", got)
		}
	})
	t.Run("NeverYieldsPopped", func(t *testing.T) {
		q := NewUnlimitedQueue[int]()
		for i := 1; i <= 3; i++ {
			_ = q.Push(i)
		}
		next, stop := pullIter(q.IteratorWait(ctx))
		defer stop()
		pullN(t, next, 1)
		q.Pop()
		q.Pop()
		_ = q.Push(4)
		if got := pullN(t, next, 2); got[0] != 3 || got[1] != 4 {
			t.Fatalf("got %v want [3 4]", got)
		}
	})
	t.Run("DrainedThenRefilled", func(t *testing.T) {
		q := NewUnlimitedQueue[int]()
		_ = q.Push(1)
		next, stop := pullIter(q.IteratorWait(ctx))
		defer stop()
		pullN(t, next, 1)
		q.Pop()
		_ = q.Push(2)
		_ = q.Push(3)
		if got := pullN(t, next, 2); got[0] != 2 || got[1] != 3 {
			t.Fatalf("got %v want [2 3]", got)
		}
	})
}

func TestQueueSinglePushWakesAllIterators(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	const n = 3
	res := make(chan int, n)
	for range n {
		go func() {
			for v := range q.IteratorWait(ctx) {
				res <- v
				return
			}
			res <- -1
		}()
	}
	time.Sleep(50 * time.Millisecond)
	_ = q.Push(7)
	for range n {
		if v := <-res; v != 7 {
			t.Fatalf("an iterator missed the push: %d", v)
		}
	}
}
