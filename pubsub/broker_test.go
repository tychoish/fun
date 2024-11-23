package pubsub

import (
	"context"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
)

type BrokerFixture[T comparable] struct {
	Name        string
	Construtor  func(ctx context.Context, t *testing.T) *Broker[T]
	BufferSize  int
	NonBlocking bool
}

func GenerateFixtures[T comparable](elems []T) []BrokerFixture[T] {
	return []BrokerFixture[T]{
		{
			Name: "Parallel/ZeroBuffer",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{ParallelDispatch: true})
			},
		},
		{
			Name: "DistributorBuffer",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				d, err := NewDeque[T](DequeOptions{Unlimited: true})
				if err != nil {
					t.Fatal(err)
				}
				return MakeDistributorBroker(ctx, d.Distributor(), BrokerOptions{})
			},
		},
		{
			Name: "Serial/ZeroBuffer",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{ParallelDispatch: false})
			},
		},
		{
			Name: "Parallel/FullyBuffered/NoBlock",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: true,
					BufferSize:       len(elems),
				})
			},
			BufferSize: len(elems),
		},
		{
			Name: "Parallel/HalfBuffered/NoBlock",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: true,
					BufferSize:       len(elems) / 2,
				})
			},
			BufferSize: len(elems) / 2,
		},
		{
			Name: "Parallel/FullyBuffered",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: true,
					BufferSize:       len(elems),
				})
			},
			BufferSize: len(elems),
		},
		{
			Name: "Serial/FullyBuffered",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: false,
					BufferSize:       len(elems),
				})
			},
			BufferSize: len(elems),
		},
		{
			Name: "Parallel/HalfBufferedNonBlocking",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: true,
					BufferSize:       len(elems) / 2,
				})
			},
			BufferSize: len(elems) / 2,
		},
		{
			Name: "Serial/HalfBuffered",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: false,
					BufferSize:       len(elems) / 2,
				})
			},
			BufferSize: len(elems) / 2,
		},
		{
			Name: "Parallel/DoubleBuffered",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: true,
					BufferSize:       len(elems) * 2,
				})
			},
			BufferSize: len(elems) * 2,
		},
		{
			Name: "Serial/DoubleBuffered",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				return NewBroker[T](ctx, BrokerOptions{
					ParallelDispatch: false,
					BufferSize:       len(elems) * 2,
				})
			},
			BufferSize: len(elems) * 2,
		},
		{
			Name: "Queue/Serial/Unbuffered/OneWorker",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   10,
					SoftQuota:   5,
					BurstCredit: 2,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker(ctx, queue, BrokerOptions{
					ParallelDispatch: false,
				})
			},
		},
		{
			Name: "Queue/Serial/Unbuffered/TwoWorker",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   10,
					SoftQuota:   5,
					BurstCredit: 2,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: false,
					WorkerPoolSize:   2,
				})
			},
		},
		{
			Name: "Queue/Parallel/Unbuffered/TwoWorker",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   10,
					SoftQuota:   5,
					BurstCredit: 2,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: true,
					WorkerPoolSize:   2,
				})
			},
		},
		{
			Name: "Queue/Serial/Unbuffered/EightWorker",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   4,
					SoftQuota:   2,
					BurstCredit: 1,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker[T](ctx, queue, BrokerOptions{
					WorkerPoolSize:   8,
					ParallelDispatch: false,
				})
			},
		},
		{
			Name: "Queue/Parallel/Unbuffered/EightWorker",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				queue, err := NewQueue[T](QueueOptions{
					HardLimit:   4,
					SoftQuota:   2,
					BurstCredit: 1,
				})
				if err != nil {
					t.Fatal(err)
				}
				return NewQueueBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: true,
					WorkerPoolSize:   8,
				})
			},
		},
		// deque cases
		{
			Name: "Deque/Serial/Unbuffered/OneWorker",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				queue := NewUnlimitedDeque[T]()
				return NewDequeBroker(ctx, queue, BrokerOptions{
					ParallelDispatch: false,
				})
			},
		},
		{
			Name: "Deque/Serial/Unbuffered/TwoWorker",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				queue := NewUnlimitedDeque[T]()
				return NewDequeBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: false,
					WorkerPoolSize:   2,
				})
			},
		},
		{
			Name: "Deque/Parallel/Unbuffered/TwoWorker",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				queue := NewUnlimitedDeque[T]()
				return NewDequeBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: true,
				})
			},
		},
		{
			Name: "Deque/Serial/Unbuffered/EightWorker",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				queue := NewUnlimitedDeque[T]()
				return NewDequeBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: false,
					WorkerPoolSize:   8,
				})
			},
		},
		{
			Name: "Deque/Parallel/Unbuffered/EightWorker",
			Construtor: func(ctx context.Context, _ *testing.T) *Broker[T] {
				queue := NewUnlimitedDeque[T]()
				return NewDequeBroker[T](ctx, queue, BrokerOptions{
					ParallelDispatch: true,
					WorkerPoolSize:   8,
				})
			},
		},
		{
			Name: "Lifo/Parallel/Unbuffered/EightWorker",
			Construtor: func(ctx context.Context, t *testing.T) *Broker[T] {
				broker, err := ers.WithRecoverDo(func() *Broker[T] {
					return NewLIFOBroker[T](ctx, BrokerOptions{
						ParallelDispatch: true,
						WorkerPoolSize:   8,
					}, len(elems)-5)
				})
				if err != nil {
					t.Fatal(err)
				}

				return broker
			},
		},
	}
}

