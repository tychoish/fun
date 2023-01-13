package fun

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

func TestBroker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for scope, elems := range map[string][]string{
		"Basic": {"a", "b", "c", "d", "e", "f", "g"},
		"ExtraLarge": func() []string {
			out := make([]string, 100)
			for idx := range out {
				out[idx] = fmt.Sprint("value=", idx)
			}
			return out
		}(),
	} {
		t.Run(scope, func(t *testing.T) {
			t.Run("Simple", func(t *testing.T) {
				for optName, opts := range map[string]BrokerOptions{
					"Parallel/ZeroBuffer":     {ParallelDispatch: true},
					"Serial/ZeroBuffer":       {ParallelDispatch: false},
					"Parallel/FullyBuffered":  {ParallelDispatch: true, BufferSize: len(elems)},
					"Serial/FullyBuffered":    {ParallelDispatch: false, BufferSize: len(elems)},
					"Parallel/HalfBuffered":   {ParallelDispatch: true, BufferSize: len(elems) / 2},
					"Serial/HalfBuffered":     {ParallelDispatch: false, BufferSize: len(elems) / 2},
					"Parallel/DoubleBuffered": {ParallelDispatch: true, BufferSize: len(elems) * 2},
					"Serial/DoubleBuffered":   {ParallelDispatch: false, BufferSize: len(elems) * 2},
				} {
					t.Run(optName, func(t *testing.T) {
						broker := NewBroker[string](opts)
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

						go func() {
							defer wgState.Add(-1)
							defer wg.Done()
							for {
								select {
								case <-ctx.Done():
									return
								case <-sig:
									return
								case str := <-ch1:
									seen1[str] = struct{}{}
								}
								if len(seen1) == len(elems) {
									return
								}
							}
						}()

						go func() {
							defer wgState.Add(-1)
							defer wg.Done()
							for {
								select {
								case <-ctx.Done():
									return
								case <-sig:
									return
								case str := <-ch2:
									seen2[str] = struct{}{}
								}
								if len(seen2) == len(elems) {
									return
								}
							}
						}()

						go func() {
							defer wg.Done()
							for idx := range elems {
								broker.Publish(ctx, elems[idx])
							}
							timer := time.NewTimer(time.Second)
							defer timer.Stop()
							ticker := time.NewTicker(10 * time.Millisecond)
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
						if opts.BufferSize == 0 {
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
					t.Run(optName+"/NonBlocking", func(t *testing.T) {
						opts.NonBlockingSubscriptions = true
						defer func() { opts.NonBlockingSubscriptions = false }()
						broker := NewBroker[string](opts)
						broker.Start(ctx)

						wg := &WaitGroup{}
						ch1 := broker.Subscribe(ctx)
						var count int

						wg.Add(1)
						go func() {
							defer wg.Done()
							for range ch1 {
								count++
								if count == len(elems) {
									broker.Unsubscribe(ctx, ch1)
									return
								}
							}
						}()

						for i := 0; i < 10; i++ {
							wg.Add(1)
							go func(id int) {
								defer wg.Done()
								for idx := range elems {
									broker.Publish(ctx, fmt.Sprint(id, "=>", elems[idx]))
								}
							}(i)
						}
						wg.Wait(ctx)
						if count != len(elems) {
							t.Fatal("did not subscribe", count)
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
