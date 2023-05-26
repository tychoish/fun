package srv

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/testt"
)

func TestImplementationHelpers(t *testing.T) {
	t.Run("Wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
		start := time.Now()
		if err := svc.Start(ctx); err != nil {
			t.Error(err)
		}

		if err := svc.Wait(); err != nil {
			t.Error(err)
		}

		dur := time.Since(start)
		if dur < 10*time.Millisecond || dur > 20*time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("Process", func(t *testing.T) {
		t.Parallel()
		t.Run("Large", func(t *testing.T) {
			count := atomic.Int64{}
			srv := ProcessIterator(
				makeIterator(100),
				func(_ context.Context, in int) error { count.Add(1); return nil },
				itertool.Options{NumWorkers: runtime.NumCPU()},
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
		})
		t.Run("Large", func(t *testing.T) {
			count := atomic.Int64{}
			srv := ProcessIterator(
				makeIterator(50),
				func(_ context.Context, in int) error {
					time.Sleep(5 * time.Millisecond)
					count.Add(1)
					return nil
				},
				itertool.Options{NumWorkers: 50},
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			start := time.Now()
			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 50 {
				t.Error(count.Load())
			}
			if time.Since(start) < 5*time.Millisecond || time.Since(start) > 20*time.Millisecond {
				t.Error(time.Since(start))
			}
		})
	})

	t.Run("WorkerPool", func(t *testing.T) {
		t.Parallel()
		t.Run("Small", func(t *testing.T) {
			count := &atomic.Int64{}
			srv := WorkerPool(
				makeQueue(t, 100, count),
				itertool.Options{NumWorkers: runtime.NumCPU()},
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}

			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
		})
		t.Run("Large", func(t *testing.T) {
			count := &atomic.Int64{}
			srv := WorkerPool(
				makeQueue(t, 100, count),
				itertool.Options{NumWorkers: 50},
			)
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
			assert.NotError(t, ctx.Err())
		})
	})

	t.Run("ObserverWorkerPool", func(t *testing.T) {
		t.Parallel()
		t.Run("Small", func(t *testing.T) {
			count := &atomic.Int64{}
			errCount := &atomic.Int64{}
			srv := ObserverWorkerPool(
				makeErroringQueue(t, 100, count),
				func(err error) {
					t.Log(err)
					check.Error(t, err)
					errCount.Add(1)
				},
				itertool.Options{NumWorkers: runtime.NumCPU()},
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}

			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
			if errCount.Load() != 100 {
				t.Error("did not observe correct errors", errCount.Load())
			}
		})
		t.Run("Large", func(t *testing.T) {
			count := &atomic.Int64{}
			errCount := &atomic.Int64{}
			srv := ObserverWorkerPool(
				makeErroringQueue(t, 100, count),
				func(err error) {
					check.Error(t, err)
					errCount.Add(1)
				},
				itertool.Options{NumWorkers: 50},
			)
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
			if errCount.Load() != 100 {
				t.Error("did not observe correct errors", errCount.Load())
			}
			assert.NotError(t, ctx.Err())
		})
	})

	t.Run("Broker", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		broker := pubsub.NewBroker[int](ctx, pubsub.BrokerOptions{})
		srv := Broker(broker)
		if err := srv.Start(ctx); err != nil {
			t.Fatal(err)
		}
		ch := broker.Subscribe(ctx)
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			num := <-ch
			if num != 42 {
				t.Error(num)
			}
		}()

		broker.Publish(ctx, 42)
		fun.WaitChannel(sig)(ctx)
	})
}

func makeIterator(size int) fun.Iterator[int] {
	slice := make([]int, size)
	for i := 0; i < size; i++ {
		slice[i] = rand.Intn(size)
	}
	return itertool.Slice(slice)
}

func makeQueue(t *testing.T, size int, count *atomic.Int64) *pubsub.Queue[fun.Worker] {
	t.Helper()
	queue := pubsub.NewUnlimitedQueue[fun.Worker]()

	for i := 0; i < size; i++ {
		assert.NotError(t, queue.Add(func(ctx context.Context) error {
			time.Sleep(2 * time.Millisecond)
			count.Add(1)
			return nil
		}))
	}
	queue.Close()
	return queue
}

func makeErroringQueue(t *testing.T, size int, count *atomic.Int64) *pubsub.Queue[fun.Worker] {
	t.Helper()
	queue := pubsub.NewUnlimitedQueue[fun.Worker]()

	for i := 0; i < size; i++ {
		idx := i
		assert.NotError(t, queue.Add(func(ctx context.Context) error {
			time.Sleep(2 * time.Millisecond)
			count.Add(1)
			return fmt.Errorf("%d.%q", idx, t.Name())
		}))
	}
	queue.Close()
	return queue
}
