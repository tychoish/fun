package pubsub

import (
	"context"
	"iter"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/fun/wpa"
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
		count := &atomic.Int64{}

		check.MinRuntime(t, 100*time.Millisecond, func() {
			for num := range RateLimit(ctx, irt.Slice(makeIntSlice(100)), 10, 100*time.Millisecond) {
				check.True(t, num >= 0)
				check.True(t, num <= 100)
				count.Add(1)
				testt.Log(t, num, count.Load(), "-->", time.Now())
			}
		})

		assert.Equal(t, 100, count.Load())
	})
	t.Run("Parallel", func(t *testing.T) {
		count := &atomic.Int64{}
		ctx := t.Context()

		check.MinRuntime(t, 100*time.Millisecond, func() {
			workers := irt.Convert(RateLimit(ctx, irt.Slice(makeIntSlice(101)), 10, 100*time.Millisecond), func(in int) fnx.Worker {
				return func(ctx context.Context) error {
					check.True(t, in >= 0)
					check.True(t, in <= 100)
					count.Add(1)
					testt.Log(t, in, count.Load(), "-->", time.Now())
					return nil
				}
			})
			wg := &sync.WaitGroup{}
			for shard := range irt.Shard(ctx, 3, workers) {
				wg.Add(1)
				go func(seq iter.Seq[fnx.Worker]) {
					defer wg.Done()
					err := wpa.RunAll(seq).Run(ctx)
					check.NotError(t, err)
				}(shard)
			}
			wg.Wait()
		})

		assert.Equal(t, 101, count.Load())
	})
	t.Run("Cancelation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		start := time.Now()
		go func() { time.Sleep(time.Second); cancel() }()
		count := &atomic.Int64{}
		for in := range RateLimit(ctx, irt.Slice(makeIntSlice(100)), 10, 100*time.Second) {
			check.True(t, in >= 0)
			check.True(t, in <= 100)
			count.Add(1)
			testt.Log(t, count.Load(), "-->", time.Now())
		}
		end := time.Now()
		dur := end.Sub(start)

		testt.Logf(t, "start at %s, end at %s; duration=%s ", start, end, dur)

		assert.Equal(t, 10, count.Load())
		assert.True(t, dur >= time.Second)
	})
}
