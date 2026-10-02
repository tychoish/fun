package irt

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShardReview(t *testing.T) {
	t.Run("EarlyBreakDoesNotStarveOthers", func(t *testing.T) {
		within(t, 5*time.Second, func() {
			shards := Collect(Shard(t.Context(), 2, Range(1, 10)))
			if len(shards) != 2 {
				t.Errorf("shards: %d", len(shards))
				return
			}
			for range shards[0] {
				break
			}
			if got := Collect(shards[1]); len(got) != 9 {
				t.Errorf("expected 9 remaining, got %v", got)
			}
		})
	})
	t.Run("NonPositive", func(t *testing.T) {
		within(t, 5*time.Second, func() {
			for _, n := range []int{0, -3} {
				shards := Collect(Shard(t.Context(), n, Range(1, 4)))
				if len(shards) != 1 || len(Collect(shards[0])) != 4 {
					t.Errorf("num %d: %v", n, shards)
				}
			}
		})
	})
	t.Run("NeverIterated", func(t *testing.T) {
		within(t, 5*time.Second, func() {
			for range Shard(t.Context(), 3, Range(1, 4)) {
			}
		})
	})
	t.Run("CancelledMidFlight", func(t *testing.T) {
		within(t, 5*time.Second, func() {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			shards := Collect(Shard(ctx, 2, Monotonic()))
			n := 0
			for range shards[0] {
				if n++; n == 5 {
					cancel()
				}
			}
			if got := Collect(shards[1]); len(got) != 0 {
				t.Errorf("expected nothing after cancel, got %d", len(got))
			}
		})
	})
	t.Run("ConcurrentConsumersTotal", func(t *testing.T) {
		within(t, 10*time.Second, func() {
			const items = 1000
			var total atomic.Int64
			var wg sync.WaitGroup
			shards := Collect(Shard(t.Context(), 4, Range(1, items)))
			for i, s := range shards {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for range s {
						total.Add(1)
						if i == 0 && total.Load() > 10 {
							return // one shard quits early
						}
					}
				}()
			}
			wg.Wait()
			if total.Load() != items {
				t.Errorf("total %d, want %d", total.Load(), items)
			}
		})
	})
}
