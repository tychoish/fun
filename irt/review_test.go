package irt

import (
	"slices"
	"testing"
	"time"
)

// within fails the test if op does not return before the timeout.
func within(t *testing.T, d time.Duration, op func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); op() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("operation timed out")
	}
}

func TestChunkReview(t *testing.T) {
	t.Run("IgnoredChunksTerminate", func(t *testing.T) {
		within(t, 2*time.Second, func() {
			n := 0
			for range Chunk(Slice([]int{1, 2, 3, 4}), 2) {
				n++
				if n > 100 {
					return
				}
			}
			if n != 2 {
				t.Errorf("got %d chunks", n)
			}
		})
	})
	t.Run("NoEmptyTail", func(t *testing.T) {
		for _, size := range []int{1, 2, 3, 4, 6, 12} {
			var got [][]int
			within(t, 2*time.Second, func() {
				for c := range Chunk(Slice([]int{1, 2, 3, 4, 5, 6}), size) {
					got = append(got, Collect(c))
				}
			})
			for _, c := range got {
				if len(c) == 0 {
					t.Fatalf("size %d empty chunk in %v", size, got)
				}
			}
		}
		var got [][]int
		for c := range Chunk(Slice([]int{1, 2, 3, 4}), 2) {
			got = append(got, Collect(c))
		}
		if len(got) != 2 || !slices.Equal(got[1], []int{3, 4}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("PartialConsumption", func(t *testing.T) {
		var got []int
		within(t, 2*time.Second, func() {
			for c := range Chunk(Slice([]int{1, 2, 3, 4, 5, 6, 7}), 3) {
				for v := range c {
					got = append(got, v)
					break
				}
			}
		})
		if !slices.Equal(got, []int{1, 4, 7}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("NonPositiveAndEmpty", func(t *testing.T) {
		within(t, time.Second, func() {
			for _, n := range []int{0, -1} {
				if len(Collect(Chunk(Slice([]int{1, 2}), n))) != 0 {
					t.Error("expected empty")
				}
			}
			if len(Collect(Chunk(Slice([]int{}), 2))) != 0 {
				t.Error("expected empty")
			}
		})
	})
}
