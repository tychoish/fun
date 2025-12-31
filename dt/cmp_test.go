package dt

import (
	"errors"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/dt/stw"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/fun/irt"
)

func randomIntSlice(size int) stw.Slice[int] {
	out := make([]int, size)
	for idx := range out {
		out[idx] = intish.Abs(rand.Intn(size) + 1)
	}
	return stw.NewSlice(out)
}

func GetPopulatedList(t testing.TB, size int) *List[int] {
	t.Helper()
	list := &List[int]{}
	PopulateList(t, size, list)
	return list
}

func ClearList(t testing.TB, list *List[int]) {
	t.Helper()
	if list.Len() == 0 {
		return
	}

	for list.PopFront().Ok() {
		continue
	}
	if list.Len() != 0 {
		t.Fatal("should have cleared list", list.Len())
	}
}

func PopulateList(t testing.TB, size int, list *List[int]) {
	t.Helper()
	for i := 0; i < size; i++ {
		list.PushBack(rand.Intn(size))
	}
	if list.Len() != size {
		t.Fatal(size, "vs", list.Len())
	}
}

func TestSort(t *testing.T) {
	t.Run("Sort", func(t *testing.T) {
		t.Run("IsSorted", func(t *testing.T) {
			t.Run("RejectsRandomList", func(t *testing.T) {
				list := GetPopulatedList(t, 1000)
				if list.IsSorted(cmp.LessThanNative[int]) {
					t.Fatal("random list should not be sorted")
				}
			})
			t.Run("Empty", func(t *testing.T) {
				list := &List[int]{}
				if !list.IsSorted(cmp.LessThanNative[int]) {
					t.Fatal("empty lists are sorted")
				}
			})
			t.Run("BuildSortedList", func(t *testing.T) {
				list := &List[int]{}
				list.PushBack(0)
				if !list.IsSorted(cmp.LessThanNative[int]) {
					t.Fatal("lists with one item are not sorted")
				}
				list.PushBack(1)
				for i := 2; i < 100; i += 2 {
					list.PushBack(i)
				}

				if !list.IsSorted(cmp.LessThanNative[int]) {
					t.Error("list should be sorted")
				}

				if !stdCheckSortedIntsFromList(t, list) {
					t.Error("confirm stdlib")
				}
			})
			t.Run("Uninitialized", func(t *testing.T) {
				var list *List[int]
				if !list.IsSorted(cmp.LessThanNative[int]) {
					t.Error("list is not yet valid")
				}
				var slice []int
				if !sort.IntsAreSorted(slice) {
					t.Error("std lib takes the same opinion")
				}
			})
			t.Run("PartiallySorted", func(t *testing.T) {
				list := &List[int]{}
				list.PushBack(0)
				list.PushBack(1)
				list.PushBack(1)
				list.PushBack(2)
				list.PushBack(rand.Int())
				list.PushBack(3)
				list.PushBack(rand.Int())
				list.PushBack(5)
				list.PushBack(rand.Int())
				list.PushBack(9)

				if list.IsSorted(cmp.LessThanNative[int]) {
					t.Error("list isn't sorted", list.Slice())
				}
			})
		})
		t.Run("BasicMergeSort", func(t *testing.T) {
			list := GetPopulatedList(t, 16)
			if list.IsSorted(cmp.LessThanNative[int]) {
				t.Fatal("should not be sorted")
			}
			list.SortMerge(cmp.LessThanNative[int])
			if !stdCheckSortedIntsFromList(t, list) {
				t.Log(irt.Collect(list.IteratorFront()))
				t.Fatal("sort should be verified, externally")
			}
			if !list.IsSorted(cmp.LessThanNative[int]) {
				t.Log(irt.Collect(list.IteratorFront()))
				t.Fatal("should be sorted")
			}
		})
		t.Run("ComparisonValidation", func(t *testing.T) {
			list := GetPopulatedList(t, 10)
			lcopy := list.Copy()
			list.SortMerge(cmp.LessThanNative[int])
			lcopy.SortQuick(cmp.LessThanNative[int])
			listVals := irt.Collect(list.IteratorFront())
			copyVals := irt.Collect(lcopy.IteratorFront())
			check.Equal(t, len(listVals), len(copyVals))
			check.Equal(t, len(listVals), len(copyVals))
			check.Equal(t, list.Len(), lcopy.Len())
			check.Equal(t, list.Len(), len(copyVals))
			check.Equal(t, list.Len(), 10)
			for i := 0; i < 10; i++ {
				if listVals[i] != copyVals[i] {
					t.Error("sort missmatch", i, listVals[i], copyVals[i])
				}
			}
		})
	})
	t.Run("Heap", func(t *testing.T) {
		t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
			ok, err := ft.WithRecoverDo(func() bool {
				var list *Heap[string]
				list.Push("hi")
				return true
			})
			if ok {
				t.Error("should have errored")
			}
			if err == nil {
				t.Fatal("should have gotten failure")
			}
			if !errors.Is(err, ErrUninitializedContainer) {
				t.Error(err)
			}

			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.ErrorIs(t, err, ErrUninitializedContainer)
		})
		t.Run("Stream", func(t *testing.T) {
			heap := &Heap[int]{LT: cmp.LessThanNative[int]}
			if heap.Len() != 0 {
				t.Fatal("heap should be empty to start")
			}
			for i := 0; i < 100; i++ {
				val := i + rand.Intn(200)
				if val == 0 {
					val++
				}
				heap.Push(val)
			}
			if heap.Len() != 100 {
				t.Fatal("heap should have expected number of items", heap.Len())
			}
			last := math.MinInt
			seen := 0
			for value := range heap.Iterator() {
				seen++
				current := value
				if current == 0 {
					t.Fatal("should not see zero ever", seen)
				}
				if last > current {
					t.Error(seen, last, ">", current)
				}
			}
			if seen != 100 {
				t.Log("saw incorrect number of items", seen)
			}
			if heap.Len() != 0 {
				t.Log("list not exhausted", heap.Len())
			}
		})
		t.Run("Pop", func(t *testing.T) {
			heap := &Heap[int]{LT: cmp.LessThanNative[int]}

			slice := randomIntSlice(100)
			if sort.IntsAreSorted(slice) {
				t.Fatal("should not be sorted")
			}
			for _, i := range slice {
				heap.Push(i)
			}
			sort.Ints(slice)
			if !sort.IntsAreSorted(slice) {
				t.Fatal("should now be sorted")
			}

			for idx, expected := range slice {
				val, ok := heap.Pop()
				if !ok {
					t.Error(idx, "ran out of heap items")
					break
				}
				if val != expected {
					t.Error("val=", val, "expected=", expected)
				}
			}
			if heap.Len() != 0 {
				t.Error("extra heap items")
			}
		})
	})
}

func stdCheckSortedIntsFromList(t *testing.T, list *List[int]) bool {
	t.Helper()

	return sort.IntsAreSorted(irt.Collect(list.IteratorFront()))
}

func BenchmarkSorts(b *testing.B) {
	const size = 100

	var e *Element[int]
	b.Run("SeedElemPool", func(b *testing.B) {
		for i := 0; i < 10*size; i++ {
			e = NewElement(i)
		}
		b.StopTimer()
		if !e.Ok() {
			b.Fatal(e)
		}
	})
	b.Run("Slice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			slice := make([]int, 0, size)
			for i := 0; i < size; i++ {
				slice = append(slice, rand.Intn(size))
			}
			b.StartTimer()
			sort.Ints(slice)
		}
	})
	b.Run("Lists", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				list := GetPopulatedList(b, size)
				b.StartTimer()
				list.SortQuick(cmp.LessThanNative[int])
			}
		})
		b.Run("Merge", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				list := GetPopulatedList(b, size)
				b.StartTimer()
				list = mergeSort(list, cmp.LessThanNative[int])
				b.StopTimer()
				if list.Len() != size {
					b.Fatal("incorrect size")
				}
			}
		})
	})
}
