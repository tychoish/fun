package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/opt"
	"github.com/tychoish/fun/wpa"
)

func Map[T any, O any](it *Stream[T], mpf fnx.Converter[T, O], optp ...opt.Provider[*wpa.WorkerGroupConf]) *Stream[O] {
	return Convert(mpf).Parallel(it, optp...)
}

func CheckSeenMap[T comparable](t *testing.T, elems []T, seen map[T]struct{}) {
	t.Helper()
	if len(seen) != len(elems) {
		t.Fatal("all elements not iterated", "seen=", len(seen), "vs", "elems=", len(elems))
	}
	for idx, val := range elems {
		if _, ok := seen[val]; !ok {
			t.Error("element a not observed", idx, val)
		}
	}
}

func sum(in []int) (out int) {
	for _, num := range in {
		out += num
	}
	return out
}

type FixtureData[T any] struct {
	Name     string
	Elements []T
}

type FixtureStreamConstructors[T any] struct {
	Name        string
	Constructor func([]T) *Stream[T]
}

type FixtureStreamFilter[T any] struct {
	Name   string
	Filter func(*Stream[T]) *Stream[T]
}

type none struct{}

func testIntIter(t *testing.T, size int) *Stream[int] {
	t.Helper()

	var count int

	t.Cleanup(func() {
		t.Helper()
		check.Equal(t, count, size)
	})

	return MakeStream(fnx.MakeFuture(func() (int, error) {
		if count >= size {
			return 0, io.EOF
		}
		count++
		return count - 1, nil
	}))
}

func GenerateRandomStringSlice(size int) []string {
	out := make([]string, size)
	for idx := range out {
		out[idx] = fmt.Sprint("value=", idx)
	}
	return out
}

