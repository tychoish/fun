package srv

import (
	"context"
	"errors"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
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

		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("RunCollect", func(t *testing.T) {
		t.Run("Happy", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			RunWaitCollect(ec, svc)(ctx)
			if err := ec.Resolve(); err != nil {
				t.Error(err)
			}
		})
		t.Run("AlreadyStarted", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			if err := svc.Start(ctx); err != nil {
				t.Error(err)
			}

			RunWaitCollect(ec, svc)(ctx)

			if err := ec.Resolve(); !errors.Is(err, ErrServiceAlreadyStarted) {
				t.Error(err)
			}
		})
	})
	t.Run("RunObserve", func(t *testing.T) {
		t.Run("Happy", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			RunWaitObserve(ec.Add, svc)(ctx)
			if err := ec.Resolve(); err != nil {
				t.Error(err)
			}
		})
		t.Run("AlreadyStarted", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ec := &erc.Collector{}
			svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
			if err := svc.Start(ctx); err != nil {
				t.Error(err)
			}

			RunWaitObserve(ec.Add, svc)(ctx)

			if err := ec.Resolve(); !errors.Is(err, ErrServiceAlreadyStarted) {
				t.Error(err)
			}
		})
	})

	t.Run("RunWait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := Wait(itertool.Variadic(fun.WaitFunc(func(context.Context) { time.Sleep(10 * time.Millisecond) })))
		start := time.Now()
		RunWait(svc)(ctx)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Millisecond {
			t.Error(time.Since(start))
		}
	})

	t.Run("ProcessIterator", func(t *testing.T) {
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
	t.Run("ProcessIteratorPrallel", func(t *testing.T) {
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
		if time.Since(start) < 5*time.Millisecond || time.Since(start) > 10*time.Millisecond {
			t.Error(time.Since(start))
		}
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
