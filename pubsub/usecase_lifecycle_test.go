package pubsub

import (
	"context"
	"errors"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

// ucBox is the common FIFO surface of a Queue and a Deque so that the
// same lifecycle scenario runs against every container.
type ucBox interface {
	Push(int) error
	WaitPush(context.Context, int) error
	Pop() (int, bool)
	WaitPop(context.Context) (int, error)
	Drain(context.Context) error
	Shutdown(context.Context) error
	Close() error
	Len() int
	Waiting(context.Context) iter.Seq[int] // non-destructive, blocking
	Popping(context.Context) iter.Seq[int] // destructive, blocking
}

type ucQueue struct{ *Queue[int] }

func (q ucQueue) Waiting(ctx context.Context) iter.Seq[int] { return q.IteratorWait(ctx) }
func (q ucQueue) Popping(ctx context.Context) iter.Seq[int] { return q.IteratorWaitPop(ctx) }

type ucDeque struct{ *Deque[int] }

func (d ucDeque) Push(v int) error                        { return d.PushBack(v) }
func (d ucDeque) WaitPush(c context.Context, v int) error { return d.WaitPushBack(c, v) }
func (d ucDeque) Pop() (int, bool)                        { return d.PopFront() }
func (d ucDeque) WaitPop(c context.Context) (int, error)  { return d.WaitPopFront(c) }
func (d ucDeque) Waiting(c context.Context) iter.Seq[int] { return d.IteratorWaitFront(c) }
func (d ucDeque) Popping(c context.Context) iter.Seq[int] { return d.IteratorWaitPopFront(c) }

// ucBoxes builds one FIFO container of each kind with the given
// hard capacity (0 means unbounded).
func ucBoxes(t *testing.T, capacity int) map[string]func() ucBox {
	t.Helper()
	return map[string]func() ucBox{
		"Queue": func() ucBox {
			if capacity == 0 {
				return ucQueue{NewUnlimitedQueue[int]()}
			}
			q, err := NewQueue[int](QueueOptions{HardLimit: capacity})
			if err != nil {
				t.Fatal(err)
			}
			return ucQueue{q}
		},
		"Deque": func() ucBox {
			dq, err := NewDeque[int](DequeOptions{Capacity: capacity})
			if err != nil {
				t.Fatal(err)
			}
			return ucDeque{dq}
		},
	}
}

// ucGuard fails the test if fn does not return within 10s. It is a
// deadlock guard, not a timing assertion.
func ucGuard(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("deadlock: %s did not return", what)
	}
}

// ucAsync starts fn in a goroutine and returns a channel that
// receives its result.
func ucAsync[T any](fn func() T) <-chan T {
	out := make(chan T, 1)
	go func() { out <- fn() }()
	return out
}

func ucRecv[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("deadlock: %s did not complete", what)
		panic("unreachable")
	}
}

func TestUseCaseContainerCloseReleasesEveryWaiter(t *testing.T) {
	for name, mk := range ucBoxes(t, 0) {
		t.Run(name+"/WaitPop", func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := mk()
			const waiters = 8
			results := make([]<-chan error, waiters)
			for i := range results {
				results[i] = ucAsync(func() error { _, err := b.WaitPop(t.Context()); return err })
			}
			ucGuard(t, "Close", func() { _ = b.Close() })
			for i, r := range results {
				if err := ucRecv(t, r, "WaitPop waiter"); !errors.Is(err, ErrQueueClosed) {
					t.Fatalf("waiter %d: %v", i, err)
				}
			}
		})
		t.Run(name+"/Iterators", func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := mk()
			const waiters = 6
			var results []<-chan int
			for range waiters {
				results = append(results, ucAsync(func() int {
					n := 0
					for range b.Popping(t.Context()) {
						n++
					}
					return n
				}))
				results = append(results, ucAsync(func() int {
					n := 0
					for range b.Waiting(t.Context()) {
						n++
					}
					return n
				}))
			}
			ucGuard(t, "Close", func() { _ = b.Close() })
			for _, r := range results {
				if n := ucRecv(t, r, "blocked iterator"); n != 0 {
					t.Fatalf("iterator over empty closed container yielded %d", n)
				}
			}
		})
	}
}

