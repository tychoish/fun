package dt

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
)

func TestStack(t *testing.T) {
	t.Run("ExpectedPanicUnitialized", func(t *testing.T) {
		var err error
		defer func() {
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrUninitializedContainer)
		}()
		defer func() { err = recover().(error) }()

		var list *Stack[string]
		list.Push("hi")
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
			assert.True(t, !stack.Head().Remove())

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
	t.Run("Iterators", func(t *testing.T) {
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
			iter := stack.IteratorPop()
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
			iter := stack.IteratorPop()
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
			assert.True(t, nl.Head().Value() == stack.Head().Value())
			assert.True(t, nl.Head().Next().Value() == stack.Head().Next().Value())
			assert.True(t, nl.Head().Next().Next().Value() == stack.Head().Next().Next().Value())
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
		t.Run("EmptyStack", func(t *testing.T) {
			stack := &Stack[int]{}
			out, err := stack.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "[]")

			nstack := &Stack[int]{}
			err = nstack.UnmarshalJSON(out)
			check.NotError(t, err)
			check.Equal(t, nstack.Len(), 0)
		})
		t.Run("LargeStack", func(t *testing.T) {
			stack := &Stack[int]{}
			for i := 0; i < 100; i++ {
				stack.Push(i)
			}

			out, err := stack.MarshalJSON()
			check.NotError(t, err)

			nstack := &Stack[int]{}
			err = nstack.UnmarshalJSON(out)
			check.NotError(t, err)
			check.Equal(t, nstack.Len(), 100)

			// Verify all elements match
			it1 := stack.Head()
			it2 := nstack.Head()
			for i := 0; i < 100; i++ {
				if it1.Value() != it2.Value() {
					t.Errorf("element %d mismatch: %d vs %d", i, it1.Value(), it2.Value())
				}
				it1 = it1.Next()
				it2 = it2.Next()
			}
		})
		t.Run("AppendDuringUnmarshal", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(1)
			stack.Push(2)
			origLen := stack.Len()

			err := stack.UnmarshalJSON([]byte("[3,4,5]"))
			check.NotError(t, err)
			check.Equal(t, stack.Len(), origLen+3)
		})
		t.Run("MultipleRoundTrips", func(t *testing.T) {
			stack := &Stack[string]{}
			stack.Append("a", "b", "c")

			out1, err := stack.MarshalJSON()
			check.NotError(t, err)

			stack2 := &Stack[string]{}
			err = stack2.UnmarshalJSON(out1)
			check.NotError(t, err)

			out2, err := stack2.MarshalJSON()
			check.NotError(t, err)

			check.Equal(t, string(out1), string(out2))
		})
		t.Run("InvalidJSON", func(t *testing.T) {
			stack := &Stack[int]{}
			err := stack.UnmarshalJSON([]byte("{invalid}"))
			check.Error(t, err)
		})
	})
	t.Run("ItemJSON", func(t *testing.T) {
		t.Run("MarshalItem", func(t *testing.T) {
			item := NewItem(42)
			out, err := item.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "42")
		})

		t.Run("MarshalItemString", func(t *testing.T) {
			item := NewItem("hello")
			out, err := item.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), `"hello"`)
		})

		t.Run("MarshalItemZeroValue", func(t *testing.T) {
			item := NewItem(0)
			out, err := item.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "0")
		})

		t.Run("MarshalItemStruct", func(t *testing.T) {
			type testStruct struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}
			item := NewItem(testStruct{Name: "test", Value: 123})
			out, err := item.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), `{"name":"test","value":123}`)
		})

		t.Run("MarshalUninitializedItem", func(t *testing.T) {
			item := &Item[int]{}
			out, err := item.MarshalJSON()
			check.NotError(t, err)
			// Uninitialized item should marshal its zero value
			check.Equal(t, string(out), "0")
		})

		t.Run("UnmarshalIntoItem", func(t *testing.T) {
			item := &Item[int]{}
			err := item.UnmarshalJSON([]byte("42"))
			check.NotError(t, err)
			check.Equal(t, item.Value(), 42)
		})

		t.Run("UnmarshalOverwritesValue", func(t *testing.T) {
			item := NewItem(10)
			check.Equal(t, item.Value(), 10)

			err := item.UnmarshalJSON([]byte("20"))
			check.NotError(t, err)
			check.Equal(t, item.Value(), 20)
		})

		t.Run("UnmarshalString", func(t *testing.T) {
			item := &Item[string]{}
			err := item.UnmarshalJSON([]byte(`"world"`))
			check.NotError(t, err)
			check.Equal(t, item.Value(), "world")
		})

		t.Run("UnmarshalStruct", func(t *testing.T) {
			type testStruct struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}
			item := &Item[testStruct]{}
			err := item.UnmarshalJSON([]byte(`{"name":"foo","value":456}`))
			check.NotError(t, err)
			check.Equal(t, item.Value().Name, "foo")
			check.Equal(t, item.Value().Value, 456)
		})

		t.Run("UnmarshalInvalidJSON", func(t *testing.T) {
			item := &Item[int]{}
			err := item.UnmarshalJSON([]byte("not valid json"))
			check.Error(t, err)
		})

		t.Run("UnmarshalTypeMismatch", func(t *testing.T) {
			item := &Item[int]{}
			err := item.UnmarshalJSON([]byte(`"string instead of int"`))
			check.Error(t, err)
		})

		t.Run("UnmarshalNull", func(t *testing.T) {
			item := NewItem(42)
			err := item.UnmarshalJSON([]byte("null"))
			check.NotError(t, err)
			// Value should be zero value after unmarshaling null
			check.Equal(t, item.Value(), 0)
		})

		t.Run("UnmarshalNilInput", func(t *testing.T) {
			item := NewItem(0)
			err := item.UnmarshalJSON(nil)
			check.Error(t, err)
		})

		t.Run("RoundTripItem", func(t *testing.T) {
			original := NewItem(999)

			// Marshal
			data, err := original.MarshalJSON()
			check.NotError(t, err)

			// Unmarshal into new item
			restored := &Item[int]{}
			err = restored.UnmarshalJSON(data)
			check.NotError(t, err)

			check.Equal(t, original.Value(), restored.Value())
		})

		t.Run("ItemInStack", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(1)
			stack.Push(2)
			stack.Push(3)

			item := stack.Head() // Should be item with value 3

			// Marshal the item
			data, err := item.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(data), "3")

			// Unmarshal into a different item
			newItem := &Item[int]{}
			err = newItem.UnmarshalJSON(data)
			check.NotError(t, err)
			check.Equal(t, newItem.Value(), 3)
		})

		t.Run("ItemSetFails", func(t *testing.T) {
			// Test that Set returns false for root/head items
			stack := &Stack[int]{}
			stack.Push(100)

			root := stack.Head()
			// Unmarshal should call Set which might fail
			// but UnmarshalJSON itself doesn't fail
			err := root.UnmarshalJSON([]byte("200"))
			check.NotError(t, err)
		})

		t.Run("MultipleUnmarshals", func(t *testing.T) {
			item := NewItem(1)

			for i := 2; i <= 5; i++ {
				data := []byte(fmt.Sprintf("%d", i*10))
				err := item.UnmarshalJSON(data)
				check.NotError(t, err)
				check.Equal(t, item.Value(), i*10)
			}
		})

		t.Run("ArrayUnmarshal", func(t *testing.T) {
			item := &Item[[]int]{}
			err := item.UnmarshalJSON([]byte("[1,2,3,4,5]"))
			check.NotError(t, err)
			check.Equal(t, len(item.Value()), 5)
			check.Equal(t, item.Value()[0], 1)
			check.Equal(t, item.Value()[4], 5)
		})

		t.Run("NestedStructUnmarshal", func(t *testing.T) {
			type inner struct {
				X int `json:"x"`
			}
			type outer struct {
				Name  string `json:"name"`
				Inner inner  `json:"inner"`
			}

			item := &Item[outer]{}
			err := item.UnmarshalJSON([]byte(`{"name":"test","inner":{"x":42}}`))
			check.NotError(t, err)
			check.Equal(t, item.Value().Name, "test")
			check.Equal(t, item.Value().Inner.X, 42)
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
	t.Run("Reverse", func(t *testing.T) {
		t.Run("EmptyStack", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Reverse()
			if stack.Len() != 0 {
				t.Error("reversing empty stack should remain empty")
			}
		})

		t.Run("SingleElement", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(42)
			stack.Reverse()
			if stack.Head().Value() != 42 {
				t.Error("single element should remain unchanged")
			}
			if stack.Len() != 1 {
				t.Error("length should be 1")
			}
		})

		t.Run("TwoElements", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Push(1)
			stack.Push(2)
			// Stack is [2, 1]

			stack.Reverse()
			// After reverse should be [1, 2]

			if stack.Head().Value() != 1 {
				t.Errorf("head should be 1, got %d", stack.Head().Value())
			}
			if stack.Head().Next().Value() != 2 {
				t.Errorf("second should be 2, got %d", stack.Head().Next().Value())
			}
			if stack.Len() != 2 {
				t.Error("length should be 2")
			}
		})

		t.Run("MultipleElements", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Append(400, 300, 42)
			// Stack is [42, 300, 400]

			original := irt.Collect(stack.Iterator())
			if len(original) != 3 || original[0] != 42 || original[1] != 300 || original[2] != 400 {
				t.Errorf("expected [42, 300, 400], got %v", original)
			}

			stack.Reverse()
			// After reverse should be [400, 300, 42]

			reversed := irt.Collect(stack.Iterator())
			if len(reversed) != 3 || reversed[0] != 400 || reversed[1] != 300 || reversed[2] != 42 {
				t.Errorf("expected [400, 300, 42], got %v", reversed)
			}
			if stack.Len() != 3 {
				t.Errorf("length should be 3, got %d", stack.Len())
			}
		})

		t.Run("DoubleReverse", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Append(5, 4, 3, 2, 1)

			original := irt.Collect(stack.Iterator())

			stack.Reverse()
			stack.Reverse()

			afterDouble := irt.Collect(stack.Iterator())
			if len(original) != len(afterDouble) {
				t.Fatalf("length mismatch: %d vs %d", len(original), len(afterDouble))
			}
			for i := range original {
				if original[i] != afterDouble[i] {
					t.Errorf("mismatch at index %d: %d vs %d", i, original[i], afterDouble[i])
				}
			}
		})

		t.Run("LargeStack", func(t *testing.T) {
			stack := &Stack[int]{}
			size := 100
			for i := 0; i < size; i++ {
				stack.Push(i)
			}

			original := irt.Collect(stack.Iterator())
			stack.Reverse()
			reversed := irt.Collect(stack.Iterator())

			// Check that reversed is actually reversed
			for i := 0; i < size; i++ {
				if original[i] != reversed[size-1-i] {
					t.Errorf("reversal failed at index %d", i)
					break
				}
			}

			assert.Equal(t, stack.Len(), size)
		})

		t.Run("PreservesStackReferences", func(t *testing.T) {
			stack := &Stack[int]{}
			stack.Append(3, 2, 1)

			// Get reference to items before reversal
			oldHead := stack.Head()

			stack.Reverse()

			// After reversal, old head should still be in the stack
			// but at a different position
			found := false
			for item := stack.Head(); item.Ok(); item = item.Next() {
				if item == oldHead {
					found = true
					break
				}
			}

			if !found {
				t.Error("item references should be preserved")
			}
		})
	})
}

func TestStackInternal(t *testing.T) {
	t.Run("RemoveInBrokenList", func(t *testing.T) {
		stack := &Stack[int]{}
		stack.Push(1)
		stack.Push(12)
		old := stack.Head()
		old.next = &Item[int]{stack: stack, next: &Item[int]{stack: stack}}
		stack.head = &Item[int]{stack: stack, ok: true}
		stack.Push(121)
		if old.Remove() {
			t.Fatal("should not let me do this")
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
