package adt

import (
	"fmt"
	"slices"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

func TestLinkedList(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		t.Run("PushBackSingle", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)

			check.Equal(t, 1, l.Len())
			check.Equal(t, 1, l.Front().item)
			check.Equal(t, 1, l.Back().item)
		})

		t.Run("PushBackMultiple", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			check.Equal(t, 3, l.Len())
			check.Equal(t, 1, l.Front().item)
			check.Equal(t, 3, l.Back().item)
		})

		t.Run("PushFrontSingle", func(t *testing.T) {
			l := &list[int]{}
			l.PushFront(1)

			check.Equal(t, 1, l.Len())
			check.Equal(t, 1, l.Front().item)
			check.Equal(t, 1, l.Back().item)
		})

		t.Run("PushFrontMultiple", func(t *testing.T) {
			l := &list[int]{}
			l.PushFront(1)
			l.PushFront(2)
			l.PushFront(3)

			check.Equal(t, 3, l.Len())
			check.Equal(t, 3, l.Front().item)
			check.Equal(t, 1, l.Back().item)
		})

		t.Run("MixedPushBackAndFront", func(t *testing.T) {
			l := &list[string]{}
			l.PushBack("middle")
			l.PushFront("front")
			l.PushBack("back")

			check.Equal(t, 3, l.Len())
			check.Equal(t, "front", l.Front().item)
			check.Equal(t, "back", l.Back().item)
		})

		t.Run("ChainedPushBack", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1).PushBack(2).PushBack(3)

			check.Equal(t, 3, l.Len())
			check.Equal(t, 1, l.Front().item)
			check.Equal(t, 3, l.Back().item)
		})
	})

	t.Run("Remove", func(t *testing.T) {
		t.Run("PopBackSingle", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)

			val := l.PopBack()

			check.Equal(t, 42, val)
			check.Equal(t, 0, l.Len())
		})

		t.Run("PopFrontSingle", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)

			val := l.PopFront()

			check.Equal(t, 42, val)
			check.Equal(t, 0, l.Len())
		})

		t.Run("PopBackMultiple", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			check.Equal(t, 3, l.PopBack())
			check.Equal(t, 2, l.Len())
			check.Equal(t, 2, l.PopBack())
			check.Equal(t, 1, l.Len())
			check.Equal(t, 1, l.PopBack())
			check.Equal(t, 0, l.Len())
		})

		t.Run("PopFrontMultiple", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			check.Equal(t, 1, l.PopFront())
			check.Equal(t, 2, l.Len())
			check.Equal(t, 2, l.PopFront())
			check.Equal(t, 1, l.Len())
			check.Equal(t, 3, l.PopFront())
			check.Equal(t, 0, l.Len())
		})

		t.Run("AlternatingPopFrontAndBack", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)
			l.PushBack(4)

			check.Equal(t, 1, l.PopFront())
			check.Equal(t, 4, l.PopBack())
			check.Equal(t, 2, l.PopFront())
			check.Equal(t, 3, l.PopBack())
			check.Equal(t, 0, l.Len())
		})

		t.Run("PopAndPushInterleaved", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			check.Equal(t, 2, l.PopBack())
			l.PushBack(3)
			check.Equal(t, 1, l.PopFront())
			l.PushFront(0)
			check.Equal(t, 2, l.Len())
			check.Equal(t, 0, l.Front().item)
			check.Equal(t, 3, l.Back().item)
		})
	})

	t.Run("Iterate", func(t *testing.T) {
		t.Run("IteratorFrontEmpty", func(t *testing.T) {
			l := &list[int]{}
			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.Equal(t, 0, len(result))
		})

		t.Run("IteratorBackEmpty", func(t *testing.T) {
			l := &list[int]{}
			var result []int
			for v := range l.IteratorBack() {
				result = append(result, v)
			}
			check.Equal(t, 0, len(result))
		})

		t.Run("IteratorFrontSingleElement", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}

			assert.Equal(t, 1, len(result))
			check.Equal(t, 42, result[0])
		})

		t.Run("IteratorBackSingleElement", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)

			var result []int
			for v := range l.IteratorBack() {
				result = append(result, v)
			}

			assert.Equal(t, 1, len(result))
			check.Equal(t, 42, result[0])
		})

		t.Run("IteratorFrontMultipleElements", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}

			check.Equal(t, 3, len(result))
			check.True(t, slices.Equal([]int{1, 2, 3}, result))
		})

		t.Run("IteratorBackMultipleElements", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			var result []int
			for v := range l.IteratorBack() {
				result = append(result, v)
			}

			check.Equal(t, 3, len(result))
			check.True(t, slices.Equal([]int{3, 2, 1}, result))
		})

		t.Run("IteratorFrontYieldsCorrectValues", func(t *testing.T) {
			l := &list[string]{}
			l.PushBack("a")
			l.PushBack("b")
			l.PushBack("c")
			l.PushBack("d")

			var result []string
			for v := range l.IteratorFront() {
				result = append(result, v)
			}

			check.True(t, slices.Equal([]string{"a", "b", "c", "d"}, result))
		})

		t.Run("IteratorBackYieldsCorrectValues", func(t *testing.T) {
			l := &list[string]{}
			l.PushBack("a")
			l.PushBack("b")
			l.PushBack("c")
			l.PushBack("d")

			var result []string
			for v := range l.IteratorBack() {
				result = append(result, v)
			}

			check.True(t, slices.Equal([]string{"d", "c", "b", "a"}, result))
		})

		t.Run("IteratorCountMatchesLen", func(t *testing.T) {
			l := &list[int]{}
			for i := 0; i < 10; i++ {
				l.PushBack(i)
			}

			count := 0
			for range l.IteratorFront() {
				count++
			}

			check.Equal(t, l.Len(), count)
		})

		t.Run("IteratorEarlyBreak", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)
			l.PushBack(4)
			l.PushBack(5)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
				if len(result) == 3 {
					break
				}
			}

			check.Equal(t, 3, len(result))
			check.True(t, slices.Equal([]int{1, 2, 3}, result))
		})
	})

	t.Run("PushFront", func(t *testing.T) {
		t.Run("OrderVerification", func(t *testing.T) {
			l := &list[int]{}
			l.PushFront(1)
			l.PushFront(2)
			l.PushFront(3)
			l.PushFront(4)
			l.PushFront(5)

			// Should be [5, 4, 3, 2, 1] front to back
			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}

			check.True(t, slices.Equal([]int{5, 4, 3, 2, 1}, result))
		})

		t.Run("ChainedPushFront", func(t *testing.T) {
			l := &list[int]{}
			l.PushFront(1).PushFront(2).PushFront(3)

			check.Equal(t, 3, l.Len())
			check.Equal(t, 3, l.Front().item)
			check.Equal(t, 1, l.Back().item)
		})

		t.Run("AlternatingPushFrontAndBack", func(t *testing.T) {
			l := &list[int]{}
			l.PushFront(2) // [2]
			l.PushBack(3)  // [2, 3]
			l.PushFront(1) // [1, 2, 3]
			l.PushBack(4)  // [1, 2, 3, 4]

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}

			check.True(t, slices.Equal([]int{1, 2, 3, 4}, result))
		})

		t.Run("PushFrontAfterPop", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PopFront()   // remove 1, now [2]
			l.PushFront(0) // [0, 2]

			check.Equal(t, 2, l.Len())
			check.Equal(t, 0, l.Front().item)
			check.Equal(t, 2, l.Back().item)
		})
	})

	t.Run("EdgeCases", func(t *testing.T) {
		t.Run("PopFromEmptyListBack", func(t *testing.T) {
			l := &list[int]{}

			// Should not panic, returns zero value
			assert.NotPanic(t, func() {
				val := l.PopBack()
				check.Equal(t, 0, val)
			})
			check.Equal(t, 0, l.Len())
		})

		t.Run("PopFromEmptyListFront", func(t *testing.T) {
			l := &list[int]{}

			// Should not panic, returns zero value
			assert.NotPanic(t, func() {
				val := l.PopFront()
				check.Equal(t, 0, val)
			})
			check.Equal(t, 0, l.Len())
		})

		t.Run("PopAllThenPopAgain", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)

			l.PopFront()
			l.PopFront()

			// Now empty, pop again
			assert.NotPanic(t, func() {
				val := l.PopFront()
				check.Equal(t, 0, val)
			})
			check.Equal(t, 0, l.Len())
		})

		t.Run("RepeatedPopFromEmpty", func(t *testing.T) {
			l := &list[int]{}

			assert.NotPanic(t, func() {
				for i := 0; i < 10; i++ {
					l.PopFront()
					l.PopBack()
				}
			})
			check.Equal(t, 0, l.Len())
		})

		t.Run("AddRemoveAddRemove", func(t *testing.T) {
			l := &list[int]{}

			for i := 0; i < 5; i++ {
				l.PushBack(i)
				l.PushFront(i * 10)
			}
			// [40, 30, 20, 10, 0, 0, 1, 2, 3, 4]
			check.Equal(t, 10, l.Len())

			for i := 0; i < 5; i++ {
				l.PopFront()
				l.PopBack()
			}
			check.Equal(t, 0, l.Len())

			// Add again after emptying
			l.PushBack(100)
			l.PushFront(200)
			check.Equal(t, 2, l.Len())
			check.Equal(t, 200, l.Front().item)
			check.Equal(t, 100, l.Back().item)
		})

		t.Run("NilPointerItems", func(t *testing.T) {
			l := &list[*int]{}

			l.PushBack(nil)
			l.PushBack(nil)
			val := 42
			l.PushBack(&val)
			l.PushFront(nil)

			check.Equal(t, 4, l.Len())

			// Front should be nil
			check.True(t, l.Front().item == nil)

			// Pop and verify
			check.True(t, l.PopFront() == nil)
			check.True(t, l.PopFront() == nil)
			check.True(t, l.PopFront() == nil)
			check.Equal(t, 42, *l.PopFront())
			check.Equal(t, 0, l.Len())
		})

		t.Run("ZeroValueItems", func(t *testing.T) {
			l := &list[int]{}

			l.PushBack(0)
			l.PushBack(0)
			l.PushBack(0)

			check.Equal(t, 3, l.Len())

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{0, 0, 0}, result))
		})

		t.Run("EmptyStringItems", func(t *testing.T) {
			l := &list[string]{}

			l.PushBack("")
			l.PushBack("a")
			l.PushBack("")

			check.Equal(t, 3, l.Len())
			check.Equal(t, "", l.Front().item)
			check.Equal(t, "", l.Back().item)
		})
	})

	t.Run("ModifyWhileIterating", func(t *testing.T) {
		t.Run("PopFrontWhileIteratingFront", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)
			l.PushBack(4)
			l.PushBack(5)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
				if v == 2 {
					l.PopFront() // Remove current front during iteration
				}
			}

			// Behavior depends on implementation - just verify no panic
			check.True(t, len(result) > 0)
		})

		t.Run("PushBackWhileIteratingFront", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			count := 0
			for v := range l.IteratorFront() {
				count++
				if v == 2 {
					l.PushBack(100) // Add element during iteration
				}
				// Prevent infinite loop
				if count > 10 {
					break
				}
			}

			// Should have iterated and possibly seen new element
			check.True(t, count >= 3)
		})

		t.Run("PushFrontWhileIteratingBack", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			count := 0
			for v := range l.IteratorBack() {
				count++
				if v == 2 {
					l.PushFront(100) // Add element during iteration
				}
				// Prevent infinite loop
				if count > 10 {
					break
				}
			}

			check.True(t, count >= 3)
		})

		t.Run("ClearListWhileIterating", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
				// Clear remaining elements
				for l.Len() > 0 {
					l.PopBack()
				}
			}

			// Should have gotten at least the first element
			check.True(t, len(result) >= 1)
			check.Equal(t, 1, result[0])
		})
	})

	t.Run("ElementMethods", func(t *testing.T) {
		t.Run("Value", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)

			check.Equal(t, 42, l.Front().Value())
			check.Equal(t, 42, l.Back().Value())
		})

		t.Run("ValueMultipleElements", func(t *testing.T) {
			l := &list[string]{}
			l.PushBack("first")
			l.PushBack("second")
			l.PushBack("third")

			check.Equal(t, "first", l.Front().Value())
			check.Equal(t, "third", l.Back().Value())
		})

		t.Run("ValueAfterSet", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)

			elem := l.Front()
			check.Equal(t, 1, elem.Value())

			elem.Set(100)
			check.Equal(t, 100, elem.Value())
			check.Equal(t, 100, l.Front().Value())
		})

		t.Run("Next", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			first := l.Front()
			check.Equal(t, 1, first.Value())

			second := first.Next()
			check.Equal(t, 2, second.Value())

			third := second.Next()
			check.Equal(t, 3, third.Value())
		})

		t.Run("NextWrapsToRoot", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)

			last := l.Back()
			check.Equal(t, 2, last.Value())

			// Next of last element should be root (not in list)
			afterLast := last.Next()
			check.True(t, !l.In(afterLast))
		})

		t.Run("Previous", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			last := l.Back()
			check.Equal(t, 3, last.Value())

			second := last.Previous()
			check.Equal(t, 2, second.Value())

			first := second.Previous()
			check.Equal(t, 1, first.Value())
		})

		t.Run("PreviousWrapsToRoot", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)

			first := l.Front()
			check.Equal(t, 1, first.Value())

			// Previous of first element should be root (not in list)
			beforeFirst := first.Previous()
			check.True(t, !l.In(beforeFirst))
		})

		t.Run("NextAndPreviousTraversal", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)
			l.PushBack(4)
			l.PushBack(5)

			// Traverse forward using Next
			var forward []int
			for e := l.Front(); l.In(e); e = e.Next() {
				forward = append(forward, e.Value())
			}
			check.True(t, slices.Equal([]int{1, 2, 3, 4, 5}, forward))

			// Traverse backward using Previous
			var backward []int
			for e := l.Back(); l.In(e); e = e.Previous() {
				backward = append(backward, e.Value())
			}
			check.True(t, slices.Equal([]int{5, 4, 3, 2, 1}, backward))
		})

		t.Run("Ok", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)

			front := l.Front()
			check.True(t, front.Ok())

			back := l.Back()
			check.True(t, back.Ok())
		})

		t.Run("OkAfterPop", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)

			front := l.Front()
			check.True(t, front.Ok())

			l.PopFront()
			// After popping, the element should no longer be in the list
			check.True(t, !front.Ok())
		})

		t.Run("OkOnRoot", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)

			// Navigate past front to get root
			root := l.Front().Previous()
			check.True(t, !root.Ok())
		})

		t.Run("ElemPushFront", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(3)

			// Get element with value 3
			second := l.Front().Next()
			check.Equal(t, 3, second.Value())

			// Create new element and push it in front of 3
			newElem := l.newElem().Set(2)
			second.PushFront(newElem)
			l.inc()

			// List should now be [1, 2, 3]
			var result []int
			for e := l.Front(); l.In(e); e = e.Next() {
				result = append(result, e.Value())
			}
			check.True(t, slices.Equal([]int{1, 2, 3}, result))
		})

		t.Run("ElemPushFrontAtFront", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(2)
			l.PushBack(3)

			// Push in front of the first element
			front := l.Front()
			newElem := l.newElem().Set(1)
			front.PushFront(newElem)
			l.inc()

			// List should now be [1, 2, 3]
			check.Equal(t, 1, l.Front().Value())

			var result []int
			for e := l.Front(); l.In(e); e = e.Next() {
				result = append(result, e.Value())
			}
			check.True(t, slices.Equal([]int{1, 2, 3}, result))
		})

		t.Run("ElemPushFrontMultiple", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(5)

			// Insert 1, 2, 3, 4 in front of 5 (each goes directly before 5)
			target := l.Front()
			for i := 1; i <= 4; i++ {
				newElem := l.newElem().Set(i)
				target.PushFront(newElem)
			}

			var result []int
			for e := l.Front(); l.In(e); e = e.Next() {
				result = append(result, e.Value())
			}

			// Verify all elements present and length is correct
			check.Equal(t, 5, len(result))
			check.Equal(t, 5, l.Len())
			// Last element should still be 5
			check.Equal(t, 5, l.Back().Value())
		})

		t.Run("NextPreviousSymmetry", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)

			// e.Next().Previous() should return e
			middle := l.Front().Next()
			check.Equal(t, 2, middle.Value())
			check.Equal(t, middle, middle.Next().Previous())
			check.Equal(t, middle, middle.Previous().Next())
		})

		t.Run("SingleElementNextPrevious", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)

			only := l.Front()
			check.Equal(t, 42, only.Value())

			// Both Next and Previous should lead to root (not in list)
			check.True(t, !l.In(only.Next()))
			check.True(t, !l.In(only.Previous()))
		})

		t.Run("ValueOnEmptyListFront", func(t *testing.T) {
			l := &list[int]{}

			// Front on empty list returns root, Value should return zero
			front := l.Front()
			check.Equal(t, 0, front.Value())
			check.True(t, !front.Ok())
		})

		t.Run("ValueWithPointerType", func(t *testing.T) {
			l := &list[*int]{}

			val1 := 10
			val2 := 20
			l.PushBack(&val1)
			l.PushBack(nil)
			l.PushBack(&val2)

			check.Equal(t, &val1, l.Front().Value())
			check.Equal(t, 10, *l.Front().Value())

			middle := l.Front().Next()
			check.True(t, middle.Value() == nil)

			check.Equal(t, &val2, l.Back().Value())
			check.Equal(t, 20, *l.Back().Value())
		})
	})
}

func intCmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func stringCmp(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func TestLinkedListSort(t *testing.T) {
	t.Run("SortQuick", func(t *testing.T) {
		t.Run("EmptyList", func(t *testing.T) {
			l := &list[int]{}
			assert.NotPanic(t, func() {
				l.SortQuick(intCmp)
			})
			check.Equal(t, 0, l.Len())
		})

		t.Run("SingleElement", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)
			l.SortQuick(intCmp)

			check.Equal(t, 1, l.Len())
			check.Equal(t, 42, l.Front().Value())
		})

		t.Run("AlreadySorted", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)
			l.PushBack(4)
			l.PushBack(5)

			l.SortQuick(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{1, 2, 3, 4, 5}, result))
		})

		t.Run("ReverseSorted", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(5)
			l.PushBack(4)
			l.PushBack(3)
			l.PushBack(2)
			l.PushBack(1)

			l.SortQuick(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{1, 2, 3, 4, 5}, result))
		})

		t.Run("RandomOrder", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(3)
			l.PushBack(1)
			l.PushBack(4)
			l.PushBack(1)
			l.PushBack(5)
			l.PushBack(9)
			l.PushBack(2)
			l.PushBack(6)

			l.SortQuick(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{1, 1, 2, 3, 4, 5, 6, 9}, result))
		})

		t.Run("WithDuplicates", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(3)
			l.PushBack(3)
			l.PushBack(1)
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(2)

			l.SortQuick(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{1, 1, 2, 2, 3, 3}, result))
		})

		t.Run("Strings", func(t *testing.T) {
			l := &list[string]{}
			l.PushBack("banana")
			l.PushBack("apple")
			l.PushBack("cherry")
			l.PushBack("date")

			l.SortQuick(stringCmp)

			var result []string
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]string{"apple", "banana", "cherry", "date"}, result))
		})

		t.Run("TwoElements", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(2)
			l.PushBack(1)

			l.SortQuick(intCmp)

			check.Equal(t, 1, l.Front().Value())
			check.Equal(t, 2, l.Back().Value())
		})

		t.Run("NegativeNumbers", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(-5)
			l.PushBack(3)
			l.PushBack(-1)
			l.PushBack(0)
			l.PushBack(2)

			l.SortQuick(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{-5, -1, 0, 2, 3}, result))
		})
	})

	t.Run("SortMerge", func(t *testing.T) {
		t.Run("EmptyList", func(t *testing.T) {
			l := &list[int]{}
			assert.NotPanic(t, func() {
				l.SortMerge(intCmp)
			})
			check.Equal(t, 0, l.Len())
		})

		t.Run("SingleElement", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(42)
			l.SortMerge(intCmp)

			check.Equal(t, 1, l.Len())
			check.Equal(t, 42, l.Front().Value())
		})

		t.Run("AlreadySorted", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(3)
			l.PushBack(4)
			l.PushBack(5)

			l.SortMerge(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			testt.Log(t, result)
			check.True(t, slices.Equal([]int{1, 2, 3, 4, 5}, result))
		})

		t.Run("ReverseSorted", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(5)
			l.PushBack(4)
			l.PushBack(3)
			l.PushBack(2)
			l.PushBack(1)

			l.SortMerge(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			testt.Log(t, result)
			check.True(t, slices.Equal([]int{1, 2, 3, 4, 5}, result))
		})

		t.Run("RandomOrder", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(3)
			l.PushBack(1)
			l.PushBack(4)
			l.PushBack(1)
			l.PushBack(5)
			l.PushBack(9)
			l.PushBack(2)
			l.PushBack(6)

			l.SortMerge(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{1, 1, 2, 3, 4, 5, 6, 9}, result))
		})

		t.Run("WithDuplicates", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(3)
			l.PushBack(3)
			l.PushBack(1)
			l.PushBack(1)
			l.PushBack(2)
			l.PushBack(2)

			l.SortMerge(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{1, 1, 2, 2, 3, 3}, result))
		})

		t.Run("Strings", func(t *testing.T) {
			l := &list[string]{}
			l.PushBack("banana")
			l.PushBack("apple")
			l.PushBack("cherry")
			l.PushBack("date")

			l.SortMerge(stringCmp)

			var result []string
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]string{"apple", "banana", "cherry", "date"}, result))
		})

		t.Run("TwoElements", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(2)
			l.PushBack(1)

			l.SortMerge(intCmp)

			check.Equal(t, 1, l.Front().Value())
			check.Equal(t, 2, l.Back().Value())
		})

		t.Run("NegativeNumbers", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(-5)
			l.PushBack(3)
			l.PushBack(-1)
			l.PushBack(0)
			l.PushBack(2)

			l.SortMerge(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{-5, -1, 0, 2, 3}, result))
		})

		t.Run("OddNumberOfElements", func(t *testing.T) {
			l := &list[int]{}
			l.PushBack(5)
			l.PushBack(2)
			l.PushBack(8)

			l.SortMerge(intCmp)

			var result []int
			for v := range l.IteratorFront() {
				result = append(result, v)
			}
			check.True(t, slices.Equal([]int{2, 5, 8}, result))
		})
	})

	t.Run("BothSortsProduceSameResult", func(t *testing.T) {
		input := []int{64, 34, 25, 12, 22, 11, 90, 42, 15, 77}

		l1 := &list[int]{}
		l2 := &list[int]{}
		for _, v := range input {
			l1.PushBack(v)
			l2.PushBack(v)
		}

		l1.SortQuick(intCmp)
		l2.SortMerge(intCmp)

		var r1, r2 []int
		for v := range l1.IteratorFront() {
			r1 = append(r1, v)
		}
		for v := range l2.IteratorFront() {
			r2 = append(r2, v)
		}

		testt.Log(t, "r1", r1)
		testt.Log(t, "r2", r2)
		check.True(t, slices.Equal(r1, r2))
		check.True(t, slices.IsSorted(r1))
	})

	t.Run("DescendingSort", func(t *testing.T) {
		reverseCmp := func(a, b int) int { return -intCmp(a, b) }

		l := &list[int]{}
		l.PushBack(1)
		l.PushBack(5)
		l.PushBack(3)
		l.PushBack(2)
		l.PushBack(4)

		l.SortQuick(reverseCmp)

		var result []int
		for v := range l.IteratorFront() {
			result = append(result, v)
		}
		testt.Log(t, result)
		check.True(t, slices.Equal([]int{5, 4, 3, 2, 1}, result))
	})

	t.Run("PopElemIterEarlyReturn", func(t *testing.T) {
		l := &list[int]{}
		l.PushBack(1)
		l.PushBack(2)
		l.PushBack(3)
		check.Equal(t, 3, l.Len())
		for value := range l.IteratorPopBackE() {
			check.Equal(t, value.Value(), 3)
			break
		}
		check.Equal(t, 2, l.Len())
		for value := range l.IteratorPopFrontE() {
			check.Equal(t, value.Value(), 1)
			break
		}
		check.Equal(t, 1, l.Len())
	})
	t.Run("PopIterEarlyReturn", func(t *testing.T) {
		l := &list[int]{}
		l.PushBack(1)
		l.PushBack(2)
		l.PushBack(3)
		check.Equal(t, 3, l.Len())
		for value := range l.IteratorPopBack() {
			check.Equal(t, value, 3)
			break
		}
		check.Equal(t, 2, l.Len())
		for value := range l.IteratorPopFront() {
			check.Equal(t, value, 1)
			break
		}
		check.Equal(t, 1, l.Len())
	})
	t.Run("Extend", func(t *testing.T) {
		l := &list[int]{}
		l.Extend(irt.Args(0, 1, 2, 3, 4, 5, 6))
		check.Equal(t, 7, l.Len())
		for expected := range 7 {
			for value := range l.IteratorPopFrontE() {
				check.Equal(t, value.Value(), expected)
				break
			}
		}
		check.Equal(t, 0, l.Len())
	})
}

