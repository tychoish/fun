package irt

import (
	"math"
	"slices"
	"testing"
	"time"
)

func TestReiterableReview(t *testing.T) {
	twice := func(t *testing.T, name string, seq func() []int) {
		t.Helper()
		first, second := seq(), seq()
		if !slices.Equal(first, second) {
			t.Errorf("%s: first %v != second %v", name, first, second)
		}
	}

	rng := Range(1, 3)
	twice(t, "Range", func() []int { return Collect(rng) })

	mono := Limit(Monotonic(), 3)
	twice(t, "Monotonic", func() []int { return Collect(mono) })

	from := Limit(MonotonicFrom(10), 3)
	twice(t, "MonotonicFrom", func() []int { return Collect(from) })

	idx := Index(Slice([]string{"a", "b"}))
	twice(t, "Index", func() (out []int) {
		for i := range idx {
			out = append(out, i)
		}
		return out
	})

	lim := Limit(Slice([]int{1, 2, 3, 4}), 2)
	twice(t, "Limit", func() []int { return Collect(lim) })

	uniq := Unique(Slice([]int{1, 1, 2}))
	twice(t, "Unique", func() []int { return Collect(uniq) })

	uniqBy := UniqueBy(Slice([]int{1, 1, 2}), func(i int) int { return i })
	twice(t, "UniqueBy", func() []int { return Collect(uniqBy) })

	t.Run("RangeEdges", func(t *testing.T) {
		within(t, 2*time.Second, func() {
			if got := Collect(Range(5, 4)); len(got) != 0 {
				t.Errorf("empty range: %v", got)
			}
			if got := Collect(Range(3, 3)); !slices.Equal(got, []int{3}) {
				t.Errorf("single: %v", got)
			}
			if got := Collect(Range(math.MaxInt-1, math.MaxInt)); len(got) != 2 {
				t.Errorf("MaxInt end: %v", got)
			}
		})
	})
}
