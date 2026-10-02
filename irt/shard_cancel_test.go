package irt

import (
	"context"
	"runtime"
	"testing"
)

func TestShardCancelReleasesPull(t *testing.T) {
	t.Run("UnfinishedShard", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		shards := Collect(Shard(ctx, 3, Monotonic()))
		for range shards[0] { // starts the pull; others never run
			break
		}
		cancel()
		goroutinesAtMost(t, base)
		for range shards[1] {
			t.Fatal("late shard yielded after cancel")
		}
	})
	t.Run("AlreadyCanceled", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, s := range Collect(Shard(ctx, 2, Monotonic())) {
			_ = s
		}
		goroutinesAtMost(t, base)
	})
	t.Run("EarlyBreakStillDoesNotStarve", func(t *testing.T) {
		shards := Collect(Shard(t.Context(), 2, Range(1, 6)))
		for range shards[0] {
			break
		}
		if got := len(Collect(shards[1])); got != 5 {
			t.Fatalf("got %d, want 5", got)
		}
	})
}
