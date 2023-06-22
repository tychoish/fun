package pubsub

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/testt"
)

func TestDistributor(t *testing.T) {
	// TODO: add specific unit tests, most of the distributor code
	// is tested via the broker.
	t.Parallel()
	t.Run("EdgeCases", func(t *testing.T) {
		t.Parallel()
		t.Run("Channel", func(t *testing.T) {
			t.Run("Cancelation", func(t *testing.T) {
				start := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				defer cancel()

				ch := make(chan string, 1)
				ch <- "merlin"
				buf := DistributorChannel(ch)
				sig := make(chan struct{})
				go func() {
					defer close(sig)
					err := buf.Send(ctx, "kip")
					if err == nil {
						t.Error("expected error")
					}
					errs := dt.Unwind(err)
					if len(errs) != 1 {
						t.Error(len(errs))
					}
				}()
				<-sig
				dur := time.Since(start)
				if dur < 10*time.Millisecond {
					t.Error(dur)
				}
			})
		})
		t.Run("Iterator", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			queue := NewUnlimitedQueue[string]()
			_ = queue.Add("string")
			_ = queue.Add("strong")
			_ = queue.Add("strung")

			dist := queue.Distributor()

			set := &dt.Set[string]{}
			set.Order()
			seen := 0
			if queue.tracker.len() != 3 {
				t.Fatal(queue.tracker.len())
			}

			iter := dist.Iterator()
			go func() {
				time.Sleep(100 * time.Millisecond)
				queue.Close()
			}()
			if err := iter.Observe(ctx, func(in string) { set.Add(in); seen++ }); err != nil {
				t.Fatal(seen, err)
			}
			if iter.Next(ctx) {
				t.Error("iterator should be empty")
			}
			if set.Len() != 3 {
				t.Error(set.Len())
			}
			if seen != 3 {
				t.Error(seen)
			}
			if queue.tracker.len() != 0 {
				t.Fatal(queue.tracker.len())
			}
			if iter.Value() != "" {
				t.Error(iter.Value())
			}
		})
	})

	t.Run("Table", func(t *testing.T) {
		const iterSize = 100
		t.Parallel()
		t.Run("Integer", func(t *testing.T) {
			t.Parallel()
			t.Run("Buffered", func(t *testing.T) {
				RunDistributorTests(t, iterSize, func() *fun.Iterator[int] {
					return fun.SliceIterator(randomIntSlice(iterSize))
				})
			})
			t.Run("Generated", func(t *testing.T) {
				RunDistributorTests(t, iterSize, func() *fun.Iterator[int] {
					ch := make(chan int)
					go func() {
						defer close(ch)
						for _, item := range randomIntSlice(iterSize) {
							ch <- item
						}
					}()

					return fun.ChannelIterator(ch)
				})
			})
		})
		t.Run("StringSimple", func(t *testing.T) {
			RunDistributorTests(t, iterSize, func() *fun.Iterator[string] {
				out := make([]string, iterSize)
				for i := 0; i < iterSize; i++ {
					out[i] = fmt.Sprintf("idx=%d random=%d", i, rand.Int63())
				}
				return fun.SliceIterator(out)
			})
		})
		t.Run("RandomString", func(t *testing.T) {
			RunDistributorTests(t, iterSize, func() *fun.Iterator[string] {
				ch := make(chan string)
				go func() {
					defer close(ch)
					for i := 0; i < iterSize; i++ {
						str := [32]byte{}
						for p := range str {
							str[p] = byte(int8(rand.Intn(math.MaxInt8)))
						}
						ch <- string(str[:])
					}
				}()
				return fun.ChannelIterator(ch)
			})
		})
	})
}

type DistGenerator[T comparable] struct {
	Name      string
	Generator func(*testing.T, *fun.Iterator[T]) Distributor[T]
}

