package dt_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

type jsonMarshlerError struct{}

func (jsonMarshlerError) MarshalJSON() ([]byte, error) { return nil, errors.New("always") }

func TestList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer runtime.GC()
	t.Run("Constructor", func(t *testing.T) {
		list := &dt.List[int]{}
		if list.Len() != 0 {
			t.Fatal("should initialize to zero")
		}
		list.PushBack(42)
		if list.Len() != 1 {
			t.Fatal("should initialize to zero", list.Len())
		}
		if v := list.PopFront().Value(); v != 42 {
			t.Fatal(v)
		}
	})
	t.Run("NewFromIterator", func(t *testing.T) {
		iter := dt.Sliceify([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}).Iterator()
		list, err := dt.NewListFromIterator(ctx, iter)
		assert.NotError(t, err)
		assert.Equal(t, list.Len(), 10)
		assert.Equal(t, list.Back().Value(), 0)
		assert.Equal(t, list.Front().Value(), 1)
	})
	t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
		ok, err := ers.Safe(func() bool {
			var list *dt.List[string]
			list.PushBack("hi")
			return true
		})
		if ok {
			t.Error("should have errored")
		}
		if err == nil {
			t.Fatal("should have gotten failure")
		}
		if !errors.Is(err, dt.ErrUninitializedContainer) {
			t.Error(err)
		}
		assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
		assert.ErrorIs(t, err, dt.ErrUninitializedContainer)
	})
	t.Run("LengthTracks", func(t *testing.T) {
		list := &dt.List[int]{}

		list.PushBack(1)
		if list.Len() != 1 {
			t.Fatal("append didn't track", list.Len())
		}

		one := list.PopBack()
		if list.Len() != 0 {
			t.Fatal("pop didn't track", list.Len())
		}

		if one.In(list) {
			t.Fatal("remove didn't work")
		}

		for i := 1; i <= 100; i++ {
			if i%2 == 0 {
				list.PushBack(i)
			} else {
				list.PushFront(i)
			}

			if l := list.Len(); i != l {
				t.Error("unexpected length during adding", i, l)
			}
		}
	})

	t.Run("FrontAndBack", func(t *testing.T) {
		list := &dt.List[int]{}

		if list.Front().Ok() {
			t.Error(list.Front())
		}

		list.PushBack(1)
		list.PushBack(2)
		// list is [1, 2]

		if list.Front().Value() != 1 {
			t.Fatal(list.Front().Next())
		}
		if list.Back().Value() != 2 {
			t.Fatal(list.Back().Value())
		}
	})
	t.Run("WrapAroundEffects", func(t *testing.T) {
		list := &dt.List[int]{}
		for i := 0; i < 21; i++ {
			if i%2 == 0 {
				list.PushBack(i)
			} else {
				list.PushFront(i)
			}
		}
		expected := []int{19, 17, 15, 13, 11, 9, 7, 5, 3, 1, 0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}

		seen := 0
		for item := list.Front(); item.Ok(); item = item.Next() {
			if expected[seen] != item.Value() {
				t.Error(seen, expected[seen], item.Value())
			}
			seen++
		}
		if seen != list.Len() {
			t.Error(list.Len(), seen)
		}
		if seen != len(expected) {
			t.Log(seen, list.Len(), ft.Must(list.Iterator().Slice(ctx)))
			t.Error(seen, len(expected), expected)

		}
	})
	t.Run("CStyleIteration", func(t *testing.T) {
		list := &dt.List[int]{}
		for i := 1; i <= 100; i++ {
			list.PushBack(i)
		}
		if list.Len() != 100 {
			t.Fatal(list.Len())
		}
		t.Run("Forwards", func(t *testing.T) {
			seen := 0
			last := -1 * (math.MaxInt - 1)
			// front to back
			for item := list.Front(); item.Ok(); item = item.Next() {
				if item.Value() < 0 && item.Value() > 100 {
					t.Fatal(item.Value())
				}
				if last >= item.Value() {
					t.Fatal(last, ">=", item.Value())
				}

				last = item.Value()
				seen++
			}
			if seen != list.Len() {
				t.Error(seen, "!=", list.Len())
			}

		})
		t.Run("Backwards", func(t *testing.T) {
			seen := 0
			last := math.MaxInt
			// front to back
			for item := list.Back(); item.Ok(); item = item.Previous() {
				if item.Value() < 0 && item.Value() > 100 {
					t.Fatal(item.Value())
				}
				if last < item.Value() {
					t.Fatal(last, "<", item.Value())
				}

				last = item.Value()
				seen++
			}
			if seen != list.Len() {
				t.Error(seen, "!=", list.Len())
			}
		})
	})
	t.Run("CStyleIterationDestructive", func(t *testing.T) {
		list := &dt.List[int]{}
		for i := 1; i <= 100; i++ {
			list.PushBack(i)
		}
		if list.Len() != 100 {
			t.Fatal(list.Len())
		}
		t.Run("Forwards", func(t *testing.T) {
			seen := 0
			last := -1 * (math.MaxInt - 1)
			// front to back
			for item := list.PopFront(); item.Ok(); item = list.PopFront() {
				if item.Value() < 0 && item.Value() > 100 {
					t.Fatal(item.Value())
				}
				if last >= item.Value() {
					t.Fatal(last, "<", item.Value())
				}

				last = item.Value()
				seen++
			}
			if seen != 100 {
				t.Error(seen, "!=", list.Len())
			}
			if list.Len() != 0 {
				t.Fatal(list.Len())
			}
		})

		list = &dt.List[int]{}
		for i := 1; i <= 100; i++ {
			list.PushBack(i)
		}
		if list.Len() != 100 {
			t.Fatal(list.Len())
		}
		t.Run("Backwards", func(t *testing.T) {
			seen := 0
			last := (math.MaxInt)
			// front to back
			for item := list.PopBack(); item.Ok(); item = list.PopBack() {
				if item.Value() < 0 && item.Value() > 100 {
					t.Fatal(item.Value())
				}
				if last < item.Value() {
					t.Fatal(last, "<", item.Value())
				}

				last = item.Value()
				seen++
			}
			if seen != 100 {
				t.Error(seen, "!=", list.Len())
			}
			if list.Len() != 0 {
				t.Fatal(list.Len())
			}
		})
	})
	t.Run("Iterators", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			list := &dt.List[int]{}
			ct := 0
			assert.NotPanic(t, func() {
				iter := list.Iterator()
				for iter.Next(ctx) {
					ct++
				}
				check.NotError(t, iter.Close())
			})
			assert.Zero(t, ct)
		})
		t.Run("Forward", func(t *testing.T) {
			list := &dt.List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			iter := list.Iterator()
			seen := 0
			last := -1*math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for iter.Next(ctx) {
				if iter.Value() < 0 && iter.Value() > 100 {
					t.Fatal(iter.Value())
				}
				if last > iter.Value() {
					t.Fatal(last, ">", iter.Value())
				}

				last = iter.Value()
				seen++
			}
			if err := iter.Close(); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("ForwardPop", func(t *testing.T) {
			list := &dt.List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			iter := list.PopIterator()
			seen := 0
			last := -1*math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for iter.Next(ctx) {
				if iter.Value() < 0 && iter.Value() > 100 {
					t.Fatal(iter.Value())
				}
				if last > iter.Value() {
					t.Fatal(last, ">", iter.Value())
				}

				last = iter.Value()
				seen++
			}
			if err := iter.Close(); err != nil {
				t.Fatal(err)
			}
			if seen != 100 {
				t.Error("didn't observe enough items")
			}
			if list.Len() != 0 {
				t.Error("did not consume enough items")
			}
		})
		t.Run("Reverse", func(t *testing.T) {
			list := &dt.List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			iter := list.Reverse()
			seen := 0
			last := math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for iter.Next(ctx) {
				if iter.Value() < 0 && iter.Value() > 100 {
					t.Fatal(iter.Value())
				}
				if last < iter.Value() {
					t.Fatal(last, ">", iter.Value())
				}

				last = iter.Value()
				seen++
			}
			if err := iter.Close(); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("PopReverse", func(t *testing.T) {
			list := &dt.List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			iter := list.PopReverse()
			seen := 0
			last := math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for iter.Next(ctx) {
				if iter.Value() < 0 && iter.Value() > 100 {
					t.Fatal(iter.Value())
				}
				if last < iter.Value() {
					t.Fatal(last, ">", iter.Value())
				}

				last = iter.Value()
				seen++
			}
			if err := iter.Close(); err != nil {
				t.Fatal(err)
			}
			if seen != 100 {
				t.Error("didn't observe enough items")
			}
			if list.Len() != 0 {
				t.Error("did not consume enough items")
			}
		})
		t.Run("Closed", func(t *testing.T) {
			list := &dt.List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			iter := list.Iterator()
			for i := 0; i < 50; i++ {
				if !iter.Next(ctx) {
					t.Error("iterator should be open", i)
				}
			}
			fun.Invariant.IsTrue(iter.Close() == nil)
			for i := 0; i < 50; i++ {
				if iter.Next(ctx) {
					t.Error("iterator should be closed", i)
				}
			}

		})

	})
	t.Run("Element", func(t *testing.T) {
		t.Run("String", func(t *testing.T) {
			elem := dt.NewElement("hi")
			if fmt.Sprint(elem) != "hi" {
				t.Fatal(fmt.Sprint(elem))
			}
			if fmt.Sprint(elem) != fmt.Sprintf("%+v", elem) {
				t.Fatal(fmt.Sprint(elem), fmt.Sprintf("%+v", elem))
			}
		})
		t.Run("ChainAppending", func(t *testing.T) {
			list := &dt.List[int]{}
			head := list.Front()
			for i := 1; i <= 100; i++ {
				head.Append(dt.NewElement(i))
			}
			if list.Len() != 100 {
				t.Fatal(list.Len())
			}
			for i := 1; i <= 100; i++ {
				head.Append(&dt.Element[int]{})
			}
			if list.Len() != 100 {
				t.Fatal(list.Len())
			}
			for i := 1; i <= 100; i++ {
				head.Append(nil)
			}
			if list.Len() != 100 {
				t.Fatal(list.Len())
			}
		})
		t.Run("Remove", func(t *testing.T) {
			list := &dt.List[int]{}
			head := list.Front()
			for i := 1; i <= 100; i++ {
				head.Append(dt.NewElement(i))
			}

			if list.Len() != 100 {
				t.Fatal(list.Len())
			}
			back := list.Back()

			if back.Remove() && list.Len() != 99 {
				t.Fatal(list.Len())
			}

			if back.In(list) {
				t.Error("should not be in list")
			}

			if !back.Ok() {
				t.Error("value should exist")
			}
			if back.Value() != 1 {
				t.Error(back.Value())
			}
		})
		t.Run("RemoveRoot", func(t *testing.T) {
			list := &dt.List[int]{}
			// this is the sentinel
			head := list.Front()
			if head.Ok() {
				t.Error("should not be a value")
			}
			if head.Remove() {
				t.Error("should not be able to remove the root")
			}
			if !head.In(list) {
				t.Error("root should be in the list still")
			}
			if head.Set(100) {
				t.Error("should not report success at setting sentinel")
			}
			if head.Ok() {
				t.Error("should not set root to a value")
			}
			if head.Value() != 0 {
				t.Error("unexpected value")
			}
			if head != list.Back() {
				t.Error("sentinel has incorrect links")
			}
		})
		t.Run("SetOrphan", func(t *testing.T) {
			elem := dt.NewElement("hello world!")
			if !elem.Ok() || elem.Value() != "hello world!" {
				t.Fatal(elem.Value())
			}
			elem.Set("hi globe!")
			if !elem.Ok() || elem.Value() != "hi globe!" {
				t.Fatal(elem.Value())
			}
		})
		t.Run("SetListMember", func(t *testing.T) {
			list := &dt.List[int]{}
			list.PushFront(4242)
			elem := list.Front()
			if !elem.Ok() || elem.Value() != 4242 {
				t.Fatal(elem.Value())
			}
			elem.Set(100)
			if !elem.Ok() || elem.Value() != 100 {
				t.Fatal(elem.Value())
			}
			list.Front().Set(100)

			if list.Front().Value() != 100 {
				t.Fatal(list.Front())
			}
			if list.Front() != elem {
				t.Fatal("identity shouldn't change if values doe")
			}
		})
		t.Run("Drop", func(t *testing.T) {
			list := &dt.List[int]{}
			list.PushFront(4242)
			list.PushFront(4242)
			if list.Len() != 2 {
				t.Fatal(list.Len())
			}

			list.Front().Drop()
			if list.Len() != 1 {
				t.Fatal(list.Len())
			}

			list.Front().Drop()
			if list.Len() != 0 {
				t.Fatal(list.Len())
			}

			for i := 0; i < 100; i++ {
				list.Front().Drop()
			}

			if list.Len() != 0 {
				t.Fatal(list.Len())
			}
		})
	})
	t.Run("Swap", func(t *testing.T) {
		t.Run("FlipAdjacent", func(t *testing.T) {
			list := &dt.List[string]{}
			list.PushBack("hello")
			list.PushBack("world")
			// ["hello", "world"]
			if list.Front().Value() != "hello" && list.Front().Next().Value() != "world" {
				t.Fatal(list.Iterator().Slice(ctx))
			}
			if !list.Front().Swap(list.Back()) {
				t.Fatal(list.Iterator().Slice(ctx))
			}

			if list.Front().Value() != "world" && list.Front().Next().Value() != "hello" {
				t.Fatal(list.Iterator().Slice(ctx))
			}
		})
		t.Run("Self", func(t *testing.T) {
			list := &dt.List[string]{}
			list.PushBack("hello")
			list.PushBack("world")

			if list.Front().Swap(list.Front()) {
				t.Fatal(list.Iterator().Slice(ctx))
			}
		})
		t.Run("NilList", func(t *testing.T) {
			list := &dt.List[string]{}
			list.PushFront("hello")

			if list.Front().Swap(dt.NewElement("world")) {
				t.Fatal(list.Iterator().Slice(ctx))
			}
		})
		t.Run("Root", func(t *testing.T) {
			list := &dt.List[int]{}
			list.PushBack(42)
			list.PushBack(84)

			if !list.Front().Previous().Swap(list.Back()) {
				t.Fatal("sholdn't object to swapping root")
			}
			if list.Front().Value() != 84 {
				t.Fatal("unexpected outcome front")
			}
			if list.Back().Value() != 42 {
				t.Fatal("unexpected outcome back")
			}

		})
		t.Run("NonAdjacent", func(t *testing.T) {
			list := &dt.List[int]{}
			list.PushBack(42)
			list.PushBack(84)
			list.PushBack(420)
			list.PushBack(840)

			// [ 42, 84, 420, 840 ]

			if !list.Back().Swap(list.Front().Next()) {
				t.Fatal(list.Iterator().Slice(ctx))
				t.Fatal("should have swapped")
			}
			// expected: [42, 840, 420, 84]
			slice := ft.Must(list.Iterator().Slice(ctx))
			if list.Len() != 4 {
				t.Log(list.Len(), slice)
				t.Fatal(list.Len())
			}

			idx := 0
			iter := list.Iterator()
			for iter.Next(ctx) {
				if slice[idx] != iter.Value() {
					t.Error(idx, slice[idx], iter.Value())
				}
				idx++
			}
			if idx != 3 {
				t.Fatal("did not iterate correctly", idx)
			}
		})
		t.Run("DifferentLists", func(t *testing.T) {
			one := &dt.List[string]{}
			two := &dt.List[string]{}
			one.PushBack("hello")
			two.PushBack("world")
			if one.Front().Swap(two.Front()) {
				t.Fatal("unallowable swap")
			}
		})
		t.Run("Nil", func(t *testing.T) {
			one := &dt.List[string]{}
			one.PushBack("hello")
			one.PushBack("world")
			if one.Front().Swap(nil) {
				t.Fatal("unallowable swap")
			}
		})
	})
	t.Run("Merge", func(t *testing.T) {
		t.Run("Extend", func(t *testing.T) {
			l1 := dt.GetPopulatedList(t, 100)
			l2 := dt.GetPopulatedList(t, 100)
			l2tail := l2.Back()

			l1.Extend(l2)
			if l1.Len() != 200 {
				t.Error(l1.Len())
			}

			if l1.Back().Value() != l2tail.Value() {
				t.Error(l1.Front().Value(), l2tail.Value())
			}

			if l2.Len() != 0 {
				t.Error(l2.Len())
			}
		})
		t.Run("Order", func(t *testing.T) {
			lOne := dt.GetPopulatedList(t, 100)
			itemsOne := ft.Must(lOne.Iterator().Slice(ctx))

			lTwo := dt.GetPopulatedList(t, 100)
			itemsTwo := ft.Must(lTwo.Iterator().Slice(ctx))

			if len(itemsOne) != lOne.Len() {
				t.Fatal("incorrect items", len(itemsOne), lOne.Len())
			}
			if len(itemsTwo) != lTwo.Len() {
				t.Fatal("incorrect items")
			}

			lOne.Extend(lTwo)
			combined := append(itemsOne, itemsTwo...)
			iter := lOne.Iterator()
			idx := 0
			for iter.Next(ctx) {
				if combined[idx] != iter.Value() {
					t.Error("missmatch", idx, combined[idx], iter.Value())
				}

				idx++
			}
			if len(combined) != lOne.Len() {
				t.Fatal("incorrect items")
			}
			if lTwo.Len() != 0 {
				t.Fatal("incorrect items", lTwo.Len())
			}
		})
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("RoundTrip", func(t *testing.T) {
			list := &dt.List[int]{}
			list.PushBack(400)
			list.PushBack(300)
			list.PushBack(42)
			out, err := list.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != "[400,300,42]" {
				t.Error(string(out))
			}
			nl := &dt.List[int]{}

			if err := nl.UnmarshalJSON(out); err != nil {
				t.Error(err)
			}
			fun.Invariant.IsTrue(nl.Front().Value() == list.Front().Value())
			fun.Invariant.IsTrue(nl.Front().Next().Value() == list.Front().Next().Value())
			fun.Invariant.IsTrue(nl.Front().Next().Next().Value() == list.Front().Next().Next().Value())
		})
		t.Run("TypeMismatch", func(t *testing.T) {
			list := &dt.List[int]{}
			list.Append(400, 300, 42)

			out, err := list.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}

			nl := &dt.List[string]{}

			if err := nl.UnmarshalJSON(out); err == nil {
				t.Error("should have errored", nl.Front())
			}
		})
		t.Run("ListUnmarshalNil", func(t *testing.T) {
			list := &dt.List[int]{}

			if err := list.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("ElementUnmarshalNil", func(t *testing.T) {
			elem := dt.NewElement(0)

			if err := elem.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("NilPointerSafety", func(t *testing.T) {
			list := &dt.List[jsonMarshlerError]{}
			list.PushBack(jsonMarshlerError{})
			if out, err := list.MarshalJSON(); err == nil {
				t.Fatal("expected error", string(out))
			}
		})
	})
}

func BenchmarkList(b *testing.B) {
	var e *dt.Element[int]
	b.Run("SeedElemPool", func(b *testing.B) {
		for i := 0; i < 200; i++ {
			e = dt.NewElement(i)
		}
		b.StopTimer()
		if !e.Ok() {
			b.Fatal(e)
		}
		runtime.GC()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const size = 1000
	b.Run("Append", func(b *testing.B) {
		b.Run("Slice", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				b.StartTimer()
				slice := []int{}
				for i := 0; i < size; i++ {
					slice = append(slice, i)
				}
				b.StopTimer()
				if len(slice) != size {
					b.Fatal("incorrect size")
				}
			}
		})
		b.Run("SlicePrealloc", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				b.StartTimer()
				slice := make([]int, 0, 100)
				for i := 0; i < size; i++ {
					slice = append(slice, i)
				}
				b.StopTimer()
				if len(slice) != size {
					b.Error("incorrect size")
				}
			}
		})
		b.Run("List", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				b.StopTimer()
				list := &dt.List[int]{}
				list.PushBack(0)
				list.PopBack()
				b.StartTimer()
				for i := 0; i < size; i++ {
					list.PushBack(i)
				}
			}
		})
		b.Run("ListElement", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				b.StopTimer()
				list := &dt.List[int]{}
				list.PushBack(0)
				list.PopBack()
				b.StartTimer()
				for i := 0; i < size; i++ {
					list.Back().Append(dt.NewElement(i))
				}
			}
		})

	})
	b.Run("Prepend", func(b *testing.B) {
		b.Run("Slice", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				slice := []int{}
				for i := 0; i < size; i++ {
					slice = append([]int{i}, slice...)
				}
			}
		})
		b.Run("List", func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				b.StopTimer()
				list := &dt.List[int]{}
				list.PushBack(0)
				list.PopBack()
				b.StartTimer()

				for i := 0; i < size; i++ {
					list.PushFront(i)
				}
			}
		})
	})
	b.Run("Deletion", func(b *testing.B) {
		b.Run("Slice", func(b *testing.B) {
			slice := []int{}
			for i := 0; i < size; i++ {
				slice = append([]int{i}, slice...)
			}

			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				for i := 0; i < len(slice)-1; i++ {
					if i > 2 && i%2 != 0 {
						slice = append(slice[:i], slice[i+1:]...)
					}
				}
			}
		})
		b.Run("List", func(b *testing.B) {
			b.Run("Drop", func(b *testing.B) {
				list := &dt.List[int]{}
				for i := 0; i < size; i++ {
					list.PushFront(i)
				}

				b.ResetTimer()
				for j := 0; j < b.N; j++ {
					idx := 0
					for e := list.Front(); e.Ok(); e = e.Next() {
						if idx > 2 && idx%2 != 0 {
							e.Previous().Drop()
						}
						idx++
					}
				}
			})
			b.Run("Remove", func(b *testing.B) {
				list := &dt.List[int]{}
				for i := 0; i < size; i++ {
					list.PushFront(i)
				}

				b.ResetTimer()
				for j := 0; j < b.N; j++ {
					idx := 0
					for e := list.Front(); e.Ok(); e = e.Next() {
						if idx > 2 && idx%2 != 0 {
							e.Previous().Remove()
						}
						idx++
					}
				}
			})
			b.Run("Iterator", func(b *testing.B) {
				list := &dt.List[int]{}
				for i := 0; i < size; i++ {
					list.PushFront(i)
				}

				b.ResetTimer()
				iter := list.Iterator()
				var value int = -1
				for j := 0; j < b.N; j++ {
					idx := 0
					for iter.Next(ctx) {
						if idx > 2 && idx%2 != 0 {
							value = iter.Value()
						}
						idx++
					}
				}
				assert.NotEqual(b, value, -1)
			})
			b.Run("RoundTrip", func(b *testing.B) {
				b.Run("PooledElements", func(b *testing.B) {
					list := &dt.List[int]{}
					for i := 0; i < size; i++ {
						list.Front().Previous().Append(dt.NewElement(i))
					}

					for j := 0; j < b.N; j++ {
						idx := 0
						for e := list.Front(); e.Ok(); e = e.Next() {
							if idx > 2 && idx%2 != 0 {
								e.Previous().Remove()
							}
							idx++
						}
					}
				})
				b.Run("Values", func(b *testing.B) {
					list := &dt.List[int]{}
					for i := 0; i < size; i++ {
						list.PushFront(i)
					}

					for j := 0; j < b.N; j++ {
						idx := 0
						for e := list.Front(); e.Ok(); e = e.Next() {
							if idx > 2 && idx%2 != 0 {
								e.Previous().Remove()
							}
							idx++
						}

					}
				})
			})
		})
	})
}
