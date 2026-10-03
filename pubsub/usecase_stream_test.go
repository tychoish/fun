package pubsub

import (
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

func ucInts(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func TestUseCaseStreamEmptyAndSingleInputs(t *testing.T) {
	ctx := t.Context()
	streams := map[string]func(in []int) *Stream[int]{
		"Slice":          SliceStream[int],
		"Buffer1":        func(in []int) *Stream[int] { return SliceStream(in).Buffer(1) },
		"Buffer0":        func(in []int) *Stream[int] { return SliceStream(in).Buffer(0) },
		"BufferParallel": func(in []int) *Stream[int] { return SliceStream(in).BufferParallel(2) },
		"Join":           func(in []int) *Stream[int] { return JoinStreams(SliceStream(in)) },
		"JoinNone":       func(in []int) *Stream[int] { return JoinStreams(SliceStream(in), SliceStream([]int{})) },
		"Merge":          func(in []int) *Stream[int] { return MergeStreams(VariadicStream(SliceStream(in))) },
		"MergeNone": func(in []int) *Stream[int] {
			return MergeStreams(VariadicStream(SliceStream(in), SliceStream[int](nil)))
		},
		"Split":      func(in []int) *Stream[int] { return SliceStream(in).Split(1)[0] },
		"Filter":     func(in []int) *Stream[int] { return SliceStream(in).Filter(func(int) bool { return true }) },
		"Convert":    func(in []int) *Stream[int] { return Convert(ucIdentity).Stream(SliceStream(in)) },
		"ConvertPar": func(in []int) *Stream[int] { return Convert(ucIdentity).Parallel(SliceStream(in)) },
	}
	for name, mk := range streams {
		t.Run(name+"/Empty", func(t *testing.T) {
			out, err := mk(nil).Slice(ctx)
			if err != nil || len(out) != 0 {
				t.Fatalf("got %v, %v", out, err)
			}
			if n := mk([]int{}).Count(ctx); n != 0 {
				t.Fatalf("count %d", n)
			}
		})
		t.Run(name+"/One", func(t *testing.T) {
			out, err := mk([]int{42}).Slice(ctx)
			if err != nil || !slices.Equal(out, []int{42}) {
				t.Fatalf("got %v, %v", out, err)
			}
		})
		t.Run(name+"/ReadAfterEnd", func(t *testing.T) {
			s := mk([]int{1})
			if _, err := s.Read(ctx); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				if _, err := s.Read(ctx); !errors.Is(err, io.EOF) {
					t.Fatalf("read after the end: %v", err)
				}
			}
			if err := s.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}

func ucIdentity(_ context.Context, in int) (int, error) { return in, nil }

func TestUseCaseStreamSplitSizes(t *testing.T) {
	for _, n := range []int{-3, 0} {
		if got := SliceStream(ucInts(5)).Split(n); got != nil {
			t.Fatalf("Split(%d) = %v, want nil", n, got)
		}
	}
	for _, parts := range []int{1, 2, 7} {
		t.Run(strconv.Itoa(parts), func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			const items = 100
			split := SliceStream(ucInts(items)).Split(parts)
			if len(split) != parts {
				t.Fatalf("got %d streams", len(split))
			}
			var mu sync.Mutex
			var all []int
			var wg sync.WaitGroup
			for _, s := range split {
				wg.Add(1)
				go func() {
					defer wg.Done()
					out, err := s.Slice(t.Context())
					if err != nil {
						t.Error(err)
					}
					mu.Lock()
					all = append(all, out...)
					mu.Unlock()
				}()
			}
			ucGuard(t, "split consumers", wg.Wait)
			slices.Sort(all)
			if !slices.Equal(all, ucInts(items)) {
				t.Fatalf("split lost or duplicated items: %d items", len(all))
			}
		})
	}
}

func TestUseCaseStreamBufferedChannelSizes(t *testing.T) {
	for _, size := range []int{0, 1, 10, 500} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			ch := SliceStream(ucInts(100)).BufferedChannel(t.Context(), size)
			if cap(ch) != size {
				t.Fatalf("capacity %d", cap(ch))
			}
			var got []int
			timeout := time.After(10 * time.Second)
			for {
				select {
				case v, ok := <-ch:
					if !ok {
						if !slices.Equal(got, ucInts(100)) {
							t.Fatalf("got %v", got)
						}
						return
					}
					got = append(got, v)
				case <-timeout:
					t.Fatal("deadlock: channel never closed")
				}
			}
		})
	}
	t.Run("CancelClosesChannel", func(t *testing.T) {
		defer testt.NoGoroutineLeak(t, 10*time.Second)()
		ctx, cancel := context.WithCancel(t.Context())
		ch := infiniteStream().Channel(ctx)
		<-ch
		cancel()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return
				}
			case <-timeout:
				t.Fatal("deadlock: channel never closed after cancel")
			}
		}
	})
}

