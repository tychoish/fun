package dt

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
)

func TestBinHeap(t *testing.T) {
	t.Run("PanicUninitialized", func(t *testing.T) {
		check.Panic(t, func() {
			heap := &BinHeap[string]{}
			heap.Push("hi")
		})
	})
	t.Run("Empty", func(t *testing.T) {
		heap := &BinHeap[int]{CF: cmp.Compare[int]}
		if _, ok := heap.Pop(); ok {
			t.Error("empty pop")
		}
		if _, ok := heap.Peek(); ok {
			t.Error("empty peek")
		}
		if heap.Len() != 0 {
			t.Error("empty len")
		}
	})
	t.Run("NeverPushed", func(t *testing.T) {
		heap := &BinHeap[int]{CF: cmp.Compare[int]}
		if heap.data != nil {
			t.Fatal("data should be nil before first push")
		}
		if v, ok := heap.Pop(); ok || v != 0 {
			t.Error("pop on never-pushed heap", v, ok)
		}
		if v, ok := heap.Peek(); ok || v != 0 {
			t.Error("peek on never-pushed heap", v, ok)
		}
		if heap.Len() != 0 {
			t.Error("len on never-pushed heap", heap.Len())
		}
		// reporting empty should not require CF
		zero := &BinHeap[int]{}
		if _, ok := zero.Pop(); ok {
			t.Error("pop on zero-value heap")
		}
		if _, ok := zero.Peek(); ok {
			t.Error("peek on zero-value heap")
		}
	})
	t.Run("Single", func(t *testing.T) {
		heap := &BinHeap[int]{CF: cmp.Compare[int]}
		heap.Push(4)
		if v, ok := heap.Peek(); !ok || v != 4 {
			t.Error(v, ok)
		}
		if v, ok := heap.Pop(); !ok || v != 4 || heap.Len() != 0 {
			t.Error(v, ok, heap.Len())
		}
	})
	t.Run("PopAndPeekOrder", func(t *testing.T) {
		heap := &BinHeap[int]{CF: cmp.Compare[int]}
		slice := randomIntSlice(200) // contains duplicates
		for _, i := range slice {
			heap.Push(i)
		}
		expected := slices.Clone(slice)
		slices.Sort(expected)

		for idx, want := range expected {
			peeked, ok := heap.Peek()
			if !ok || peeked != want {
				t.Fatal(idx, "peek", peeked, ok, "expected", want)
			}
			if heap.Len() != len(expected)-idx {
				t.Fatal("peek must not change len", heap.Len())
			}
			val, ok := heap.Pop()
			if !ok || val != want {
				t.Fatal(idx, "pop", val, ok, "expected", want)
			}
		}
		if heap.Len() != 0 {
			t.Error("extra heap items")
		}
	})
	t.Run("Iterator", func(t *testing.T) {
		heap := &BinHeap[int]{CF: cmp.Compare[int]}
		for _, i := range randomIntSlice(100) {
			heap.Push(i)
		}
		items := irt.Collect(heap.Iterator())
		if len(items) != 100 || heap.Len() != 100 {
			t.Fatal("iteration must be non-destructive", len(items), heap.Len())
		}
		if lowest, _ := heap.Peek(); items[0] != lowest {
			t.Error("minimum should be first", items[0], lowest)
		}
	})
	t.Run("JSON", func(t *testing.T) {
		heap := &BinHeap[int]{CF: cmp.Compare[int]}
		for _, i := range randomIntSlice(50) {
			heap.Push(i)
		}
		out, err := json.Marshal(heap)
		if err != nil {
			t.Fatal(err)
		}
		rt := &BinHeap[int]{CF: cmp.Compare[int]}
		if err := json.Unmarshal(out, rt); err != nil {
			t.Fatal(err)
		}
		if rt.Len() != heap.Len() {
			t.Fatal("length mismatch", rt.Len(), heap.Len())
		}
		for range heap.Len() {
			a, _ := heap.Pop()
			b, _ := rt.Pop()
			if a != b {
				t.Fatal(a, b)
			}
		}
		check.Error(t, json.Unmarshal([]byte(`[1, "x"]`), &BinHeap[int]{CF: cmp.Compare[int]}))
	})
}

func BenchmarkHeapPush(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 10000} {
		values := randomIntSlice(n)
		b.Run("Heap/"+strconv.Itoa(n), func(b *testing.B) {
			for b.Loop() {
				h := &Heap[int]{CF: cmp.Compare[int]}
				for _, v := range values {
					h.Push(v)
				}
			}
		})
		b.Run("BinHeap/"+strconv.Itoa(n), func(b *testing.B) {
			for b.Loop() {
				h := &BinHeap[int]{CF: cmp.Compare[int]}
				for _, v := range values {
					h.Push(v)
				}
			}
		})
	}
}
