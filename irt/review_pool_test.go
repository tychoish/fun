package irt

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// capturePanic runs op on the calling goroutine and reports what it panicked with.
func capturePanic(op func()) (val any) {
	defer func() { val = recover() }()
	op()
	return nil
}

func TestPoolPanicReview(t *testing.T) {
	// each case must run on this goroutine so recover can see the panic;
	// a worker-goroutine panic would crash the whole test binary.
	cases := map[string]func(){
		"Op": func() {
			for range Pool(t.Context(), 4, Range(1, 100), func(i int) int {
				if i == 5 {
					panic("op boom")
				}
				return i
			}) {
			}
		},
		"LoopBody": func() {
			for range Pool(t.Context(), 4, Range(1, 100), func(i int) int { return i }) {
				panic("body boom")
			}
		},
		"Source": func() {
			src := func(yield func(int) bool) {
				yield(1)
				panic("source boom")
			}
			for range Pool(t.Context(), 4, src, func(i int) int { return i }) {
			}
		},
		"Pool3Op": func() {
			for range Pool3(t.Context(), 4, Range(1, 100), func(i int) (int, int) {
				if i == 5 {
					panic("op boom")
				}
				return i, i
			}) {
			}
		},
		"Pool3LoopBody": func() {
			for range Pool3(t.Context(), 4, Range(1, 100), func(i int) (int, int) { return i, i }) {
				panic("body boom")
			}
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			within(t, 5*time.Second, func() {
				if capturePanic(run) == nil {
					t.Error("panic did not reach the caller")
				}
			})
		})
	}
}

func TestPoolContextReview(t *testing.T) {
	t.Run("CancelledBeforeStart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var pulled, ops atomic.Int64
		src := func(yield func(int) bool) {
			for i := range 10 {
				pulled.Add(1)
				if !yield(i) {
					return
				}
			}
		}
		within(t, 5*time.Second, func() {
			Collect(Pool(ctx, 3, src, func(i int) int { ops.Add(1); return i }))
			Collect(First(Pool3(ctx, 3, src, func(i int) (int, int) { ops.Add(1); return i, i })))
		})
		if pulled.Load() != 0 || ops.Load() != 0 {
			t.Errorf("pulled=%d ops=%d after pre-cancelled ctx", pulled.Load(), ops.Load())
		}
	})
	t.Run("NoPullAfterCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var pulled atomic.Int64
		src := func(yield func(int) bool) {
			for i := range 1000 {
				pulled.Add(1)
				if !yield(i) {
					return
				}
			}
		}
		within(t, 5*time.Second, func() {
			n := 0
			for range Pool(ctx, 1, src, func(i int) int { return i }) {
				if n++; n == 3 {
					cancel()
				}
			}
		})
		if pulled.Load() != 3 {
			t.Errorf("pulled %d elements, want exactly 3", pulled.Load())
		}
	})
	t.Run("NumLargerThanItems", func(t *testing.T) {
		within(t, 5*time.Second, func() {
			if got := Collect(Pool(t.Context(), 64, Range(1, 3), func(i int) int { return i })); len(got) != 3 {
				t.Errorf("got %v", got)
			}
			if got := Collect(Pool(t.Context(), 64, Slice([]int{}), func(i int) int { return i })); len(got) != 0 {
				t.Errorf("got %v", got)
			}
		})
	})
}