func TestUseCaseStreamIteratorEarlyBreakEveryPosition(t *testing.T) {
	const items = 5
	for pos := 1; pos <= items; pos++ {
		t.Run(strconv.Itoa(pos), func(t *testing.T) {
			s := SliceStream(ucInts(items))
			count, violations := ucYieldGuard(s.Iterator(t.Context()), pos)
			if count != pos || violations != 0 {
				t.Fatalf("count %d violations %d", count, violations)
			}
			// the stream resumes where the consumer stopped.
			rest, err := s.Slice(t.Context())
			if err != nil || !slices.Equal(rest, ucInts(items)[pos:]) {
				t.Fatalf("remaining %v, %v", rest, err)
			}
		})
	}
	t.Run("SecondIterationOfDrainedStreamIsEmpty", func(t *testing.T) {
		seq := SliceStream(ucInts(3)).Iterator(t.Context())
		if got := slices.Collect(seq); len(got) != 3 {
			t.Fatalf("got %v", got)
		}
		if got := slices.Collect(seq); len(got) != 0 {
			t.Fatalf("second iteration yielded %v", got)
		}
	})
}

func TestUseCaseStreamCloseHooks(t *testing.T) {
	var order []int
	s := SliceStream(ucInts(2)).
		WithHook(func(*Stream[int]) { order = append(order, 1) }).
		WithHook(func(*Stream[int]) { order = append(order, 2) })
	for range 3 {
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(order, []int{1, 2}) {
		t.Fatalf("hooks ran %v, want each exactly once in order", order)
	}
	if _, err := s.Read(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatalf("read after Close: %v", err)
	}
}

func TestUseCaseStreamHookErrorsSurfaceFromClose(t *testing.T) {
	boom := errors.New("hook failed")
	s := SliceStream(ucInts(2)).WithHook(func(st *Stream[int]) { st.AddError(boom) })
	if err := s.Close(); !errors.Is(err, boom) {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); !errors.Is(err, boom) {
		t.Fatalf("second Close lost the hook error: %v", err)
	}
}

// TestUseCaseStreamGeneratorPanic pins that a panicking generator is
// converted to an error by the bulk consumers (ReadAll, Slice, Count);
// a bare Read lets the panic through to the caller.
func TestUseCaseStreamGeneratorPanic(t *testing.T) {
	gen := func(context.Context) (int, error) { panic("generator exploded") }
	s := MakeStream(gen)
	if _, err := s.Slice(t.Context()); !errors.Is(err, ers.ErrRecoveredPanic) {
		t.Fatalf("Slice: %v", err)
	}
	if n := MakeStream(gen).Count(t.Context()); n != 0 {
		t.Fatalf("Count %d", n)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("bare Read no longer propagates the generator panic; update this pin")
			}
		}()
		_, _ = MakeStream(gen).Read(t.Context())
	}()
}

func TestUseCaseStreamReducerPanic(t *testing.T) {
	s := SliceStream(ucInts(3)).Reduce(func(int, int) (int, error) { panic("reducer exploded") })
	if _, err := s.Read(t.Context()); !errors.Is(err, ers.ErrRecoveredPanic) {
		t.Fatalf("Read: %v", err)
	}
}