func makeTestList(size int) *list[int] {
	l := &list[int]{}
	// Use a simple pseudo-random sequence for reproducibility
	val := 12345
	for i := 0; i < size; i++ {
		val = (val*1103515245 + 12345) & 0x7fffffff
		l.PushBack(val % 10000)
	}
	return l
}

func BenchmarkListSort(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("SortQuick/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := makeTestList(size)
				b.StartTimer()
				l.SortQuick(intCmp)
			}
		})

		b.Run(fmt.Sprintf("SortMerge/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := makeTestList(size)
				b.StartTimer()
				l.SortMerge(intCmp)
			}
		})
	}
}

func BenchmarkListSortAlreadySorted(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("SortQuick/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := &list[int]{}
				for j := 0; j < size; j++ {
					l.PushBack(j)
				}
				b.StartTimer()
				l.SortQuick(intCmp)
			}
		})

		b.Run(fmt.Sprintf("SortMerge/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := &list[int]{}
				for j := 0; j < size; j++ {
					l.PushBack(j)
				}
				b.StartTimer()
				l.SortMerge(intCmp)
			}
		})
	}
}

func BenchmarkListSortReversed(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("SortQuick/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := &list[int]{}
				for j := size - 1; j >= 0; j-- {
					l.PushBack(j)
				}
				b.StartTimer()
				l.SortQuick(intCmp)
			}
		})

		b.Run(fmt.Sprintf("SortMerge/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				l := &list[int]{}
				for j := size - 1; j >= 0; j-- {
					l.PushBack(j)
				}
				b.StartTimer()
				l.SortMerge(intCmp)
			}
		})
	}
}
