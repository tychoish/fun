package pubsub

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

type ucBrokerKind struct {
	name string
	fifo bool
	mk   func(ctx context.Context, opts BrokerOptions) *Broker[int]
}

func ucBrokerKinds() []ucBrokerKind {
	return []ucBrokerKind{
		{"Plain", true, func(ctx context.Context, o BrokerOptions) *Broker[int] { return NewBroker[int](ctx, o) }},
		{"Queue", true, func(ctx context.Context, o BrokerOptions) *Broker[int] {
			return NewQueueBroker(ctx, NewUnlimitedQueue[int](), o)
		}},
		{"BoundedQueue", true, func(ctx context.Context, o BrokerOptions) *Broker[int] {
			q, _ := NewQueue[int](QueueOptions{HardLimit: 2})
			return NewQueueBroker(ctx, q, o)
		}},
		{"Deque", true, func(ctx context.Context, o BrokerOptions) *Broker[int] {
			return NewDequeBroker(ctx, &Deque[int]{}, o)
		}},
		{"LIFO", false, func(ctx context.Context, o BrokerOptions) *Broker[int] {
			return NewLIFOBroker(ctx, &Deque[int]{}, o)
		}},
	}
}

func ucSubscribe(t *testing.T, b *Broker[int]) chan int {
	t.Helper()
	ch, err := b.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

// ucCollect reads n values from ch (or fails after a 10s deadlock guard).
func ucCollect(t *testing.T, ch <-chan int, n int) []int {
	t.Helper()
	out := make([]int, 0, n)
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for len(out) < n {
		select {
		case v := <-ch:
			out = append(out, v)
		case <-timeout.C:
			t.Fatalf("deadlock: received %d of %d messages", len(out), n)
		}
	}
	return out
}

func ucSendAll(t *testing.T, b *Broker[int], n int) {
	t.Helper()
	for i := range n {
		if err := b.Send(t.Context(), i); err != nil {
			t.Errorf("send %d: %v", i, err)
			return
		}
	}
}

func TestUseCaseBrokerDeliversEverythingAcrossWorkerPoolSizes(t *testing.T) {
	const msgs = 150
	for _, kind := range ucBrokerKinds() {
		for _, workers := range []int{0, 1, 4} {
			for _, parallel := range []bool{false, true} {
				name := kind.name + "/workers" + string(rune('0'+workers))
				if parallel {
					name += "/parallel"
				}
				t.Run(name, func(t *testing.T) {
					defer testt.NoGoroutineLeak(t, 10*time.Second)()
					b := kind.mk(t.Context(), BrokerOptions{WorkerPoolSize: workers, ParallelDispatch: parallel})
					defer func() { b.Stop(); b.Wait(t.Context()) }()
					sub := ucSubscribe(t, b)

					go ucSendAll(t, b, msgs)
					got := ucCollect(t, sub, msgs)
					if kind.fifo && workers <= 1 && !slices.IsSorted(got) {
						t.Fatalf("single worker lost FIFO order: %v", got)
					}
					slices.Sort(got)
					for i, v := range got {
						if v != i {
							t.Fatalf("message %d missing or duplicated", i)
						}
					}
				})
			}
		}
	}
}

func TestUseCaseBrokerPerSubscriberOrdering(t *testing.T) {
	const msgs, subs = 100, 3
	for _, kind := range ucBrokerKinds() {
		if !kind.fifo {
			continue
		}
		for _, parallel := range []bool{false, true} {
			t.Run(kind.name+map[bool]string{false: "/sequential", true: "/parallel"}[parallel], func(t *testing.T) {
				defer testt.NoGoroutineLeak(t, 10*time.Second)()
				b := kind.mk(t.Context(), BrokerOptions{ParallelDispatch: parallel})
				defer func() { b.Stop(); b.Wait(t.Context()) }()

				chans := make([]chan int, subs)
				for i := range chans {
					chans[i] = ucSubscribe(t, b)
				}
				go ucSendAll(t, b, msgs)

				var wg sync.WaitGroup
				for i, ch := range chans {
					wg.Add(1)
					go func() {
						defer wg.Done()
						got := ucCollect(t, ch, msgs)
						for j, v := range got {
							if v != j {
								t.Errorf("subscriber %d: position %d holds %d", i, j, v)
								return
							}
						}
					}()
				}
				ucGuard(t, "subscribers", wg.Wait)
			})
		}
	}
}

func TestUseCaseBrokerSubscriptionBufferSizes(t *testing.T) {
	for _, size := range []int{0, 1, 3} {
		t.Run("Size"+string(rune('0'+size)), func(t *testing.T) {
			b := NewQueueBroker(t.Context(), NewUnlimitedQueue[int](), BrokerOptions{BufferSize: size})
			defer func() { b.Stop(); b.Wait(t.Context()) }()
			sub := ucSubscribe(t, b)
			if cap(sub) != size {
				t.Fatalf("subscription capacity %d, want %d", cap(sub), size)
			}
			// the broker's queue absorbs messages whatever the
			// subscriber buffer, so sends never wait for the reader.
			ucGuard(t, "sends", func() { ucSendAll(t, b, 5) })
			if got := ucCollect(t, sub, 5); !slices.Equal(got, []int{0, 1, 2, 3, 4}) {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestUseCaseBrokerManySubscribersEachGetEverything(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	const subs, msgs = 20, 30
	b := NewQueueBroker(t.Context(), NewUnlimitedQueue[int](), BrokerOptions{})
	defer func() { b.Stop(); b.Wait(t.Context()) }()
	seen := make(map[chan int]struct{})
	chans := make([]chan int, subs)
	for i := range chans {
		chans[i] = ucSubscribe(t, b)
		if _, dup := seen[chans[i]]; dup {
			t.Fatal("two subscriptions share a channel")
		}
		seen[chans[i]] = struct{}{}
	}
	go ucSendAll(t, b, msgs)
	var wg sync.WaitGroup
	for _, ch := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := ucCollect(t, ch, msgs)
			if !slices.IsSorted(got) || got[0] != 0 || got[msgs-1] != msgs-1 {
				t.Errorf("got %v", got)
			}
		}()
	}
	ucGuard(t, "subscribers", wg.Wait)
}

func TestUseCaseBrokerManyConcurrentSenders(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	const senders, per = 8, 50
	b := NewQueueBroker(t.Context(), NewUnlimitedQueue[int](), BrokerOptions{})
	defer func() { b.Stop(); b.Wait(t.Context()) }()
	sub := ucSubscribe(t, b)

	var wg sync.WaitGroup
	for s := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range per {
				if err := b.Send(t.Context(), s*1000+i); err != nil {
					t.Errorf("sender %d: %v", s, err)
					return
				}
			}
		}()
	}
	last := make([]int, senders)
	for i := range last {
		last[i] = -1
	}
	for _, v := range ucCollect(t, sub, senders*per) {
		s, i := v/1000, v%1000
		if i != last[s]+1 {
			t.Fatalf("sender %d: %d after %d (per-sender order lost)", s, i, last[s])
		}
		last[s] = i
	}
	ucGuard(t, "senders", wg.Wait)
	if st := b.Stats(t.Context()); st.MessageCount != senders*per {
		t.Fatalf("MessageCount %d, want %d", st.MessageCount, senders*per)
	}
}

func TestUseCaseBrokerSendWithNoSubscribersNeverBlocks(t *testing.T) {
	for _, kind := range ucBrokerKinds() {
		t.Run(kind.name, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := kind.mk(t.Context(), BrokerOptions{})
			defer func() { b.Stop(); b.Wait(t.Context()) }()
			ucGuard(t, "sends", func() { ucSendAll(t, b, 100) })
			// subscribing afterwards works: a late subscriber may see
			// backlog that was still queued, but it always sees the
			// new message, and (for FIFO brokers) in order.
			sub := ucSubscribe(t, b)
			// sent concurrently: an unbuffered broker may still be
			// handing backlog to this subscriber.
			sent := ucAsync(func() error { return b.Send(t.Context(), 1000) })
			defer func() {
				if err := ucRecv(t, sent, "Send"); err != nil {
					t.Error(err)
				}
			}()
			prev := -1
			for {
				v := ucCollect(t, sub, 1)[0]
				if kind.fifo && v <= prev {
					t.Fatalf("late subscriber saw %d after %d", v, prev)
				}
				if v == 1000 {
					return
				}
				prev = v
			}
		})
	}
}

func TestUseCaseBrokerChurnThenStop(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	for _, kind := range ucBrokerKinds() {
		t.Run(kind.name, func(t *testing.T) {
			b := kind.mk(t.Context(), BrokerOptions{BufferSize: 1})
			var senders, churners sync.WaitGroup
			stopSending := make(chan struct{})
			senders.Add(1)
			go func() {
				defer senders.Done()
				for i := 0; ; i++ {
					select {
					case <-stopSending:
						return
					default:
					}
					if err := b.Send(t.Context(), i); err != nil {
						return
					}
				}
			}()
			for range 4 {
				churners.Add(1)
				go func() {
					defer churners.Done()
					for range 25 {
						ch, err := b.Subscribe(t.Context())
						if err != nil {
							t.Errorf("subscribe: %v", err)
							return
						}
						select {
						case <-ch:
						default:
						}
						if err := b.Unsubscribe(t.Context(), ch); err != nil {
							t.Errorf("unsubscribe: %v", err)
							return
						}
					}
				}()
			}
			ucGuard(t, "churners", churners.Wait)
			close(stopSending)
			b.Stop()
			ucGuard(t, "senders", senders.Wait)
			ucGuard(t, "Wait", func() { b.Wait(t.Context()) })
			if st := b.Stats(t.Context()); st.State != BrokerStateClosed {
				t.Fatalf("state %v", st.State)
			}
		})
	}
}

func TestUseCaseBrokerStopAndWaitSemantics(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	b := NewQueueBroker(t.Context(), NewUnlimitedQueue[int](), BrokerOptions{WorkerPoolSize: 3})

	// Wait with an already-cancelled context returns even though the
	// broker is still running.
	cctx, cancel := context.WithCancel(t.Context())
	cancel()
	ucGuard(t, "Wait(cancelled)", func() { b.Wait(cctx) })
	if st := b.Stats(t.Context()); st.State == BrokerStateClosed {
		t.Fatal("a cancelled Wait stopped the broker")
	}

	b.Stop()
	b.Stop() // idempotent
	ucGuard(t, "Wait after Stop", func() { b.Wait(t.Context()) })
	ucGuard(t, "second Wait", func() { b.Wait(t.Context()) })
}

func TestUseCaseBrokerParentContextCancelClosesEverything(t *testing.T) {
	for _, kind := range ucBrokerKinds() {
		t.Run(kind.name, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			parent, cancel := context.WithCancel(t.Context())
			b := kind.mk(parent, BrokerOptions{})
			sub := ucSubscribe(t, b)
			cancel()
			ucGuard(t, "Wait", func() { b.Wait(t.Context()) })

			if err := b.Send(t.Context(), 1); !errors.Is(err, ErrBrokerClosed) {
				t.Fatalf("Send: %v", err)
			}
			if _, err := b.Subscribe(t.Context()); !errors.Is(err, ErrBrokerClosed) {
				t.Fatalf("Subscribe: %v", err)
			}
			if err := b.Unsubscribe(t.Context(), sub); !errors.Is(err, ErrBrokerClosed) {
				t.Fatalf("Unsubscribe: %v", err)
			}
			if st := b.Stats(t.Context()); st.State != BrokerStateClosed {
				t.Fatalf("state %v", st.State)
			}
		})
	}
}

