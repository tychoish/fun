package pubsub

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/risky"
	"github.com/tychoish/fun/testt"
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
	*Queue[string]

	t *testing.T
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
	if lenCap := float64(getTracker().hardLimit - getTracker().softQuota); getTracker().credit > lenCap {
		t.Errorf("Wrong credit: got %f, want ≤ %f", getTracker().credit, lenCap)
	}
}

func TestQueueClose(t *testing.T) {
	q := mustQueue(t, QueueOptions{SoftQuota: 2, HardLimit: 10})
	q.mustAdd("alpha")
	q.mustAdd("bravo")
	q.mustAdd("charlie")
	assert.NotError(t, q.Close())

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
	t.Parallel()

	q := mustQueue(t, QueueOptions{SoftQuota: 2, HardLimit: 2})

	// A wait on an empty queue should time out.
	t.Run("WaitTimeout", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
		got, err := q.Wait(ctx)
		if err == nil {
			t.Errorf("Wait: got %v, want error", got)
		} else if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Wait should have encountered a timeout, but got: %v", err)
		}
	})

	// A wait on a non-empty queue should report an item.
	t.Run("WaitNonEmpty", func(t *testing.T) {
		ctx := testt.Context(t)

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
		ctx := testt.Context(t)
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
		ctx := testt.Context(t)

		done := make(chan struct{})
		go func() {
			defer close(done)
			got, err := q.Wait(ctx)
			if err != ErrQueueClosed {
				t.Errorf("Wait: got (%v, %v), want %v", got, err, ErrQueueClosed)
			}
		}()

		assert.NotError(t, q.Close())
		<-done
	})
}