func TestUseCaseContainerCancelReleasesEveryWaiter(t *testing.T) {
	for name, mk := range ucBoxes(t, 1) {
		t.Run(name, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := mk()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var results []<-chan error
			// full container: pushers block.
			if err := b.Push(0); err != nil {
				t.Fatal(err)
			}
			for range 4 {
				results = append(results, ucAsync(func() error { return b.WaitPush(ctx, 1) }))
			}
			cancel()
			for _, r := range results {
				if err := ucRecv(t, r, "WaitPush"); !errors.Is(err, context.Canceled) {
					t.Fatalf("WaitPush: %v", err)
				}
			}
			if b.Len() != 1 {
				t.Fatalf("cancelled pushers changed the length: %d", b.Len())
			}

			// empty container: poppers block.
			b = mk()
			ctx2, cancel2 := context.WithCancel(t.Context())
			defer cancel2()
			results = nil
			for range 4 {
				results = append(results, ucAsync(func() error { _, err := b.WaitPop(ctx2); return err }))
			}
			cancel2()
			for _, r := range results {
				if err := ucRecv(t, r, "WaitPop"); !errors.Is(err, context.Canceled) {
					t.Fatalf("WaitPop: %v", err)
				}
			}
			// the container is still usable after the waiters left.
			if err := b.Push(7); err != nil {
				t.Fatal(err)
			}
			if v, err := b.WaitPop(t.Context()); err != nil || v != 7 {
				t.Fatalf("got %d, %v", v, err)
			}
		})
	}
}

func TestUseCaseContainerCancelledWaitOpsConsumeConsistently(t *testing.T) {
	for name, mk := range ucBoxes(t, 4) {
		t.Run(name, func(t *testing.T) {
			b := mk()
			for i := range 3 {
				if err := b.Push(i); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			// Whatever a cancelled WaitPop decides, it must never
			// consume an item and also report an error.
			before := b.Len()
			v, err := b.WaitPop(ctx)
			want := before
			if err == nil {
				want--
				if v != 0 {
					t.Fatalf("WaitPop returned %d, want the oldest item", v)
				}
			}
			if b.Len() != want {
				t.Fatalf("WaitPop err=%v len %d -> %d", err, before, b.Len())
			}

			// same for a cancelled WaitPush on a container with room.
			before = b.Len()
			err = b.WaitPush(ctx, 99)
			want = before
			if err == nil {
				want++
			}
			if b.Len() != want {
				t.Fatalf("WaitPush err=%v len %d -> %d", err, before, b.Len())
			}

			// the iterators never yield for a cancelled context.
			before = b.Len()
			for range b.Popping(ctx) {
				t.Fatal("pop iterator yielded for a cancelled context")
			}
			for range b.Waiting(ctx) {
				t.Fatal("wait iterator yielded for a cancelled context")
			}
			if b.Len() != before {
				t.Fatalf("cancelled iterators changed length %d -> %d", before, b.Len())
			}
		})
	}
}

func TestUseCaseContainerReuseAfterClose(t *testing.T) {
	for name, mk := range ucBoxes(t, 4) {
		t.Run(name, func(t *testing.T) {
			b := mk()
			_ = b.Push(1)
			_ = b.Push(2)
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			// closing is idempotent and the backlog is retained.
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			if b.Len() != 2 {
				t.Fatalf("Close changed the length: %d", b.Len())
			}
			if err := b.Push(3); !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("Push: %v", err)
			}
			if err := b.WaitPush(t.Context(), 3); !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("WaitPush: %v", err)
			}
			if _, ok := b.Pop(); ok {
				t.Fatal("Pop succeeded on a closed container")
			}
			if _, err := b.WaitPop(t.Context()); !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("WaitPop: %v", err)
			}
			if err := b.Drain(t.Context()); !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("Drain with a backlog: %v", err)
			}
			if err := b.Shutdown(t.Context()); !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("Shutdown with a backlog: %v", err)
			}
			for range b.Popping(t.Context()) {
				t.Fatal("pop iterator yielded on a closed container")
			}
			if b.Len() != 2 {
				t.Fatalf("operations on a closed container changed the length: %d", b.Len())
			}
		})
	}
}

