package seq

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
)

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
		// pass
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

type userOrderable struct {
	val int
}

func (u userOrderable) LessThan(in userOrderable) bool { return u.val < in.val }

func TestSort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Comparators", func(t *testing.T) {
		t.Run("Native", func(t *testing.T) {
			for idx, b := range []bool{
				LessThanNative(1, 2),
				LessThanNative(0, 40),
				LessThanNative(-1, 0),
				LessThanNative(1.5, 1.9),
				LessThanNative(440, 9001),
				LessThanNative("abc", "abcd"),
			} {
				if !b {
					t.Error(idx, "expected true")
				}
			}

			for idx, b := range []bool{
				LessThanNative(0, -2),
				LessThanNative(2.1, 1.9),
				LessThanNative(999440, 9001),
				LessThanNative("zzzz", "aaa"),
			} {
				if b {
					t.Error(idx, "expected false")
				}
			}
		})
		t.Run("Reversed", func(t *testing.T) {
			for idx, b := range []bool{
				Reverse(LessThanNative[int])(1, 2),
				Reverse(LessThanNative[int])(0, 40),
				Reverse(LessThanNative[int])(-1, 0),
				Reverse(LessThanNative[float64])(1.5, 1.9),
				Reverse(LessThanNative[uint])(440, 9001),
				Reverse(LessThanNative[string])("abc", "abcd"),
			} {
				if b {
					t.Error(idx, "expected not true (false)")
				}
			}

			for idx, b := range []bool{
				Reverse(LessThanNative[int8])(0, -2),
				Reverse(LessThanNative[float32])(2.1, 1.9),
				Reverse(LessThanNative[uint64])(999440, 9001),
				Reverse(LessThanNative[string])("zzzz", "aaa"),
			} {
				if !b {
					t.Error(idx, "expected not false (true)")
				}
			}

		})
		t.Run("Time", func(t *testing.T) {
			if LessThanTime(time.Now(), time.Now().Add(-time.Hour)) {
				t.Error("the past should not be before the future")
			}
			if LessThanTime(time.Now().Add(365*24*time.Hour), time.Now()) {
				t.Error("the future should be after the present")
			}
		})
		t.Run("Custom", func(t *testing.T) {
			if !LessThanCustom(userOrderable{1}, userOrderable{199}) {
				t.Error("custom error")
			}
			if LessThanCustom(userOrderable{1000}, userOrderable{199}) {
				t.Error("custom error")
			}
		})
	})
	t.Run("Sort", func(t *testing.T) {
		t.Run("IsSorted", func(t *testing.T) {
			t.Run("RejectsRandomList", func(t *testing.T) {
				if IsSorted(GetPopulatedList(t, 1000), LessThanNative[int]) {
					t.Fatal("random list should not be sorted")
				}
			})
			t.Run("Empty", func(t *testing.T) {
				list := &List[int]{}
				if !IsSorted(list, LessThanNative[int]) {
					t.Fatal("empty lists are sorted")
				}
			})
			t.Run("BuildSortedList", func(t *testing.T) {
				list := &List[int]{}
				list.PushBack(0)
				if !IsSorted(list, LessThanNative[int]) {
					t.Fatal("lists with one item are not sorted")
				}
				list.PushBack(1)
				for i := 2; i < 100; i += 2 {
					list.PushBack(i)
				}

				if !IsSorted(list, LessThanNative[int]) {
					t.Error("list should be sorted")
				}

				if !stdCheckSortedIntsFromList(ctx, t, list) {
					t.Error("confirm stdlib")
				}
			})
			t.Run("Uninitialized", func(t *testing.T) {
				var list *List[int]
				if !IsSorted(list, LessThanNative[int]) {
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

				if IsSorted(list, LessThanNative[int]) {
					t.Error("list isn't sorted", getSliceForList(ctx, t, list))
				}
			})
		})
		t.Run("BasicMergeSort", func(t *testing.T) {
			list := GetPopulatedList(t, 16)
			if IsSorted(list, LessThanNative[int]) {
				t.Fatal("should not be sorted")
			}
			SortListMerge(list, LessThanNative[int])
			if !stdCheckSortedIntsFromList(ctx, t, list) {
				t.Log(itertool.CollectSlice(ctx, ListValues(list.Iterator())))
				t.Fatal("sort should be verified, externally")
			}
			if !IsSorted(list, LessThanNative[int]) {
				t.Log(itertool.CollectSlice(ctx, ListValues(list.Iterator())))
				t.Fatal("should be sorted")
			}
		})
		t.Run("ComparisonValidation", func(t *testing.T) {
			list := GetPopulatedList(t, 10)
			lcopy := list.Copy()
			SortListMerge(list, LessThanNative[int])
			SortListQuick(lcopy, LessThanNative[int])
			listVals := fun.Must(itertool.CollectSlice(ctx, ListValues(list.Iterator())))
			copyVals := fun.Must(itertool.CollectSlice(ctx, ListValues(lcopy.Iterator())))
			t.Log("merge", listVals)
			t.Log("quick", copyVals)
			for i := 0; i < 10; i++ {
				if listVals[i] != copyVals[i] {
					t.Error("sort missmatch", i, listVals[i], copyVals[i])
				}
			}
		})
	})
	t.Run("Heap", func(t *testing.T) {
		t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
			ok, err := fun.Safe(func() bool {
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
			if !errors.Is(err, ErrUninitialized) {
				t.Error(err)
			}
			if expected := fmt.Sprint("panic: ", ErrUninitialized.Error()); expected != err.Error() {
				t.Fatal(expected, "->", err)
			}
		})
		t.Run("Iterator", func(t *testing.T) {
			heap := &Heap[int]{LT: LessThanNative[int]}
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
			var last = math.MinInt
			iter := heap.Iterator()
			seen := 0
			for iter.Next(ctx) {
				seen++
				current := iter.Value()
				if current == 0 {
					t.Fatal("should not see zero ever", seen)
				}
				if last > current {
					t.Error(seen, last, ">", current)
				}
			}
			fun.Invariant(iter.Close(ctx) == nil)
			if seen != 100 {
				t.Log("saw incorrect number of items", seen)
			}
			if heap.Len() != 0 {
				t.Log("list not exhausted", heap.Len())
			}
		})
		t.Run("Pop", func(t *testing.T) {
			heap := &Heap[int]{LT: LessThanNative[int]}

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
func getSliceForList(ctx context.Context, t *testing.T, list *List[int]) []int {
	t.Helper()
	return fun.Must(itertool.CollectSlice(ctx, ListValues(list.Iterator())))
}

func stdCheckSortedIntsFromList(ctx context.Context, t *testing.T, list *List[int]) bool {
	t.Helper()

	return sort.IntsAreSorted(getSliceForList(ctx, t, list))
}

func randomIntSlice(size int) []int {
	out := make([]int, size)
	for idx := range out {
		out[idx] = rand.Intn(size * 2)
	}
	return out
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
				SortListQuick(list, LessThanNative[int])
			}
		})
		b.Run("Merge", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				list := GetPopulatedList(b, size)
				b.StartTimer()
				list = mergeSort(list, LessThanNative[int])
				b.StopTimer()
				if list.Len() != size {
					b.Fatal("incorrect size")
				}
			}
		})
	})
}