func MakeGenerators[T comparable](size int) []DistGenerator[T] {
	return []DistGenerator[T]{
		{
			Name: "ChannelBuffered",
			Generator: func(t *testing.T, input *fun.Iterator[T]) Distributor[T] {
				ctx := testt.Context(t)
				out := make(chan T, size)
				for input.Next(ctx) {
					out <- input.Value()
				}
				close(out)
				assert.NotError(t, input.Close())
				return DistributorChannel(out)
			},
		},
		{
			Name: "DirectChannel",
			Generator: func(t *testing.T, input *fun.Iterator[T]) Distributor[T] {
				ctx := testt.Context(t)
				out := make(chan T)
				go func() {
					defer close(out)

					for input.Next(ctx) {
						select {
						case out <- input.Value():
						case <-ctx.Done():
							return
						}
					}

					check.NotError(t, input.Close())
				}()
				return DistributorChannel(out)
			},
		},
		{
			Name: "DistChannel",
			Generator: func(t *testing.T, input *fun.Iterator[T]) Distributor[T] {
				ctx := testt.Context(t)
				ch := make(chan T)
				out := DistributorChannel(ch)
				go func() {
					defer close(ch)
					for input.Next(ctx) {
						err := out.Send(ctx, input.Value())
						if err != nil {
							break
						}
					}

					check.NotError(t, input.Close())
				}()
				return out
			},
		},
		{
			Name: "DistQueue",
			Generator: func(t *testing.T, input *fun.Iterator[T]) Distributor[T] {
				ctx := testt.Context(t)
				queue := NewUnlimitedQueue[T]()
				out := queue.Distributor()
				send := out.Processor()
				go func() {
					defer queue.Close()
					for input.Next(ctx) {
						err := send(ctx, input.Value())
						if err != nil {
							break
						}
					}

					check.NotError(t, input.Close())
				}()
				return out
			},
		},
	}
}

type DistCase[T comparable] struct {
	Name string
	Test func(*testing.T, Distributor[T])
}

func MakeCases[T comparable](size int) []DistCase[T] {
	return []DistCase[T]{
		{
			Name: "Seen",
			Test: func(t *testing.T, d Distributor[T]) {
				ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
				seen := &dt.Set[T]{}
				seen.Synchronize()
				signal := make(chan struct{})
				go func() {
					defer close(signal)
					for {
						pop, err := d.Receive(ctx)
						if err != nil {
							break
						}
						seen.Add(pop)
					}
				}()
				<-signal
				assert.Equal(t, size, seen.Len())
				assert.Zero(t, d.Len())
			},
		},
		{
			Name: "PoolSeen",
			Test: func(t *testing.T, d Distributor[T]) {
				ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
				seen := &dt.Set[T]{}
				seen.Synchronize()
				wg := &sync.WaitGroup{}
				receive := d.Producer()
				for i := 0; i < 8; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							pop, err := receive(ctx)
							if err != nil {
								break
							}
							seen.Add(pop)
						}
					}()

				}
				wg.Wait()
				assert.Equal(t, size, seen.Len())
				assert.Zero(t, d.Len())
				assert.NotError(t, ctx.Err())
			},
		},
		{
			Name: "Count",
			Test: func(t *testing.T, d Distributor[T]) {
				ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
				count := &atomic.Int64{}
				signal := make(chan struct{})
				go func() {
					defer close(signal)
					for {
						_, err := d.Receive(ctx)
						if err != nil {
							break
						}
						count.Add(1)
					}
				}()
				<-signal
				assert.Equal(t, size, int(count.Load()))
				assert.Zero(t, d.Len())
			},
		},
		{
			Name: "PoolCount",
			Test: func(t *testing.T, d Distributor[T]) {
				ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
				count := &atomic.Int64{}
				wg := &sync.WaitGroup{}
				for i := 0; i < 8; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							_, err := d.Receive(ctx)
							if err != nil {
								break
							}
							count.Add(1)
						}
					}()

				}
				wg.Wait()
				assert.Equal(t, size, int(count.Load()))
				assert.Zero(t, d.Len())
			},
		},
	}
}

func RunDistributorTests[T comparable](t *testing.T, size int, producer func() *fun.Iterator[T]) {
	t.Parallel()
	for _, gent := range MakeGenerators[T](size) {
		t.Run(gent.Name, func(t *testing.T) {
			for _, tt := range MakeCases[T](size) {
				t.Run(tt.Name, func(t *testing.T) {
					tt.Test(t, gent.Generator(t, producer()))
				})
			}
		})
	}
}