func TestUseCaseContainerShutdownAfterShutdown(t *testing.T) {
	for name, mk := range ucBoxes(t, 0) {
		t.Run(name, func(t *testing.T) {
			b := mk()
			ucGuard(t, "first Shutdown", func() {
				if err := b.Shutdown(t.Context()); err != nil {
					t.Error(err)
				}
			})
			ucGuard(t, "second Shutdown", func() {
				if err := b.Shutdown(t.Context()); !errors.Is(err, ErrQueueClosed) {
					t.Errorf("second Shutdown on an empty closed container: got %v, want ErrQueueClosed", err)
				}
			})
			if err := b.Drain(t.Context()); !errors.Is(err, ErrQueueClosed) {
				t.Fatalf("Drain of empty closed container: got %v, want ErrQueueClosed", err)
			}
		})
	}
}

func TestUseCaseContainerConcurrentShutdownCallers(t *testing.T) {
	for name, mk := range ucBoxes(t, 0) {
		t.Run(name, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := mk()
			const items = 5
			for i := range items {
				if err := b.Push(i); err != nil {
					t.Fatal(err)
				}
			}
			results := []<-chan error{
				ucAsync(func() error { return b.Shutdown(t.Context()) }),
				ucAsync(func() error { return b.Shutdown(t.Context()) }),
				ucAsync(func() error { return b.Drain(t.Context()) }),
			}
			var got []int
			for len(got) < items {
				v, err := b.WaitPop(t.Context())
				if err != nil {
					t.Fatalf("consumer: %v after %v", err, got)
				}
				got = append(got, v)
			}
			if !slices.IsSorted(got) {
				t.Fatalf("FIFO order lost: %v", got)
			}
			// Shutdown/Drain are not synchronized to start together, so a
			// caller that reaches the container after a sibling has
			// already drained and closed it legitimately observes
			// ErrQueueClosed (closed is reported before ctx and before
			// any wait, per the closed-first precedence rule); a caller
			// that starts while the container is still open and only
			// racing the drain itself completes with nil.
			for _, r := range results {
				if err := ucRecv(t, r, "Shutdown/Drain caller"); err != nil && !errors.Is(err, ErrQueueClosed) {
					t.Fatalf("a drainer reported %v after the container emptied", err)
				}
			}
		})
	}
}

func TestUseCaseContainerCancelledDrainLeavesContainerUsable(t *testing.T) {
	for name, mk := range ucBoxes(t, 0) {
		t.Run(name, func(t *testing.T) {
			b := mk()
			_ = b.Push(1)
			ctx, cancel := context.WithCancel(t.Context())
			res := ucAsync(func() error { return b.Drain(ctx) })
			cancel()
			if err := ucRecv(t, res, "Drain"); !errors.Is(err, context.Canceled) {
				t.Fatalf("Drain: %v", err)
			}
			if b.Len() != 1 {
				t.Fatalf("cancelled Drain removed items: %d", b.Len())
			}
			// not closed, not draining: pushes succeed again.
			if err := b.Push(2); err != nil {
				t.Fatalf("Push after a cancelled Drain: %v", err)
			}

			// same for a cancelled Shutdown: it must not close.
			ctx2, cancel2 := context.WithCancel(t.Context())
			res = ucAsync(func() error { return b.Shutdown(ctx2) })
			cancel2()
			if err := ucRecv(t, res, "Shutdown"); !errors.Is(err, context.Canceled) {
				t.Fatalf("Shutdown: %v", err)
			}
			if err := b.Push(3); err != nil {
				t.Fatalf("Push after a cancelled Shutdown: %v", err)
			}
			if b.Len() != 3 {
				t.Fatalf("length %d, want 3", b.Len())
			}
		})
	}
}