func TestStream(t *testing.T) {
	t.Run("ReadAll", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{})
			assert.NotError(t, iter.ReadAll(fnx.FromHandler(func(_ int) { t.Fatal("should not be called") })).Run(ctx))

			_, err := iter.Read(ctx)
			assert.ErrorIs(t, err, io.EOF)
		})
		t.Run("PanicSafety", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := 0
			err := SliceStream([]int{1, 2, 34, 56}).ReadAll(fnx.FromHandler(func(in int) {
				called++
				if in > 3 {
					panic("eep!")
				}
			})).Run(ctx)
			if err == nil {
				t.Fatal("should error")
			}
			if called != 3 {
				t.Error(called)
			}
			if !errors.Is(err, ers.ErrRecoveredPanic) {
				t.Error(err)
			}
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			count := 0
			assert.Error(t, ctx.Err())
			err := SliceStream([]int{1, 2, 34, 56}).ReadAll(fnx.FromHandler(func(int) {
				count++
			})).Run(ctx)
			t.Log(err)
			if !errors.Is(err, context.Canceled) {
				t.Error(err, ctx.Err())
			}
			if count != 0 {
				t.Error("expected no ops", count)
			}
		})
	})
	t.Run("Continue", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := 0
		iter := MakeStream(func(context.Context) (int, error) {
			count++
			switch {
			case count > 128:
				count = 128
				return -1, io.EOF
			case count%2 == 0:
				return count, nil
			default:
				return -1, ers.ErrCurrentOpSkip
			}
		})
		ints, err := iter.Slice(ctx)
		check.NotError(t, err)
		check.Equal(t, len(ints), 64)
		check.Equal(t, 128, count)
	})
	t.Run("Continue", func(t *testing.T) {
		count := 0
		observes := 0
		sum := 0
		err := MakeStream(fnx.MakeFuture(func() (int, error) {
			count++
			switch count {
			case 25, 50, 75:
				return 100, nil
			case 100:
				return 0, io.EOF
			default:
				return -1, ers.ErrCurrentOpSkip
			}
		})).ReadAll(fnx.FromHandler(func(in int) {
			observes++

			assert.True(t, in%5 == 0)
			assert.True(t, observes > 0)
			assert.True(t, observes <= 3)
			assert.True(t, count <= 100)

			sum += in
		})).Run(t.Context())

		assert.NotError(t, err)
		assert.Equal(t, sum, 300)
		assert.Equal(t, observes, 3)
		assert.Equal(t, count, 100)
	})
	t.Run("Transform", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		out, err := VariadicStream(4, 8, 16, 32, 64, 128, 256, 512, 1024).Transform(fnx.MakeConverter(func(in int) int { return in / 4 })).Slice(ctx)
		check.NotError(t, err)

		check.EqualItems(t, out, []int{1, 2, 4, 8, 16, 32, 64, 128, 256})
	})
	t.Run("Process", func(t *testing.T) {
		t.Run("Process", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0

			err := iter.ReadAll(fnx.FromHandler(func(int) { count++ })).Run(ctx)
			assert.NotError(t, err)
			check.Equal(t, 9, count)
		})
		t.Run("ProcessWorker", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			op := iter.ReadAll(fnx.FromHandler(func(int) { count++ }))
			err := op(ctx)
			assert.NotError(t, err)
			check.Equal(t, 9, count)
		})
		t.Run("Abort", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.ReadAll(func(_ context.Context, _ int) error { count++; return io.EOF }).Run(ctx)
			assert.NotError(t, err)
			assert.Equal(t, 1, count)
		})
		t.Run("OperationError", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.ReadAll(func(_ context.Context, _ int) error { count++; return ers.ErrLimitExceeded }).Run(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrLimitExceeded)
			assert.Equal(t, 1, count)
		})
		t.Run("Panic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.ReadAll(func(_ context.Context, _ int) error { count++; panic(ers.ErrLimitExceeded) }).Run(ctx)
			assert.Error(t, err)
			check.Equal(t, 1, count)
			check.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("ContextExpired", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.ReadAll(func(ctx context.Context, _ int) error { count++; cancel(); return ctx.Err() }).Run(ctx)
			assert.Error(t, err)
			check.Equal(t, 1, count)
			check.ErrorIs(t, err, context.Canceled)
		})

		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := &atomic.Int64{}
			err := iter.Parallel(
				func(_ context.Context, in int) error { count.Add(1); return nil },
				wpa.WorkerGroupConfNumWorkers(2),
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfContinueOnPanic(),
			).Run(ctx)
			assert.NotError(t, err)
			check.Equal(t, 9, count.Load())
		})
	})
	t.Run("Transform", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := SliceStream([]string{
				fmt.Sprint(10),
				fmt.Sprint(10),
				fmt.Sprint(20),
				fmt.Sprint(2),
			})
			calls := 0

			out := Convert(fnx.MakeConverterErr(func(in string) (int, error) {
				calls++
				return strconv.Atoi(in)
			})).Stream(input)
			sum := 0
			for out.Next(ctx) {
				sum += out.Value()
			}
			assert.NotError(t, out.Close())
			assert.Equal(t, 42, sum)
			assert.Equal(t, calls, 4)
		})
		t.Run("Skips", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := SliceStream([]string{
				fmt.Sprint(10),
				fmt.Sprint(10),
				fmt.Sprint(20),
				fmt.Sprint(2),
			})
			calls := 0
			out := Convert(fnx.MakeConverterErr(func(in string) (int, error) {
				if in == "2" {
					return 0, ers.ErrCurrentOpSkip
				}
				calls++
				return strconv.Atoi(in)
			})).Stream(input)

			sum := 0
			for out.Next(ctx) {
				sum += out.Value()
			}
			assert.NotError(t, out.Close())
			assert.Equal(t, 40, sum)
			assert.Equal(t, calls, 3)
		})
		t.Run("ErrorPropogation", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := SliceStream([]string{
				fmt.Sprint(10),
				fmt.Sprint(10),
				fmt.Sprint(20),
				fmt.Sprint(2),
			})
			calls := 0
			out := Convert(fnx.MakeConverterErr(func(in string) (int, error) {
				if in == "20" {
					return 0, ers.ErrInvalidInput
				}
				calls++
				return strconv.Atoi(in)
			})).Stream(input)

			sum := 0
			for out.Next(ctx) {
				sum += out.Value()
			}
			check.Error(t, out.Close())
			check.ErrorIs(t, out.Close(), ers.ErrInvalidInput)

			assert.Equal(t, 20, sum)
			assert.Equal(t, calls, 2)
		})
	})
	t.Run("Future", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run("BasicOperation", func(t *testing.T) {
			iter := MakeStream(func(context.Context) (int, error) {
				return 1, nil
			})

			if iter.Value() != 0 {
				t.Error("should initialize to zero")
			}
			if !iter.Next(ctx) {
				t.Error("should always iterate at least once")
			}
			if err := iter.Close(); err != nil {
				t.Error(err)
			}
			if iter.Next(ctx) {
				t.Error("should not iterate after close")
			}
		})
		t.Run("RespectEOF", func(t *testing.T) {
			count := 0
			iter := MakeStream(func(context.Context) (int, error) {
				count++
				if count > 10 {
					return 1000, io.EOF
				}
				return count, nil
			})

			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 10 {
					t.Error("unexpected value", iter.Value())
				}
			}

			if seen > count {
				t.Error(seen, "vs", "count")
			}
		})
		t.Run("PropogateErrors", func(t *testing.T) {
			count := 0
			expected := errors.New("kip")
			returned := false
			iter := MakeStream(func(context.Context) (int, error) {
				count++
				if count > 10 {
					returned = true
					return 1000, expected
				}
				return count, nil
			})

			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 10 {
					t.Error("unexpected value", iter.Value())
				}
			}

			if seen > count {
				t.Error(seen, "vs", "count")
			}
			if !returned {
				t.Error("should have returned error", count)
			}
			if err := iter.Close(); !errors.Is(err, expected) {
				t.Error(err)
			}
		})
	})
	t.Run("Filter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		evens := testIntIter(t, 100).Filter(func(in int) bool { return in%2 == 0 })
		assert.Equal(t, evens.Count(ctx), 50)
		for evens.Next(ctx) {
			assert.True(t, evens.Value()%2 == 0)
		}
		assert.NotError(t, evens.Close())
	})
	t.Run("Split", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		input := SliceStream(GenerateRandomStringSlice(100))

		splits := input.Split(0)
		if splits != nil {
			t.Fatal("should be nil if empty")
		}

		splits = input.Split(10)
		if len(splits) != 10 {
			t.Fatal("didn't make enough split")
		}

		count := &atomic.Int64{}

		wg := &fnx.WaitGroup{}
		for _, iter := range splits {
			wg.Add(1)

			go func(it *Stream[string]) {
				defer wg.Done()
				for it.Next(ctx) {
					count.Add(1)
				}
			}(iter)
		}

		wg.Wait(ctx)

		if count.Load() != 100 {
			t.Error("did not iterate enough")
		}
	})
	t.Run("Buffer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		input := SliceStream(GenerateRandomStringSlice(128))
		buf := input.Buffer(256)
		check.Equal(t, buf.Count(ctx), 128)
		check.True(t, buf.closer.state.Load())
		check.True(t, input.closer.state.Load())
	})
	t.Run("ParallelBuffer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		input := SliceStream(GenerateRandomStringSlice(128))
		buf := input.BufferParallel(256)
		out, err := buf.Slice(ctx)
		check.NotError(t, err)
		check.Equal(t, len(out), 128)
		check.True(t, buf.closer.state.Load())
		check.True(t, input.closer.state.Load())
	})
	t.Run("Slice", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceStream([]int{1, 2, 3, 4})
			seen := 0
			for iter.Next(ctx) {
				seen++
				if iter.Value() > 4 || iter.Value() <= 0 {
					t.Error(iter.Value())
				}
			}
			if seen != 4 {
				t.Error(seen)
			}
			if iter.Close() != nil {
				t.Error(iter.Close())
			}
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			iter := &Stream[int]{
				operation: func(context.Context) (int, error) { return 0, nil },
			}
			iter.closer.ops = []func(){
				func() {
					seen := 0
					for iter.Next(ctx) {
						seen++
					}
					if seen != 0 {
						t.Error(seen)
					}
					if iter.Close() != nil {
						t.Error(iter.Close())
					}
				},
			}
		})
	})
	t.Run("ReadOne", func(t *testing.T) {
		ch := make(chan string, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		ch <- "buddy"
		defer cancel()
		out, err := Blocking(ch).Receive().Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if out != "buddy" {
			t.Fatal(out)
		}

		ch <- "buddy"
		cancel()
		seenCondition := false
		for i := 0; i < 10; i++ {
			t.Log(i)
			_, err = Blocking(ch).Receive().Read(ctx)
			if errors.Is(err, context.Canceled) {
				seenCondition = true
			}
			t.Log(err)

			select {
			case ch <- "buddy":
			default:
			}
		}
		if !seenCondition {
			t.Error("should have observed a context canceled")
		}
	})
	t.Run("ReadOneEOF", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		ch := make(chan string, 1)
		close(ch)

		_, err := Blocking(ch).Receive().Read(ctx)
		if !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
	})
	t.Run("NonBlockingReadOne", func(t *testing.T) {
		t.Parallel()
		t.Run("BlockingCompatibility", func(t *testing.T) {
			ch := make(chan string, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			ch <- "buddy"
			defer cancel()
			out, err := NonBlocking(ch).Receive().Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if out != "buddy" {
				t.Fatal(out)
			}

			ch <- "buddy"
			cancel()
			seenCondition := false
			for i := 0; i < 10; i++ {
				_, err = NonBlocking(ch).Receive().Read(ctx)
				if errors.Is(err, context.Canceled) {
					seenCondition = true
				}
				t.Log(i, err)

				select {
				case ch <- "buddy":
				default:
				}
			}
			if !seenCondition {
				t.Error("should have observed a context canceled")
			}
		})
		t.Run("NonBlocking", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			out, err := Chan[string]().NonBlocking().Receive().Read(ctx)
			assert.Zero(t, out)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)
		})
		t.Run("Closed", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan string)
			close(ch)
			out, err := NonBlocking(ch).Receive().Read(ctx)
			assert.Zero(t, out)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
		})
	})
	t.Run("Merged", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		elems := GenerateRandomStringSlice(100)

		iter := MergeStreams(
			VariadicStream(
				SliceStream(elems),
				SliceStream(elems),
				SliceStream(elems),
				SliceStream(elems),
				SliceStream(elems),
			),
		)
		seen := make(map[string]struct{}, len(elems))
		var count int
		for iter.Next(ctx) {
			count++
			seen[iter.Value()] = none{}
		}
		if count != 5*len(elems) {
			t.Fatal("did not iterate enough", count, 5*len(elems))
		}
		for idx, str := range elems {
			_, ok := seen[str]
			assert.True(t, ok)
			if !ok {
				t.Log("mismatch", idx, str)
			}
		}

		if err := iter.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("MergeReleases", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string)
		iter := JoinStreams(
			ChannelStream(pipe),
			ChannelStream(pipe),
			ChannelStream(pipe),
			ChannelStream(pipe),
		)

		ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		if iter.Next(ctx) {
			t.Error("no iteration", iter.Value())
		}
	})
}

