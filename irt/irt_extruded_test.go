package irt

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"
)

func TestKeepOk(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		input := func(yield func(int, bool) bool) {
			_ = yield(1, true) && yield(2, false) && yield(3, true)
		}
		got := Collect(KeepOk(input))
		want := []int{1, 3}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("NilInput", func(t *testing.T) {
		got := Collect(KeepOk[int](nil))
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var count atomic.Int32
		input := func(yield func(int, bool) bool) {
			for i := 0; i < 10; i++ {
				count.Add(1)
				if !yield(i, true) {
					return
				}
			}
		}
		// Stop after 2 items
		i := 0
		for range KeepOk(input) {
			i++
			if i == 2 {
				break
			}
		}
		if count.Load() != 2 {
			t.Errorf("expected 2 iterations, got %d", count.Load())
		}
	})
}

func TestWhileOk(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		input := func(yield func(int, bool) bool) {
			_ = yield(1, true) && yield(2, true) && yield(3, false) && yield(4, true)
		}
		got := Collect(WhileOk(input))
		want := []int{1, 2}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("NilInput", func(t *testing.T) {
		got := Collect(WhileOk[int](nil))
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})
}

func TestWhileSuccess(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		err := errors.New("fail")
		input := func(yield func(int, error) bool) {
			_ = yield(1, nil) && yield(2, nil) && yield(3, err) && yield(4, nil)
		}
		got := Collect(WhileSuccess(input))
		want := []int{1, 2}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestWithHooks(t *testing.T) {
	t.Run("ExecutionOrder", func(t *testing.T) {
		var log []string
		seq := WithHooks(
			Slice([]int{1}),
			func() { log = append(log, "before") },
			func() { log = append(log, "after") },
		)
		for range seq {
			log = append(log, "item")
		}
		want := []string{"before", "item", "after"}
		if !slices.Equal(log, want) {
			t.Errorf("got %v, want %v", log, want)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var afterCalled bool
		seq := WithHooks(
			Slice([]int{1, 2, 3}),
			nil,
			func() { afterCalled = true },
		)
		for range seq {
			break
		}
		if !afterCalled {
			t.Error("after hook should be called even on early return")
		}
	})
	t.Run("NilSafety", func(t *testing.T) {
		seq := WithHooks[int](nil, nil, nil)
		Collect(seq) // Should not panic
	})
}

func TestWithSetup(t *testing.T) {
	t.Run("Once", func(t *testing.T) {
		var count atomic.Int32
		seq := WithSetup(Slice([]int{1, 2}), func() { count.Add(1) })
		Collect(seq)
		Collect(seq)
		if count.Load() != 1 {
			t.Errorf("setup called %d times, want 1", count.Load())
		}
	})
	t.Run("NilSafety", func(t *testing.T) {
		seq := WithSetup[int](nil, nil)
		Collect(seq) // Should not panic
	})
}
