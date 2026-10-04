package pubsub

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

type BrokerFixture[T comparable] struct {
	Name       string
	Construtor func(ctx context.Context, t *testing.T) *Broker[T]
	BufferSize int
}

func GenerateFixtures[T comparable](axis string, elems []T, opts BrokerOptions) iter.Seq[BrokerFixture[T]] {
	return irt.Slice([]BrokerFixture[T]{
		{
			Name: fmt.Sprintf("Channel/ZeroBuffer/%s", axis),
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, opts)
			},
		},
		//
		// queue cases
		//
		{
			Name: fmt.Sprintf("Queue/Unlimited/%s", axis),
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				return NewQueueBroker(ctx, NewUnlimitedQueue[T](), opts)
			},
		},

		{
			Name: fmt.Sprintf("Queue/Limit16/%s", axis),
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   16,
					SoftQuota:   8,
					BurstCredit: 4,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker(ctx, queue, opts)
			},
		},
		//
		// deque cases
		//
		{
			Name: fmt.Sprintf("Deque/FIFO/Unlimited/%s", axis),
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewDequeBroker(
					ctx,
					NewUnlimitedDeque[T](),
					opts,
				)
			},
		},
		{
			Name: fmt.Sprintf("Deque/LIFO/Unlimited/%s", axis),
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				return NewLIFOBroker(
					ctx,
					NewUnlimitedDeque[T](),
					opts,
				)
			},
		},
		{
			Name: fmt.Sprintf("Deque/FIFO/Limit16/%s", axis),
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewDequeBroker(
					ctx,
					erc.Must(NewDeque[T](DequeOptions{
						Capacity: 16,
					})),
					opts,
				)
			},
		},
		{
			Name: fmt.Sprintf("Deque/LIFO/Limit16/%s", axis),
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				return NewLIFOBroker(
					ctx,
					erc.Must(NewDeque[T](DequeOptions{
						Capacity: 16,
					})),
					opts,
				)
			},
		},
	})
}

func makeFixtures[T comparable](elems []T) iter.Seq[BrokerFixture[T]] {
	return irt.Chain(
		irt.Args(
			GenerateFixtures("Serial", elems, BrokerOptions{ParallelDispatch: false}),
			GenerateFixtures("Parallel", elems, BrokerOptions{ParallelDispatch: true}),
			GenerateFixtures("Serial/Worker8", elems, BrokerOptions{ParallelDispatch: false, WorkerPoolSize: 8}),
			GenerateFixtures("Parallel/Worker8", elems, BrokerOptions{ParallelDispatch: true, WorkerPoolSize: 8}),
		),
	)
}

func RunBrokerTests[T comparable](pctx context.Context, t *testing.T, elems []T) {
	for fix := range makeFixtures(elems) {
		t.Run(fix.Name, func(t *testing.T) {
			opts := fix

			t.Parallel()
			ctx, cancel := context.WithTimeout(pctx, time.Second)
			defer cancel()

			broker := opts.Construtor(ctx, t)

			ch1 := mustSubscribe(t, broker, ctx)
			ch2 := mustSubscribe(t, broker, ctx)

			if stat := broker.Stats(ctx); ctx.Err() != nil {
				t.Error(stat)
			}

			seen1 := &adt.Set[T]{}
			seen2 := &adt.Set[T]{}
			wg := &fnx.WaitGroup{}
			wg.Add(3)
			sig := make(chan struct{})

			wgState := &atomic.Int32{}
			wgState.Add(2)

			total := len(elems)
			started1 := make(chan struct{})
			started2 := make(chan struct{})
			go func() {
				defer wgState.Add(-1)
				defer wg.Done()
				close(started1)
				for {
					select {
					case <-ctx.Done():
						return
					case <-sig:
						return
					case str := <-ch1:
						seen1.Add(str)
					}
					if seen1.Len() == total {
						return
					}
					if seen1.Len()/2 > total {
						return
					}
				}
			}()

			go func() {
				defer wgState.Add(-1)
				defer wg.Done()
				close(started2)
				for {
					select {
					case <-ctx.Done():
						return
					case <-sig:
						return
					case str := <-ch2:
						seen2.Add(str)
					}
					if seen2.Len() == total {
						return
					}
					if seen2.Len()/2 > total {
						return
					}
				}
			}()
			select {
			case <-ctx.Done():
				return
			case <-started1:
			}
			select {
			case <-ctx.Done():
				return
			case <-started2:
			}

			go func() {
				defer wg.Done()
				for idx := range elems {
					_ = broker.Send(ctx, elems[idx])
					runtime.Gosched()
				}
				timer := time.NewTimer(250 * time.Millisecond)
				defer timer.Stop()
				ticker := time.NewTicker(20 * time.Millisecond)
				defer ticker.Stop()

			WAITLOOP:
				for {
					select {
					case <-ctx.Done():
						break WAITLOOP
					case <-timer.C:
						break WAITLOOP
					case <-ticker.C:
						if num := wgState.Load(); num == 0 {
							break WAITLOOP
						}
					}
				}
				_ = broker.Unsubscribe(ctx, ch2)
				_ = broker.Unsubscribe(ctx, ch1)
				close(sig)
			}()

			wg.Wait(ctx)
			if seen1.Len() == seen2.Len() {
				irt.Equal(seen1.Iterator(), seen2.Iterator())
			} else if seen1.Len() == 0 && seen2.Len() == 0 {
				t.Error("should observe some events")
			}

			broker.Stop()
			broker.Wait(ctx)
			cctx, ccancel := context.WithCancel(ctx)
			ccancel()
			if ch, err := broker.Subscribe(cctx); ch != nil || err == nil {
				t.Error("should not subscribe with canceled context", cctx.Err())
			}
			_ = broker.Unsubscribe(cctx, ch1)
			check.Equal(t, broker.Stats(cctx).State, BrokerStateClosed)
		})
	}
}