func TestEmptyIteration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan int)
	close(ch)

	t.Run("EmptyReadAll", func(t *testing.T) {
		assert.NotError(t, SliceStream([]int{}).ReadAll(fnx.FromHandler(func(_ int) { t.Fatal("should not be called") })).Run(ctx))
		assert.NotError(t, VariadicStream[int]().ReadAll(fnx.FromHandler(func(_ int) { t.Fatal("should not be called") })).Run(ctx))
		assert.NotError(t, ChannelStream(ch).ReadAll(fnx.FromHandler(func(_ int) { t.Fatal("should not be called") })).Run(ctx))
	})
}

func TestChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	num := []int{1, 2, 3, 5, 7, 9, 11, 13, 17, 19}
	iter := SliceStream(num).Join(SliceStream(num))

	n := iter.Count(ctx)

	assert.Equal(t, len(num)*2, n)

	iter = SliceStream(num).Join(SliceStream(num), SliceStream(num), SliceStream(num))
	cancel()
	n = iter.Count(ctx)
	assert.Equal(t, n, 0)
}

func TestJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("RoundTrip", func(t *testing.T) {
		iter := SliceStream([]int{400, 300, 42})
		out, err := iter.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "[400,300,42]" {
			t.Error(string(out))
		}
		nl := []int{}
		if err := json.Unmarshal(out, &nl); err != nil {
			t.Error(err)
		}
	})
	t.Run("MarshalErrors", func(t *testing.T) {
		iter := SliceStream([]fnx.Worker{func(context.Context) error { return nil }})
		_, err := iter.MarshalJSON()
		if err == nil {
			t.Fatal(err)
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		iter := SliceStream([]string{})
		if err := iter.UnmarshalJSON([]byte(`["foo", "arg"]`)); err != nil {
			t.Error(err)
		}

		vals, err := iter.Slice(ctx)
		if err != nil {
			t.Error(err)
		}
		if len(vals) != 2 {
			t.Fatal(len(vals), vals)
		}
		if vals[0] != "foo" {
			t.Error(vals[0])
		}
		if vals[1] != "arg" {
			t.Error(vals[1])
		}

		if err := iter.UnmarshalJSON([]byte(`[foo", "arg"]`)); err == nil {
			t.Error(err)
		}
	})
	t.Run("ErrorHandler", func(t *testing.T) {
		iter := SliceStream([]string{})

		ec := iter.ErrorHandler()

		ec(io.EOF)

		ec(ers.ErrInvalidInput)
		ec(io.ErrUnexpectedEOF)
		ec(context.Canceled)

		err := iter.Close()
		assert.Error(t, err)
		t.Log(ers.Unwind(err))
		check.Equal(t, len(ers.Unwind(err)), 4)

		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
		assert.ErrorIs(t, err, context.Canceled)
		assert.ErrorIs(t, err, io.EOF)

		assert.True(t, ers.IsTerminating(err))
	})
	t.Run("Channel", func(t *testing.T) {
		iter := SliceStream([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Millisecond)
		defer cancel()

		ch := iter.Channel(ctx)
		count := 0
	OUTER:
		for {
			select {
			case <-ctx.Done():
				t.Error("context should not have expired")
				break OUTER
			case it, ok := <-ch:
				if !ok {
					break OUTER
				}

				count++
				check.NotZero(t, it)
				continue OUTER
			}
		}
		assert.Equal(t, 9, count)
		assert.NotError(t, ctx.Err())
	})
	t.Run("ParallelInvalidConfig", func(t *testing.T) {
		counter := 0
		err := SliceStream([]int{1, 1, 1}).Parallel(fnx.MakeHandler(func(in int) error { counter += in; return nil }),
			wpa.WorkerGroupConfWithErrorCollector(nil),
		).Run(t.Context())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
		assert.Zero(t, counter)
	})
	t.Run("Filter", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			out, err := SliceStream([]int{1, 1, 1, -1, -1, -1}).Filter(func(i int) bool { return i > 0 }).Slice(t.Context())
			assert.NotError(t, err)
			assert.EqualItems(t, out, []int{1, 1, 1})
		})
		t.Run("Continue", func(t *testing.T) {
			count := 0
			out, err := MakeStream(fnx.MakeFuture(func() (int, error) {
				count++
				if count%2 == 0 {
					return -1, ers.ErrCurrentOpSkip
				}
				if count > 10 {
					return -2, io.EOF
				}
				return count, nil
			})).Filter(func(in int) bool { return in != 5 }).Slice(t.Context())
			assert.NotError(t, err)
			t.Log(out)
			assert.EqualItems(t, out, []int{1, 3, 7, 9})
		})
	})
	t.Run("PanicReadCoverage", func(t *testing.T) {
		str := MakeStream(func(context.Context) (int, error) { return 7, ers.ErrRecoveredPanic })
		num, err := str.Read(t.Context())
		check.Equal(t, num, 7)
		check.Error(t, err)
		check.ErrorIs(t, err, ers.ErrRecoveredPanic)
	})
}

