package pubsub

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
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
	if err := q.Push(item); err != nil {
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
	if err := q.Push("bar"); err != ErrQueueFull {
		t.Errorf("Add: got err=%v, want %v", err, ErrQueueFull)
	}
}

func TestQueueSoftQuota(t *testing.T) {
	q := mustQueue(t, QueueOptions{SoftQuota: 1, HardLimit: 4})
	q.mustAdd("foo")
	q.mustAdd("bar")
	if err := q.Push("baz"); err != ErrQueueNoCredit {
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
	if err := q.Push("foxtrot"); err == nil {
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
		if err := queue.Push("one"); err != nil {
			t.Fatal(err)
		}
		if err := queue.Push("two"); err != nil {
			t.Fatal(err)
		}
		if err := queue.Push("thr"); err != nil {
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
		if err := queue.Push("four"); err != nil {
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

		if err := queue.Push("one"); err != nil {
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
			if err := queue.Push(fmt.Sprint("item ", i)); err != nil {
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
			if err := queue.Push(fmt.Sprint("item ", i)); err != nil {
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

		if err = queue.Push("one"); err != nil {
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
		assert.NotError(t, queue.Push("one"))

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
			_ = queue.Push(31)
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

			tt := erc.Must(NewQueue[int](QueueOptions{HardLimit: 2}))

			assert.True(t, tt.WaitPush(ctx, 1) == nil)
			assert.True(t, tt.WaitPush(ctx, 1) == nil)

			canceled, trigger := context.WithCancel(context.Background())
			trigger()

			err := tt.WaitPush(canceled, 4)
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

			tt := erc.Must(NewQueue[int](QueueOptions{HardLimit: 2}))
			assert.True(t, tt.WaitPush(ctx, 1) == nil)
			assert.True(t, tt.WaitPush(ctx, 1) == nil)

			if err := tt.Close(); err != nil {
				t.Fatal(err)
			}

			err := tt.WaitPush(ctx, 4)
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

			tt := erc.Must(NewQueue[int](QueueOptions{HardLimit: 2}))
			assert.True(t, tt.WaitPush(ctx, 1) == nil)
			assert.True(t, tt.WaitPush(ctx, 1) == nil)
			start := time.Now()
			sig := make(chan struct{})
			var end time.Time
			go func() {
				defer close(sig)
				defer func() { end = time.Now() }()
				if err := tt.WaitPush(ctx, 42); err != nil {
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

		check.MinRuntime(t, 99*time.Millisecond, func() {
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
			ctx := t.Context()
			queue := NewUnlimitedQueue[string]()
			listener := IteratorStream(queue.IteratorWaitPop(ctx))
			assert.NotError(t, queue.Push("foo"))
			flag := &atomic.Int64{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				flag.Add(1)
				defer flag.Add(1)
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				check.NotError(t, queue.Shutdown(ctx))
			}()
			time.Sleep(time.Millisecond)
			assert.ErrorIs(t, queue.Push("bar"), ErrQueueDraining)
			assert.Equal(t, flag.Load(), 1)
			val, err := listener.Read(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, "foo", val)

			_, _ = listener.Read(t.Context())
			<-sig

			assert.Equal(t, flag.Load(), 2)
		})
		t.Run("Drain", func(t *testing.T) {
			ctx := t.Context()
			queue := NewUnlimitedQueue[string]()
			listener := IteratorStream(queue.IteratorWaitPop(ctx))
			assert.NotError(t, queue.Push("foo"))
			flag := &atomic.Int64{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				flag.Add(1)
				defer flag.Add(1)
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				check.NotError(t, queue.Shutdown(ctx))
			}()
			time.Sleep(time.Millisecond)
			assert.ErrorIs(t, queue.WaitPush(ctx, "bar"), ErrQueueDraining)
			assert.Equal(t, flag.Load(), 1)
			val, err := listener.Read(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, "foo", val)

			<-sig

			assert.Equal(t, flag.Load(), 2)
		})
		t.Run("ContextCancelation", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			cancel()

			queue := NewUnlimitedQueue[string]()
			listener := IteratorStream(queue.IteratorWaitPop(ctx))
			assert.NotError(t, queue.Push("foo"))
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
			check.Equal(t, flag.Load(), 2)
			val, err := listener.Read(ctx)
			check.Error(t, err)
			check.Zero(t, val)

			val, err = listener.Read(ctx)
			check.ErrorIs(t, err, context.Canceled)
			check.Zero(t, val)
			ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			check.Error(t, queue.Shutdown(ctx))
		})
	})
}

func TestQueueIteratorPop(t *testing.T) {
	t.Run("PopExistingItems", func(t *testing.T) {
		ctx := context.Background()
		queue := NewUnlimitedQueue[string]()

		check.NotError(t, queue.Push("one"))
		check.NotError(t, queue.Push("two"))
		check.NotError(t, queue.Push("three"))
		queue.Close()

		assert.Equal(t, queue.Len(), 3)

		values := irt.Collect(queue.IteratorWaitPop(ctx))
		assert.Equal(t, len(values), 3)
		assert.Equal(t, values[0], "one")
		assert.Equal(t, values[1], "two")
		assert.Equal(t, values[2], "three")
		assert.Equal(t, queue.Len(), 0)
	})

	t.Run("BlocksWaitingForItems", func(t *testing.T) {
		ctx := context.Background()
		queue := NewUnlimitedQueue[string]()

		iter := queue.IteratorWaitPop(ctx)
		sig := make(chan struct{})
		values := make([]string, 0)

		go func() {
			defer close(sig)
			for val := range iter {
				values = append(values, val)
				if len(values) >= 2 {
					break
				}
			}
		}()

		time.Sleep(10 * time.Millisecond)
		check.NotError(t, queue.Push("first"))
		time.Sleep(10 * time.Millisecond)
		check.NotError(t, queue.Push("second"))

		<-sig
		assert.Equal(t, len(values), 2)
		assert.Equal(t, values[0], "first")
		assert.Equal(t, values[1], "second")
		assert.Equal(t, queue.Len(), 0)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		queue := NewUnlimitedQueue[string]()

		check.NotError(t, queue.Push("item"))

		iter := queue.IteratorWaitPop(ctx)
		values := make([]string, 0)

		for val := range iter {
			values = append(values, val)
			cancel()
		}

		assert.Equal(t, len(values), 1)
		assert.Equal(t, values[0], "item")
		assert.Equal(t, queue.Len(), 0)
	})

	t.Run("ContextCancellationWhileWaiting", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		queue := NewUnlimitedQueue[string]()
		iter := queue.IteratorWaitPop(ctx)

		count := 0
		for range iter {
			count++
		}

		assert.Equal(t, count, 0)
		assert.Equal(t, queue.Len(), 0)
	})

	t.Run("QueueClosure", func(t *testing.T) {
		ctx := context.Background()
		queue := NewUnlimitedQueue[string]()

		check.NotError(t, queue.Push("one"))
		check.NotError(t, queue.Push("two"))

		iter := queue.IteratorWaitPop(ctx)
		values := make([]string, 0)

		go func() {
			time.Sleep(20 * time.Millisecond)
			queue.Close()
		}()

		for val := range iter {
			values = append(values, val)
			time.Sleep(10 * time.Millisecond)
		}

		assert.Equal(t, len(values), 2)
		assert.Equal(t, values[0], "one")
		assert.Equal(t, values[1], "two")
		assert.Equal(t, queue.Len(), 0)
	})

	t.Run("EmptyQueueClosed", func(t *testing.T) {
		ctx := context.Background()
		queue := NewUnlimitedQueue[string]()
		queue.Close()

		iter := queue.IteratorWaitPop(ctx)
		count := 0
		for range iter {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("RemovesItemsFromQueue", func(t *testing.T) {
		ctx := context.Background()
		queue := NewUnlimitedQueue[int]()

		for i := 0; i < 10; i++ {
			check.NotError(t, queue.Push(i))
		}
		assert.Equal(t, queue.Len(), 10)

		iter := queue.IteratorWaitPop(ctx)
		count := 0
		for range iter {
			count++
			if count >= 5 {
				break
			}
		}

		assert.Equal(t, count, 5)
		assert.Equal(t, queue.Len(), 5)
	})

	t.Run("ConcurrentProducerConsumer", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		queue := NewUnlimitedQueue[int]()
		consumed := &atomic.Int64{}

		go func() {
			iter := queue.IteratorWaitPop(ctx)
			for range iter {
				consumed.Add(1)
			}
		}()

		time.Sleep(10 * time.Millisecond)

		for i := 0; i < 20; i++ {
			check.NotError(t, queue.Push(i))
			time.Sleep(5 * time.Millisecond)
		}

		<-ctx.Done()
		assert.True(t, consumed.Load() >= 10)
		assert.True(t, queue.Len() < 10)
	})
}

func TestQueueDrain(t *testing.T) {
	t.Run("EmptyQueueDrainsImmediately", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
		queue := NewUnlimitedQueue[int]()

		start := time.Now()
		err := queue.Drain(ctx)
		duration := time.Since(start)

		assert.NotError(t, err)
		assert.True(t, duration < 50*time.Millisecond)
		assert.Equal(t, 0, queue.Len())
	})

	t.Run("BlocksUntilQueueEmpty", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 2*time.Second)
		queue := NewUnlimitedQueue[string]()

		// Add items to queue
		assert.NotError(t, queue.Push("item1"))
		assert.NotError(t, queue.Push("item2"))
		assert.NotError(t, queue.Push("item3"))
		assert.Equal(t, 3, queue.Len())

		drainComplete := &atomic.Bool{}
		drainErr := make(chan error, 1)

		// Start draining in background
		go func() {
			err := queue.Drain(ctx)
			drainComplete.Store(true)
			drainErr <- err
		}()

		// Give drain a moment to start
		time.Sleep(10 * time.Millisecond)

		// Drain should be waiting
		assert.True(t, !drainComplete.Load())
		assert.Equal(t, 3, queue.Len())

		// Remove items one by one
		val, ok := queue.Remove()
		assert.True(t, ok)
		assert.Equal(t, "item1", val)
		time.Sleep(5 * time.Millisecond)
		assert.True(t, !drainComplete.Load())

		val, ok = queue.Remove()
		assert.True(t, ok)
		assert.Equal(t, "item2", val)
		time.Sleep(5 * time.Millisecond)
		assert.True(t, !drainComplete.Load())

		// Remove last item - drain should complete
		val, ok = queue.Remove()
		assert.True(t, ok)
		assert.Equal(t, "item3", val)

		// Wait for drain to complete
		select {
		case err := <-drainErr:
			assert.NotError(t, err)
			assert.True(t, drainComplete.Load())
			assert.Equal(t, 0, queue.Len())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("drain did not complete after queue was emptied")
		}
	})

	t.Run("PreventsNewAddsWhileDraining", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 2*time.Second)
		queue := NewUnlimitedQueue[int]()

		// Add items
		assert.NotError(t, queue.Push(1))
		assert.NotError(t, queue.Push(2))

		drainStarted := make(chan struct{})
		drainErr := make(chan error, 1)

		// Start draining
		go func() {
			close(drainStarted)
			drainErr <- queue.Drain(ctx)
		}()

		<-drainStarted
		time.Sleep(10 * time.Millisecond)

		// Try to add while draining - should fail
		err := queue.Push(3)
		assert.ErrorIs(t, err, ErrQueueDraining)

		// Try BlockingAdd while draining - should also fail
		blockCtx := testt.ContextWithTimeout(t, 50*time.Millisecond)
		err = queue.WaitPush(blockCtx, 4)
		assert.ErrorIs(t, err, ErrQueueDraining)

		// Empty the queue to complete drain
		queue.Remove()
		queue.Remove()

		// Wait for drain to complete
		select {
		case err := <-drainErr:
			assert.NotError(t, err)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("drain did not complete")
		}
	})

	t.Run("QueueNotClosedAfterDrain", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 1*time.Second)
		queue := NewUnlimitedQueue[string]()

		// Add and drain
		assert.NotError(t, queue.Push("first"))
		assert.NotError(t, queue.Push("second"))

		drainComplete := make(chan error, 1)
		go func() {
			drainComplete <- queue.Drain(ctx)
		}()

		// Remove items to complete drain
		time.Sleep(10 * time.Millisecond)
		queue.Remove()
		queue.Remove()

		// Wait for drain
		err := <-drainComplete
		assert.NotError(t, err)

		// After drain, queue should accept new items (not closed)
		err = queue.Push("after-drain-1")
		assert.NotError(t, err)

		err = queue.Push("after-drain-2")
		assert.NotError(t, err)

		assert.Equal(t, 2, queue.Len())

		// Can remove items
		val, ok := queue.Remove()
		assert.True(t, ok)
		assert.Equal(t, "after-drain-1", val)

		val, ok = queue.Remove()
		assert.True(t, ok)
		assert.Equal(t, "after-drain-2", val)
	})

	t.Run("ContextCancellationDuringDrain", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		queue := NewUnlimitedQueue[int]()

		// Add items that won't be removed
		assert.NotError(t, queue.Push(1))
		assert.NotError(t, queue.Push(2))
		assert.NotError(t, queue.Push(3))

		drainErr := make(chan error, 1)
		go func() {
			drainErr <- queue.Drain(ctx)
		}()

		// Give drain time to start waiting
		time.Sleep(20 * time.Millisecond)

		// Cancel context
		cancel()

		// Drain should return with error
		select {
		case err := <-drainErr:
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			// Queue should still have items
			assert.True(t, queue.Len() > 0)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("drain did not return after context cancellation")
		}

		// After failed drain, queue should accept new adds again
		freshCtx := testt.ContextWithTimeout(t, 100*time.Millisecond)
		err := queue.WaitPush(freshCtx, 4)
		assert.NotError(t, err)
	})

	t.Run("ConcurrentDrainAndConsume", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 2*time.Second)
		queue := NewUnlimitedQueue[int]()

		// Add many items
		for i := 0; i < 100; i++ {
			assert.NotError(t, queue.Push(i))
		}

		drainErr := make(chan error, 1)
		consumed := &atomic.Int32{}

		// Start draining
		go func() {
			drainErr <- queue.Drain(ctx)
		}()

		// Concurrently consume items
		go func() {
			for queue.Len() > 0 || consumed.Load() < 100 {
				if val, ok := queue.Remove(); ok {
					consumed.Add(1)
					_ = val
					time.Sleep(time.Millisecond)
				} else {
					time.Sleep(time.Millisecond)
				}
			}
		}()

		// Drain should complete once all items consumed
		select {
		case err := <-drainErr:
			assert.NotError(t, err)
			assert.Equal(t, int32(100), consumed.Load())
			assert.Equal(t, 0, queue.Len())
		case <-time.After(3 * time.Second):
			t.Fatal("drain did not complete")
		}
	})

	t.Run("MultipleDrainCalls", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 2*time.Second)
		queue := NewUnlimitedQueue[string]()

		// First drain on empty queue
		err := queue.Drain(ctx)
		assert.NotError(t, err)

		// Add items
		assert.NotError(t, queue.Push("a"))
		assert.NotError(t, queue.Push("b"))

		drainErr := make(chan error, 1)
		go func() {
			drainErr <- queue.Drain(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		// Empty queue
		queue.Remove()
		queue.Remove()

		err = <-drainErr
		assert.NotError(t, err)

		// Third drain on empty queue
		err = queue.Drain(ctx)
		assert.NotError(t, err)
	})

	t.Run("DrainWithWait", func(t *testing.T) {
		ctx := testt.ContextWithTimeout(t, 2*time.Second)
		queue := NewUnlimitedQueue[int]()

		// Add items
		assert.NotError(t, queue.Push(10))
		assert.NotError(t, queue.Push(20))

		drainErr := make(chan error, 1)
		go func() {
			drainErr <- queue.Drain(ctx)
		}()

		time.Sleep(10 * time.Millisecond)

		// Use Wait() to consume items
		val, err := queue.Wait(ctx)
		assert.NotError(t, err)
		assert.Equal(t, 10, val)

		val, err = queue.Wait(ctx)
		assert.NotError(t, err)
		assert.Equal(t, 20, val)

		// Drain should complete
		select {
		case err := <-drainErr:
			assert.NotError(t, err)
			assert.Equal(t, 0, queue.Len())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("drain did not complete")
		}
	})
}