func TestBroker(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("Strings", func(t *testing.T) {
		t.Parallel()
		RunBrokerTests(ctx, t, randomStringSlice(50))
	})
	t.Run("Integers", func(t *testing.T) {
		t.Parallel()
		RunBrokerTests(ctx, t, randomIntSlice(50))
	})
	t.Run("MakeBrokerDetectsNegativeBufferSizes", func(t *testing.T) {
		opts := BrokerOptions{BufferSize: -1}
		broker := makeBroker[string](opts)
		if broker.opts.BufferSize != 0 {
			t.Fatal("buffer size can't be less than 0")
		}
	})
	t.Run("SubscribeBlocking", func(t *testing.T) {
		broker := NewBroker[int](ctx, BrokerOptions{})
		nctx, ncancel := context.WithCancel(context.Background())
		ncancel()
		if ch, err := broker.Subscribe(nctx); ch != nil || err == nil {
			t.Error("subscription should be nil with a canceled context")
		}
	})
	t.Run("ClosedQueue", func(t *testing.T) {
		t.Parallel()
		t.Run("PublishOne", func(t *testing.T) {
			t.Parallel()
			queue := NewUnlimitedQueue[string]()
			erc.InvariantOk(queue.Close() == nil, "cannot error")
			broker := NewQueueBroker(ctx, queue, BrokerOptions{})

			nctx, ncancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ncancel()
			_ = broker.Send(nctx, "foo")
			if nctx.Err() != nil {
				t.Error("publish to a closed queue blocked")
			}
		})
		t.Run("PublishOneWithSubScriber", func(t *testing.T) {
			t.Parallel()
			queue := NewUnlimitedQueue[string]()
			erc.InvariantOk(queue.Close() == nil, "cannot error")
			broker := NewQueueBroker(ctx, queue, BrokerOptions{})

			nctx, ncancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer ncancel()
			// publishing to a closed queue stops the broker, after
			// which operations fail promptly rather than blocking.
			check.ErrorIs(t, broker.Send(nctx, "foo"), ErrBrokerClosed)
			check.ErrorIs(t, broker.Send(nctx, "foo"), ErrBrokerClosed)
			_, err := broker.Subscribe(nctx)
			check.Error(t, err)
			_ = broker.Send(nctx, "foo")
			if nctx.Err() != nil {
				t.Error("operations on a stopped broker blocked")
			}
		})
	})
	t.Run("ContextCanceled", func(t *testing.T) {
		broker := NewBroker[int](ctx, BrokerOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for range 10 {
			if err := broker.Send(ctx, 123); err != nil {
				check.ErrorIs(t, err, context.Canceled)
				return
			}
			time.Sleep(time.Microsecond)
		}
		t.Error("should have seen one context cancelation error after a send by now")
	})

	t.Run("Populate", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		broker := NewBroker[int](ctx, BrokerOptions{})
		seen := &adt.Set[int]{}
		sig := make(chan struct{})
		sub := mustSubscribe(t, broker, ctx)
		go func() {
			defer close(sig)

			for {
				select {
				case <-ctx.Done():
					return
				case item := <-sub:
					seen.Add(item)
				}
				if seen.Len() == 100 {
					break
				}
			}
		}()
		popsig := make(chan struct{})
		go func() {
			defer close(popsig)
			for _, it := range randomIntSlice(100) {
				check.NotError(t, broker.Send(ctx, it))
			}
		}()

		select {
		case <-ctx.Done():
			t.Fatal("should not have exited")
		case <-sig:
		}

		if seen.Len() != 100 {
			t.Error("unexpected items received", seen.Len())
		}
		select {
		case <-ctx.Done():
			t.Fatal("should not have exited")
		case <-popsig:
		}
	})
}

func randomIntSlice(size int) []int {
	out := make([]int, size)
	for idx := range out {
		out[idx] = rand.Int()
	}
	return out
}

func checkMatchingSets[T comparable](t *testing.T, set1, set2 map[T]struct{}) {
	t.Helper()
	if len(set1) != len(set2) {
		t.Fatal("sets are of different lengths", len(set1), len(set2))
	}

	for k := range set1 {
		if _, ok := set2[k]; !ok {
			t.Error("saw unknown key in set2", k)
		}
	}

	for k := range set2 {
		if _, ok := set1[k]; !ok {
			t.Error("saw unknown key in set1", k)
		}
	}
}