func RunBrokerTests[T comparable](pctx context.Context, t *testing.T, elems []T) {
	t.Parallel()
	for _, fix := range GenerateFixtures(elems) {
		t.Run(fix.Name, func(t *testing.T) {
			fix := fix
			t.Parallel()
			t.Run("EndToEnd", func(t *testing.T) {
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

				seen1 := make(map[T]struct{}, len(elems))
				seen2 := make(map[T]struct{}, len(elems))
				wg := &fun.WaitGroup{}
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
							seen1[str] = struct{}{}
						}
						if len(seen1) == total {
							return
						}
						if len(seen1)/2 > total {
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
							seen2[str] = struct{}{}
						}
						if len(seen2) == total {
							return
						}
						if len(seen2)/2 > total {
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
				if len(seen1) == len(seen2) {
					checkMatchingSets(t, seen1, seen2)
				} else if len(seen1) == 0 && len(seen2) == 0 {
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
			if fix.NonBlocking {
				t.Run("NonBlocking", func(t *testing.T) {
					elems := elems
					opts := fix

					t.Parallel()
					ctx, cancel := context.WithTimeout(pctx, 2*time.Second)
					defer cancel()

					broker := opts.Construtor(pctx, t)

					wg := &fun.WaitGroup{}
					ch1 := broker.Subscribe(ctx)
					count := &atomic.Int32{}

					wg.Add(1)
					go func() {
						defer wg.Done()
						defer broker.Unsubscribe(ctx, ch1)
						for range ch1 {
							count.Add(1)
							if int(count.Load()) == len(elems) {
								return
							}
						}
					}()

					if stat := broker.Stats(ctx); stat.Subscriptions != 1 {
						t.Error(stat)
					}

					for i := 0; i < 20; i++ {
						wg.Add(1)
						go func(_ int) {
							defer wg.Done()
							for idx := range elems {
								broker.Publish(ctx, elems[idx])

								if int(count.Load()) == len(elems) {
									return
								}
							}
						}(i)
					}

					if stat := broker.Stats(ctx); stat.BufferDepth < 10 {
						t.Error(stat)
					}

					wg.Wait(ctx)
					broker.Stop()
					broker.Wait(ctx)
					if int(count.Load()) != len(elems) {
						t.Log("context.Err=", ctx.Err())
						t.Fatal("saw", count, "out of", len(elems))
					}
				})
			}
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
	t.Run("NewLifoBrokerRejectsNegativeCapacity", func(t *testing.T) {
		_, err := ers.WithRecoverDo(func() *Broker[string] {
			return NewLIFOBroker[string](ctx, BrokerOptions{}, -42)
		})
		if err != nil {
			// we round all capacities up to 1
			t.Fatal(err)
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
			fun.Invariant.IsTrue(queue.Close() == nil, "cannot error")
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
			fun.Invariant.IsTrue(queue.Close() == nil, "cannot error")
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
			fun.Invariant.IsTrue(queue.Close() == nil, "cannot error")
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

	t.Run("Populate", func(t *testing.T) {
		t.Parallel()
		input := randomIntSlice(100)

		iter := fun.SliceIterator(input)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		broker := NewBroker[int](ctx, BrokerOptions{})
		seen := &dt.Set[int]{}
		seen.Synchronize()
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
			if err := broker.Populate(iter).Run(ctx); err != nil {
				t.Error(err)
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
