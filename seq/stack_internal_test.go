package seq

import (
	"runtime"
	"testing"
	"time"
)

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
	t.Run("Pool ", func(t *testing.T) {
		pool := getItemPool[string]("")
		elems := []*Item[string]{}
		for i := 0; i < 10000; i++ {
			elems = append(elems, makeItem("hi"))
		}
		elems = nil
		time.Sleep(time.Millisecond)

		runtime.GC() // kick the finalizers

		time.Sleep(time.Millisecond)

		e := pool.Get().(*Item[string])
		// ensure that the finalizer resets the value,
		// conversely. we have to keep an eye on the coverage
		// here.
		if e.value == "hi" {
			t.Error("unlucky", e.value)
		}
	})

}
