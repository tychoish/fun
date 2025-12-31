package pubsub

import (
	"context"
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

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
		ctx := t.Context()
		start := time.Now()
		count := &intish.Atomic[int]{}
		assert.NotError(t, fun.IteratorStream(RateLimit(ctx, irt.Slice(makeIntSlice(100)), 10, 100*time.Millisecond)).
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
		ctx := t.Context()
		workers := irt.Convert(RateLimit(ctx, irt.Slice(makeIntSlice(101)), 10, 10*time.Millisecond), func(in int) fnx.Worker {
			return func(ctx context.Context) error {
				check.True(t, in >= 0)
				check.True(t, in <= 100)
				count.Add(1)
				testt.Log(t, count.Get(), "-->", time.Now())
				return nil
			}
		})
		wg := &sync.WaitGroup{}
		for shard := range irt.Shard(ctx, 3, workers) {
			wg.Add(1)
			go func(seq iter.Seq[fnx.Worker]) {
				err := fnx.RunAll(seq).Run(ctx)
				assert.NotError(t, err)
			}(shard)
		}
		wg.Done()
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
		for in := range RateLimit(ctx, irt.Slice(makeIntSlice(100)), 10, 100*time.Second) {
			check.True(t, in >= 0)
			check.True(t, in <= 100)
			count.Add(1)
			testt.Log(t, count.Get(), "-->", time.Now())
		}
		end := time.Now()
		dur := end.Sub(start)

		testt.Logf(t, "start at %s, end at %s; duration=%s ", start, end, dur)

		assert.Equal(t, 10, count.Get())
		assert.True(t, dur >= time.Second)
	})
}