func TestIteratorStream(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Basic",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 5; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)
				defer stream.Close()

				result, err := stream.Slice(t.Context())
				assert.NotError(t, err)
				assert.EqualItems(t, result, []int{0, 1, 2, 3, 4})
			},
		},
		{
			name: "EmptyIterator",
			test: func(t *testing.T) {
				emptyIter := func(yield func(int) bool) {}

				stream := IteratorStream(emptyIter)
				defer stream.Close()

				result, err := stream.Slice(t.Context())
				assert.NotError(t, err)
				assert.Equal(t, len(result), 0)
			},
		},
		{
			name: "EarlyTermination",
			test: func(t *testing.T) {
				largeIter := func(yield func(int) bool) {
					for i := 0; i < 1000; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(largeIter)
				defer stream.Close()

				var result []int
				for i := 0; i < 3; i++ {
					val, err := stream.Read(t.Context())
					if err != nil {
						break
					}
					result = append(result, val)
				}

				assert.EqualItems(t, result, []int{0, 1, 2})
			},
		},
		{
			name: "ContextCancellation",
			test: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())

				iter := func(yield func(int) bool) {
					for i := 0; i < 1000; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)
				defer stream.Close()

				_, err := stream.Read(ctx)
				assert.NotError(t, err)

				cancel()

				_, err = stream.Read(ctx)
				assert.Error(t, err)
				assert.ErrorIs(t, err, context.Canceled)
			},
		},
		{
			name: "StreamClose",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 100; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)

				_, err := stream.Read(t.Context())
				assert.NotError(t, err)

				err = stream.Close()
				assert.NotError(t, err)

				_, err = stream.Read(t.Context())
				assert.Error(t, err)
				assert.ErrorIs(t, err, io.EOF)
			},
		},
		{
			name: "ConcurrentReads",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 200; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)
				defer stream.Close()

				var mu sync.Mutex
				seen := make(map[int]bool)
				wg := &sync.WaitGroup{}

				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for j := 0; j < 10; j++ {
							val, err := stream.Read(context.Background())
							if err != nil {
								return
							}
							mu.Lock()
							seen[val] = true
							mu.Unlock()
						}
					}()
				}

				wg.Wait()

				mu.Lock()
				count := len(seen)
				mu.Unlock()

				assert.Equal(t, count, 100)
			},
		},
		{
			name: "Filter",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 10; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter).Filter(func(i int) bool { return i%2 == 0 })
				defer stream.Close()

				result, err := stream.Slice(t.Context())
				assert.NotError(t, err)
				assert.EqualItems(t, result, []int{0, 2, 4, 6, 8})
			},
		},
		{
			name: "Transform",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 1; i <= 5; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter).Transform(fnx.MakeConverter(func(i int) int {
					return i * 2
				}))
				defer stream.Close()

				result, err := stream.Slice(t.Context())
				assert.NotError(t, err)
				assert.EqualItems(t, result, []int{2, 4, 6, 8, 10})
			},
		},
		{
			name: "Count",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 42; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)
				defer stream.Close()

				count := stream.Count(t.Context())
				assert.Equal(t, count, 42)
			},
		},
		{
			name: "StringIterator",
			test: func(t *testing.T) {
				words := []string{"hello", "world", "from", "iterator"}
				iter := func(yield func(string) bool) {
					for _, word := range words {
						if !yield(word) {
							return
						}
					}
				}

				stream := IteratorStream(iter)
				defer stream.Close()

				result, err := stream.Slice(t.Context())
				assert.NotError(t, err)
				assert.EqualItems(t, result, words)
			},
		},
		{
			name: "MultipleCloseCalls",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 5; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)

				err1 := stream.Close()
				err2 := stream.Close()
				err3 := stream.Close()

				assert.NotError(t, err1)
				assert.NotError(t, err2)
				assert.NotError(t, err3)
			},
		},
		{
			name: "IteratorChannel",
			test: func(t *testing.T) {
				iter := func(yield func(int) bool) {
					for i := 0; i < 10; i++ {
						if !yield(i) {
							return
						}
					}
				}

				stream := IteratorStream(iter)
				defer stream.Close()

				ch := stream.Channel(t.Context())

				var result []int
				for val := range ch {
					result = append(result, val)
				}

				assert.Equal(t, len(result), 10)
				assert.EqualItems(t, result, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestTools(t *testing.T) {
	t.Parallel()
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint("Iteration", i), func(t *testing.T) {
			t.Parallel()
			t.Run("CancelCollectChannel", func(t *testing.T) {
				bctx, bcancel := context.WithCancel(context.Background())
				defer bcancel()

				ctx, cancel := context.WithCancel(bctx)
				defer cancel()

				pipe := make(chan string)
				sig := make(chan struct{})

				go func() {
					defer close(sig)
					for {
						select {
						case <-bctx.Done():
							return
						case pipe <- t.Name():
							continue
						}
					}
				}()

				output := ChannelStream(pipe).Channel(ctx)
				runtime.Gosched()

				count := 0
			CONSUME:
				for {
					select {
					case _, ok := <-output:
						if ok {
							count++
							cancel()
							runtime.Gosched()
						}
						if !ok {
							break CONSUME
						}
					case <-sig:
						break CONSUME
					case <-time.After(100 * time.Millisecond):
						break CONSUME
					}
				}
				if count != 1 {
					t.Error(count)
				}
			})
		})
	}
}

func TestMapReduce(t *testing.T) {
	t.Parallel()
	t.Run("MapWorkerSendingBlocking", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string, 1)
		output := make(chan int)
		wg := &fnx.WaitGroup{}
		pipe <- t.Name()

		var mf fnx.Converter[string, int] = func(_ context.Context, _ string) (int, error) { return 53, nil }

		ChannelStream(pipe).
			ReadAll(
				(&converter[string, int]{op: mf}).mapPullProcess(Blocking(output).Send().Write, &wpa.WorkerGroupConf{}),
			).
			Ignore().
			Add(ctx, wg)

		time.Sleep(10 * time.Millisecond)
		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		wg.Wait(ctx)

		count := 0
	CONSUME:
		for {
			select {
			case _, ok := <-output:
				if ok {
					count++
					continue
				}
				break CONSUME
			case <-ctx.Done():
				break CONSUME
			}
		}
	})
}

