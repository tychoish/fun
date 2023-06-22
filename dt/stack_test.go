package dt_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/risky"
)

func TestStack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
		ok, err := ers.Safe(func() bool {
			var list *dt.Stack[string]
			list.Push("hi")
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
	t.Run("NewFromIterator", func(t *testing.T) {
		iter := fun.SliceIterator([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0})
		stack, err := dt.NewStackFromIterator(ctx, iter)
		assert.NotError(t, err)
		assert.Equal(t, stack.Len(), 10)
		assert.Equal(t, stack.Head().Value(), 0)
	})
	t.Run("Constructor", func(t *testing.T) {
		stack := &dt.Stack[int]{}

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
		stack := &dt.Stack[int]{}

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
		stack := &dt.Stack[int]{}

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
		stack := &dt.Stack[int]{}
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
			elem := &dt.Item[string]{}
			elem.Set("hi")
			if fmt.Sprint(elem) != "hi" {
				t.Fatal(fmt.Sprint(elem))
			}
			if fmt.Sprint(elem) != fmt.Sprintf("%+v", elem) {
				t.Fatal(fmt.Sprint(elem), fmt.Sprintf("%+v", elem))
			}
		})
		t.Run("RemovingRoot", func(t *testing.T) {
			stack := &dt.Stack[int]{}
			stack.Push(100)
			if !stack.Head().Remove() {
				t.Fatal("should have removed element")
			}
			if stack.Len() != 0 {
				t.Fatal("Should be empty")
			}
			if stack.Head() == nil {
				t.Fatal("should give us the root node")
			}
			if stack.Head().In(stack) {
				t.Fatal(stack.Len(), stack.Head().Value())
			}
		})
		t.Run("SetRoot", func(t *testing.T) {
			stack := &dt.Stack[int]{}
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
			item := dt.NewItem(100)
			if item.Remove() {
				t.Fatal("shouldn't be able to remove detached self")
			}
		})
		t.Run("AppendDetached", func(t *testing.T) {
			item := dt.NewItem(100)
			if item != item.Append(nil) {
				t.Error("nil appends should appear as noops")
			}
			if item != item.Append(dt.NewItem(200)) {
				t.Error("can't append to detached")
			}
		})
		t.Run("AppendInSameList", func(t *testing.T) {
			stack := &dt.Stack[int]{}
			stack.Push(100)
			stack.Push(200)
			stack.Push(400)

			if stack.Head() != stack.Head().Append(stack.Head().Next()) {
				t.Error("appending items from the same list should not work")
			}
		})
		t.Run("AppendInDifferentList", func(t *testing.T) {
			stack := &dt.Stack[int]{}
			stack.Push(100)
			stack.Push(200)
			stack.Push(400)

			stackTwo := &dt.Stack[int]{}
			stackTwo.Push(800)

			if stack.Head() != stack.Head().Append(stackTwo.Head()) {
				t.Error("appending items from the same list should not work")
			}
		})
		t.Run("Removed", func(t *testing.T) {
			stack := &dt.Stack[int]{}
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
				if one.Head().Attach(&dt.Stack[int]{}) {
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
				item := &dt.Item[string]{}
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
				item := dt.NewItem(43)
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
	t.Run("Iterators", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			list := &dt.Stack[int]{}
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
		t.Run("Content", func(t *testing.T) {
			stack := GenerateStack(t, 100)
			items := risky.Force(stack.Iterator().Slice(ctx))
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
			iter := stack.Iterator()
			seen := 0
			for iter.Next(ctx) {
				seen++
				iter.Value()
			}
			fun.Invariant.IsTrue(iter.Close() == nil)
			if seen != stack.Len() {
				t.Fatal("did not see all values")
			}
		})
		t.Run("Destructive", func(t *testing.T) {
			stack := GenerateStack(t, 50)
			iter := stack.PopIterator()
			seen := 0
			for iter.Next(ctx) {
				seen++
				iter.Value()
			}
			fun.Invariant.IsTrue(iter.Close() == nil)
			if seen != 50 {
				t.Fatal("did not see all values", seen)
			}
			if stack.Len() != 0 {
				t.Fatal("stack should be empty", stack.Len())
			}
		})
		t.Run("EmptyStart", func(t *testing.T) {
			stack := &dt.Stack[int]{}
			iter := stack.PopIterator()
			seen := 0
			for iter.Next(ctx) {
				seen++
				iter.Value()
			}
			fun.Invariant.IsTrue(iter.Close() == nil)
			if seen != 0 {
				t.Fatal("stack should be empty", stack.Len())
			}
		})
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("RoundTrip", func(t *testing.T) {
			stack := &dt.Stack[int]{}
			stack.Append(400, 300, 42)
			out, err := stack.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != "[42,300,400]" {
				t.Error(string(out))
			}
			nl := &dt.Stack[int]{}

			if err := nl.UnmarshalJSON(out); err != nil {
				t.Error(err)
			}
			fun.Invariant.IsTrue(nl.Head().Value() == stack.Head().Value())
			fun.Invariant.IsTrue(nl.Head().Next().Value() == stack.Head().Next().Value())
			fun.Invariant.IsTrue(nl.Head().Next().Next().Value() == stack.Head().Next().Next().Value())
		})
		t.Run("TypeMismatch", func(t *testing.T) {
			stack := &dt.Stack[int]{}
			stack.Append(400, 300, 42)

			out, err := stack.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}

			nl := &dt.Stack[string]{}

			if err := nl.UnmarshalJSON(out); err == nil {
				t.Error("should have errored", nl.Head())
			}
		})
		t.Run("StackUnmarshalNil", func(t *testing.T) {
			stack := &dt.Stack[int]{}

			if err := stack.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("ItemUnmarshalNil", func(t *testing.T) {
			elem := dt.NewItem(0)

			if err := elem.UnmarshalJSON(nil); err == nil {
				t.Error("should error")
			}
		})
		t.Run("NilPointerSafety", func(t *testing.T) {
			stack := &dt.Stack[jsonMarshlerError]{}
			stack.Push(jsonMarshlerError{})
			if out, err := stack.MarshalJSON(); err == nil {
				t.Fatal("expected error", string(out))
			}
		})
	})
}

func GenerateStack(t *testing.T, size int) *dt.Stack[int] {
	t.Helper()
	stack := &dt.Stack[int]{}
	for i := 0; i < size; i++ {
		stack.Push(1 + i + rand.Int())
	}
	if stack.Len() != size {
		t.Fatal("could not produce fixture", size, stack.Len())
	}

	return stack
}