func TestBrokerDropsMessagesOnQueueFull(t *testing.T) {
	t.Parallel()

	t.Run("QueueFullDropsMessages", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create a queue with very limited capacity
		queue, err := NewQueue[string](QueueOptions{
			HardLimit: 3,
			SoftQuota: 3,
		})
		if err != nil {
			t.Fatal(err)
		}

		// Create broker using non-blocking Add which returns ErrQueueFull immediately
		broker := makeInternalBrokerImpl(
			ctx,
			queue.IteratorWaitPop,
			fnx.MakeHandler(queue.Push), // Non-blocking add
			queue.Len,
			BrokerOptions{
				WorkerPoolSize: 1,
			},
		)
		defer broker.Stop()

		// Subscribe immediately but don't read from it
		// This blocks workers on dispatch, allowing queue to fill
		blockingSub := mustSubscribe(t, broker, ctx)
		if blockingSub == nil {
			t.Fatal("failed to subscribe")
		}
		defer broker.Unsubscribe(ctx, blockingSub)

		// Send first message - worker will pull it and block trying to dispatch
		err = broker.Send(ctx, fmt.Sprintf("msg-0"))
		check.NotError(t, err)
		eventually(t, func() bool { return queue.Len() == 0 })

		// Now fill the queue to capacity while worker is blocked
		for i := 1; i <= 3; i++ {
			err := broker.Send(ctx, fmt.Sprintf("msg-%d", i))
			check.NotError(t, err)
		}

		// Queue should be at capacity (msg-0 is held by worker, msg-1,2,3 are in queue)
		stats := broker.Stats(ctx)
		check.Equal(t, stats.BufferDepth, 3)

		// Try to publish more messages - these should be dropped due to queue full
		droppedCount := 5
		for i := 4; i < 4+droppedCount; i++ {
			err := broker.Send(ctx, fmt.Sprintf("msg-%d", i))
			// Send reports the shed message
			check.ErrorIs(t, err, ErrQueueFull)
		}

		// Queue should still be at capacity (messages were dropped)
		stats = broker.Stats(ctx)
		check.Equal(t, stats.BufferDepth, 3)

		// Now consume the messages from blockingSub - we should get first 4 (0-3)
		received := make([]string, 0)
		timeout := time.After(200 * time.Millisecond)

	receiveLoop:
		for len(received) < 4 {
			select {
			case msg := <-blockingSub:
				received = append(received, msg)
			case <-timeout:
				break receiveLoop
			}
		}

		// Should have received exactly 4 messages (msg-0 through msg-3)
		check.Equal(t, len(received), 4)
		check.Equal(t, received[0], "msg-0")
		check.Equal(t, received[1], "msg-1")
		check.Equal(t, received[2], "msg-2")
		check.Equal(t, received[3], "msg-3")

		// Try to receive more - should timeout, confirming dropped messages weren't delivered
		select {
		case msg := <-blockingSub:
			t.Errorf("unexpected message received: %s (messages should have been dropped)", msg)
		case <-time.After(100 * time.Millisecond):
			// Expected - no more messages
		}
	})

	t.Run("QueueNoCreditDropsMessages", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create a queue with soft quota (will return ErrQueueNoCredit when exceeded)
		queue, err := NewQueue[int](QueueOptions{
			HardLimit:   10,
			SoftQuota:   3,
			BurstCredit: 0,
		})
		if err != nil {
			t.Fatal(err)
		}

		// Create broker using non-blocking Add
		broker := makeInternalBrokerImpl(
			ctx,
			queue.IteratorWaitPop,
			fnx.MakeHandler(queue.Push),
			queue.Len,
			BrokerOptions{
				WorkerPoolSize: 1,
			},
		)
		defer broker.Stop()

		// Subscribe to receive messages
		sub := mustSubscribe(t, broker, ctx)
		if sub == nil {
			t.Fatal("failed to subscribe")
		}
		defer broker.Unsubscribe(ctx, sub)

		// Fill queue to soft quota
		for i := range 3 {
			err := broker.Send(ctx, i)
			check.NotError(t, err)
		}

		time.Sleep(50 * time.Millisecond)

		// Publish more messages - these may hit ErrQueueNoCredit and be dropped
		for i := 3; i < 8; i++ {
			err := broker.Send(ctx, i)
			if err != nil {
				check.ErrorIs(t, err, ErrQueueNoCredit)
			}
		}

		time.Sleep(50 * time.Millisecond)

		// The important part: broker should still be running (not crashed)
		// and can publish messages
		stats := broker.Stats(ctx)
		check.Equal(t, stats.Subscriptions, 1)

		// Consume all available messages
		received := make([]int, 0)
		timeout := time.After(200 * time.Millisecond)

	consumeLoop:
		for {
			select {
			case msg := <-sub:
				received = append(received, msg)
			case <-timeout:
				break consumeLoop
			}
		}

		// Should have received some messages (at least the initial ones)
		check.True(t, len(received) >= 3)
		check.True(t, len(received) < 8) // But not all, some were dropped
	})

	t.Run("BrokerContinuesAfterDrops", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create a queue with very limited capacity
		queue, err := NewQueue[string](QueueOptions{
			HardLimit: 2,
			SoftQuota: 2,
		})
		if err != nil {
			t.Fatal(err)
		}

		broker := makeInternalBrokerImpl(
			ctx,
			queue.IteratorWaitPop,
			fnx.MakeHandler(queue.Push),
			queue.Len,
			BrokerOptions{},
		)
		defer broker.Stop()

		sub := mustSubscribe(t, broker, ctx)
		if sub == nil {
			t.Fatal("failed to subscribe")
		}
		defer broker.Unsubscribe(ctx, sub)

		// Send first message - worker pulls and blocks on dispatch
		broker.Send(ctx, "msg-0")
		eventually(t, func() bool { return queue.Len() == 0 })

		// Fill the queue to capacity (HardLimit: 2)
		broker.Send(ctx, "msg-1")
		broker.Send(ctx, "msg-2")

		// Try to send more - these should be dropped (queue full)
		broker.Send(ctx, "dropped-3")
		broker.Send(ctx, "dropped-4")
		broker.Send(ctx, "dropped-5")

		// Consume messages
		received := make([]string, 0, 3)
		for range 3 {
			msg := <-sub
			received = append(received, msg)
		}

		// Should have received only the 3 messages before queue filled
		check.Equal(t, len(received), 3)
		check.Equal(t, received[0], "msg-0")
		check.Equal(t, received[1], "msg-1")
		check.Equal(t, received[2], "msg-2")

		// Now queue has space - new messages should go through
		broker.Send(ctx, "after-drop")

		msg := <-sub
		check.Equal(t, msg, "after-drop")

		// Verify no more messages (dropped ones aren't delivered)
		select {
		case unexpected := <-sub:
			t.Errorf("unexpected message: %s", unexpected)
		case <-time.After(50 * time.Millisecond):
			// Expected - no more messages
		}
	})
}

// eventually polls cond until it is true, failing the test if it is not
// true within a generous deadline. It replaces fixed sleeps that wait
// for a background goroutine to make progress.
func eventually(t testing.TB, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never became true")
		}
		time.Sleep(time.Millisecond)
	}
}

