package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tychoish/fun"
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
		if err := queue.Add("one"); err != nil {
			t.Fatal(err)
		}
		if err := queue.Add("two"); err != nil {
			t.Fatal(err)
		}
		if err := queue.Add("thr"); err != nil {
			t.Fatal(err)
		}

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
		if err := queue.Add("four"); err != nil {
			t.Fatal(err)
		}
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

		if err := queue.Add("one"); err != nil {
			t.Fatal(err)
		}

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

		if err := queue.Add("one"); err != nil {
			t.Fatal(err)
		}

		iter := queue.Iterator()
		if !iter.Next(ctx) {
			t.Fatal("should iterate once")
		}

		err = iter.Close()
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

		if err := queue.Add("one"); err != nil {
			t.Fatal(err)
		}

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

		err = iter.Close()
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
		if err := queue.Add("one"); err != nil {
			t.Fatal(err)
		}
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

		err = iter.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("WaitRespectsQueue", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		queue := NewUnlimitedQueue[int]()
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			if err := queue.unsafeWaitForNew(ctx); !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		}()
		sa := time.Now()
		cancel()
		<-sig
		if dur := time.Since(sa); dur > time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("IteratorRetrySpecialCase", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		queue := NewUnlimitedQueue[int]()
		iter := queue.Iterator()
		toctx, toccancel := context.WithTimeout(ctx, time.Millisecond)
		defer toccancel()
		sa := time.Now()
		if iter.Next(toctx) {
			t.Error("should have reported false early", time.Since(sa))
		}
		go func() {
			time.Sleep(2 * time.Millisecond)
			_ = queue.Add(31)
		}()
		if !iter.Next(ctx) {
			t.Error("should have reported item", time.Since(sa), iter.Value())
		}
	})

	t.Run("WaitAdd", func(t *testing.T) {
		t.Run("ContextCanceled", func(t *testing.T) {
			tt := fun.Must(NewQueue[int](QueueOptions{HardLimit: 2}))

			fun.Invariant(tt.BlockingAdd(ctx, 1) == nil)
			fun.Invariant(tt.BlockingAdd(ctx, 1) == nil)

			canceled, trigger := context.WithCancel(context.Background())
			trigger()

			err := tt.BlockingAdd(canceled, 4)
			if err == nil {
				t.Fatal("should be error")
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
		t.Run("Closed", func(t *testing.T) {
			tt := fun.Must(NewQueue[int](QueueOptions{HardLimit: 2}))
			fun.Invariant(tt.BlockingAdd(ctx, 1) == nil)
			fun.Invariant(tt.BlockingAdd(ctx, 1) == nil)

			if err := tt.Close(); err != nil {
				t.Fatal(err)
			}

			err := tt.BlockingAdd(ctx, 4)
			if err == nil {
				t.Fatal("should be error")
			}
			if !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}
		})
		t.Run("RealWait", func(t *testing.T) {
			tt := fun.Must(NewQueue[int](QueueOptions{HardLimit: 2}))
			fun.Invariant(tt.BlockingAdd(ctx, 1) == nil)
			fun.Invariant(tt.BlockingAdd(ctx, 1) == nil)
			start := time.Now()
			sig := make(chan struct{})
			var end time.Time
			go func() {
				defer close(sig)
				defer func() { end = time.Now() }()
				if err := tt.BlockingAdd(ctx, 42); err != nil {
					t.Error(err)
				}
			}()
			time.Sleep(100 * time.Millisecond)
			sig2 := make(chan struct{})
			go func() {
				defer close(sig2)
				one, ok := tt.Remove()
				if !ok {
					t.Error("should have popped")
				}
				if one != 1 {
					t.Error(one)
				}
				one, ok = tt.Remove()
				if !ok {
					t.Error("should have popped")
				}
				if one != 1 {
					t.Error(one)
				}
			}()

			time.Sleep(100 * time.Millisecond)
			select {
			case <-sig2:
			case <-time.After(10 * time.Millisecond):
				t.Error("should not have timed out")
			}
			select {
			case <-time.After(10 * time.Millisecond):
				t.Error("should not have timed out")
			case <-sig:
				out, ok := tt.Remove()
				if !ok {
					t.Error("should have popped")
				}
				if out != 42 {
					t.Error(out)
				}
				if time.Since(end) < 20*time.Millisecond {
					t.Error(time.Since(end))
				}
				if time.Since(start)-time.Since(end) < 100*time.Millisecond {
					t.Error(time.Since(end) - time.Since(start))

				}
				return
			}
		})
	})
}
