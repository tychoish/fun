package fun

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestBroker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Parallel()

	for _, scope := range []struct {
		Name  string
		Elems []string
	}{
		{
			Name:  "Basic",
			Elems: []string{"a", "b", "c", "d", "e", "f", "g"},
		},
		{
			Name: "ExtraLarge",
			Elems: func() []string {
				out := make([]string, 50)
				for idx := range out {
					out[idx] = fmt.Sprint("value=", idx)
				}
				return out
			}()},
	} {
		t.Run(scope.Name, func(t *testing.T) {
			elems := scope.Elems
			t.Run("Simple", func(t *testing.T) {
				for _, opts := range []struct {
					Name string
					Opts BrokerOptions
				}{
					{
						Name: "Parallel/ZeroBuffer",
						Opts: BrokerOptions{
							ParallelDispatch: true,
						},
					},
					{
						Name: "Serial/ZeroBuffer",
						Opts: BrokerOptions{
							ParallelDispatch: false,
						},
					},
					{
						Name: "Parallel/FullyBuffered",
						Opts: BrokerOptions{
							ParallelDispatch: true,
							BufferSize:       len(elems),
						},
					},
					{
						Name: "Serial/FullyBuffered",
						Opts: BrokerOptions{
							ParallelDispatch: false,
							BufferSize:       len(elems),
						},
					},
					{
						Name: "Parallel/HalfBuffered",
						Opts: BrokerOptions{
							ParallelDispatch: true,
							BufferSize:       len(elems) / 2,
						},
					},
					{
						Name: "Serial/HalfBuffered",
						Opts: BrokerOptions{
							ParallelDispatch: false,
							BufferSize:       len(elems) / 2,
						},
					},
					{
						Name: "Parallel/DoubleBuffered",
						Opts: BrokerOptions{
							ParallelDispatch: true,
							BufferSize:       len(elems) * 2,
						},
					},
					{
						Name: "Serial/DoubleBuffered",
						Opts: BrokerOptions{
							ParallelDispatch: false,
							BufferSize:       len(elems) * 2},
					},
				} {
					t.Run(opts.Name, func(t *testing.T) {
						ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
						defer cancel()

						elems := scope.Elems
						opts := opts

						t.Parallel()
						broker := NewBroker[string](opts.Opts)
						broker.Start(ctx)

						// sometimes start it
						// a second time,
						// should have no effect
						if rand.Float32() > 0.5 {
							broker.Start(ctx)
						}

						ch1 := broker.Subscribe(ctx)
						ch2 := broker.Subscribe(ctx)

						seen1 := make(map[string]struct{}, len(elems))
						seen2 := make(map[string]struct{}, len(elems))
						wg := &WaitGroup{}
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
								if opts.Opts.BufferSize == 0 && len(seen1) == total {
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
								if opts.Opts.BufferSize == 0 && len(seen2) == total {
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
							timer := time.NewTimer(500 * time.Millisecond)
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
						if opts.Opts.BufferSize == 0 {
							checkMatchingSets(t, seen1, seen2)
						} else if len(seen1) == 0 && len(seen2) == 0 {
							t.Error("should observe some events")

						}

						broker.Stop()
						broker.Wait(ctx)
						cctx, ccancel := context.WithCancel(ctx)
						ccancel()
						if broker.Subscribe(cctx) != nil {
							t.Error("should not subscribe with canceled context")
						}
						broker.Unsubscribe(cctx, ch1)
					})
					t.Run(opts.Name+"/NonBlocking", func(t *testing.T) {
						elems := scope.Elems
						opts := opts

						t.Parallel()

						ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
						defer cancel()

						opts.Opts.NonBlockingSubscriptions = true
						broker := NewBroker[string](opts.Opts)
						broker.Start(ctx)

						wg := &WaitGroup{}
						ch1 := broker.Subscribe(ctx)
						var count int

						wg.Add(1)
						go func() {
							defer wg.Done()
							defer broker.Unsubscribe(ctx, ch1)
							for range ch1 {
								count++
								if count == len(elems) {
									return
								}
							}
						}()

						for i := 0; i < 30; i++ {
							wg.Add(1)
							go func(id int) {
								defer wg.Done()
								for idx := range elems {
									broker.Publish(ctx, fmt.Sprint(id, "=>", elems[idx]))
									runtime.Gosched()
								}
							}(i)
						}

						wg.Wait(ctx)

						if count != len(elems) {
							t.Log(ctx.Err())
							t.Fatal("saw", count, "out of", len(elems))
						}
					})

				}
			})
		})
	}
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