func brokerConstructors(t *testing.T) map[string]func(ctx context.Context) *Broker[int] {
	return map[string]func(ctx context.Context) *Broker[int]{
		"Channel": func(ctx context.Context) *Broker[int] {
			return NewBroker[int](ctx, BrokerOptions{WorkerPoolSize: 3})
		},
		"Queue": func(ctx context.Context) *Broker[int] {
			return NewQueueBroker(ctx, NewUnlimitedQueue[int](), BrokerOptions{WorkerPoolSize: 3})
		},
		"Deque": func(ctx context.Context) *Broker[int] {
			dq, err := NewDeque[int](DequeOptions{Capacity: 10})
			if err != nil {
				t.Fatal(err)
			}
			return NewDequeBroker(ctx, dq, BrokerOptions{WorkerPoolSize: 3})
		},
		"LIFO": func(ctx context.Context) *Broker[int] {
			dq, err := NewDeque[int](DequeOptions{Capacity: 10})
			if err != nil {
				t.Fatal(err)
			}
			return NewLIFOBroker(ctx, dq, BrokerOptions{WorkerPoolSize: 3})
		},
	}
}

func TestBrokerStatsWithExpiredContextDoesNotWedge(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			b := mk(ctx)
			defer b.Stop()

			for i := range 200 {
				sctx, scancel := context.WithTimeout(ctx, time.Duration(i%5)*10*time.Microsecond)
				_ = b.Stats(sctx)
				scancel()
			}
			dead, dcancel := context.WithCancel(ctx)
			dcancel()
			for range 50 {
				_ = b.Stats(dead)
			}

			sendCtx, sendCancel := context.WithTimeout(ctx, time.Second)
			defer sendCancel()
			if err := b.Send(sendCtx, 1); err != nil {
				t.Fatalf("broker wedged after Stats: %v", err)
			}
		})
	}
}

func TestQueueBrokerDrainsQueue(t *testing.T) {
	t.Run("BlockingDefault", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		queue, err := NewQueue[int](QueueOptions{HardLimit: 2, SoftQuota: 2})
		if err != nil {
			t.Fatal(err)
		}
		b := NewQueueBroker(ctx, queue, BrokerOptions{})
		defer b.Stop()
		sub := mustSubscribe(t, b, ctx)
		defer b.Unsubscribe(ctx, sub)

		const count = 10
		go func() {
			for i := range count {
				_ = b.Send(ctx, i)
			}
		}()
		for i := range count {
			select {
			case v := <-sub:
				check.Equal(t, v, i)
			case <-ctx.Done():
				t.Fatalf("timed out after %d messages", i)
			}
		}
		check.Equal(t, queue.Len(), 0)
		check.Equal(t, b.Stats(ctx).BufferDepth, 0)
	})
	t.Run("NonBlockingPushSheds", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		queue, err := NewQueue[int](QueueOptions{HardLimit: 2, SoftQuota: 2})
		if err != nil {
			t.Fatal(err)
		}
		b := NewQueueBroker(ctx, queue, BrokerOptions{NonBlockingPush: true})
		defer b.Stop()
		sub := mustSubscribe(t, b, ctx)
		defer b.Unsubscribe(ctx, sub)

		// the worker takes one message and blocks on the
		// subscriber; the queue holds two more.
		full := 0
		for i := range 10 {
			sctx, scancel := context.WithTimeout(ctx, time.Second)
			if err := b.Send(sctx, i); errors.Is(err, ErrQueueFull) {
				full++
			}
			scancel()
			if i == 0 {
				// the worker must hold the first message before the
				// queue is filled.
				eventually(t, func() bool { return queue.Len() == 0 })
			}
		}
		check.Equal(t, full, 7)
		got := 0
	loop:
		for {
			select {
			case <-sub:
				got++
			case <-time.After(100 * time.Millisecond):
				break loop
			}
		}
		check.Equal(t, got, 3)
		check.Equal(t, queue.Len(), 0)
	})
}

func TestBrokerUnsubscribeBlockedSubscriber(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			b := mk(ctx)
			defer b.Stop()

			stuck := mustSubscribe(t, b, ctx)
			// back the broker up: nobody reads from stuck.
			for i := range 3 {
				go func() {
					pctx, pcancel := context.WithTimeout(ctx, 2*time.Second)
					defer pcancel()
					_ = b.Send(pctx, i)
				}()
			}
			time.Sleep(100 * time.Millisecond)

			uctx, ucancel := context.WithTimeout(ctx, time.Second)
			defer ucancel()
			_ = b.Unsubscribe(uctx, stuck)
			if uctx.Err() != nil {
				t.Fatal("unsubscribe did not complete")
			}

			sctx, scancel := context.WithTimeout(ctx, time.Second)
			defer scancel()
			if n := b.Stats(sctx).Subscriptions; n != 0 {
				t.Fatalf("expected no subscriptions, got %d", n)
			}

			live := mustSubscribe(t, b, ctx)
			defer b.Unsubscribe(ctx, live)
			pctx, pcancel := context.WithTimeout(ctx, 2*time.Second)
			defer pcancel()
			_ = b.Send(pctx, 99)
			select {
			case <-live:
			case <-time.After(2 * time.Second):
				t.Fatal("broker did not recover after unsubscribing blocked subscriber")
			}
		})
	}
}

func TestBrokerSubscribeUnsubscribeOrdering(t *testing.T) {
	for name, mk := range map[string]func(ctx context.Context) *Broker[int]{
		"Channel": func(ctx context.Context) *Broker[int] {
			return NewBroker[int](ctx, BrokerOptions{BufferSize: 8})
		},
		"Queue": func(ctx context.Context) *Broker[int] {
			return NewQueueBroker(ctx, NewUnlimitedQueue[int](), BrokerOptions{BufferSize: 8})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			b := mk(ctx)
			defer b.Stop()

			for range 500 {
				ch := mustSubscribe(t, b, ctx)
				_ = b.Unsubscribe(ctx, ch)
			}
			// a concurrent burst, each goroutine ordered internally.
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					for range 100 {
						ch := mustSubscribe(t, b, ctx)
						_ = b.Unsubscribe(ctx, ch)
					}
				})
			}
			wg.Wait()
			check.Equal(t, b.Stats(ctx).Subscriptions, 0)
		})
	}
}

