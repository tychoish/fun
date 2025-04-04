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
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

type Collector struct {
	mtx sync.Mutex
	err error
	num int
}

func (c *Collector) HasErrors() bool { defer with(lock(&c.mtx)); return c.err != nil }
func (c *Collector) Resolve() error  { defer with(lock(&c.mtx)); return c.err }
func (c *Collector) Len() int        { defer with(lock(&c.mtx)); return c.num }
func (c *Collector) Add(err error) {
	if err == nil {
		return
	}

	defer with(lock(&c.mtx))
	c.num++
	c.err = ers.Join(err, c.err)
}

type none struct{}

func testIntIter(t *testing.T, size int) *Iterator[int] {
	t.Helper()

	var count int

	t.Cleanup(func() {
		t.Helper()
		check.Equal(t, count, size)
	})

	return Generator(func(context.Context) (int, error) {
		if count >= size {
			return 0, io.EOF
		}
		count++
		return count - 1, nil
	})

}

func GenerateRandomStringSlice(size int) []string {
	out := make([]string, size)
	for idx := range out {
		out[idx] = fmt.Sprint("value=", idx)
	}
	return out
}

func TestIterator(t *testing.T) {
	t.Run("Observe", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{})
			assert.NotError(t, iter.Observe(func(_ int) { t.Fatal("should not be called") }).Run(ctx))

			_, err := iter.ReadOne(ctx)
			assert.ErrorIs(t, err, io.EOF)
		})
		t.Run("PanicSafety", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := 0
			err := SliceIterator([]int{1, 2, 34, 56}).Observe(func(in int) {
				called++
				if in > 3 {
					panic("eep!")
				}
			}).Run(ctx)
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
			err := SliceIterator([]int{1, 2, 34, 56}).Observe(func(int) {
				count++
			}).Run(ctx)
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
		iter := Generator(func(_ context.Context) (int, error) {
			count++
			switch {
			case count > 128:
				count = 128
				return -1, io.EOF
			case count%2 == 0:
				return count, nil
			default:
				return -1, ErrIteratorSkip
			}
		})
		ints, err := iter.Slice(ctx)
		check.NotError(t, err)
		check.Equal(t, len(ints), 64)
		check.Equal(t, 128, count)
	})
	t.Run("Monotonic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		const size = 37017
		count := 0
		last := -1
		check.NotError(t, HF.Counter(size).Observe(func(in int) { count++; check.True(t, last < in); last = in }).Run(ctx))
		check.Equal(t, size, count)
		check.Equal(t, last, count)
	})
	t.Run("Transform", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		out := ft.Must(VariadicIterator(4, 8, 16, 32, 64, 128, 256, 512, 1024).Transform(Converter(func(in int) int { return in / 4 })).Slice(ctx))

		check.EqualItems(t, out, []int{1, 2, 4, 8, 16, 32, 64, 128, 256})
	})
	t.Run("Process", func(t *testing.T) {
		t.Run("Process", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.Process(func(_ context.Context, _ int) error { count++; return nil }).Run(ctx)
			assert.NotError(t, err)
			check.Equal(t, 9, count)
		})
		t.Run("ProcessWorker", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			op := Processor[int](func(_ context.Context, _ int) error { count++; return nil }).Iterator(iter)
			err := op(ctx)
			assert.NotError(t, err)
			check.Equal(t, 9, count)
		})
		t.Run("Abort", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.Process(func(_ context.Context, _ int) error { count++; return io.EOF }).Run(ctx)
			assert.NotError(t, err)
			assert.Equal(t, 1, count)
		})
		t.Run("OperationError", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.Process(func(_ context.Context, _ int) error { count++; return ers.ErrLimitExceeded }).Run(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrLimitExceeded)
			assert.Equal(t, 1, count)
		})
		t.Run("Panic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.Process(func(_ context.Context, _ int) error { count++; panic(ers.ErrLimitExceeded) }).Run(ctx)
			assert.Error(t, err)
			check.Equal(t, 1, count)
			check.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("ContextExpired", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := 0
			err := iter.Process(func(ctx context.Context, _ int) error { count++; cancel(); return ctx.Err() }).Run(ctx)
			assert.Error(t, err)
			check.Equal(t, 1, count)
			check.ErrorIs(t, err, context.Canceled)
		})

		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
			count := &atomic.Int64{}
			err := iter.ProcessParallel(
				func(_ context.Context, _ int) error { count.Add(1); return nil },
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
			input := SliceIterator([]string{
				fmt.Sprint(10),
				fmt.Sprint(10),
				fmt.Sprint(20),
				fmt.Sprint(2),
			})
			calls := 0

			out := ConvertIterator(input,
				ConverterOk(func(in string) (int, bool) {
					calls++
					return ers.WithRecoverOk(func() (int, error) { return strconv.Atoi(in) })
				}),
			)
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
			input := SliceIterator([]string{
				fmt.Sprint(10),
				fmt.Sprint(10),
				fmt.Sprint(20),
				fmt.Sprint(2),
			})
			calls := 0

			out := ConvertIterator(input, ConverterErr(func(in string) (int, error) {
				if in == "2" {
					return 0, ErrIteratorSkip
				}
				calls++
				return strconv.Atoi(in)
			}))
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
			input := SliceIterator([]string{
				fmt.Sprint(10),
				fmt.Sprint(10),
				fmt.Sprint(20),
				fmt.Sprint(2),
			})
			calls := 0

			out := ConvertIterator(input, ConverterErr(func(in string) (int, error) {
				if in == "20" {
					return 0, ers.ErrInvalidInput
				}
				calls++
				return strconv.Atoi(in)
			}))
			sum := 0
			for out.Next(ctx) {
				sum += out.Value()
			}
			assert.Error(t, out.Close())
			assert.ErrorIs(t, out.Close(), ers.ErrInvalidInput)

			assert.Equal(t, 20, sum)
			assert.Equal(t, calls, 2)

		})
	})
	t.Run("Generator", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run("BasicOperation", func(t *testing.T) {
			iter := Generator(func(context.Context) (int, error) {
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
			iter := Generator(func(context.Context) (int, error) {
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
			iter := Generator(func(context.Context) (int, error) {
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
		input := SliceIterator(GenerateRandomStringSlice(100))

		splits := input.Split(0)
		if splits != nil {
			t.Fatal("should be nil if empty")
		}

		splits = input.Split(10)
		if len(splits) != 10 {
			t.Fatal("didn't make enough split")
		}

		count := &atomic.Int64{}

		wg := &WaitGroup{}
		for _, iter := range splits {
			wg.Add(1)

			go func(it *Iterator[string]) {
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
		input := SliceIterator(GenerateRandomStringSlice(128))
		buf := input.Buffer(256)
		check.Equal(t, buf.Count(ctx), 128)
		check.True(t, buf.closer.state.Load())
		check.True(t, input.closer.state.Load())
	})
	t.Run("ParallelBuffer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		input := SliceIterator(GenerateRandomStringSlice(128))
		buf := input.ParallelBuffer(256)
		check.Equal(t, buf.Count(ctx), 128)
		check.True(t, buf.closer.state.Load())
		check.True(t, input.closer.state.Load())
	})
	t.Run("Slice", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			iter := SliceIterator([]int{1, 2, 3, 4})
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
			iter := &Iterator[int]{
				operation: func(context.Context) (int, error) { return 0, nil },
			}
			iter.closer.op = func() {}
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
		})
	})
	t.Run("ReadOne", func(t *testing.T) {
		ch := make(chan string, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		ch <- "merlin"
		defer cancel()
		out, err := Blocking(ch).Receive().Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if out != "merlin" {
			t.Fatal(out)
		}

		ch <- "merlin"
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
			case ch <- "merlin":
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
			ch <- "merlin"
			defer cancel()
			out, err := NonBlocking(ch).Receive().Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if out != "merlin" {
				t.Fatal(out)
			}

			ch <- "merlin"
			cancel()
			seenCondition := false
			for i := 0; i < 10; i++ {
				t.Log(i)
				_, err = NonBlocking(ch).Receive().Read(ctx)
				if errors.Is(err, context.Canceled) {
					seenCondition = true
				}
				t.Log(err)

				select {
				case ch <- "merlin":
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

		iter := FlattenIterators(
			VariadicIterator(
				SliceIterator(elems),
				SliceIterator(elems),
				SliceIterator(elems),
				SliceIterator(elems),
				SliceIterator(elems),
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
		iter := ChainIterators(
			Blocking(pipe).Iterator(),
			Blocking(pipe).Iterator(),
			Blocking(pipe).Iterator(),
		)

		ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		if iter.Next(ctx) {
			t.Error("no iteration", iter.Value())
		}
	})

}

func TestAny(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sl := []int{1, 1, 2, 3, 5, 8, 9, 5}
	count := 0
	err := SliceIterator(sl).Any().Observe(func(in any) {
		count++
		_, ok := in.(int)
		check.True(t, ok)
	}).Run(ctx)
	assert.NotError(t, err)
	assert.Equal(t, count, 8)
}

func TestEmptyIteration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan int)
	close(ch)

	t.Run("EmptyObserve", func(t *testing.T) {
		assert.NotError(t, SliceIterator([]int{}).Observe(func(_ int) { t.Fatal("should not be called") }).Run(ctx))
		assert.NotError(t, VariadicIterator[int]().Observe(func(_ int) { t.Fatal("should not be called") }).Run(ctx))
		assert.NotError(t, ChannelIterator(ch).Observe(func(_ int) { t.Fatal("should not be called") }).Run(ctx))
	})

}

func TestChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	num := []int{1, 2, 3, 5, 7, 9, 11, 13, 17, 19}
	iter := SliceIterator(num).Join(SliceIterator(num))

	n := iter.Count(ctx)

	assert.Equal(t, len(num)*2, n)

	iter = SliceIterator(num).Join(SliceIterator(num), SliceIterator(num), SliceIterator(num))
	cancel()
	n = iter.Count(ctx)
	assert.Equal(t, n, 0)
}

func TestJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("RoundTrip", func(t *testing.T) {
		iter := SliceIterator([]int{400, 300, 42})
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
		iter := SliceIterator([]Worker{func(context.Context) error { return nil }})
		_, err := iter.MarshalJSON()
		if err == nil {
			t.Fatal(err)
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		iter := SliceIterator([]string{})
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
		iter := SliceIterator([]string{})

		ec := iter.ErrorHandler()
		ec(io.EOF)
		ec(ers.ErrInvalidInput)
		ec(io.ErrUnexpectedEOF)
		ec(context.Canceled)

		err := iter.Close()
		assert.Error(t, err)
		assert.Equal(t, len(ers.Unwind(err)), 4)

		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.True(t, ers.IsExpiredContext(err))
		assert.True(t, ers.IsTerminating(err))
		assert.ErrorIs(t, err, ers.ErrInvalidInput)
	})
	t.Run("Channel", func(t *testing.T) {
		iter := SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
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
}
