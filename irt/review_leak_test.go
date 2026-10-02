package irt

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// goroutinesAtMost waits for the goroutine count to drop to at most base.
func goroutinesAtMost(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutine leak: have %d, want <= %d", runtime.NumGoroutine(), base)
}

func TestWithBufferLeakReview(t *testing.T) {
	base := runtime.NumGoroutine()
	for range 10 {
		for range WithBuffer(t.Context(), Monotonic(), 1) {
			break
		}
	}
	goroutinesAtMost(t, base)
}

func TestAsChannel(t *testing.T) {
	t.Run("Abandonment", func(t *testing.T) {
		base := runtime.NumGoroutine()
		stops := make([]func(), 0, 10)
		for range 10 {
			ch, stop := AsChannel(t.Context(), Monotonic())
			<-ch // read one, then abandon
			stops = append(stops, stop)
		}
		for _, stop := range stops {
			stop()
		}
		goroutinesAtMost(t, base)
	})
	t.Run("StopWithoutReading", func(t *testing.T) {
		base := runtime.NumGoroutine()
		for range 10 {
			_, stop := AsChannel(t.Context(), Monotonic())
			stop()
		}
		goroutinesAtMost(t, base)
	})
	t.Run("StopClosesChannel", func(t *testing.T) {
		ch, stop := AsChannel(t.Context(), Monotonic())
		<-ch
		stop()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return
				}
			case <-deadline:
				t.Fatal("channel not closed after stop")
			}
		}
	})
	t.Run("DoubleStop", func(t *testing.T) {
		ch, stop := AsChannel(t.Context(), Slice([]int{1}))
		stop()
		stop()
		for range ch {
		}
	})
	t.Run("StopAfterExhaustion", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ch, stop := AsChannel(t.Context(), Slice([]int{1, 2, 3}))
		var got []int
		for v := range ch {
			got = append(got, v)
		}
		stop()
		stop()
		if len(got) != 3 || got[0] != 1 || got[2] != 3 {
			t.Fatalf("got %v", got)
		}
		goroutinesAtMost(t, base)
	})
	t.Run("ContextCancel", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(t.Context())
		ch, stop := AsChannel(ctx, Monotonic())
		defer stop()
		<-ch
		cancel()
		goroutinesAtMost(t, base)
		for range ch {
		}
	})
	t.Run("AlreadyCancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		ch, stop := AsChannel(ctx, Slice([]int{1, 2, 3}))
		defer stop()
		for range ch {
			t.Fatal("received from a cancelled AsChannel")
		}
	})
}
