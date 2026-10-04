package irt

import (
	"context"
	"iter"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUseCaseConcurrentIterationOfOneSequence(t *testing.T) {
	for _, c := range ucCases() {
		t.Run(c.name, func(t *testing.T) {
			if c.single {
				t.Skip("documented single-use")
			}
			src := new(ucSource)
			seq := c.build(t, src.seq(20))
			want, _ := ucDrive(c.build(t, new(ucSource).seq(20)), 0)
			want = ucNorm(c, want)

			const readers = 4
			results := make([][]int, readers)
			var wg sync.WaitGroup
			for i := range readers {
				wg.Go(func() { results[i], _ = ucDrive(seq, 0) })
			}
			wg.Wait()
			for i, got := range results {
				if !slices.Equal(ucNorm(c, got), want) {
					t.Fatalf("reader %d saw %v, want %v", i, got, want)
				}
			}
			ucReleased(t, src)
		})
	}
}

func TestUseCaseShardOuterBreak(t *testing.T) {
	const items, shards = 30, 4
	for _, handed := range []int{1, 2, shards} {
		t.Run("Unconsumed"+strconv.Itoa(handed), func(t *testing.T) {
			base := runtime.NumGoroutine()
			src := new(ucSource)
			n := 0
			for range Shard(t.Context(), shards, src.seq(items)) {
				if n++; n == handed {
					break
				}
			}
			// shards were never iterated: the source was never pulled
			// or has been released.
			ucReleased(t, src)
			goroutinesAtMost(t, base)
		})
		t.Run("Consumed"+strconv.Itoa(handed), func(t *testing.T) {
			base := runtime.NumGoroutine()
			src := new(ucSource)
			var total int
			n := 0
			for shard := range Shard(t.Context(), shards, src.seq(items)) {
				total += Count(shard)
				if n++; n == handed {
					break
				}
			}
			if total != items {
				t.Fatalf("first shard should drain everything: got %d of %d", total, items)
			}
			ucReleased(t, src)
			goroutinesAtMost(t, base)
		})
	}
	t.Run("RetainedShardsAfterBreak", func(t *testing.T) {
		src := new(ucSource)
		var kept []iter.Seq[int]
		for shard := range Shard(t.Context(), shards, src.seq(items)) {
			kept = append(kept, shard)
			if len(kept) == 2 {
				break
			}
		}
		// the two handed-out shards still work and together see everything.
		var got []int
		for _, shard := range kept {
			got = append(got, Collect(shard)...)
		}
		slices.Sort(got)
		if len(got) != items {
			t.Fatalf("retained shards saw %d of %d items", len(got), items)
		}
		ucReleased(t, src)
	})
	t.Run("Shard2", func(t *testing.T) {
		src := new(ucSource)
		n := 0
		for shard := range Shard2(t.Context(), shards, ucLift2(src.seq(items))) {
			if n++; n == 2 {
				if Count2(shard) != items {
					t.Fatal("a lone shard should see every pair")
				}
				break
			}
		}
		ucReleased(t, src)
	})
}

func TestUseCaseShardSizes(t *testing.T) {
	for _, num := range []int{1, 2, 7, 50} {
		for _, items := range []int{0, 1, 10, 500} {
			t.Run(strconv.Itoa(num)+"x"+strconv.Itoa(items), func(t *testing.T) {
				src := new(ucSource)
				var mu sync.Mutex
				var all []int
				var wg sync.WaitGroup
				for shard := range Shard(t.Context(), num, src.seq(items)) {
					wg.Go(func() {
						part := Collect(shard)
						mu.Lock()
						all = append(all, part...)
						mu.Unlock()
					})
				}
				wg.Wait()
				slices.Sort(all)
				if len(all) != items {
					t.Fatalf("saw %d of %d items", len(all), items)
				}
				for i, v := range all {
					if v != i+1 {
						t.Fatalf("item %d lost or duplicated", i+1)
					}
				}
				ucReleased(t, src)
			})
		}
	}
}

func TestUseCasePoolProcessesEachElementOnce(t *testing.T) {
	for _, num := range []int{-1, 0, 1, 2, 5, 64} {
		for _, items := range []int{0, 1, 3, 200} {
			t.Run(strconv.Itoa(num)+"x"+strconv.Itoa(items), func(t *testing.T) {
				base := runtime.NumGoroutine()
				seen := make([]atomic.Int32, items+1)
				got := Collect(Pool(t.Context(), num, Range(1, items), func(v int) int {
					seen[v].Add(1)
					return v * 2
				}))
				slices.Sort(got)
				if len(got) != items {
					t.Fatalf("got %d results for %d items", len(got), items)
				}
				for i, v := range got {
					if v != (i+1)*2 {
						t.Fatalf("result %d: %d", i, v)
					}
				}
				for i := 1; i <= items; i++ {
					if seen[i].Load() != 1 {
						t.Fatalf("element %d processed %d times", i, seen[i].Load())
					}
				}
				goroutinesAtMost(t, base)
			})
		}
	}
}

func TestUseCasePoolRunsOpsInParallel(t *testing.T) {
	// every op waits until `workers` ops are in flight at once, which
	// can only happen if the pool really runs them concurrently.
	for _, workers := range []int{2, 4, 8} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			var arrived sync.WaitGroup
			arrived.Add(workers)
			var once sync.Once
			release := make(chan struct{})
			out := Pool(t.Context(), workers, Range(1, workers), func(v int) int {
				arrived.Done()
				arrived.Wait()
				once.Do(func() { close(release) })
				<-release
				return v
			})
			done := make(chan []int, 1)
			go func() { done <- Collect(out) }()
			if got := ucRecvIrt(t, done); len(got) != workers {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func ucRecvIrt[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: operation did not complete")
		panic("unreachable")
	}
}

func TestUseCasePoolCancelStopsWithoutConsumingMore(t *testing.T) {
	for _, workers := range []int{1, 4} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			base := runtime.NumGoroutine()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var processed atomic.Int64
			count := 0
			for range Pool(ctx, workers, Monotonic(), func(v int) int { processed.Add(1); return v }) {
				if count++; count == 10 {
					cancel()
				}
			}
			if count < 10 {
				t.Fatalf("only %d results", count)
			}
			goroutinesAtMost(t, base)
		})
	}
}

