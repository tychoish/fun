package irt

import (
	"slices"
	"testing"
	"time"
)

func TestSizeArgumentsReview(t *testing.T) {
	t.Run("WithBufferNonPositive", func(t *testing.T) {
		for _, size := range []int{-100, -1, 0} {
			within(t, 5*time.Second, func() {
				got := Collect(WithBuffer(t.Context(), Range(1, 3), size))
				if !slices.Equal(got, []int{1, 2, 3}) {
					t.Errorf("size %d: %v", size, got)
				}
				if got := Collect(WithBuffer(t.Context(), Slice([]int{}), size)); len(got) != 0 {
					t.Errorf("size %d empty: %v", size, got)
				}
			})
		}
	})
	t.Run("CollectFirstNHuge", func(t *testing.T) {
		within(t, 5*time.Second, func() {
			got := CollectFirstN(Range(1, 3), 1<<62)
			if !slices.Equal(got, []int{1, 2, 3}) {
				t.Errorf("got %v", got)
			}
			if got := CollectFirstN(Slice([]int{}), 1<<62); len(got) != 0 {
				t.Errorf("got %v", got)
			}
			if got := CollectFirstN(Monotonic(), 2000); len(got) != 2000 {
				t.Errorf("got %d", len(got))
			}
		})
	})
}
