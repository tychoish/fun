package itertool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/fun/testt"
)

func TestSmoke(t *testing.T) {
	t.Parallel()
	t.Run("Worker", func(t *testing.T) {
		ctx := testt.Context(t)

		count := &atomic.Int64{}
		err := WorkerPool(fun.SliceStream([]fnx.Operation{
			func(context.Context) { count.Add(1) },
			func(context.Context) { count.Add(1) },
			func(context.Context) { count.Add(1) },
		})).Run(ctx)
		assert.NotError(t, err)
		assert.Equal(t, count.Load(), 3)
	})
	t.Run("MapReduce", func(t *testing.T) {
		prod := MapReduce(
			fun.VariadicStream(1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024),
			func(_ context.Context, in int) (string, error) {
				return fmt.Sprint(in), nil
			},
			func(strv string, rv int16) (int16, error) {
				out, err := strconv.Atoi(strv)
				if err != nil {
					return 0, err
				}
				return rv + int16(out), nil
			},
			0,
			fun.WorkerGroupConfNumWorkers(2),
		)
		ctx := testt.Context(t)

		value, err := prod(ctx)
		assert.NotError(t, err)
		assert.Equal(t, value, 2047)
	})
	t.Run("LegacyReduce", func(t *testing.T) {
		type none struct{}
		t.Run("Reduce", func(t *testing.T) {
			ctx := testt.Context(t)

			elems := makeIntSlice(32)
			iter := fun.SliceStream(elems)
			seen := make(map[int]struct{}, len(elems))
			sum, err := Reduce(
				iter,
				func(in int, value string) (string, error) {
					seen[in] = struct{}{}
					return fmt.Sprint(value, in), nil
				}, "").Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(seen) != len(elems) {
				t.Errorf("incorrect seen %d, reduced %s", seen, sum)
			}
			check.Equal(t, 54, len(sum))
		})
		t.Run("ReduceError", func(t *testing.T) {
			ctx := testt.Context(t)

			elems := makeIntSlice(32)
			iter := fun.SliceStream(elems)

			seen := map[int]none{}

			count := 0
			_, err := Reduce(
				iter,
				func(in int, val int) (int, error) {
					check.Zero(t, val)
					seen[in] = none{}
					if len(seen) == 2 {
						return val, errors.New("boop")
					}
					count++
					return in + val, nil
				},
				0,
			).Read(ctx)

			check.Equal(t, count, 1)
			if err == nil {
				t.Fatal("expected error")
			}

			if e := err.Error(); e != "boop" {
				t.Error("unexpected error:", e)
			}
			if l := len(seen); l != 2 {
				t.Error("seen", l, seen)
			}
		})
		t.Run("ReduceSkip", func(t *testing.T) {
			ctx := testt.Context(t)

			elems := makeIntSlice(32)
			iter := fun.SliceStream(elems)
			count := 0
			sum, err := Reduce(
				iter,
				func(_ int, value int) (int, error) {
					count++
					if count == 1 {
						return 42, nil
					}
					return value, ers.ErrCurrentOpSkip
				}, 0).Read(ctx)
			assert.NotError(t, err)
			assert.Equal(t, sum, 42)
			assert.Equal(t, count, 32)
		})
		t.Run("ReduceEarlyExit", func(t *testing.T) {
			ctx := testt.Context(t)
			elems := makeIntSlice(32)
			iter := fun.SliceStream(elems)
			count := 0
			sum, err := Reduce(
				iter,
				func(int, int) (int, error) {
					count++
					if count == 16 {
						return 0, io.EOF
					}
					return 42, nil
				}, 0).Read(ctx)
			assert.NotError(t, err)
			assert.Equal(t, sum, 42)
			assert.Equal(t, count, 16)
		})
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Error", func(t *testing.T) {
			buf := &bytes.Buffer{}
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))

			iter := JSON[int](buf)
			count := 0
			ctx := testt.Context(t)
			for iter.Next(ctx) {
				count++
				check.Equal(t, 0, iter.Value())
			}
			err := iter.Close()
			testt.Log(t, err)

			check.Equal(t, 0, count)
			check.Error(t, err)
		})
		t.Run("Pass", func(t *testing.T) {
			buf := &bytes.Buffer{}
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))
			buf.WriteString(`{"a": 1}`)
			buf.Write([]byte("\n"))

			iter := JSON[map[string]int](buf)
			count := 0
			ctx := testt.Context(t)
			for iter.Next(ctx) {
				count++
				mp := iter.Value()
				check.True(t, mp != nil)
				check.Equal(t, len(mp), 1)
				check.Equal(t, mp["a"], 1)
			}
			err := iter.Close()
			testt.Log(t, err)
			check.Equal(t, 4, count)
			check.NotError(t, err)
		})
	})
}

func TestChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	num := []int{1, 2, 3, 5, 7, 9, 11, 13, 17, 19}
	iter := fun.JoinStreams[int](fun.SliceStream(num), fun.SliceStream(num))
	n := iter.Count(ctx)
	assert.Equal(t, len(num)*2, n)

	iter = fun.JoinStreams[int](fun.SliceStream(num), fun.SliceStream(num), fun.SliceStream(num), fun.SliceStream(num))
	cancel()
	n = iter.Count(ctx)
	assert.Equal(t, n, 0)
}

func makeIntSlice(size int) []int {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = i
	}
	return out
}

func TestRateLimit(t *testing.T) {
	t.Parallel()

	t.Run("Serial", func(t *testing.T) {
		start := time.Now()
		count := &intish.Atomic[int]{}
		assert.NotError(t, RateLimit(
			fun.SliceStream(makeIntSlice(100)), 10, 100*time.Millisecond).
			ReadAll(fnx.FromHandler(func(in int) {
				check.True(t, in >= 0)
				check.True(t, in <= 100)
				count.Add(1)
				testt.Log(t, count.Get(), "-->", time.Now())
			})).Run(testt.Context(t)))
		end := time.Now()
		dur := end.Sub(start)

		testt.Logf(t, "start at %s, end at %s; duration=%s ", start, end, dur)

		assert.True(t, dur >= 100*time.Millisecond)
		assert.Equal(t, 100, count.Get())
	})
	t.Run("Parallel", func(t *testing.T) {
		start := time.Now()
		count := &intish.Atomic[int]{}
		assert.NotError(t, RateLimit(fun.SliceStream(makeIntSlice(101)), 10, 10*time.Millisecond).
			Parallel(fnx.FromHandler(func(in int) {
				check.True(t, in >= 0)
				check.True(t, in <= 100)
				count.Add(1)
				testt.Log(t, count.Get(), "-->", time.Now())
			}), fun.WorkerGroupConfNumWorkers(4)).Run(testt.Context(t)))
		end := time.Now()
		dur := end.Sub(start)

		testt.Logf(t, "start at %s, end at %s; duration=%s ", start, end, dur)

		assert.True(t, dur >= 5*time.Millisecond)
		assert.Equal(t, 101, count.Get())
	})
	t.Run("Cancelation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		start := time.Now()
		go func() { time.Sleep(time.Second); cancel() }()
		count := &intish.Atomic[int]{}
		err := RateLimit(fun.SliceStream(makeIntSlice(100)), 10, 100*time.Second).
			ReadAll(fnx.FromHandler(func(in int) {
				check.True(t, in >= 0)
				check.True(t, in <= 100)
				count.Add(1)
				testt.Log(t, count.Get(), "-->", time.Now())
			})).Run(ctx)
		end := time.Now()
		dur := end.Sub(start)

		assert.Error(t, err)
		assert.True(t, ers.IsExpiredContext(err))
		testt.Logf(t, "start at %s, end at %s; duration=%s ", start, end, dur)

		assert.Equal(t, 10, count.Get())
		assert.True(t, dur >= time.Second)
	})
}