func TestBrokerOperationsAfterStop(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			b := mk(context.Background())
			sub := mustSubscribe(t, b, context.Background())
			b.Stop()

			done := make(chan struct{})
			var sendErr, pubErr, subErr, unsubErr error
			var subCh chan int
			var stats BrokerStats
			go func() {
				defer close(done)
				bg := context.Background()
				sendErr = b.Send(bg, 1)
				pubErr = b.Send(bg, 2)
				subCh, subErr = b.Subscribe(bg)
				unsubErr = b.Unsubscribe(bg, sub)
				stats = b.Stats(bg)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("operations blocked after Stop")
			}
			check.ErrorIs(t, sendErr, ErrBrokerClosed)
			check.ErrorIs(t, pubErr, ErrBrokerClosed)
			check.ErrorIs(t, subErr, ErrBrokerClosed)
			check.ErrorIs(t, unsubErr, ErrBrokerClosed)
			check.True(t, subCh == nil)
			check.Equal(t, stats, BrokerStats{State: BrokerStateClosed})
		})
	}
}

// Canceled caller contexts surface as the context error.
func TestBrokerCanceledContextErrors(t *testing.T) {
	b := NewBroker[int](t.Context(), BrokerOptions{})
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := b.Subscribe(cctx)
	check.ErrorIs(t, err, context.Canceled)
	check.ErrorIs(t, b.Send(cctx, 1), context.Canceled)
	// the control channel is unbuffered and the loop is idle, so the
	// request may be accepted; a blocked one reports the ctx error.
	check.ErrorIs(t, stalledBroker(0).Unsubscribe(cctx, nil), context.Canceled)
}

func TestBrokerStopWhileWaiting(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			b := mk(context.Background())
			waited := make(chan struct{})
			go func() { defer close(waited); b.Wait(context.Background()) }()
			time.Sleep(50 * time.Millisecond)

			stopped := make(chan struct{})
			go func() { defer close(stopped); b.Stop() }()
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("Stop deadlocked behind Wait")
			}
			select {
			case <-waited:
			case <-time.After(2 * time.Second):
				t.Fatal("Wait did not return after Stop")
			}
		})
	}
}

func TestBrokerStopReleasesWorkers(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			leakCheck := testt.NoGoroutineLeak(t, 5*time.Second)
			b := mk(context.Background())
			b.Stop()

			done := make(chan struct{})
			go func() { defer close(done); b.Wait(context.Background()) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Wait did not return after Stop")
			}
			leakCheck()
		})
	}
}

