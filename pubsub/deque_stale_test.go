package pubsub

import (
	"context"
	"iter"
	"testing"
	"time"
)

func pullN(t *testing.T, next func() (int, bool), n int) []int {
	t.Helper()
	out := make([]int, 0, n)
	for range n {
		v, ok := next()
		if !ok {
			t.Fatalf("iterator ended early after %v", out)
		}
		out = append(out, v)
	}
	return out
}

func TestDequeIteratorSurvivesPopOfCurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	t.Run("TailPopped", func(t *testing.T) {
		dq := &Deque[int]{}
		_ = dq.PushBack(1)
		next, stop := pullIter(dq.IteratorWaitFront(ctx))
		defer stop()
		if got := pullN(t, next, 1); got[0] != 1 {
			t.Fatal(got)
		}
		_, _ = dq.PopFront()
		_ = dq.PushBack(2)
		if got := pullN(t, next, 1); got[0] != 2 {
			t.Fatalf("got %v want [2]", got)
		}
	})
	t.Run("NeverYieldsPopped", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := 1; i <= 3; i++ {
			_ = dq.PushBack(i)
		}
		next, stop := pullIter(dq.IteratorWaitFront(ctx))
		defer stop()
		pullN(t, next, 1)
		_, _ = dq.PopFront() // 1
		_, _ = dq.PopFront() // 2
		_ = dq.PushBack(4)
		if got := pullN(t, next, 2); got[0] != 3 || got[1] != 4 {
			t.Fatalf("got %v want [3 4]", got)
		}
	})
	t.Run("BackwardPopped", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := 1; i <= 3; i++ {
			_ = dq.PushBack(i)
		}
		next, stop := pullIter(dq.IteratorWaitBack(ctx))
		defer stop()
		pullN(t, next, 1) // 3
		_, _ = dq.PopBack()
		_, _ = dq.PopBack()
		_ = dq.PushFront(0)
		if got := pullN(t, next, 2); got[0] != 1 || got[1] != 0 {
			t.Fatalf("got %v want [1 0]", got)
		}
	})
	t.Run("BackwardSkipsLaterPush", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := 1; i <= 3; i++ {
			_ = dq.PushBack(i)
		}
		next, stop := pullIter(dq.IteratorWaitBack(ctx))
		defer stop()
		pullN(t, next, 1) // 3
		_, _ = dq.PopBack()
		_ = dq.PushBack(4) // behind a backward iterator
		if got := pullN(t, next, 1); got[0] != 2 {
			t.Fatalf("got %v want [2]", got)
		}
	})
	t.Run("FrontPushAfterPop", func(t *testing.T) {
		dq := &Deque[int]{}
		_ = dq.PushBack(5)
		next, stop := pullIter(dq.IteratorWaitFront(ctx))
		defer stop()
		pullN(t, next, 1)
		_, _ = dq.PopFront()
		_ = dq.PushFront(9) // logically before the iterator's position
		_ = dq.PushBack(6)
		if got := pullN(t, next, 1); got[0] != 6 {
			t.Fatalf("got %v want [6]", got)
		}
	})
}

func TestDequePoppedElementReleased(t *testing.T) {
	dq := &Deque[*int]{}
	v := new(int)
	_ = dq.PushBack(v)
	el := dq.root.next
	if got, ok := dq.PopFront(); !ok || got != v {
		t.Fatal("pop failed")
	}
	if el.item != nil || el.next != nil || el.prev != nil {
		t.Fatalf("popped element still references payload or neighbors: %+v", el)
	}
}

func pullIter[T any](seq iter.Seq[T]) (func() (T, bool), func()) { return iter.Pull(seq) }
