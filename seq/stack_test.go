package seq_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
)

func TestStack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Constructor", func(t *testing.T) {
		stack := &seq.Stack[int]{}

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
		stack := &seq.Stack[int]{}

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
		stack := &seq.Stack[int]{}

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
		stack := &seq.Stack[int]{}
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
			elem := &seq.Item[string]{}
			elem.Set("hi")
			if fmt.Sprint(elem) != "hi" {
				t.Fatal(fmt.Sprint(elem))
			}
			if fmt.Sprint(elem) != fmt.Sprintf("%+v", elem) {
				t.Fatal(fmt.Sprint(elem), fmt.Sprintf("%+v", elem))
			}
		})
		t.Run("RemovingRoot", func(t *testing.T) {
			stack := &seq.Stack[int]{}
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
			stack := &seq.Stack[int]{}
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
			item := seq.NewItem(100)
			if item.Remove() {
				t.Fatal("shouldn't be able to remove detached self")
			}
		})
		t.Run("AppendDetached", func(t *testing.T) {
			item := seq.NewItem(100)
			if item != item.Append(nil) {
				t.Error("nil appends should appear as noops")
			}
			if item != item.Append(seq.NewItem(200)) {
				t.Error("can't append to detached")
			}
		})
		t.Run("AppendInSameList", func(t *testing.T) {
			stack := &seq.Stack[int]{}
			stack.Push(100)
			stack.Push(200)
			stack.Push(400)

			if stack.Head() != stack.Head().Append(stack.Head().Next()) {
				t.Error("appending items from the same list should not work")
			}
		})
		t.Run("AppendInDifferentList", func(t *testing.T) {
			stack := &seq.Stack[int]{}
			stack.Push(100)
			stack.Push(200)
			stack.Push(400)

			stackTwo := &seq.Stack[int]{}
			stackTwo.Push(800)

			if stack.Head() != stack.Head().Append(stackTwo.Head()) {
				t.Error("appending items from the same list should not work")
			}
		})
		t.Run("Removed", func(t *testing.T) {
			stack := &seq.Stack[int]{}
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
				if one.Head().Attach(&seq.Stack[int]{}) {
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
				item := &seq.Item[string]{}
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
				item := seq.NewItem(43)
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
		t.Run("Content", func(t *testing.T) {
			stack := GenerateStack(t, 100)
			items := fun.Must(itertool.CollectSlice(ctx, seq.StackValues(stack.Iterator())))
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
			fun.Invariant(iter.Close(ctx) == nil)
			if seen != stack.Len() {
				t.Fatal("did not see all values")
			}
		})
		t.Run("Destructive", func(t *testing.T) {
			stack := GenerateStack(t, 50)
			iter := stack.IteratorPop()
			seen := 0
			for iter.Next(ctx) {
				seen++
				iter.Value()
			}
			fun.Invariant(iter.Close(ctx) == nil)
			if seen != 50 {
				t.Fatal("did not see all values")
			}
			if stack.Len() != 0 {
				t.Fatal("stack should be empty", stack.Len())
			}
		})
		t.Run("EmptyStart", func(t *testing.T) {
			stack := &seq.Stack[int]{}
			iter := stack.IteratorPop()
			seen := 0
			for iter.Next(ctx) {
				seen++
				iter.Value()
			}
			fun.Invariant(iter.Close(ctx) == nil)
			if seen != 0 {
				t.Fatal("stack should be empty", stack.Len())
			}
		})
	})
}

func GenerateStack(t *testing.T, size int) *seq.Stack[int] {
	t.Helper()
	stack := &seq.Stack[int]{}
	for i := 0; i < size; i++ {
		stack.Push(1 + i + rand.Int())
	}
	if stack.Len() != size {
		t.Fatal("could not produce fixture", size, stack.Len())
	}

	return stack
}
