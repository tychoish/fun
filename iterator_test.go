package fun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
)

func TestIteratorTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Observe", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			iter := SliceIterator([]int{})
			assert.NotError(t, iter.Observe(ctx, func(in int) { t.Fatal("should not be called") }))

			_, err := iter.ReadOne(ctx)
			assert.ErrorIs(t, err, io.EOF)
		})
		t.Run("PanicSafety", func(t *testing.T) {
			called := 0
			err := Sliceify([]int{1, 2, 34, 56}).Iterator().Observe(ctx, func(in int) {
				called++
				if in > 3 {
					panic("eep!")
				}
			})
			if err == nil {
				t.Fatal("should error")
			}
			if called != 3 {
				t.Error(called)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error(err)
			}
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			count := 0
			assert.Error(t, ctx.Err())
			err := Sliceify([]int{1, 2, 34, 56}).Iterator().Observe(ctx, func(int) {
				count++
			})
			t.Log(err)
			if !errors.Is(err, context.Canceled) {
				t.Error(err, ctx.Err())
			}
			if count != 0 {
				t.Error("expected no ops", count)
			}
		})
	})
	t.Run("IterateOne", func(t *testing.T) {
		t.Run("First", func(t *testing.T) {
			it, err := IterateOne[int](ctx, internal.NewSliceIter([]int{101, 2, 34, 56}))
			assert.NotError(t, err)
			assert.Equal(t, 101, it)
		})

		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			it, err := IterateOne[int](ctx, internal.NewSliceIter([]int{101, 2, 34, 56}))
			assert.ErrorIs(t, err, context.Canceled)
			assert.Zero(t, it)
		})
		t.Run("Empty", func(t *testing.T) {
			it, err := IterateOne[int](ctx, internal.NewSliceIter([]int{}))
			assert.Zero(t, it)
			assert.ErrorIs(t, err, io.EOF)
		})

		t.Run("ReadOneable", func(t *testing.T) {
			input := &TestReadoneableImpl{
				Iterable: internal.NewSliceIter([]string{
					fmt.Sprint(10),
					fmt.Sprint(10),
					fmt.Sprint(20),
					fmt.Sprint(2),
				}),
			}
			val, err := IterateOne[string](ctx, input)
			assert.NotError(t, err)
			assert.Equal(t, "sparta", val)

			val, err = IterateOne[string](ctx, input)
			assert.ErrorIs(t, err, io.EOF)
			assert.Zero(t, val)
		})

	})
	t.Run("Transform", func(t *testing.T) {
		input := SliceIterator([]string{
			fmt.Sprint(10),
			fmt.Sprint(10),
			fmt.Sprint(20),
			fmt.Sprint(2),
		})

		out := Transform[string](input, func(in string) (int, error) { return strconv.Atoi(in) })
		sum := 0
		for out.Next(ctx) {
			sum += out.Value()
		}
		assert.NotError(t, out.Close())
		assert.Equal(t, 42, sum)
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
		evens := testIntIter(t, 100).Filter(func(in int) bool { return in%2 == 0 })
		assert.Equal(t, evens.Count(ctx), 50)
		for evens.Next(ctx) {
			assert.True(t, evens.Value()%2 == 0)
		}
		assert.NotError(t, evens.Close())
	})
	t.Run("MapConverter", func(t *testing.T) {
		in := map[string]string{
			"hi":  "there",
			"how": "are you doing",
		}
		iter := MapIterator(in)
		seen := 0
		for iter.Next(ctx) {
			item := iter.Value()
			switch {
			case item.Key == "hi":
				check.Equal(t, item.Value, "there")
			case item.Key == "how":
				check.Equal(t, item.Value, "are you doing")
			default:
				t.Errorf("unexpected value: %s", item)
			}
			seen++
		}
		assert.Equal(t, seen, len(in))
	})
	t.Run("Split", func(t *testing.T) {
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
}

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

type TestReadoneableImpl struct {
	Iterable[string]
	once bool
}

func (iter *TestReadoneableImpl) ReadOne(ctx context.Context) (string, error) {
	if !iter.once {
		iter.once = true
		return "sparta", nil
	}
	return "", io.EOF
}
func GenerateRandomStringSlice(size int) []string {
	out := make([]string, size)
	for idx := range out {
		out[idx] = fmt.Sprint("value=", idx)
	}
	return out
}
