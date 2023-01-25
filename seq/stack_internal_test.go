package seq

import "testing"

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
