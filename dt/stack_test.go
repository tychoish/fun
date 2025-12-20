package dt

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

func TestStack(t *testing.T) {
	t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
		ok, err := ft.WithRecoverDo(func() bool {
			var list *Stack[string]
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
	t.Run("Constructor", func(t *testing.T) {
		stack := &Stack[int]{}

		if stack.Len() != 0 {
			t.Fatal(stack.Len())
		}
		stack.Push(42)
		if stack.Len() != 1 {
			t.Fatal(stack.Len())
		}
		v := stack.Pop()
		if !v.Ok() {
			t.Fatal(v)
		}
		if v.Value() != 42 {
			t.Fatal(v)
		}
	})
	t.Run("PopEmpty", func(t *testing.T) {
		stack := &Stack[int]{}

		elem := stack.Pop()
		// it didn't panic!
		if elem == nil {
			t.Error("should have lazily initialized")
		}
		if elem.Ok() {
			t.Error("shouldn't be ok")
		}
	})
	t.Run("HeadEmpty", func(t *testing.T) {
		stack := &Stack[int]{}

		elem := stack.Head()
		// it didn't panic!
		if elem == nil {
			t.Error("should have lazily initialized")
		}
		if elem.Ok() {
			t.Error("shouldn't be ok")
		}
	})
	t.Run("NonDestructiveAccess", func(t *testing.T) {
		stack := &Stack[int]{}
		stack.Push(100)
		stack.Push(42)
		if stack.Head().Value() != 42 {
			t.Fatal(stack.Head().Value())
		}
		if stack.Head().Next().Value() != 100 {
			t.Fatal(stack.Head().Next().Value())
		}
	})
	t.Run("Item", func(t *testing.T) {
		t.Run("String", func(t *testing.T) {
			elem := &Item[string]{}
			elem.Set("hi")
			if fmt.Sprint(elem) != "hi" {
				t.Fatal(fmt.Sprint(elem))
			}
			if fmt.Sprint(elem) != fmt.Sprintf("%+v", elem) {
				t.Fatal(fmt.Sprint(elem), fmt.Sprintf("%+v", elem))
			}
		})
		t.Run("RemovingRoot", func(t *testing.T) {
			stack := &Stack[int]{}
			assert.Equal(t, stack.Len(), 0)

			// head is non-nil and we can't remove it
			assert.NotNil(t, stack.Head())
			assert.True(t, ft.Not(stack.Head().Remove()))

			// add an element
			stack.Push(100)
			assert.Equal(t, stack.Len(), 1)

			// we can remove it
			assert.True(t, stack.Head().Remove())
			assert.Equal(t, stack.Len(), 0)

			// head still isn't nil
			assert.NotNil(t, stack.Head())

			// the root node is in the stack
			if stack.Head().In(stack) {
				t.Fatal(stack.Len(), stack.Head().Value())
			}
		})
		t.Run("SetRoot", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(100)
			if val := stack.Pop(); !val.Ok() {
				t.Fatal("should have removed element")
			}
			root := stack.Head()
			if root.Ok() || root.Value() != 0 {
				t.Fatal("does not look like root", root)
			}
			if root.Set(200) {
				t.Fatal("should not set list")
			}
			if stack.Len() != 0 {
				t.Fatal(stack.Len())
			}
		})

		t.Run("RemoveDetached", func(t *testing.T) {
			item := NewItem(100)
			if item.Remove() {
				t.Fatal("shouldn't be able to remove detached self")
			}
		})
		t.Run("AppendDetached", func(t *testing.T) {
			item := NewItem(100)
			if item != item.Append(nil) {
				t.Error("nil appends should appear as noops")
			}
			if item != item.Append(NewItem(200)) {
				t.Error("can't append to detached")
			}
		})
		t.Run("AppendInSameList", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(100)
			stack.Push(200)
			stack.Push(400)

			if stack.Head() != stack.Head().Append(stack.Head().Next()) {
				t.Error("appending items from the same list should not work")
			}
		})
		t.Run("AppendInDifferentList", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(100)
			stack.Push(200)
			stack.Push(400)

			stackTwo := &Stack[int]{}
			stackTwo.Push(800)

			if stack.Head() != stack.Head().Append(stackTwo.Head()) {
				t.Error("appending items from the same list should not work")
			}
		})
		t.Run("Removed", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(100)
			poped := stack.Pop()
			if poped.Remove() {
				t.Error("shouldn't get this")
			}
		})
		t.Run("Attach", func(t *testing.T) {
			t.Run("Merge", func(t *testing.T) {
				one := GenerateStack(t, 16)
				two := GenerateStack(t, 16)
				if !one.Head().Attach(two) {
					t.Error("should succeed in the simple case")
				}
				if one.Len() != 32 {
					t.Error("length should be correct")
				}
				if two.Len() != 0 {
					t.Error("second stack should be exhausted", two.Len())
				}
			})
			t.Run("Nil", func(t *testing.T) {
				one := GenerateStack(t, 16)
				if one.Head().Attach(nil) {
					t.Error("do not attach empty stack")
				}
				if one.Len() != 16 {
					t.Error("length should not be modified")
				}
			})
			t.Run("Empty", func(t *testing.T) {
				one := GenerateStack(t, 16)
				if one.Head().Attach(&Stack[int]{}) {
					t.Error("do not attach empty stack")
				}
				if one.Len() != 16 {
					t.Error("length should not be modified")
				}
			})
			t.Run("Self", func(t *testing.T) {
				one := GenerateStack(t, 16)
				if one.Head().Attach(one) {
					t.Error("do not attach empty stack")
				}
				if one.Len() != 16 {
					t.Error("length should not be modified")
				}
			})
		})
		t.Run("Detach", func(t *testing.T) {
			t.Run("UnconfiguredItem", func(t *testing.T) {
				item := &Item[string]{}
				if item.Ok() {
					t.Error("invalid item")
				}
				stack := item.Detach()
				if item.In(stack) {
					t.Error("can't be a member of a stack if you aren't valid")
				}
				if !item.Set("hi") {
					t.Error("item should have it's value set")
				}
				stack = item.Detach()
				if !item.In(stack) {
					t.Error("once valid, item.Detach works")
				}
			})
			t.Run("DetachedItem", func(t *testing.T) {
				item := NewItem(43)
				stack := item.Detach()
				if stack.Head() != item || !item.In(stack) {
					t.Fatal("item should be in the new list")
				}
			})
			t.Run("DetachHead", func(t *testing.T) {
				stack := GenerateStack(t, 4)
				other := stack.Head().Detach()
				if stack != other {
					t.Error("detaching the head is just the stack")
				}
			})
			t.Run("SmallCase", func(t *testing.T) {
				stack := GenerateStack(t, 4)
				newHead := stack.Head().Next().Next()
				other := newHead.Detach()
				if stack == other {
					t.Error("should have two different stacks")
				}
				if stack.Len() != other.Len() && stack.Len() != 2 {
					t.Error("stacks of equal size")
				}
				if !newHead.In(other) {
					t.Error("stack should have split")
				}
			})
		})
	})
	t.Run("Streams", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			list := &Stack[int]{}
			ct := 0
			assert.NotPanic(t, func() {
				for range list.Iterator() {
					ct++
				}
			})
			assert.Zero(t, ct)
		})
		t.Run("Content", func(t *testing.T) {
			stack := GenerateStack(t, 100)
			items := irt.Collect(stack.Iterator())
			if len(items) != stack.Len() {
				t.Fatal("unexpected collection", len(items), stack.Len())
			}
			idx := 0
			seen := 0
			t.Log(items)
			for it := stack.Head(); it.Ok(); it = it.Next() {
				seen++
				if it.Value() != items[idx] {
					t.Fatal(seen, idx, items[idx], it.Value())
				}
				idx++
			}
			if seen != len(items) {
				t.Error("missed some, items")
			}
		})
		t.Run("Interface", func(t *testing.T) {
			stack := GenerateStack(t, 5)
			seen := 0
			for val := range stack.Iterator() {
				seen++
				check.NotZero(t, val)
			}

			if seen != stack.Len() {
				t.Fatal("did not see all values")
			}
		})
		t.Run("Destructive", func(t *testing.T) {
			stack := GenerateStack(t, 50)
			iter := stack.StreamPop()
			seen := 0
			for value := range iter {
				seen++
				check.NotZero(t, value)
			}
			if seen != 50 {
				t.Fatal("did not see all values", seen)
			}
			if stack.Len() != 0 {
				t.Fatal("stack should be empty", stack.Len())
			}
		})
		t.Run("EmptyStart", func(t *testing.T) {
			stack := &Stack[int]{}
			iter := stack.StreamPop()
			seen := 0
			for value := range iter {
				seen++
				check.NotZero(t, value)
			}
			if seen != 0 {
				t.Fatal("stack should be empty", stack.Len())
			}
		})
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("RoundTrip", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Append(400, 300, 42)
			out, err := stack.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != "[42,300,400]" {
				t.Error(string(out))
			}
			nl := &Stack[int]{}

			if err := nl.UnmarshalJSON(out); err != nil {
				t.Error(err)
			}
			fun.Invariant.IsTrue(nl.Head().Value() == stack.Head().Value())
			fun.Invariant.IsTrue(nl.Head().Next().Value() == stack.Head().Next().Value())
			fun.Invariant.IsTrue(nl.Head().Next().Next().Value() == stack.Head().Next().Next().Value())
		})
		t.Run("TypeMismatch", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Append(400, 300, 42)

			out, err := stack.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}

			nl := &Stack[string]{}

			if err := nl.UnmarshalJSON(out); err == nil {
				t.Error("should have errored", nl.Head())
			}
		})
		t.Run("StackUnmarshalNil", func(t *testing.T) {
			stack := &Stack[int]{}

			if err := stack.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("ItemUnmarshalNil", func(t *testing.T) {
			elem := NewItem(0)

			if err := elem.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("NilPointerSafety", func(t *testing.T) {
			stack := &Stack[jsonMarshlerError]{}
			stack.Push(jsonMarshlerError{})
			if out, err := stack.MarshalJSON(); err == nil {
				t.Fatal("expected error", string(out))
			}
		})
	})
	t.Run("Seq", func(t *testing.T) {
		stack := &Stack[int]{}
		stack.Append(100_000, 10_000, 1_000, 100, 10, 1)

		last := 1 // initialize to one to avoid dividing by zero
		for item := range stack.Iterator() {
			check.True(t, item%last == 0)
			if t.Failed() {
				t.Log(item, last, item%last, item%last == 0)
				t.FailNow()
			}
			last = item
		}
	})
}

func GenerateStack(t *testing.T, size int) *Stack[int] {
	t.Helper()
	stack := &Stack[int]{}
	for i := 0; i < size; i++ {
		stack.Push(1 + i + rand.Int())
	}
	if stack.Len() != size {
		t.Fatal("could not produce fixture", size, stack.Len())
	}

	return stack
}
