package pubsub

import (
	"context"
	"fmt"
	"iter"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
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
		{
			Name: fmt.Sprintf("Queue/Limit8/%s", axis),
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   8,
					SoftQuota:   4,
					BurstCredit: 2,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker[T](ctx, queue, opts)
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
			Name: fmt.Sprintf("Deque/FIFO/Limit8/%s", axis),
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewDequeBroker(
					ctx,
					erc.Must(NewDeque[T](DequeOptions{
						Capacity: 8,
					})),
					opts,
				)
			},
		},
		{
			Name: fmt.Sprintf("Deque/LIFO/Limit8/%s", axis),
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				return NewLIFOBroker(
					ctx,
					erc.Must(NewDeque[T](DequeOptions{
						Capacity: 8,
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
			GenerateFixtures("Parallel/Worker16", elems, BrokerOptions{ParallelDispatch: true, WorkerPoolSize: 16}),
		),
	)
}

func RunBrokerTests[T comparable](pctx context.Context, t *testing.T, elems []T) {
	for fix := range makeFixtures(elems) {
		t.Run(fix.Name, func(t *testing.T) {
			opts := fix

			t.Parallel()
			ctx, cancel := context.WithTimeout(pctx, 5*time.Second)
			defer cancel()

			broker := opts.Construtor(ctx, t)

			ch1 := broker.Subscribe(ctx)
			ch2 := broker.Subscribe(ctx)

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
					broker.Publish(ctx, elems[idx])
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
				broker.Unsubscribe(ctx, ch2)
				broker.Unsubscribe(ctx, ch1)
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
			if broker.Subscribe(cctx) != nil {
				t.Error("should not subscribe with canceled context", cctx.Err())
			}
			broker.Unsubscribe(cctx, ch1)
			check.Zero(t, broker.Stats(cctx))
		})
	}
}

func TestBroker(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("Strings", func(t *testing.T) {
		t.Parallel()
		for _, scope := range []struct {
			Name  string
			Elems []string
		}{
			{
				Name:  "Basic",
				Elems: randomStringSlice(10),
			},
			{
				Name:  "Medium",
				Elems: randomStringSlice(250),
			},
		} {
			t.Run(scope.Name, func(t *testing.T) {
				t.Parallel()
				RunBrokerTests(ctx, t, scope.Elems)
			})
		}
	})
	t.Run("Integers", func(t *testing.T) {
		t.Parallel()
		for _, scope := range []struct {
			Name  string
			Elems []int
		}{
			{
				Name:  "Small",
				Elems: randomIntSlice(20),
			},
			{
				Name:  "Large",
				Elems: randomIntSlice(500),
			},
		} {
			t.Run(scope.Name, func(t *testing.T) {
				t.Parallel()
				RunBrokerTests(ctx, t, scope.Elems)
			})
		}
	})
	t.Run("MakeBrokerDetectsNegativeBufferSizes", func(t *testing.T) {
		opts := BrokerOptions{BufferSize: -1}
		broker := makeBroker[string](opts)
		if broker.opts.BufferSize != 0 {
			t.Fatal("buffer size can't be less than 0")
		}
		if cap(broker.publishCh) != 0 {
			t.Fatal("channel capacity should be the buffer size")
		}
	})
	t.Run("SubscribeBlocking", func(t *testing.T) {
		broker := NewBroker[int](ctx, BrokerOptions{})
		nctx, ncancel := context.WithCancel(context.Background())
		ncancel()
		if broker.Subscribe(nctx) != nil {
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

			sa := time.Now()
			nctx, ncancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer ncancel()
			broker.Publish(nctx, "foo")
			dur := time.Since(sa)
			if dur > 5*time.Millisecond {
				t.Error(dur)
			}
		})
		t.Run("PublishOneWithSubScriber", func(t *testing.T) {
			t.Parallel()
			queue := NewUnlimitedQueue[string]()
			erc.InvariantOk(queue.Close() == nil, "cannot error")
			broker := NewQueueBroker(ctx, queue, BrokerOptions{})

			sa := time.Now()
			nctx, ncancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer ncancel()
			ch := broker.Subscribe(ctx)
			if ch == nil {
				t.Error("should be able to subscribe")
			}
			broker.Publish(nctx, "foo")
			dur := time.Since(sa)
			if dur > 5*time.Millisecond {
				t.Error(dur)
			}
		})
		t.Run("PublishMany", func(t *testing.T) {
			t.Parallel()
			queue := NewUnlimitedQueue[string]()
			erc.InvariantOk(queue.Close() == nil, "cannot error")
			broker := NewQueueBroker(ctx, queue, BrokerOptions{})

			sa := time.Now()
			nctx, ncancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer ncancel()
			count := int64(0)
			for {
				ch := broker.Subscribe(nctx)
				if ch == nil {
					break
				}
				broker.Publish(nctx, "foo")
				count++
			}
			dur := time.Since(sa)
			if dur < 20*time.Millisecond || dur > 40*time.Millisecond {
				t.Error(count, dur)
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
		input := randomIntSlice(100)

		iter := SliceStream(input)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		broker := NewBroker[int](ctx, BrokerOptions{})
		seen := &adt.Set[int]{}
		sig := make(chan struct{})
		sub := broker.Subscribe(ctx)
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
			for it := range iter.Iterator(ctx) {
				broker.Send(ctx, it)
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
			queue.IteratorWaitPop(ctx),
			fnx.MakeHandler(queue.Add), // Non-blocking add
			queue.Len,
			BrokerOptions{
				WorkerPoolSize: 1,
			},
		)
		defer broker.Stop()

		// Subscribe immediately but don't read from it
		// This blocks workers on dispatch, allowing queue to fill
		blockingSub := broker.Subscribe(ctx)
		if blockingSub == nil {
			t.Fatal("failed to subscribe")
		}
		defer broker.Unsubscribe(ctx, blockingSub)

		// Send first message - worker will pull it and block trying to dispatch
		err = broker.Send(ctx, fmt.Sprintf("msg-0"))
		check.NotError(t, err)
		time.Sleep(20 * time.Millisecond)

		// Now fill the queue to capacity while worker is blocked
		for i := 1; i <= 3; i++ {
			err := broker.Send(ctx, fmt.Sprintf("msg-%d", i))
			check.NotError(t, err)
		}

		// Give time for messages to reach the queue
		time.Sleep(20 * time.Millisecond)

		// Queue should be at capacity (msg-0 is held by worker, msg-1,2,3 are in queue)
		stats := broker.Stats(ctx)
		check.Equal(t, stats.BufferDepth, 3)

		// Try to publish more messages - these should be dropped due to queue full
		droppedCount := 5
		for i := 4; i < 4+droppedCount; i++ {
			err := broker.Send(ctx, fmt.Sprintf("msg-%d", i))
			// Send should succeed even though messages are dropped
			check.NotError(t, err)
		}

		// Give time for the publish goroutine to process
		time.Sleep(20 * time.Millisecond)

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
			queue.IteratorWaitPop(ctx),
			fnx.MakeHandler(queue.Add),
			queue.Len,
			BrokerOptions{
				WorkerPoolSize: 1,
			},
		)
		defer broker.Stop()

		// Subscribe to receive messages
		sub := broker.Subscribe(ctx)
		if sub == nil {
			t.Fatal("failed to subscribe")
		}
		defer broker.Unsubscribe(ctx, sub)

		// Fill queue to soft quota
		for i := 0; i < 3; i++ {
			err := broker.Send(ctx, i)
			check.NotError(t, err)
		}

		time.Sleep(50 * time.Millisecond)

		// Publish more messages - these may hit ErrQueueNoCredit and be dropped
		for i := 3; i < 8; i++ {
			err := broker.Send(ctx, i)
			check.NotError(t, err)
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
			queue.IteratorWaitPop(ctx),
			fnx.MakeHandler(queue.Add),
			queue.Len,
			BrokerOptions{},
		)
		defer broker.Stop()

		sub := broker.Subscribe(ctx)
		if sub == nil {
			t.Fatal("failed to subscribe")
		}
		defer broker.Unsubscribe(ctx, sub)

		// Send first message - worker pulls and blocks on dispatch
		broker.Send(ctx, "msg-0")
		time.Sleep(20 * time.Millisecond)

		// Fill the queue to capacity (HardLimit: 2)
		broker.Send(ctx, "msg-1")
		broker.Send(ctx, "msg-2")
		time.Sleep(20 * time.Millisecond)

		// Try to send more - these should be dropped (queue full)
		broker.Send(ctx, "dropped-3")
		broker.Send(ctx, "dropped-4")
		broker.Send(ctx, "dropped-5")
		time.Sleep(20 * time.Millisecond)

		// Consume messages
		received := []string{}
		for i := 0; i < 3; i++ {
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
		time.Sleep(20 * time.Millisecond)

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