func getConstructors[T comparable]() []FixtureStreamConstructors[T] {
	return []FixtureStreamConstructors[T]{
		{
			Name: "Slice",
			Constructor: func(elems []T) *Stream[T] {
				return SliceStream(elems)
			},
		},
		{
			Name: "VariadicStream",
			Constructor: func(elems []T) *Stream[T] {
				return VariadicStream(elems...)
			},
		},
		{
			Name: "ChannelStream",
			Constructor: func(elems []T) *Stream[T] {
				vals := make(chan T, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return ChannelStream(vals)
			},
		},
	}
}

func TestStreamImplementations(t *testing.T) {
	t.Parallel()
	elems := []FixtureData[string]{
		{
			Name:     "Basic",
			Elements: []string{"a", "b", "c", "d"},
		},
		{
			Name:     "Large",
			Elements: GenerateRandomStringSlice(100),
		},
	}

	t.Run("SimpleOperations", func(t *testing.T) {
		RunStreamImplementationTests(t, elems, getConstructors[string]())
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunStreamStringAlgoTests(t, elems, getConstructors[string]())
	})
}

func TestStreamAlgoInts(t *testing.T) {
	t.Parallel()

	elemFuture := func() []int {
		e := make([]int, 100)
		for idx := range e {
			e[idx] = idx
		}
		return e
	}

	elems := []FixtureData[int]{
		{
			Name:     "Basic",
			Elements: []int{1, 2, 3, 4, 5},
		},
		{
			Name:     "Large",
			Elements: elemFuture(),
		},
	}

	t.Run("SimpleOperations", func(t *testing.T) {
		RunStreamImplementationTests(t, elems, getConstructors[int]())
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunStreamIntegerAlgoTests(t, elems, getConstructors[int]())
	})
}

func TestParallelForEach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Basic", func(t *testing.T) {
		for i := int64(-1); i <= 12; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				i := i
				t.Parallel()
				elems := makeIntSlice(200)
				seen := &atomic.Int64{}
				count := &atomic.Int64{}

				err := SliceStream(elems).Parallel(
					func(ctx context.Context, in int) error {
						abs := int64(math.Abs(float64(i)))

						count.Add(1)
						time.Sleep(time.Duration(rand.Int63n(2 + abs*int64(time.Millisecond))))

						if err := ctx.Err(); err != nil {
							return err
						}

						seen.Add(int64(in))
						return nil
					},
					wpa.WorkerGroupConfNumWorkers(int(i))).Run(ctx)

				check.NotError(t, err)

				check.Equal(t, int(seen.Load()), sum(elems))

				if int(count.Load()) != len(elems) {
					t.Error("unequal length slices", count.Load(), len(elems))
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		count := &atomic.Int64{}
		errCount := &atomic.Int64{}
		err := SliceStream(makeIntSlice(200)).Parallel(
			func(_ context.Context, in int) error {
				count.Add(1)
				runtime.Gosched()
				if in >= 100 {
					errCount.Add(1)
					panic("error")
				}
				return nil
			},
			wpa.WorkerGroupConfNumWorkers(3),
			wpa.WorkerGroupConfContinueOnPanic(),
		).Run(ctx)
		if err == nil {
			t.Error("should have errored", err)
		}
		if errCount.Load() != 100 {
			t.Error(errCount.Load())
		}
		if count.Load() != 200 {
			t.Error(count.Load())
		}
		// panic recovery errors plus panics themselves
		check.Equal(t, 200, len(ers.Unwind(err)))
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seenCount := &atomic.Int64{}
		paned := &atomic.Bool{}

		err := SliceStream(makeIntSlice(10)).
			Parallel(
				func(_ context.Context, in int) error {
					if in == 8 {
						paned.Store(true)
						// make sure something else
						// has a chance to run before
						// the event.
						runtime.Gosched()
						panic("gotcha")
					}

					seenCount.Add(1)
					return nil
				},
				wpa.WorkerGroupConfNumWorkers(4),
			).Run(ctx)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		check.True(t, paned.Load())
		if seenCount.Load() != 9 {
			t.Error("should have seen", 9, "saw", seenCount.Load())
		}
		errs := ers.Unwind(err)
		if len(errs) != 2 {
			// panic + expected
			t.Log(errs)
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceStream(makeIntSlice(10)).
			Parallel(
				func(_ context.Context, in int) error {
					if in == 4 {
						panic("gotcha")
					}
					return nil
				},
				wpa.WorkerGroupConfNumWorkers(8),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}
	})
	t.Run("CollectAllContinuedErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := &atomic.Int64{}

		err := SliceStream(makeIntSlice(10)).
			Parallel(
				func(_ context.Context, in int) error {
					count.Add(1)
					return fmt.Errorf("errored=%d", in)
				},
				wpa.WorkerGroupConfNumWorkers(4),
				wpa.WorkerGroupConfContinueOnError(),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}

		check.Equal(t, 10, count.Load())

		errs := ers.Unwind(err)
		if len(errs) != 10 {
			t.Log(errs)
			t.Error(len(errs), "!= 10", errs)
		}
	})
	t.Run("CollectAllErrors/Double", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceStream(makeIntSlice(100)).
			Parallel(
				func(_ context.Context, in int) error {
					return fmt.Errorf("errored=%d", in)
				},
				wpa.WorkerGroupConfNumWorkers(2),
			).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}

		errs := ers.Unwind(err)
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > 2 {
			t.Log("errs", errs)
			t.Error("num-errors", len(errs))
		}
	})
	t.Run("CollectAllErrors/Cores", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := SliceStream(makeIntSlice(100)).Parallel(
			func(_ context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
			},
			wpa.WorkerGroupConfWorkerPerCPU(),
		).Run(ctx)
		if err == nil {
			t.Error("should have propogated an error")
		}

		errs := ers.Unwind(err)
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > runtime.NumCPU() {
			t.Log("num-workers", runtime.NumCPU())
			t.Error("num-errors", len(errs))
		}
	})
	t.Run("IncludeContextErrors", func(t *testing.T) {
		t.Run("SuppressErrorsByDefault", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := SliceStream(makeIntSlice(2)).Parallel(
				func(_ context.Context, _ int) error {
					return context.Canceled
				},
			).Run(ctx)
			check.NotError(t, err)
			if err != nil {
				t.Error("should have skipped all errors", err)
			}
		})
		t.Run("WithErrors", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := SliceStream(makeIntSlice(2)).Parallel(
				func(_ context.Context, _ int) error {
					return context.Canceled
				},
				wpa.WorkerGroupConfIncludeContextErrors(),
			).Run(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, context.Canceled)
		})
	})
}