func TestUseCaseStreamReducerError(t *testing.T) {
	boom := errors.New("reducer failed")
	s := SliceStream(ucInts(3)).Reduce(func(int, int) (int, error) { return 0, boom })
	if _, err := s.Read(t.Context()); !errors.Is(err, boom) {
		t.Fatalf("Read: %v", err)
	}
}

func TestUseCaseStreamParallelHandlerPanicAndError(t *testing.T) {
	t.Run("Panic", func(t *testing.T) {
		defer testt.NoGoroutineLeak(t, 10*time.Second)()
		err := SliceStream(ucInts(10)).Parallel(func(_ context.Context, v int) error {
			if v == 3 {
				panic("handler exploded")
			}
			return nil
		}).Run(t.Context())
		if !errors.Is(err, ers.ErrRecoveredPanic) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("ConvertPanicIsReported", func(t *testing.T) {
		defer testt.NoGoroutineLeak(t, 10*time.Second)()
		s := Convert(func(_ context.Context, v int) (int, error) {
			if v == 2 {
				panic("converter exploded")
			}
			return v, nil
		}).Parallel(SliceStream(ucInts(10)))
		_, _ = s.Slice(t.Context())
		if err := s.Close(); !errors.Is(err, ers.ErrRecoveredPanic) {
			t.Fatalf("Close: %v", err)
		}
	})
}

func TestUseCaseStreamBufferParallelKeepsEveryItem(t *testing.T) {
	for _, n := range []int{1, 3, 64} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			out, err := SliceStream(ucInts(300)).BufferParallel(n).Slice(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(out)
			if !slices.Equal(out, ucInts(300)) {
				t.Fatalf("lost or duplicated items: %d", len(out))
			}
		})
	}
}

func TestUseCaseStreamBufferCancelStopsProducer(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	var produced atomic.Int64
	src := MakeStream(func(context.Context) (int, error) { return int(produced.Add(1)), nil })
	ctx, cancel := context.WithCancel(t.Context())
	buffered := src.Buffer(4)
	if _, err := buffered.Read(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := buffered.Read(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read with cancelled ctx: %v", err)
	}
	_ = buffered.Close()
}

func TestUseCaseStreamMergeOrderingAndEmptyOuter(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 10*time.Second)()
	if out, err := MergeStreams(VariadicStream[*Stream[int]]()).Slice(t.Context()); err != nil || len(out) != 0 {
		t.Fatalf("empty outer: %v, %v", out, err)
	}
	a, b := ucInts(50), ucInts(50)
	merged, err := MergeStreams(VariadicStream(SliceStream(a), SliceStream(b))).Slice(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(merged)
	want := append(ucInts(50), ucInts(50)...)
	slices.Sort(want)
	if !slices.Equal(merged, want) {
		t.Fatalf("merge lost items: %d", len(merged))
	}
}

func TestUseCaseStreamJoinKeepsOrder(t *testing.T) {
	out, err := JoinStreams(SliceStream([]int{1, 2}), SliceStream[int](nil), SliceStream([]int{3}), VariadicStream(4, 5)).Slice(t.Context())
	if err != nil || !slices.Equal(out, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("got %v, %v", out, err)
	}
	if out, err := JoinStreams[int]().Slice(t.Context()); err != nil || len(out) != 0 {
		t.Fatalf("zero streams: %v, %v", out, err)
	}
}

func TestUseCaseRateLimitInvalidRate(t *testing.T) {
	for _, num := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("RateLimit(num=%d) did not panic", num)
				}
			}()
			_ = RateLimit(t.Context(), SliceStream([]int{1}).Iterator(t.Context()), num, time.Second)
		}()
	}
}

func TestUseCaseRateLimitCancelledContextYieldsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for range RateLimit(ctx, SliceStream(ucInts(3)).Iterator(t.Context()), 1, time.Hour) {
		t.Fatal("yielded after cancel")
	}
}