func mustSubscribe[T any](t testing.TB, b *Broker[T], ctx context.Context) chan T {
	t.Helper()
	ch, err := b.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

// stalledBroker has no control loop, so control requests are accepted
// (up to buffer) but never processed or acknowledged.
func stalledBroker(buffer int) *Broker[int] {
	b := makeBroker[int](BrokerOptions{BufferSize: buffer})
	b.ctx, b.close = context.WithCancel(context.Background())
	return b
}

func TestBrokerControlOperationsInterrupted(t *testing.T) {
	t.Run("SubscribeCtxWhileSending", func(t *testing.T) {
		b := stalledBroker(0)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := b.Subscribe(ctx)
		check.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("SubscribeStopWhileSending", func(t *testing.T) {
		b := stalledBroker(0)
		time.AfterFunc(10*time.Millisecond, b.Stop)
		_, err := b.Subscribe(context.Background())
		check.ErrorIs(t, err, ErrBrokerClosed)
	})
	t.Run("SubscribeCtxWhileAwaitingAck", func(t *testing.T) {
		b := stalledBroker(2)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := b.Subscribe(ctx)
		check.ErrorIs(t, err, context.DeadlineExceeded)
		// the matching removal was queued behind the subscription.
		check.Equal(t, len(b.ctlCh), 2)
	})
	t.Run("SubscribeStopWhileAwaitingAck", func(t *testing.T) {
		b := stalledBroker(1)
		time.AfterFunc(10*time.Millisecond, b.Stop)
		_, err := b.Subscribe(context.Background())
		check.ErrorIs(t, err, ErrBrokerClosed)
	})
	t.Run("UnsubscribeStopWhileSending", func(t *testing.T) {
		b := stalledBroker(0)
		time.AfterFunc(10*time.Millisecond, b.Stop)
		check.ErrorIs(t, b.Unsubscribe(context.Background(), nil), ErrBrokerClosed)
	})
	t.Run("StatsStopWhileSending", func(t *testing.T) {
		b := stalledBroker(0)
		time.AfterFunc(10*time.Millisecond, b.Stop)
		check.Equal(t, b.Stats(context.Background()), BrokerStats{State: BrokerStateClosed})
	})
	t.Run("StatsStopWhileAwaitingReply", func(t *testing.T) {
		b := stalledBroker(1)
		time.AfterFunc(10*time.Millisecond, b.Stop)
		check.Equal(t, b.Stats(context.Background()), BrokerStats{State: BrokerStateClosed})
	})
	t.Run("DuplicateSubscribeIsIdempotent", func(t *testing.T) {
		b := NewBroker[int](t.Context(), BrokerOptions{})
		ch := make(chan int)
		for range 2 {
			req := ctlRequest[int]{ch: ch, ack: make(chan struct{})}
			b.ctlCh <- req
			<-req.ack
		}
		check.Equal(t, b.Stats(t.Context()).Subscriptions, 1)
	})
}

func TestBrokerSendStoppedWhileSinkBlocked(t *testing.T) {
	b := makeInternalBrokerImpl(
		t.Context(),
		func(context.Context) iter.Seq[int] { return func(func(int) bool) {} },
		func(ctx context.Context, _ int) error { <-ctx.Done(); return ctx.Err() },
		func() int { return 0 },
		BrokerOptions{},
	)
	time.AfterFunc(20*time.Millisecond, b.Stop)
	err := b.Send(context.Background(), 1)
	check.ErrorIs(t, err, ErrBrokerClosed)
	check.ErrorIs(t, err, context.Canceled)
}

// Send runs the sink on the caller's goroutine and Subscribe waits for
// registration: a Send after a successful Subscribe reaches that
// subscriber, and queue-full errors come back from Send itself.
func TestBrokerSendSemantics(t *testing.T) {
	ctx := t.Context()
	t.Run("SubscribeAckMeansNextSendIsDelivered", func(t *testing.T) {
		b := NewBroker[int](ctx, BrokerOptions{BufferSize: 1})
		for i := range 50 {
			sub := mustSubscribe(t, b, ctx)
			check.NotError(t, b.Send(ctx, i))
			// dispatch iterates the live subscriber set, so a message
			// still being dispatched when this Subscribe landed may
			// arrive first: drain until the message sent after it.
			for got := -1; got != i; {
				select {
				case got = <-sub:
					check.True(t, got == i || got == i-1)
				case <-time.After(time.Second):
					t.Fatal("message sent after Subscribe was not delivered")
				}
			}
			check.NotError(t, b.Unsubscribe(ctx, sub))
		}
	})
	t.Run("QueueFullReportedToCaller", func(t *testing.T) {
		q, err := NewQueue[int](QueueOptions{HardLimit: 1, SoftQuota: 1})
		if err != nil {
			t.Fatal(err)
		}
		b := NewQueueBroker(ctx, q, BrokerOptions{NonBlockingPush: true})
		sub := mustSubscribe(t, b, ctx)
		defer func() { _ = b.Unsubscribe(ctx, sub) }()
		var full bool
		for i := range 20 {
			if err := b.Send(ctx, i); errors.Is(err, ErrQueueFull) {
				full = true
			}
		}
		// the unread subscriber blocks the worker, so the queue fills.
		check.True(t, full)
	})
}

// Dispatch iterates the live subscription map; concurrent subscribe
// and unsubscribe during dispatch must be race-free (run with -race).
func TestBrokerDispatchConcurrentSubscriptionChanges(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		t.Run(fmt.Sprint("Parallel=", parallel), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			b := NewBroker[int](ctx, BrokerOptions{ParallelDispatch: parallel, BufferSize: 4, WorkerPoolSize: 2})
			var wg sync.WaitGroup
			stop := make(chan struct{})
			wg.Go(func() {
				for i := 0; ; i++ {
					select {
					case <-stop:
						return
					default:
						_ = b.Send(ctx, i)
					}
				}
			})
			for range 4 {
				wg.Go(func() {
					for range 100 {
						ch, err := b.Subscribe(ctx)
						if err != nil {
							return
						}
						select {
						case <-ch:
						case <-time.After(5 * time.Millisecond):
						}
						_ = b.Unsubscribe(ctx, ch)
					}
				})
			}
			time.Sleep(200 * time.Millisecond)
			close(stop)
			wg.Wait()
			b.Stop()
			b.Wait(ctx)
		})
	}
}

// BenchmarkBrokerDispatch measures dispatch over the live SyncMap; compare
// with BenchmarkBrokerDispatchSnapshot, which copies the keys per message.
func BenchmarkBrokerDispatch(b *testing.B) {
	for _, subs := range []int{1, 16, 128} {
		b.Run(fmt.Sprint("Subscribers=", subs), func(b *testing.B) {
			m := &adt.SyncMap[chan int, chan struct{}]{}
			for range subs {
				ch := make(chan int, 1)
				m.Store(ch, make(chan struct{}))
				go func() {
					for range ch {
					}
				}()
			}
			br := stalledBroker(0)
			ctx := b.Context()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				br.dispatchMessage(ctx, m.Iterator(), i)
			}
		})
	}
}

func BenchmarkBrokerDispatchSnapshot(b *testing.B) {
	for _, subs := range []int{1, 16, 128} {
		b.Run(fmt.Sprint("Subscribers=", subs), func(b *testing.B) {
			m := &adt.SyncMap[chan int, chan struct{}]{}
			for range subs {
				ch := make(chan int, 1)
				m.Store(ch, make(chan struct{}))
				go func() {
					for range ch {
					}
				}()
			}
			br := stalledBroker(0)
			ctx := b.Context()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				snap := maps.Collect(m.Iterator())
				br.dispatchMessage(ctx, maps.All(snap), i)
			}
		})
	}
}

// A slow subscriber under ParallelDispatch delays the message (head of
// line) but does not lose it or stop other subscribers receiving it.
func TestBrokerParallelDispatchSlowSubscriber(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	b := NewBroker[int](ctx, BrokerOptions{ParallelDispatch: true})
	fast := mustSubscribe(t, b, ctx)
	slow := mustSubscribe(t, b, ctx)

	check.NotError(t, b.Send(ctx, 1))
	select {
	case v := <-fast:
		check.Equal(t, v, 1)
	case <-time.After(time.Second):
		t.Fatal("fast subscriber blocked by slow subscriber")
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case v := <-slow:
		check.Equal(t, v, 1)
	case <-time.After(time.Second):
		t.Fatal("slow subscriber never received the message")
	}
}

