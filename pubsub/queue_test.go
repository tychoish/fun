package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQueueNew(t *testing.T) {
	tests := []struct {
		desc string
		opts QueueOptions
		want error
	}{
		{"empty options", QueueOptions{}, errHardLimit},
		{"zero limit negative quota", QueueOptions{SoftQuota: -1}, errHardLimit},
		{"zero limit and quota", QueueOptions{SoftQuota: 0}, errHardLimit},
		{"zero limit", QueueOptions{SoftQuota: 1, HardLimit: 0}, errHardLimit},
		{"limit less than quota", QueueOptions{SoftQuota: 5, HardLimit: 3}, errHardLimit},
		{"negative credit", QueueOptions{SoftQuota: 1, HardLimit: 1, BurstCredit: -6}, errBurstCredit},
		{"valid defaultable", QueueOptions{SoftQuota: -1, HardLimit: 1, BurstCredit: 0}, nil},
		{"valid default credit", QueueOptions{SoftQuota: 1, HardLimit: 2, BurstCredit: 0}, nil},
		{"valid explicit credit", QueueOptions{SoftQuota: 1, HardLimit: 5, BurstCredit: 10}, nil},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got, err := NewQueue[string](test.opts)
			if err != test.want {
				t.Errorf("New(%+v): got (%+v, %v), want err=%v", test.opts, got, err, test.want)
			}
		})
	}
}

type testQueue struct {
	t *testing.T
	*Queue[string]
}

func (q testQueue) mustAdd(item string) {
	q.t.Helper()
	if err := q.Add(item); err != nil {
		q.t.Errorf("Add(%q): unexpected error: %v", item, err)
	}
}

func (q testQueue) mustRemove(want string) {
	q.t.Helper()
	got, ok := q.Remove()
	if !ok {
		q.t.Error("Remove: queue is empty")
	} else if got != want {
		q.t.Errorf("Remove: got %q, want %q", got, want)
	}
}

func mustQueue(t *testing.T, opts QueueOptions) testQueue {
	t.Helper()

	q, err := NewQueue[string](opts)
	if err != nil {
		t.Fatalf("New(%+v): unexpected error: %v", opts, err)
	}
	return testQueue{t: t, Queue: q}
}

func TestQueueHardLimit(t *testing.T) {
	q := mustQueue(t, QueueOptions{SoftQuota: 1, HardLimit: 1})
	q.mustAdd("foo")
	if err := q.Add("bar"); err != ErrQueueFull {
		t.Errorf("Add: got err=%v, want %v", err, ErrQueueFull)
	}
}

func TestQueueSoftQuota(t *testing.T) {
	q := mustQueue(t, QueueOptions{SoftQuota: 1, HardLimit: 4})
	q.mustAdd("foo")
	q.mustAdd("bar")
	if err := q.Add("baz"); err != ErrQueueNoCredit {
		t.Errorf("Add: got err=%v, want %v", err, ErrQueueNoCredit)
	}
}

func TestQueueBurstCredit(t *testing.T) {
	q := mustQueue(t, QueueOptions{SoftQuota: 2, HardLimit: 5})
	q.mustAdd("foo")
	q.mustAdd("bar")
	getTracker := func() *queueLimitTrackerImpl {
		return q.tracker.(*queueLimitTrackerImpl)
	}

	// We should still have all our initial credit.
	if getTracker().credit < 2 {
		t.Errorf("Wrong credit: got %f, want ≥ 2", getTracker().credit)
	}

	// Removing an item below soft quota should increase our credit.
	q.mustRemove("foo")
	if getTracker().credit <= 2 {
		t.Errorf("wrong credit: got %f, want > 2", getTracker().credit)
	}

	// Credit should be capped by the hard limit.
	q.mustRemove("bar")
	q.mustAdd("baz")
	q.mustRemove("baz")
	if cap := float64(getTracker().hardLimit - getTracker().softQuota); getTracker().credit > cap {
		t.Errorf("Wrong credit: got %f, want ≤ %f", getTracker().credit, cap)
	}
}

func TestQueueClose(t *testing.T) {
	q := mustQueue(t, QueueOptions{SoftQuota: 2, HardLimit: 10})
	q.mustAdd("alpha")
	q.mustAdd("bravo")
	q.mustAdd("charlie")
	q.Close()

	// After closing the queue, subsequent writes should fail.
	if err := q.Add("foxtrot"); err == nil {
		t.Error("Add should have failed after Close")
	}

	// However, the remaining contents of the queue should still work.
	q.mustRemove("alpha")
	q.mustRemove("bravo")
	q.mustRemove("charlie")
}

func TestQueueWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := mustQueue(t, QueueOptions{SoftQuota: 2, HardLimit: 2})

	// A wait on an empty queue should time out.
	t.Run("WaitTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		got, err := q.Wait(ctx)
		if err == nil {
			t.Errorf("Wait: got %v, want error", got)
		} else if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Wait should have encountered a timeout, but got: %v", err)
		}
	})

	// A wait on a non-empty queue should report an item.
	t.Run("WaitNonEmpty", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		const input = "figgy pudding"
		q.mustAdd(input)

		got, err := q.Wait(ctx)
		if err != nil {
			t.Errorf("Wait: unexpected error: %v", err)
		} else if got != input {
			t.Errorf("Wait: got %q, want %q", got, input)
		}
	})

	// Wait should block until an item arrives.
	t.Run("WaitOnEmpty", func(t *testing.T) {
		const input = "fleet footed kittens"

		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err := q.Wait(ctx)
			if err != nil {
				t.Errorf("Wait: unexpected error: %v", err)
			} else if got != input {
				t.Errorf("Wait: got %q, want %q", got, input)
			}
		}()

		q.mustAdd(input)
		<-done
	})

	// Closing the queue unblocks a wait.
	t.Run("UnblockOnClose", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err := q.Wait(ctx)
			if err != ErrQueueClosed {
				t.Errorf("Wait: got (%v, %v), want %v", got, err, ErrQueueClosed)
			}
		}()

		q.Close()
		<-done
	})
}

func TestQueueIterator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("EndToEnd", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}
		queue.Add("one")
		queue.Add("two")
		queue.Add("thr")

		if queue.tracker.len() != 3 {
			t.Fatal("unexpected queue length", queue.tracker.len())
		}

		iter := queue.Iterator()
		if !iter.Next(ctx) {
			t.Fatal("should have iterated")
		}
		if val := iter.Value(); val != "one" {
			t.Fatalf("unexpected value %q", val)
		}

		if !iter.Next(ctx) {
			t.Fatal("should have iterated")
		}
		if val := iter.Value(); val != "two" {
			t.Fatal("unexpected value", val)
		}
		if !iter.Next(ctx) {
			t.Fatal("should have iterated")
		}
		if val := iter.Value(); val != "thr" {
			t.Fatal("unexpected value", val)
		}

		startAt := time.Now()
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 400*time.Millisecond)
		defer timeoutCancel()
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			start := time.Now()
			if !iter.Next(timeoutCtx) {
				t.Error("expected to eventually be true", time.Since(start))
			}
		}()
		time.Sleep(10 * time.Millisecond)
		queue.Add("four")
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-sig:
		}

		if queue.tracker.len() != 4 {
			t.Error("unexpected queue length", queue.tracker.len())
		}

		if time.Since(startAt) > 150*time.Millisecond {
			// if we get here, we hit a timeout
			t.Error("hit timeout didn't wait long enough", time.Since(startAt))
		}
		if time.Since(startAt) < time.Millisecond {
			t.Error("returned too soon")
		}

		if val := iter.Value(); val != "four" {
			t.Log("unexpected value", val)
		}
	})
	t.Run("EndToEndStartEmpty", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}

		iter := queue.Iterator()

		startAt := time.Now()
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 400*time.Millisecond)
		defer timeoutCancel()
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			start := time.Now()
			if !iter.Next(timeoutCtx) {
				t.Error("expected to eventually be true", time.Since(start))
			}
		}()
		time.Sleep(10 * time.Millisecond)
		queue.Add("one")

		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-sig:
		}

		if queue.tracker.len() != 1 {
			t.Error("unexpected queue length", queue.tracker.len())
		}

		if time.Since(startAt) > 150*time.Millisecond {
			// if we get here, we hit a timeout
			t.Error("hit timeout didn't wait long enough", time.Since(startAt))
		}
		if time.Since(startAt) < time.Millisecond {
			t.Error("returned too soon")
		}

		if val := iter.Value(); val != "one" {
			t.Log("unexpected value", val)
		}
	})
	t.Run("ClosedDoesNotIterate", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}
		queue.Add("one")

		iter := queue.Iterator()
		if !iter.Next(ctx) {
			t.Fatal("should iterate once")
		}

		err = iter.Close(ctx)
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 2; i++ {
			if iter.Next(ctx) {
				t.Error("should not iterate")
			}
		}

	})
	t.Run("CanceledDoesNotIterate", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}
		queue.Add("one")

		iter := queue.Iterator()
		if !iter.Next(ctx) {
			t.Fatal("should iterate once")
		}
		cancel()
		for i := 0; i < 2; i++ {
			if iter.Next(ctx) {
				t.Log("should not iterate")
			}
		}

		err = iter.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ClosedQueueDoesNotIterate", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}
		queue.Add("one")

		iter := queue.Iterator()
		if !iter.Next(ctx) {
			t.Fatal("should iterate once")
		}

		queue.Close()
		for i := 0; i < 2; i++ {
			if iter.Next(ctx) {
				t.Log("should not iterate")
			}
		}

		err = iter.Close(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	})

}