func TestQueueStream(t *testing.T) {
	t.Parallel()

	t.Run("EndToEnd", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
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

		if queue.Len() != 3 {
			t.Fatal("unexpected queue length", queue.Len())
		}

		values := irt.Collect(queue.Iterator())
		if len(values) != 3 {
			t.Log(values)
			t.Fatal("unexpected length")
		}

		if val := values[0]; val != "one" {
			t.Fatalf("unexpected value %q", val)
		}
		if val := values[1]; val != "two" {
			t.Fatal("unexpected value", val)
		}
		if val := values[2]; val != "thr" {
			t.Fatal("unexpected value", val)
		}

		startAt := time.Now()
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 400*time.Millisecond)
		defer timeoutCancel()
		iter := queue.IteratorWait(timeoutCtx)
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			start := time.Now()
			t.Log("started iteration check")
			count := 0
			for val := range iter {
				t.Log("saw:", val)
				count++
				break
			}
			if count != 1 {
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
	})
	t.Run("EndToEndStartEmpty", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}

		startAt := time.Now()
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, 400*time.Millisecond)
		iter := queue.IteratorWait(timeoutCtx)
		defer timeoutCancel()
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			start := time.Now()
			for range iter {
				return
			}
			t.Error("expected to eventually be true", time.Since(start))
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
	})
	t.Run("ClosedQueueIteratesAndDoesNotBlock", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		queue := NewUnlimitedQueue[string]()

		for i := 0; i < 100; i++ {
			if err := queue.Add(fmt.Sprint("item ", i)); err != nil {
				t.Fatal(err)
			}
		}
		count := 0
		iter := queue.IteratorWait(ctx)
		for range iter {
			count++
			if count == 100 {
				break
			}
		}
		if count != 100 {
			t.Error("count should be 100: ", count)
		}
	})
	t.Run("ClosedStreamDoesNotBlock", func(t *testing.T) {
		ctx := testt.Context(t)
		queue := NewUnlimitedQueue[string]()

		for i := 0; i < 100; i++ {
			if err := queue.Add(fmt.Sprint("item ", i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := queue.Close(); err != nil {
			t.Error(err)
		}

		count := 0
		for range queue.IteratorWait(ctx) {
			count++
			if count == 100 {
				break
			}
		}
		if count != 100 {
			t.Error("count should be 100: ", count)
		}
	})
	t.Run("CanceledDoesNotIterate", func(t *testing.T) {
		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		if err != nil {
			t.Fatal(err)
		}

		if err = queue.Add("one"); err != nil {
			t.Fatal(err)
		}

		count := 0
		for range queue.Iterator() {
			count++
		}
		if count != 1 {
			t.Error("should have one item", count)
		}

		count = 0
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		for range queue.IteratorWait(ctx) {
			count++
		}
		if count != 0 {
			t.Fatal("should not iterate", count)
		}

		val, err := queue.Wait(t.Context())
		assert.NotError(t, err)
		check.Equal(t, val, "one")
		for range queue.IteratorWait(ctx) {
			count++
		}
		if count != 0 {
			t.Fatal("should not iterate", count)
		}
	})
	t.Run("ClosedQueueDoesNotIterate", func(t *testing.T) {
		ctx := testt.Context(t)
		queue, err := NewQueue[string](QueueOptions{HardLimit: 5, SoftQuota: 3})
		assert.NotError(t, err)
		assert.NotError(t, queue.Add("one"))

		count := 0
		for range queue.Iterator() {
			count++
		}
		if count != 1 {
			t.Error("should iterate at least once", count)
		}

		check.NotError(t, queue.Close())
		count = 0
		for range queue.IteratorWait(ctx) {
			count++
		}
		if count != 1 {
			t.Error("should only once")
		}
	})
	t.Run("WaitRespectsQueue", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		queue := NewUnlimitedQueue[int]()
		sig := make(chan struct{})
		go func() {
			defer close(sig)

			if err := queue.waitForNew(ctx); !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		}()
		sa := time.Now()
		cancel()
		runtime.Gosched()
		<-sig
		if dur := time.Since(sa); dur > 10*time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("StreamRetrySpecialCase", func(t *testing.T) {
		ctx := testt.Context(t)
		queue := NewUnlimitedQueue[int]()
		toctx, toccancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer toccancel()
		iter := queue.IteratorWait(toctx)
		sa := time.Now()
		count := 0
		for range iter {
			count++
		}
		go func() {
			time.Sleep(5 * time.Millisecond)
			_ = queue.Add(31)
		}()
		time.Sleep(20 * time.Millisecond)
		assert.Zero(t, count)
		for range queue.IteratorWait(ctx) {
			count++
			break
		}
		assert.Equal(t, count, 1)
		assert.True(t, time.Since(sa) >= 10*time.Millisecond)
	})

	t.Run("WaitAdd", func(t *testing.T) {
		t.Parallel()
		t.Run("ContextCanceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tt := risky.Force(NewQueue[int](QueueOptions{HardLimit: 2}))

			fun.Invariant.Ok(tt.BlockingAdd(ctx, 1) == nil)
			fun.Invariant.Ok(tt.BlockingAdd(ctx, 1) == nil)

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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tt := risky.Force(NewQueue[int](QueueOptions{HardLimit: 2}))
			fun.Invariant.Ok(tt.BlockingAdd(ctx, 1) == nil)
			fun.Invariant.Ok(tt.BlockingAdd(ctx, 1) == nil)

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
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tt := risky.Force(NewQueue[int](QueueOptions{HardLimit: 2}))
			fun.Invariant.Ok(tt.BlockingAdd(ctx, 1) == nil)
			fun.Invariant.Ok(tt.BlockingAdd(ctx, 1) == nil)
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
	t.Run("WaitOnEmpty", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
		queue := NewUnlimitedQueue[string]()
		count := 0

		check.MinRuntime(t, 100*time.Millisecond, func() {
			count++
			for range queue.IteratorWait(ctx) {
				count++
			}
			count++
		})

		check.Equal(t, 2, count)
	})
	t.Run("WaitOnClosed", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
		queue := NewUnlimitedQueue[string]()
		sig := make(chan struct{})
		iter := queue.IteratorWait(ctx)
		go func() {
			defer close(sig)
			count := 0
			for range iter {
				count++
			}
			check.Zero(t, count)
		}()
		time.Sleep(10 * time.Millisecond)
		assert.NotError(t, queue.Close())
	})
	t.Run("DrainAndShutdown", func(t *testing.T) {
		t.Run("Shutdown", func(t *testing.T) {
			queue := NewUnlimitedQueue[string]()
			listener := fun.InterfaceStream(queue.Distributor())
			assert.NotError(t, queue.Add("foo"))
			flag := &atomic.Int64{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				flag.Add(1)
				defer flag.Add(1)
				check.NotError(t, queue.Shutdown(t.Context()))
			}()
			time.Sleep(time.Millisecond)
			assert.ErrorIs(t, queue.Add("bar"), ErrQueueDraining)
			assert.Equal(t, flag.Load(), 1)
			val, err := listener.Read(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, "foo", val)

			_, _ = listener.Read(t.Context())
			<-sig

			assert.Equal(t, flag.Load(), 2)
		})
		t.Run("Drain", func(t *testing.T) {
			queue := NewUnlimitedQueue[string]()
			listener := fun.InterfaceStream(queue.Distributor())
			assert.NotError(t, queue.Add("foo"))
			flag := &atomic.Int64{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				flag.Add(1)
				defer flag.Add(1)
				check.NotError(t, queue.Drain(t.Context()))
			}()
			time.Sleep(time.Millisecond)
			assert.ErrorIs(t, queue.BlockingAdd(t.Context(), "bar"), ErrQueueDraining)
			assert.Equal(t, flag.Load(), 1)
			val, err := listener.Read(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, "foo", val)

			<-sig

			assert.Equal(t, flag.Load(), 2)
		})
		t.Run("ContextCancelation", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			queue := NewUnlimitedQueue[string]()
			listener := fun.InterfaceStream(queue.Distributor())
			assert.NotError(t, queue.Add("foo"))
			flag := &atomic.Int64{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				flag.Add(1)
				defer flag.Add(1)
				err := queue.Shutdown(ctx)
				check.Error(t, err)
				check.ErrorIs(t, err, context.Canceled)
			}()
			time.Sleep(10 * time.Millisecond)

			assert.Equal(t, flag.Load(), 2)
			val, err := listener.Read(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, val, "foo")
			check.NotError(t, queue.Shutdown(t.Context()))
		})
	})
}