func RunStreamImplementationTests[T comparable](
	t *testing.T,
	elements []FixtureData[T],
	builders []FixtureStreamConstructors[T],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					// name := builder.Name
					t.Parallel()

					builder := func() *Stream[T] { return baseBuilder(elems) }

					t.Run("Single", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						seen := make(map[T]struct{}, len(elems))
						iter := builder()

						for iter.Next(ctx) {
							seen[iter.Value()] = struct{}{}
						}
						if err := iter.Close(); err != nil {
							t.Error(err)
						}

						CheckSeenMap(t, elems, seen)
					})
					t.Run("Canceled", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						cancel()
						var count int

						for iter.Next(ctx) {
							count++
						}
						err := iter.Close()
						if count > len(elems) && !errors.Is(err, context.Canceled) {
							t.Fatal("should not have iterated or reported err", count, err)
						}
					})
					t.Run("PanicSafety", func(t *testing.T) {
						t.Run("Map", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map[T](
								baseBuilder(elems),
								func(_ context.Context, _ T) (T, error) {
									panic("whoop")
								},
							).Slice(ctx)

							check.Error(t, err)
							check.ErrorIs(t, err, ers.ErrRecoveredPanic)

							if len(out) != 0 {
								t.Error("unexpected output", out)
							}
						})
						t.Run("ParallelMeap", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, _ T) (T, error) {
									panic("whoop")
								},
								wpa.WorkerGroupConfNumWorkers(2),
								wpa.WorkerGroupConfContinueOnError(),
							).Slice(ctx)

							if err == nil {
								t.Error("expected error")
							}

							assert.ErrorIs(t, err, ers.ErrRecoveredPanic)

							if !strings.Contains(err.Error(), "whoop") {
								t.Fatalf("panic error isn't propogated %q", err.Error())
							}
							if len(out) != 0 {
								t.Fatal("unexpected output", out)
							}
						})
					})
				})
			}
		})
	}
}