func TestUseCaseBrokerUnsubscribeSemantics(t *testing.T) {
	b := NewQueueBroker(t.Context(), NewUnlimitedQueue[int](), BrokerOptions{BufferSize: 2})
	defer func() { b.Stop(); b.Wait(t.Context()) }()
	a, keep, c := ucSubscribe(t, b), ucSubscribe(t, b), ucSubscribe(t, b)
	if st := b.Stats(t.Context()); st.Subscriptions != 3 {
		t.Fatalf("subscriptions %d", st.Subscriptions)
	}

	if err := b.Unsubscribe(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	// unsubscribing twice, or something that never subscribed, is harmless.
	if err := b.Unsubscribe(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	if err := b.Unsubscribe(t.Context(), make(chan int)); err != nil {
		t.Fatal(err)
	}
	// Stats is ordered behind the control requests above.
	if st := b.Stats(t.Context()); st.Subscriptions != 2 {
		t.Fatalf("subscriptions %d after unsubscribe, want 2", st.Subscriptions)
	}

	if err := b.Send(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	if got := ucCollect(t, keep, 1); got[0] != 7 {
		t.Fatalf("got %v", got)
	}
	if got := ucCollect(t, c, 1); got[0] != 7 {
		t.Fatalf("got %v", got)
	}
	if len(a) != 0 {
		t.Fatal("an unsubscribed channel received a message")
	}
}

func TestUseCaseBrokerSendCancelledWhileBackPressured(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	q, err := NewQueue[int](QueueOptions{HardLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	b := NewQueueBroker(t.Context(), q, BrokerOptions{})
	defer func() { b.Stop(); b.Wait(t.Context()) }()
	stuck := ucSubscribe(t, b) // never read: stalls dispatch

	if err := b.Send(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	if err := b.Send(t.Context(), 2); err != nil {
		t.Fatal(err)
	}
	// message 1 is held by the stalled dispatch, 2 fills the queue.
	deadline := time.Now().Add(10 * time.Second)
	for b.Stats(t.Context()).BufferDepth != 1 {
		if time.Now().After(deadline) {
			t.Fatal("queue never filled")
		}
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithCancel(t.Context())
	res := ucAsync(func() error { return b.Send(ctx, 3) })
	cancel()
	if err := ucRecv(t, res, "back-pressured Send"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Send: %v", err)
	}

	// release the stall: the cancelled message must never be delivered.
	reader := ucSubscribe(t, b)
	if err := b.Unsubscribe(t.Context(), stuck); err != nil {
		t.Fatal(err)
	}
	seen2 := false
	for !seen2 {
		switch v := ucCollect(t, reader, 1)[0]; v {
		case 2:
			seen2 = true
		case 1: // in flight when the reader subscribed
		default:
			t.Fatalf("unexpected message %d", v)
		}
	}
	if st := b.Stats(t.Context()); st.BufferDepth != 0 {
		t.Fatalf("buffer depth %d, want 0: cancelled Send left a message behind", st.BufferDepth)
	}
}

func TestUseCaseBrokerStopReleasesBlockedSendAndSubscribe(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	q, err := NewQueue[int](QueueOptions{HardLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	b := NewQueueBroker(t.Context(), q, BrokerOptions{})
	_ = ucSubscribe(t, b) // stalls dispatch
	_ = b.Send(t.Context(), 1)
	_ = b.Send(t.Context(), 2)

	const senders = 5
	var results []<-chan error
	for range senders {
		results = append(results, ucAsync(func() error { return b.Send(t.Context(), 3) }))
	}
	b.Stop()
	for _, r := range results {
		err := ucRecv(t, r, "blocked Send")
		if !errors.Is(err, ErrBrokerClosed) && !errors.Is(err, ErrQueueFull) {
			t.Fatalf("blocked Send after Stop returned %v", err)
		}
	}
	ucGuard(t, "Wait", func() { b.Wait(t.Context()) })
}
