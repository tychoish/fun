package dt

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
)

type jsonMarshlerError struct{}

func (jsonMarshlerError) MarshalJSON() ([]byte, error) { return nil, errors.New("always") }

func TestList(t *testing.T) {
	defer runtime.GC()
	t.Run("Constructor", func(t *testing.T) {
		list := &List[int]{}
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
	t.Run("NilSafeLen", func(t *testing.T) {
		var l *List[int]
		assert.NotPanic(t, func() { l.Len() })
		assert.Zero(t, l.Len())
	})
	t.Run("IteratorList", func(t *testing.T) {
		list := IteratorList(irt.Slice([]int{1, 2, 3, 4, 5}))
		assert.Equal(t, list.Len(), 5)
		assert.Equal(t, list.Front().Value(), 1)
		assert.Equal(t, list.Back().Value(), 5)
	})
	t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
		var err error
		defer func() {
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrUninitializedContainer)
		}()
		defer func() { err = recover().(error) }()

		var list *List[string]
		list.PushBack("hi")
	})
	t.Run("LengthTracks", func(t *testing.T) {
		list := &List[int]{}

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
		list := &List[int]{}

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
		list := &List[int]{}
		for i := 0; i < 21; i++ {
			if i%2 == 0 {
				list.PushBack(i)
			} else {
				list.PushFront(i)
			}
		}
		expected := []int{19, 17, 15, 13, 11, 9, 7, 5, 3, 1, 0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20}

		seen := 0
		for item := range list.IteratorFront() {
			if expected[seen] != item {
				t.Error(seen, expected[seen], item)
			}
			seen++
		}
		if seen != list.Len() {
			t.Error(list.Len(), seen)
		}
		if seen != len(expected) {
			t.Log(seen, list.Len(), irt.Collect(list.IteratorFront()))
			t.Error(seen, len(expected), expected)
		}
	})
	t.Run("CStyleIteration", func(t *testing.T) {
		list := &List[int]{}
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
		list := &List[int]{}
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

		list = &List[int]{}
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
	t.Run("Streams", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			list := &List[int]{}
			ct := 0
			assert.NotPanic(t, func() {
				for range list.IteratorFront() {
					ct++
				}
			})
			assert.Zero(t, ct)
		})
		t.Run("Forward", func(t *testing.T) {
			list := &List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			seen := 0
			last := -1*math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for value := range list.IteratorFront() {
				if value < 0 && value > 100 {
					t.Fatal(value)
				}
				if last > value {
					t.Fatal(last, ">", value)
				}

				last = value
				seen++
			}
		})
		t.Run("ForwardPop", func(t *testing.T) {
			list := &List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			seen := 0
			last := 0
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for value := range list.IteratorPopFront() {
				if value < 0 && value > 100 {
					t.Fatal(value)
				}
				if last > value {
					t.Fatal(last, ">", value)
				}

				last = value
				seen++
			}
			if seen != 100 {
				t.Error("didn't observe enough items", seen)
			}
			if list.Len() != 0 {
				t.Error("did not consume enough items", list.Len())
			}
		})
		t.Run("Reverse", func(t *testing.T) {
			list := &List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			seen := 0
			last := math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for value := range list.IteratorBack() {
				if value < 0 && value > 100 {
					t.Fatal(value)
				}
				if last < value {
					t.Fatal(last, ">", value)
				}

				last = value
				seen++
			}
		})
		t.Run("PopReverse", func(t *testing.T) {
			list := &List[int]{}
			for i := 1; i <= 100; i++ {
				list.PushBack(i)
			}
			// list is [1, 2, ... 100]
			if list.Front().Value() != 1 {
				t.Error(list.Front().Value())
			}

			seen := 0
			last := math.MaxInt - 1
			t.Log(list.Front().Value(), "->", list.Back().Value())
			for value := range list.IteratorPopBack() {
				if value < 0 && value > 100 {
					t.Fatal(value)
				}
				if last < value {
					t.Fatal(last, ">", value)
				}

				last = value
				seen++
			}
			if seen != 100 {
				t.Error("didn't observe enough items")
			}
			if list.Len() != 0 {
				t.Error("did not consume enough items")
			}
		})
	})
	t.Run("Element", func(t *testing.T) {
		t.Run("String", func(t *testing.T) {
			elem := NewElement("hi")
			if fmt.Sprint(elem) != "hi" {
				t.Fatal(fmt.Sprint(elem))
			}
			if fmt.Sprint(elem) != fmt.Sprintf("%+v", elem) {
				t.Fatal(fmt.Sprint(elem), fmt.Sprintf("%+v", elem))
			}
		})
		t.Run("ChainAppending", func(t *testing.T) {
			list := &List[int]{}
			head := list.Front()
			for i := 1; i <= 100; i++ {
				head.Append(NewElement(i))
			}
			if list.Len() != 100 {
				t.Fatal(list.Len())
			}
			for i := 1; i <= 100; i++ {
				head.Append(&Element[int]{})
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
			list := &List[int]{}
			head := list.Front()
			for i := 1; i <= 100; i++ {
				head.Append(NewElement(i))
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
			list := &List[int]{}
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
			elem := NewElement("hello world!")
			if !elem.Ok() || elem.Value() != "hello world!" {
				t.Fatal(elem.Value())
			}
			elem.Set("hi globe!")
			if !elem.Ok() || elem.Value() != "hi globe!" {
				t.Fatal(elem.Value())
			}
		})
		t.Run("SetListMember", func(t *testing.T) {
			list := &List[int]{}
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
			list := &List[int]{}
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
			list := &List[string]{}
			list.PushBack("hello")
			list.PushBack("world")
			// ["hello", "world"]
			if list.Front().Value() != "hello" && list.Front().Next().Value() != "world" {
				t.Fatal(irt.Collect(list.IteratorFront()))
			}
			if !list.Front().Swap(list.Back()) {
				t.Fatal(irt.Collect(list.IteratorFront()))
			}

			if list.Front().Value() != "world" && list.Front().Next().Value() != "hello" {
				t.Fatal(irt.Collect(list.IteratorFront()))
			}
		})
		t.Run("Self", func(t *testing.T) {
			list := &List[string]{}
			list.PushBack("hello")
			list.PushBack("world")

			if list.Front().Swap(list.Front()) {
				t.Fatal(irt.Collect(list.IteratorFront()))
			}
		})
		t.Run("NilList", func(t *testing.T) {
			list := &List[string]{}
			list.PushFront("hello")

			if list.Front().Swap(NewElement("world")) {
				t.Fatal(irt.Collect(list.IteratorFront()))
			}
		})
		t.Run("Root", func(t *testing.T) {
			list := &List[int]{}
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
			list := &List[int]{}
			list.PushBack(42)
			list.PushBack(84)
			list.PushBack(420)
			list.PushBack(840)

			// [ 42, 84, 420, 840 ]

			if !list.Back().Swap(list.Front().Next()) {
				t.Log("should have swapped")
				t.Fatal(irt.Collect(list.IteratorFront()))
			}
			// expected: [42, 840, 420, 84]
			slice := irt.Collect(list.IteratorFront())
			if list.Len() != 4 {
				t.Log(list.Len(), slice)
				t.Fatal(list.Len())
			}

			idx := 0
			for value := range list.IteratorFront() {
				if slice[idx] != value {
					t.Error(idx, slice[idx], value)
				}
				idx++
			}
			if idx != 3 {
				t.Fatal("did not iterate correctly", idx)
			}
		})
		t.Run("DifferentLists", func(t *testing.T) {
			one := &List[string]{}
			two := &List[string]{}
			one.PushBack("hello")
			two.PushBack("world")
			if one.Front().Swap(two.Front()) {
				t.Fatal("unallowable swap")
			}
		})
		t.Run("Nil", func(t *testing.T) {
			one := &List[string]{}
			one.PushBack("hello")
			one.PushBack("world")
			if one.Front().Swap(nil) {
				t.Fatal("unallowable swap")
			}
		})
	})
	t.Run("Merge", func(t *testing.T) {
		t.Run("Extend", func(t *testing.T) {
			l1 := GetPopulatedList(t, 100)
			l2 := GetPopulatedList(t, 100)
			l2tail := l2.Back()
			if l1.Len() != 100 {
				t.Error(l1.Len())
			}

			l1.Extend(l2.IteratorPopFront())
			if l1.Len() != 200 {
				t.Error(l1.Len())
			}

			if l1.Back().Value() != l2tail.Value() {
				t.Error(l1.Front().Value(), l2tail.Value())
			}

			if l2.Len() != 0 {
				t.Error(l2.Len())
			}
			if l1.Len() != 200 {
				t.Error(l2.Len())
			}
		})
		t.Run("Order", func(t *testing.T) {
			lOne := GetPopulatedList(t, 100)
			itemsOne := irt.Collect(lOne.IteratorFront())

			lTwo := GetPopulatedList(t, 100)
			itemsTwo := irt.Collect(lTwo.IteratorFront())

			if len(itemsOne) != lOne.Len() {
				t.Fatal("incorrect items", len(itemsOne), lOne.Len())
			}
			if len(itemsTwo) != lTwo.Len() {
				t.Fatal("incorrect items")
			}

			lOne.Extend(lTwo.IteratorPopFront())
			combined := append(itemsOne, itemsTwo...)
			idx := 0
			for value := range lOne.IteratorFront() {
				if combined[idx] != value {
					t.Error("missmatch", idx, combined[idx], value)
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
			list := &List[int]{}
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
			nl := &List[int]{}

			if err := nl.UnmarshalJSON(out); err != nil {
				t.Error(err)
			}
			assert.True(t, nl.Front().Value() == list.Front().Value())
			assert.True(t, nl.Front().Next().Value() == list.Front().Next().Value())
			assert.True(t, nl.Front().Next().Next().Value() == list.Front().Next().Next().Value())
		})
		t.Run("TypeMismatch", func(t *testing.T) {
			list := VariadicList(400, 300, 42)

			out, err := list.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}

			nl := &List[string]{}

			if err := nl.UnmarshalJSON(out); err == nil {
				t.Error("should have errored", nl.Front())
			}
		})
		t.Run("ListUnmarshalNil", func(t *testing.T) {
			list := &List[int]{}

			if err := list.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("NilPointerSafety", func(t *testing.T) {
			list := &List[jsonMarshlerError]{}
			list.PushBack(jsonMarshlerError{})
			if out, err := list.MarshalJSON(); err == nil {
				t.Fatal("expected error", string(out))
			}
		})
		t.Run("ElementMarshal", func(t *testing.T) {
			var e *Element[int]

			out, err := e.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "null")

			e = &Element[int]{}
			out, err = e.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "null")

			e.Set(0)
			out, err = e.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "0")

			e = NewElement(42)
			out, err = e.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "42")
		})
	})
	t.Run("Rest", func(t *testing.T) {
		list := &List[int]{}
		list.PushBack(4)
		list.PushBack(8)
		list.PushBack(16)
		list.PushBack(32)
		list.PushBack(64)
		list.PushBack(128)
		list.PushBack(256)
		list.PushFront(0)

		check.Equal(t, list.Len(), 8)
		check.Equal(t, list.Front().Value(), 0)
		check.Equal(t, list.Back().Value(), 256)

		elem := list.Back().Previous().Previous()

		check.NotNil(t, elem)
		check.Equal(t, elem.Value(), 64)
		check.True(t, elem.Ok())

		list.Reset()

		check.Equal(t, list.Len(), 0)
		check.NotNil(t, elem)
		check.Equal(t, elem.Value(), 64)
		check.True(t, elem.list == nil)
	})
}