func TestUseCaseWithBufferSizes(t *testing.T) {
	for _, size := range []int{-1, 0, 1, 7, 1000} {
		t.Run(strconv.Itoa(size)+"/Order", func(t *testing.T) {
			base := runtime.NumGoroutine()
			got := Collect(WithBuffer(t.Context(), Range(1, 500), size))
			if !slices.Equal(got, Collect(Range(1, 500))) {
				t.Fatalf("buffer of %d lost order or items (%d items)", size, len(got))
			}
			goroutinesAtMost(t, base)
		})
		t.Run(strconv.Itoa(size)+"/Empty", func(t *testing.T) {
			if got := Collect(WithBuffer(t.Context(), Zero[int](), size)); len(got) != 0 {
				t.Fatalf("got %v", got)
			}
		})
		t.Run(strconv.Itoa(size)+"/CancelWhileConsuming", func(t *testing.T) {
			base := runtime.NumGoroutine()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			n := 0
			for range WithBuffer(ctx, Monotonic(), size) {
				if n++; n == 3 {
					cancel()
				}
			}
			goroutinesAtMost(t, base)
		})
	}
}

func TestUseCaseAsChannelLifecycle(t *testing.T) {
	t.Run("FullDeliveryThenStopIsNil", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ch, stop := AsChannel(t.Context(), Range(1, 50))
		var got []int
		for v := range ch {
			got = append(got, v)
		}
		if len(got) != 50 {
			t.Fatalf("got %d items", len(got))
		}
		for range 3 {
			if err := stop(); err != nil {
				t.Fatal(err)
			}
		}
		goroutinesAtMost(t, base)
	})
	t.Run("ConcurrentStops", func(t *testing.T) {
		base := runtime.NumGoroutine()
		src := new(ucSource)
		ch, stop := AsChannel(t.Context(), src.seq(1000))
		<-ch
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() { _ = stop() })
		}
		wg.Wait()
		// the channel closes (drained of at most the one in-flight item).
		for range ch {
		}
		ucReleased(t, src)
		goroutinesAtMost(t, base)
	})
	t.Run("ContextCancelClosesChannel", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		ch, stop := AsChannel(ctx, Monotonic())
		<-ch
		cancel()
		for range ch {
		}
		if err := stop(); err != nil {
			t.Fatal(err)
		}
		goroutinesAtMost(t, base)
	})
	t.Run("EmptySource", func(t *testing.T) {
		ch, stop := AsChannel(t.Context(), Zero[int]())
		if _, ok := <-ch; ok {
			t.Fatal("empty source produced a value")
		}
		if err := stop(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("CancelledBeforeStart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		ch, stop := AsChannel(ctx, Range(1, 5))
		for range ch {
		}
		_ = stop()
	})
}

func TestUseCasePipeDeliversAndCloses(t *testing.T) {
	base := runtime.NumGoroutine()
	var got []int
	for v := range Pipe(t.Context(), Range(1, 100)) {
		got = append(got, v)
	}
	if !slices.Equal(got, Collect(Range(1, 100))) {
		t.Fatalf("got %d items", len(got))
	}
	ctx, cancel := context.WithCancel(t.Context())
	ch := Pipe(ctx, Monotonic())
	<-ch
	cancel()
	for range ch {
	}
	if got := Collect(Channel(t.Context(), Pipe(t.Context(), Zero[int]()))); len(got) != 0 {
		t.Fatal(got)
	}
	goroutinesAtMost(t, base)
}

func TestUseCaseAsGeneratorLifecycle(t *testing.T) {
	t.Run("ExhaustionIsSticky", func(t *testing.T) {
		gen := AsGenerator(Range(1, 3))
		var got []int
		for {
			v, ok := gen(t.Context())
			if !ok {
				break
			}
			got = append(got, v)
		}
		if !slices.Equal(got, []int{1, 2, 3}) {
			t.Fatalf("got %v", got)
		}
		for range 3 {
			if _, ok := gen(t.Context()); ok {
				t.Fatal("generator resumed after exhaustion")
			}
		}
	})
	t.Run("EmptySource", func(t *testing.T) {
		if _, ok := AsGenerator(Zero[int]())(t.Context()); ok {
			t.Fatal("value from an empty source")
		}
	})
	t.Run("AbandonedWithCancel", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		gen := AsGenerator(Monotonic())
		if v, ok := gen(ctx); !ok || v != 1 {
			t.Fatal(v, ok)
		}
		cancel()
		if _, ok := gen(ctx); ok {
			t.Fatal("cancelled call produced a value")
		}
		goroutinesAtMost(t, base)
	})
	t.Run("ConcurrentCallersShareOneStream", func(t *testing.T) {
		gen := AsGenerator(Range(1, 400))
		var mu sync.Mutex
		var all []int
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for {
					v, ok := gen(t.Context())
					if !ok {
						return
					}
					mu.Lock()
					all = append(all, v)
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		slices.Sort(all)
		if !slices.Equal(all, Collect(Range(1, 400))) {
			t.Fatalf("got %d items", len(all))
		}
	})
}

func TestUseCaseSinkNeverYieldsAfterStop(t *testing.T) {
	t.Run("Sink", func(t *testing.T) {
		var calls, afterStop atomic.Int32
		var stopped atomic.Bool
		push := Sink(func(int) bool {
			if stopped.Load() {
				afterStop.Add(1)
			}
			if calls.Add(1) == 5 {
				stopped.Store(true)
				return false
			}
			return true
		})
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for i := range 100 {
					if !push(i) {
						return
					}
				}
			})
		}
		wg.Wait()
		if calls.Load() != 5 || afterStop.Load() != 0 {
			t.Fatalf("yield called %d times, %d after stop", calls.Load(), afterStop.Load())
		}
		if push(1) {
			t.Fatal("sink resumed")
		}
	})
	t.Run("Sink2", func(t *testing.T) {
		var calls atomic.Int32
		push := Sink2(func(int, int) bool { return calls.Add(1) < 3 })
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for i := range 100 {
					if !push(i, i) {
						return
					}
				}
			})
		}
		wg.Wait()
		if calls.Load() != 3 {
			t.Fatalf("yield called %d times", calls.Load())
		}
	})
	t.Run("PanicLeavesSinkDone", func(t *testing.T) {
		var calls int
		push := Sink(func(int) bool {
			calls++
			panic("yield boom")
		})
		if got := capturePanic(func() { push(1) }); got != "yield boom" {
			t.Fatalf("got %v", got)
		}
		if push(2) || calls != 1 {
			t.Fatalf("sink called yield again after a panic (%d calls)", calls)
		}
	})
}