func RunStreamIntegerAlgoTests(
	t *testing.T,
	elements []FixtureData[int],
	builders []FixtureStreamConstructors[int],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					t.Parallel()

					t.Run("Map", func(t *testing.T) {
						t.Run("ErrorDoesNotAbort", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
									if input == elems[2] {
										return 0, errors.New("whoop")
									}
									return input, nil
								},
								wpa.WorkerGroupConfContinueOnError(),
							).Slice(ctx)
							if err == nil {
								t.Fatal("expected error", out)
							}

							if err.Error() != "whoop: whoop" {
								t.Error(err)
							}

							if len(out) != len(elems)-1 {
								t.Fatal("unexpected output", len(out), "->", out, len(elems)-1)
							}
						})

						t.Run("PanicDoesNotAbort", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
									if input == elems[3] {
										panic("whoops")
									}
									return input, nil
								},
								wpa.WorkerGroupConfContinueOnPanic(),
								wpa.WorkerGroupConfNumWorkers(1),
							).Slice(ctx)

							if err == nil {
								t.Error("expected error", err)
							}
							check.ErrorIs(t, err, ers.ErrRecoveredPanic)
							if len(out) != len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ErrorAborts", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()
							expectedErr := errors.New("whoop")
							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
									if input >= elems[2] {
										return 0, expectedErr
									}
									return input, nil
								},
								wpa.WorkerGroupConfNumWorkers(1),
							).Slice(ctx)
							if err == nil {
								t.Error("expected error")
							}
							if !errors.Is(err, expectedErr) {
								t.Error(err)
							}
							// we should abort, but there's some asynchronicity.
							if len(out) > len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out)
							}
						})
						t.Run("ParallelErrorDoesNotAbort", func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							expectedErr := errors.New("whoop")
							out, err := Map(
								baseBuilder(elems),
								func(_ context.Context, input int) (int, error) {
									if input == len(elems)/2+1 {
										return 0, expectedErr
									}
									return input, nil
								},
								wpa.WorkerGroupConfNumWorkers(4),
								wpa.WorkerGroupConfContinueOnError(),
							).Slice(ctx)
							if err == nil {
								t.Error("expected error")
							}
							if !errors.Is(err, expectedErr) {
								t.Error(err)
							}

							if len(out) != len(elems)-1 {
								t.Error("unexpected output", len(out), "->", out, len(elems))
							}
						})
					})
				})
			}
		})
	}
}

func RunStreamStringAlgoTests(
	t *testing.T,
	elements []FixtureData[string],
	builders []FixtureStreamConstructors[string],
) {
	for _, elems := range elements {
		t.Run(elems.Name, func(t *testing.T) {
			for _, builder := range builders {
				t.Run(builder.Name, func(t *testing.T) {
					baseBuilder := builder.Constructor
					elems := elems.Elements
					t.Parallel()

					builder := func() *Stream[string] { return (baseBuilder(elems)) }
					t.Run("Channel", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						seen := make(map[string]struct{}, len(elems))
						iter := builder()
						ch := iter.Channel(ctx)
						for str := range ch {
							seen[str] = struct{}{}
						}
						CheckSeenMap(t, elems, seen)
						if err := iter.Close(); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("Collect", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						vals, err := iter.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}
						check.EqualItems(t, elems, vals)
						if err := iter.Close(); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("Map", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						out := Map[string, string](
							iter,
							func(_ context.Context, str string) (string, error) {
								return str, nil
							},
						)

						vals, err := out.Slice(ctx)
						if err != nil {
							t.Fatal(err)
						}

						check.EqualItems(t, elems, vals)
					})
					t.Run("ParallelMap", func(t *testing.T) {
						vals, err := Map(
							MergeStreams(VariadicStream(builder(), builder(), builder())),
							func(_ context.Context, str string) (string, error) {
								for _, c := range []string{"a", "e", "i", "o", "u"} {
									str = strings.ReplaceAll(str, c, "")
								}
								return strings.TrimSpace(str), nil
							},
							wpa.WorkerGroupConfNumWorkers(4),
						).Slice(t.Context())
						if err != nil {
							t.Error(len(vals), " >>:", err)
						}
						longString := strings.Join(vals, "")
						count := 0
						for _, i := range longString {
							switch i {
							case 'a', 'e', 'i', 'o', 'u':
								count++
							case '\n', '\t':
								count += 100
							}
						}
						if count != 0 {
							t.Error("unexpected result", count)
						}
					})

					t.Run("Reduce", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()
						seen := make(map[string]struct{}, len(elems))
						sum, err := iter.Reduce(func(in string, value string) (string, error) {
							seen[in] = struct{}{}
							return fmt.Sprint(value, in), nil
						}).Read(ctx)
						if err != nil {
							t.Fatal(err)
						}
						CheckSeenMap(t, elems, seen)
						seenSum := 0
						for str := range seen {
							seenSum += len(str)
						}
						if seenSum != len(sum) {
							t.Errorf("incorrect seen %d, reduced %v", seenSum, sum)
						}
					})
					t.Run("ReduceError", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						iter := builder()

						seen := map[string]none{}
						expectedError := errors.New("boop")

						count := 0
						_, err := iter.Reduce(
							func(in string, val string) (string, error) {
								check.Zero(t, val)
								seen[in] = none{}
								count++
								if len(seen) == 3 {
									return val, expectedError
								}
								return "", nil
							},
						).Read(ctx)
						check.Equal(t, count, 3)
						if err == nil {
							t.Fatal("expected error")
						}

						if !errors.Is(err, expectedError) {
							t.Error("unexpected error:", err)
						}
						errs := ers.Unwind(err)
						if len(errs) != 2 {
							t.Error(len(errs), errs)
						}

						if l := len(seen); l != 3 {
							t.Error("seen", l, seen)
						}
					})
					t.Run("ReduceSkip", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						elems := makeIntSlice(32)
						iter := SliceStream(elems)
						count := 0
						sum, err := iter.Reduce(func(_ int, value int) (int, error) {
							count++
							if count == 1 {
								return 42, nil
							}
							return value, ers.ErrCurrentOpSkip
						}).Read(ctx)
						assert.NotError(t, err)
						assert.Equal(t, sum, 42)
						assert.Equal(t, count, 32)
					})
					t.Run("ReduceEarlyExit", func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						elems := makeIntSlice(32)
						iter := SliceStream(elems)
						count := 0
						sum, err := iter.Reduce(func(_ int, _ int) (int, error) {
							count++
							if count == 16 {
								return 300, io.EOF
							}
							return 42, nil
						}).Read(ctx)
						assert.ErrorIs(t, err, io.EOF)
						assert.Equal(t, sum, 42)
						assert.Equal(t, count, 16)
					})
				})
			}
		})
	}
	t.Run("ParallelProccesingInvalidConfig", func(t *testing.T) {
		counter := 0
		out, err := ConvertFn(func(in int) string { counter++; return fmt.Sprint(in) }).Parallel(
			SliceStream([]int{42, 84, 21}),
			wpa.WorkerGroupConfWithErrorCollector(nil),
		).Slice(t.Context())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
		assert.Zero(t, counter)
		assert.Nil(t, out)
	})
	t.Run("ConvertContinue", func(t *testing.T) {
		count := 0
		st := MakeStream(fnx.MakeFuture(func() (int, error) {
			count++

			if count%2 == 0 {
				return 0, ers.ErrCurrentOpSkip
			}
			if count < 16 {
				return count, nil
			}
			if count >= 16 {
				return 0, io.EOF
			}

			return count, ers.ErrInvariantViolation
		}))

		newSl, err := Convert(fnx.MakeConverterErr(func(in int) (string, error) {
			return strconv.Itoa(in), nil
		})).Stream(st).Slice(t.Context())
		check.NotError(t, err)
		check.Equal(t, len(newSl), 8)
		check.Equal(t, newSl[3], "7")
		check.Equal(t, false, slices.ContainsFunc(newSl, func(in string) bool {
			return in == "4"
		}))
		check.Equal(t, newSl[0], "1")
	})
}