func BenchmarkList(b *testing.B) {
	var e *Element[int]
	b.Run("SeedElemPool", func(b *testing.B) {
		for i := 0; i < 200; i++ {
			e = NewElement(i)
		}
		b.StopTimer()
		if !e.Ok() {
			b.Fatal(e)
		}
		runtime.GC()
	})

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
				list := &List[int]{}
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
				list := &List[int]{}
				list.PushBack(0)
				list.PopBack()
				b.StartTimer()
				for i := 0; i < size; i++ {
					list.Back().Append(NewElement(i))
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
				list := &List[int]{}
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
				list := &List[int]{}
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
				list := &List[int]{}
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
			b.Run("Stream", func(b *testing.B) {
				list := &List[int]{}
				for i := 0; i < size; i++ {
					list.PushFront(i)
				}

				b.ResetTimer()
				value := -1
				for j := 0; j < b.N; j++ {
					idx := 0
					for inner := range list.IteratorFront() {
						if idx > 2 && idx%2 != 0 {
							value = inner
						}
						idx++
					}
				}
				assert.NotEqual(b, value, -1)
			})
			b.Run("RoundTrip", func(b *testing.B) {
				b.Run("PooledElements", func(b *testing.B) {
					list := &List[int]{}
					for i := 0; i < size; i++ {
						list.Front().Previous().Append(NewElement(i))
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
					list := &List[int]{}
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