// A panic in a subscriber's goroutine is its own; a panicking sink
// propagates on the caller's goroutine (Send runs the sink inline) and
// leaves the broker usable.
func TestBrokerPanics(t *testing.T) {
	ctx := t.Context()
	t.Run("SinkPanicReachesCaller", func(t *testing.T) {
		b := makeInternalBrokerImpl(
			ctx,
			func(context.Context) iter.Seq[int] { return func(func(int) bool) {} },
			func(context.Context, int) error { panic("boom") },
			func() int { return 0 },
			BrokerOptions{},
		)
		func() {
			defer func() { check.Equal(t, fmt.Sprint(recover()), "boom") }()
			_ = b.Send(ctx, 1)
		}()
		// still responsive afterwards.
		sub := mustSubscribe(t, b, ctx)
		check.NotError(t, b.Unsubscribe(ctx, sub))
		_ = b.Stats(ctx)
		b.Stop()
		b.Wait(ctx)
	})
	t.Run("SubscriberPanicDoesNotAffectBroker", func(t *testing.T) {
		b := NewBroker[int](ctx, BrokerOptions{})
		sub := mustSubscribe(t, b, ctx)
		got := make(chan any, 1)
		go func() {
			defer func() { got <- recover() }()
			<-sub
			panic("subscriber")
		}()
		check.NotError(t, b.Send(ctx, 1))
		check.Equal(t, fmt.Sprint(<-got), "subscriber")
		check.NotError(t, b.Unsubscribe(ctx, sub))
		check.NotError(t, b.Send(ctx, 2))
		b.Stop()
	})
}

// Send with a canceled caller context fails fast with that error and
// does not count as a message, for every broker.
func TestBrokerSendCanceledContext(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			b := mk(t.Context())
			cctx, cancel := context.WithCancel(context.Background())
			cancel()
			check.ErrorIs(t, b.Send(cctx, 1), context.Canceled)
			check.Equal(t, b.Stats(t.Context()).MessageCount, 0)
			b.Stop()
		})
	}
}

// Publish is deprecated but must keep behaving like Send. It is
// called through a local interface so that the deprecation check does
// not flag the call.
func TestBrokerPublishDelegatesToSend(t *testing.T) {
	type publisher interface {
		Publish(context.Context, int) error
	}
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			b := mk(t.Context())
			var p publisher = b
			sub := mustSubscribe(t, b, t.Context())
			check.NotError(t, p.Publish(t.Context(), 7))
			select {
			case v := <-sub:
				check.Equal(t, v, 7)
			case <-time.After(time.Second):
				t.Fatal("message not delivered")
			}
			cctx, cancel := context.WithCancel(context.Background())
			cancel()
			check.ErrorIs(t, p.Publish(cctx, 1), context.Canceled)
			b.Stop()
			check.ErrorIs(t, p.Publish(t.Context(), 1), ErrBrokerClosed)
		})
	}
}

func brokerCPUTime(t *testing.T) time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Skip(err)
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

// Idle brokers with a worker pool must not spin.
func TestBrokerIdleCPU(t *testing.T) {
	for name, mk := range brokerConstructors(t) {
		t.Run(name, func(t *testing.T) {
			b := mk(t.Context())
			defer b.Stop()
			time.Sleep(50 * time.Millisecond)
			before := brokerCPUTime(t)
			time.Sleep(500 * time.Millisecond)
			if used := brokerCPUTime(t) - before; used > 150*time.Millisecond {
				t.Fatalf("idle broker used %s CPU in 500ms", used)
			}
		})
	}
}

func TestBrokerStatsState(t *testing.T) {
	t.Run("ZeroValueIsUnknown", func(t *testing.T) {
		check.Equal(t, BrokerStats{}.State, BrokerStateUnknown)
	})
	t.Run("Empty", func(t *testing.T) {
		b := NewBroker[int](t.Context(), BrokerOptions{})
		check.Equal(t, b.Stats(t.Context()).State, BrokerStateEmpty)
	})
	t.Run("Active", func(t *testing.T) {
		b := NewBroker[int](t.Context(), BrokerOptions{})
		mustSubscribe(t, b, t.Context())
		check.Equal(t, b.Stats(t.Context()).State, BrokerStateActive)
	})
	t.Run("CallerCtxWhileAwaitingReply", func(t *testing.T) {
		b := stalledBroker(1)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		check.Equal(t, b.Stats(ctx), BrokerStats{})
	})
	t.Run("Closed", func(t *testing.T) {
		b := NewBroker[int](t.Context(), BrokerOptions{})
		b.Stop()
		check.Equal(t, b.Stats(t.Context()), BrokerStats{State: BrokerStateClosed})
	})
	t.Run("ClosedWinsOverCanceledCaller", func(t *testing.T) {
		b := NewBroker[int](t.Context(), BrokerOptions{})
		b.Stop()
		cctx, cancel := context.WithCancel(t.Context())
		cancel()
		check.Equal(t, b.Stats(cctx).State, BrokerStateClosed)
	})
	t.Run("CanceledCallerOnRunningBrokerIsNotClosed", func(t *testing.T) {
		b := NewBroker[int](t.Context(), BrokerOptions{})
		cctx, cancel := context.WithCancel(t.Context())
		cancel()
		check.Equal(t, b.Stats(cctx), BrokerStats{})
	})
	t.Run("String", func(t *testing.T) {
		check.Equal(t, BrokerStateUnknown.String(), "unknown")
		check.Equal(t, BrokerStateEmpty.String(), "empty")
		check.Equal(t, BrokerStateActive.String(), "active")
		check.Equal(t, BrokerStateClosed.String(), "closed")
		check.Equal(t, BrokerState(99).String(), "unknown")
	})
}

