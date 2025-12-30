package fun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/internal"
)

type Collector struct {
	mtx sync.Mutex
	err error
	num int
}

func (c *Collector) HasErrors() bool { defer internal.With(internal.Lock(&c.mtx)); return c.err != nil }
func (c *Collector) Resolve() error  { defer internal.With(internal.Lock(&c.mtx)); return c.err }
func (c *Collector) Len() int        { defer internal.With(internal.Lock(&c.mtx)); return c.num }
func (c *Collector) Add(err error) {
	if err == nil {
		return
	}

	defer internal.With(internal.Lock(&c.mtx))
	c.num++
	c.err = erc.Join(err, c.err)
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
				WorkerGroupConfNumWorkers(2),
				WorkerGroupConfContinueOnError(),
				WorkerGroupConfContinueOnPanic(),
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
			WorkerGroupConfWithErrorCollector(nil),
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
		name   string
		test   func(t *testing.T)
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