// TestUseCaseChunkMisuse pins what happens when the documented
// contract (consume each chunk before advancing the outer loop) is
// not followed: nothing hangs, nothing is duplicated, and the source
// is released.
func TestUseCaseChunkMisuse(t *testing.T) {
	t.Run("RetainedChunksAreEmptyOnceTheOuterLoopMovedOn", func(t *testing.T) {
		src := new(ucSource)
		chunks := Collect(Chunk(src.seq(6), 2))
		if len(chunks) != 3 {
			t.Fatalf("got %d chunks", len(chunks))
		}
		for i, c := range chunks {
			if got := Collect(c); len(got) != 0 {
				t.Fatalf("stale chunk %d yielded %v", i, got)
			}
		}
		ucReleased(t, src)
	})
	t.Run("ChunkIsSingleUse", func(t *testing.T) {
		src := new(ucSource)
		for c := range Chunk(src.seq(6), 3) {
			first, second := Collect(c), Collect(c)
			if len(first) != 3 || len(second) != 0 {
				t.Fatalf("first pass %v, second pass %v", first, second)
			}
		}
		ucReleased(t, src)
	})
	t.Run("BreakingTheOuterLoopWithAnUnreadChunk", func(t *testing.T) {
		src := new(ucSource)
		for range Chunk(src.seq(6), 2) {
			break
		}
		ucReleased(t, src)
	})
	t.Run("InnerBreakKeepsChunkBoundaries", func(t *testing.T) {
		var firsts []int
		for c := range Chunk(Range(1, 10), 4) {
			for v := range c {
				firsts = append(firsts, v)
				break
			}
		}
		if !slices.Equal(firsts, []int{1, 5, 9}) {
			t.Fatalf("got %v", firsts)
		}
	})
	t.Run("PanicInsideChunkReleasesSource", func(t *testing.T) {
		src := new(ucSource)
		got := capturePanic(func() {
			for c := range Chunk(src.seq(6), 3) {
				for range c {
					panic("chunk boom")
				}
			}
		})
		if got != "chunk boom" {
			t.Fatalf("got %v", got)
		}
		ucReleased(t, src)
	})
	t.Run("Sizes", func(t *testing.T) {
		for _, num := range []int{1, 2, 3, 6, 7, 1000} {
			total, chunks := 0, 0
			for c := range Chunk(Range(1, 6), num) {
				chunks++
				n := Count(c)
				if n == 0 || n > num {
					t.Fatalf("size %d: chunk of %d", num, n)
				}
				total += n
			}
			if total != 6 || chunks != (6+num-1)/num {
				t.Fatalf("size %d: %d chunks holding %d items", num, chunks, total)
			}
		}
	})
}