func TestStreamReadAllWithErrStreamContinue(t *testing.T) {
	t.Run("SkipErrorsAndCollectValidValues", func(t *testing.T) {
		callCount := &atomic.Int64{}
		valueCount := 0
		validValues := []int{1, 3, 5, 7, 9}

		// Create a stream where operation returns ErrStreamContinue for even call counts
		st := MakeStream(func(ctx context.Context) (int, error) {
			count := int(callCount.Add(1))

			if count > 10 {
				return 0, io.EOF
			}

			// Return ErrStreamContinue for even counts (2, 4, 6, 8, 10)
			if count%2 == 0 {
				return 0, ErrStreamContinue
			}

			// Return valid values for odd counts (1, 3, 5, 7, 9)
			return count, nil
		})

		// Collect values using ReadAll
		collected := []int{}
		err := st.ReadAll(fnx.FromHandler(func(val int) {
			collected = append(collected, val)
			valueCount++
		})).Run(t.Context())

		check.NotError(t, err)

		// Verify operation was called more times than values collected
		assert.Equal(t, callCount.Load(), int64(11)) // Called 11 times (1-10 + final EOF check)
		assert.Equal(t, valueCount, 5)               // Only 5 valid values collected

		// Verify only odd values were collected
		assert.EqualItems(t, collected, validValues)
	})

	t.Run("MultipleConsecutiveSkips", func(t *testing.T) {
		callCount := &atomic.Int64{}
		pattern := []bool{true, false, false, false, true, false, true, true}

		st := MakeStream(func(ctx context.Context) (int, error) {
			count := int(callCount.Add(1))
			idx := count - 1

			if idx >= len(pattern) {
				return 0, io.EOF
			}

			if pattern[idx] {
				return count, nil
			}
			return 0, ers.ErrCurrentOpSkip
		})

		collected := []int{}
		err := st.ReadAll(fnx.FromHandler(func(val int) {
			collected = append(collected, val)
		})).Run(t.Context())

		check.NotError(t, err)

		// Operation called len(pattern) times + 1 for EOF
		assert.Equal(t, callCount.Load(), int64(len(pattern)+1))

		// Only positions where pattern is true should be collected
		expectedValues := []int{1, 5, 7, 8}
		assert.EqualItems(t, collected, expectedValues)
	})

	t.Run("AllSkipsUntilEOF", func(t *testing.T) {
		callCount := &atomic.Int64{}
		maxCalls := 10

		st := MakeStream(func(ctx context.Context) (int, error) {
			count := int(callCount.Add(1))

			if count > maxCalls {
				return 0, io.EOF
			}

			// Always return ErrStreamContinue
			return 0, ErrStreamContinue
		})

		collected := []int{}
		err := st.ReadAll(fnx.FromHandler(func(val int) {
			collected = append(collected, val)
		})).Run(t.Context())

		check.NotError(t, err)

		// Operation should have been called maxCalls+1 times
		assert.Equal(t, callCount.Load(), int64(maxCalls+1))

		// No values should have been collected
		assert.Equal(t, len(collected), 0)
	})

	t.Run("HandlerReturnsErrStreamContinue", func(t *testing.T) {
		callCount := &atomic.Int64{}
		handlerCallCount := &atomic.Int64{}

		st := MakeStream(func(ctx context.Context) (int, error) {
			count := int(callCount.Add(1))

			if count > 6 {
				return 0, io.EOF
			}

			return count, nil
		})

		collected := []int{}
		err := st.ReadAll(func(ctx context.Context, val int) error {
			handlerCallCount.Add(1)

			// Skip even values in the handler
			if val%2 == 0 {
				return ErrStreamContinue
			}

			collected = append(collected, val)
			return nil
		}).Run(t.Context())

		check.NotError(t, err)

		// Handler called 6 times (once for each value)
		assert.Equal(t, handlerCallCount.Load(), int64(6))

		// Only odd values collected
		assert.EqualItems(t, collected, []int{1, 3, 5})
	})
}
