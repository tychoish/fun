package irt

import (
	"maps"
	"slices"
	"sync"
	"testing"
)

// TestModifyReiterable is a regression test: the sequences returned by
// Modify, ModifyAll, Modify2, and ModifyAll2 must re-derive their
// transformation on every iteration, rather than mutating the captured
// input sequence in place (which would stack the transformation on
// repeated iteration and race under concurrent iteration).
func TestModifyReiterable(t *testing.T) {
	double := func(i int) int { return i * 2 }
	doublePair := func(a, b int) (int, int) { return a * 2, b * 2 }

	t.Run("Modify", func(t *testing.T) {
		seq := Modify(Slice([]int{1, 2, 3}), double)
		first := Collect(seq)
		second := Collect(seq)
		if !slices.Equal(first, second) {
			t.Fatalf("iterating twice should produce identical results: first %v != second %v", first, second)
		}
		if want := []int{2, 4, 6}; !slices.Equal(first, want) {
			t.Fatalf("got %v, want %v", first, want)
		}
	})

	t.Run("ModifyAll", func(t *testing.T) {
		seq := ModifyAll(Slice([]int{1, 2, 3}), double, double)
		first := Collect(seq)
		second := Collect(seq)
		if !slices.Equal(first, second) {
			t.Fatalf("iterating twice should produce identical results: first %v != second %v", first, second)
		}
		if want := []int{4, 8, 12}; !slices.Equal(first, want) {
			t.Fatalf("got %v, want %v", first, want)
		}
	})

	t.Run("Modify2", func(t *testing.T) {
		seq := Modify2(Map(map[int]int{1: 1}), doublePair)
		first := Collect2(seq)
		second := Collect2(seq)
		if !maps.Equal(first, second) {
			t.Fatalf("iterating twice should produce identical results: first %v != second %v", first, second)
		}
		if want := map[int]int{2: 2}; !maps.Equal(first, want) {
			t.Fatalf("got %v, want %v", first, want)
		}
	})

	t.Run("ModifyAll2", func(t *testing.T) {
		seq := ModifyAll2(Map(map[int]int{1: 1}), doublePair, doublePair)
		first := Collect2(seq)
		second := Collect2(seq)
		if !maps.Equal(first, second) {
			t.Fatalf("iterating twice should produce identical results: first %v != second %v", first, second)
		}
		if want := map[int]int{4: 4}; !maps.Equal(first, want) {
			t.Fatalf("got %v, want %v", first, want)
		}
	})
}

// TestModifyNilPassthrough asserts that a nil operation (or empty/all-nil
// ops list) leaves the sequence unchanged.
func TestModifyNilPassthrough(t *testing.T) {
	t.Run("Modify", func(t *testing.T) {
		var op func(int) int
		seq := Modify(Slice([]int{1, 2, 3}), op)
		got := Collect(seq)
		if want := []int{1, 2, 3}; !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("ModifyAll/empty", func(t *testing.T) {
		seq := ModifyAll[int, func(int) int](Slice([]int{1, 2, 3}))
		got := Collect(seq)
		if want := []int{1, 2, 3}; !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("ModifyAll/all-nil", func(t *testing.T) {
		var op func(int) int
		seq := ModifyAll(Slice([]int{1, 2, 3}), op, op)
		got := Collect(seq)
		if want := []int{1, 2, 3}; !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("Modify2", func(t *testing.T) {
		var op func(int, int) (int, int)
		seq := Modify2(Map(map[int]int{1: 9}), op)
		got := Collect2(seq)
		if want := map[int]int{1: 9}; !maps.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("ModifyAll2/empty", func(t *testing.T) {
		seq := ModifyAll2(Map(map[int]int{1: 9}))
		got := Collect2(seq)
		if want := map[int]int{1: 9}; !maps.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("ModifyAll2/all-nil", func(t *testing.T) {
		var op func(int, int) (int, int)
		seq := ModifyAll2(Map(map[int]int{1: 9}), op, op)
		got := Collect2(seq)
		if want := map[int]int{1: 9}; !maps.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

// TestModifyConcurrentIteration ranges the SAME returned sequence from
// two goroutines concurrently. Run with -race: before the fix, the
// returned closures reassigned their captured seq parameter, so
// concurrent iteration raced on that variable.
func TestModifyConcurrentIteration(t *testing.T) {
	double := func(i int) int { return i * 2 }
	doublePair := func(a, b int) (int, int) { return a * 2, b * 2 }

	// run builds a single shared sequence via makeIterate, then ranges
	// that SAME sequence value concurrently from two goroutines,
	// synchronized by a start barrier so both begin as close to
	// simultaneously as possible. Repeated several times to maximize
	// the chance the race detector observes the concurrent
	// write/read of the captured seq variable inside the returned
	// closure.
	run := func(t *testing.T, name string, makeIterate func() func()) {
		t.Run(name, func(t *testing.T) {
			for range 50 {
				iterate := makeIterate()
				start := make(chan struct{})
				var wg sync.WaitGroup
				for range 2 {
					wg.Go(func() {
						<-start
						iterate()
					})
				}
				close(start)
				wg.Wait()
			}
		})
	}

	source := func() []int {
		out := make([]int, 0, 100)
		for i := range 100 {
			out = append(out, i)
		}
		return out
	}()

	run(t, "Modify", func() func() {
		seq := Modify(Slice(source), double)
		return func() { _ = Collect(seq) }
	})

	run(t, "ModifyAll", func() func() {
		seq := ModifyAll(Slice(source), double, double)
		return func() { _ = Collect(seq) }
	})

	pairSource := func() map[int]int {
		out := make(map[int]int, 100)
		for i := range 100 {
			out[i] = i
		}
		return out
	}()

	run(t, "Modify2", func() func() {
		seq := Modify2(Map(pairSource), doublePair)
		return func() { _ = Collect2(seq) }
	})

	run(t, "ModifyAll2", func() func() {
		seq := ModifyAll2(Map(pairSource), doublePair, doublePair)
		return func() { _ = Collect2(seq) }
	})
}