func TestBrokerStopKeepsBacklog(t *testing.T) {
	const total = 5
	cases := map[string]func(context.Context) (*Broker[int], func() int){
		"Queue": func(ctx context.Context) (*Broker[int], func() int) {
			q := NewUnlimitedQueue[int]()
			return NewQueueBroker(ctx, q, BrokerOptions{}), q.Len
		},
		"Deque": func(ctx context.Context) (*Broker[int], func() int) {
			dq, err := NewDeque[int](DequeOptions{Capacity: 10})
			assert.NotError(t, err)
			return NewDequeBroker(ctx, dq, BrokerOptions{}), dq.Len
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			b, length := mk(ctx)
			_ = mustSubscribe(t, b, ctx) // never read: dispatch blocks on it
			for i := range total {
				assert.NotError(t, b.Publish(ctx, i))
			}
			// the broker takes one message and blocks sending it.
			deadline := time.Now().Add(5 * time.Second)
			for length() != total-1 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			assert.Equal(t, length(), total-1)

			b.Stop()
			b.Wait(ctx)
			check.Equal(t, length(), total-1)
		})
	}
}

// TestBrokerBlockedSendReleasedByClose covers a Send that is blocked
// in the sink (queue/deque push), not merely in dispatch: a
// non-reading subscriber stalls the dispatch worker mid-message,
// which backs up the queue/deque to its hard limit, and a subsequent
// Send then blocks in WaitPush/WaitPushFront waiting for room.
//
// Two release paths are exercised for both Queue- and Deque-backed
// brokers:
//
//   - Stop(): the broker's own shutdown must release the blocked Send
//     promptly with ErrBrokerClosed.
//   - closing the underlying Queue/Deque directly, bypassing the
//     broker's Stop(): this must also release the blocked Send
//     promptly (with ErrQueueClosed), and the broker itself must
//     transition to BrokerStateClosed.
func TestBrokerBlockedSendReleasedByClose(t *testing.T) {
	const releaseTimeout = 2 * time.Second

	type harness struct {
		broker  *Broker[int]
		length  func() int
		closeCh func() error // closes the underlying queue/deque directly
	}

	cases := map[string]func(t *testing.T) harness{
		"Queue": func(t *testing.T) harness {
			queue, err := NewQueue[int](QueueOptions{HardLimit: 1, SoftQuota: 1})
			assert.NotError(t, err)
			b := NewQueueBroker(context.Background(), queue, BrokerOptions{})
			return harness{broker: b, length: queue.Len, closeCh: queue.Close}
		},
		"Deque": func(t *testing.T) harness {
			dq, err := NewDeque[int](DequeOptions{Capacity: 1})
			assert.NotError(t, err)
			b := NewDequeBroker(context.Background(), dq, BrokerOptions{})
			return harness{broker: b, length: dq.Len, closeCh: dq.Close}
		},
	}

	// blockSend subscribes a channel that is never read (so the
	// dispatch worker stalls delivering the first message), then
	// sends enough messages to fill the queue/deque to its hard
	// limit of 1. It returns a channel that will receive the result
	// of a further Send call, which must block because the
	// queue/deque is full and nothing is draining it.
	blockSend := func(t *testing.T, h harness) <-chan error {
		t.Helper()
		ctx := context.Background()
		_ = mustSubscribe(t, h.broker, ctx) // never read: dispatch stalls on it

		// first message: the worker pulls it immediately and then
		// blocks forever trying to deliver it to the subscriber.
		assert.NotError(t, h.broker.Send(ctx, 1))
		eventually(t, func() bool { return h.length() == 0 })

		// second message: fills the queue/deque to its hard limit
		// of 1, since nothing is draining it anymore.
		assert.NotError(t, h.broker.Send(ctx, 2))
		eventually(t, func() bool { return h.length() == 1 })

		// third message: must block, since the queue/deque is full.
		result := make(chan error, 1)
		go func() { result <- h.broker.Send(ctx, 3) }()

		// give the goroutine a moment to actually enter the blocking
		// WaitPush/WaitPushFront call before we trigger a release.
		time.Sleep(50 * time.Millisecond)
		select {
		case err := <-result:
			t.Fatalf("Send did not block on the full queue/deque: %v", err)
		default:
		}
		return result
	}

	for name, mk := range cases {
		t.Run(name+"/StopReleasesBlockedSend", func(t *testing.T) {
			h := mk(t)
			defer h.broker.Stop()
			result := blockSend(t, h)

			h.broker.Stop()
			select {
			case err := <-result:
				check.ErrorIs(t, err, ErrBrokerClosed)
			case <-time.After(releaseTimeout):
				t.Fatal("Stop() did not release a Send blocked on a full queue/deque")
			}
		})

		t.Run(name+"/ExternalCloseReleasesBlockedSend", func(t *testing.T) {
			h := mk(t)
			defer h.broker.Stop()
			result := blockSend(t, h)

			assert.NotError(t, h.closeCh())
			select {
			case err := <-result:
				check.ErrorIs(t, err, ErrQueueClosed)
				check.ErrorIs(t, err, ErrBrokerClosed)
			case <-time.After(releaseTimeout):
				t.Fatal("closing the underlying queue/deque did not release a blocked Send")
			}

			// the broker itself must observe the external close and
			// transition to closed, not just the one blocked Send.
			select {
			case <-h.broker.ctx.Done():
			case <-time.After(releaseTimeout):
				t.Fatal("broker did not transition to closed after the underlying queue/deque closed")
			}
			check.Equal(t, h.broker.Stats(context.Background()), BrokerStats{State: BrokerStateClosed})
		})
	}
}

func TestIteratorWaitPopCancelledContextYieldsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	q := NewUnlimitedQueue[int]()
	assert.NotError(t, q.Push(1))
	for range q.IteratorWaitPop(ctx) {
		t.Fatal("queue yielded under a cancelled context")
	}
	check.Equal(t, q.Len(), 1)

	dq, err := NewDeque[int](DequeOptions{Capacity: 10})
	assert.NotError(t, err)
	assert.NotError(t, dq.PushFront(1))
	for range dq.IteratorWaitPopFront(ctx) {
		t.Fatal("deque front yielded under a cancelled context")
	}
	for range dq.IteratorWaitPopBack(ctx) {
		t.Fatal("deque back yielded under a cancelled context")
	}
	check.Equal(t, dq.Len(), 1)
}
