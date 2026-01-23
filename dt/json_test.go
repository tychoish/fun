package dt

import (
	"cmp"
	"testing"

	"github.com/tychoish/fun/irt"
)

func TestOrderedMapMarshalUnmarshal(t *testing.T) {
	t.Run("EmptyMap", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, int]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Len() != 0 {
			t.Errorf("UnmarshalJSON() length = %d, want 0", m2.Len())
		}
	})

	t.Run("SimpleMap", func(t *testing.T) {
		m := &OrderedMap[string, int]{}
		m.Set("one", 1)
		m.Set("two", 2)
		m.Set("three", 3)

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, int]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Len() != 3 {
			t.Errorf("UnmarshalJSON() length = %d, want 3", m2.Len())
		}

		if m2.Get("one") != 1 {
			t.Errorf("UnmarshalJSON() Get(\"one\") = %d, want 1", m2.Get("one"))
		}
		if m2.Get("two") != 2 {
			t.Errorf("UnmarshalJSON() Get(\"two\") = %d, want 2", m2.Get("two"))
		}
		if m2.Get("three") != 3 {
			t.Errorf("UnmarshalJSON() Get(\"three\") = %d, want 3", m2.Get("three"))
		}
	})

	t.Run("OrderPreserved", func(t *testing.T) {
		m := &OrderedMap[string, string]{}
		m.Set("first", "1")
		m.Set("second", "2")
		m.Set("third", "3")

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, string]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		// Check order is preserved
		keys := irt.Collect(irt.First(m2.Iterator()))
		expected := []string{"first", "second", "third"}

		if len(keys) != len(expected) {
			t.Fatalf("Iterator() length = %d, want %d", len(keys), len(expected))
		}

		for i, key := range keys {
			if key != expected[i] {
				t.Errorf("Iterator()[%d] = %q, want %q", i, key, expected[i])
			}
		}
	})

	t.Run("ComplexValues", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		m := &OrderedMap[string, Person]{}
		m.Set("alice", Person{Name: "Alice", Age: 30})
		m.Set("bob", Person{Name: "Bob", Age: 25})

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, Person]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Len() != 2 {
			t.Errorf("UnmarshalJSON() length = %d, want 2", m2.Len())
		}

		alice := m2.Get("alice")
		if alice.Name != "Alice" || alice.Age != 30 {
			t.Errorf("UnmarshalJSON() Get(\"alice\") = %+v, want {Name:Alice Age:30}", alice)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		var m OrderedMap[string, int]
		err := m.UnmarshalJSON([]byte(`{"invalid"`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for invalid JSON, got nil")
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		var m OrderedMap[string, int]
		err := m.UnmarshalJSON([]byte(`{"key":"not an int"}`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for type mismatch, got nil")
		}
	})

	t.Run("NotAnObject", func(t *testing.T) {
		var m OrderedMap[string, int]
		err := m.UnmarshalJSON([]byte(`[1,2,3]`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for array instead of object, got nil")
		}
	})
}

func TestHeapMarshalUnmarshal(t *testing.T) {
	t.Run("EmptyHeap", func(t *testing.T) {
		h := &Heap[int]{CF: cmp.Compare[int]}

		data, err := h.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		h2 := &Heap[int]{CF: cmp.Compare[int]}
		if err := h2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if h2.Len() != 0 {
			t.Errorf("UnmarshalJSON() length = %d, want 0", h2.Len())
		}
	})

	t.Run("IntHeap", func(t *testing.T) {
		h := &Heap[int]{CF: cmp.Compare[int]}
		h.Push(5)
		h.Push(2)
		h.Push(8)
		h.Push(1)
		h.Push(9)

		data, err := h.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		h2 := &Heap[int]{CF: cmp.Compare[int]}
		if err := h2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if h2.Len() != 5 {
			t.Errorf("UnmarshalJSON() length = %d, want 5", h2.Len())
		}

		// Pop all elements and verify they come out in sorted order
		var results []int
		for h2.Len() > 0 {
			v, ok := h2.Pop()
			if !ok {
				t.Fatal("Pop() returned false")
			}
			results = append(results, v)
		}

		expected := []int{1, 2, 5, 8, 9}
		if len(results) != len(expected) {
			t.Fatalf("Heap produced %d elements, want %d", len(results), len(expected))
		}

		for i, v := range results {
			if v != expected[i] {
				t.Errorf("Heap[%d] = %d, want %d", i, v, expected[i])
			}
		}
	})

	t.Run("StringHeap", func(t *testing.T) {
		h := &Heap[string]{CF: cmp.Compare[string]}
		h.Push("cherry")
		h.Push("apple")
		h.Push("banana")

		data, err := h.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		h2 := &Heap[string]{CF: cmp.Compare[string]}
		if err := h2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if h2.Len() != 3 {
			t.Errorf("UnmarshalJSON() length = %d, want 3", h2.Len())
		}

		// First element should be "apple" (smallest)
		v, ok := h2.Pop()
		if !ok || v != "apple" {
			t.Errorf("Pop() = %q, want \"apple\"", v)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		h := &Heap[int]{CF: cmp.Compare[int]}
		err := h.UnmarshalJSON([]byte(`[1,2,`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for invalid JSON, got nil")
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		h := &Heap[int]{CF: cmp.Compare[int]}
		err := h.UnmarshalJSON([]byte(`["string"]`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for type mismatch, got nil")
		}
	})

	t.Run("NotAnArray", func(t *testing.T) {
		h := &Heap[int]{CF: cmp.Compare[int]}
		err := h.UnmarshalJSON([]byte(`{"key":"value"}`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for object instead of array, got nil")
		}
	})

	t.Run("UninitializedCF", func(t *testing.T) {
		h := &Heap[int]{}
		data := []byte(`[1,2,3]`)

		defer func() {
			if r := recover(); r == nil {
				t.Error("UnmarshalJSON() expected panic for uninitialized CF, got none")
			}
		}()

		h.UnmarshalJSON(data)
	})
}

func TestRingMarshalUnmarshal(t *testing.T) {
	t.Run("EmptyRing", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(5)

		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		r2 := &Ring[int]{}
		r2.Setup(5)
		if err := r2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if r2.Len() != 0 {
			t.Errorf("UnmarshalJSON() length = %d, want 0", r2.Len())
		}
	})

	t.Run("PartiallyFilledRing", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(10)
		r.Push(1)
		r.Push(2)
		r.Push(3)

		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		r2 := &Ring[int]{}
		r2.Setup(10)
		if err := r2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if r2.Len() != 3 {
			t.Errorf("UnmarshalJSON() length = %d, want 3", r2.Len())
		}

		// Verify elements in FIFO order
		expected := []int{1, 2, 3}
		results := make([]int, 0, 3)
		for v := range r2.FIFO() {
			results = append(results, v)
		}

		if len(results) != len(expected) {
			t.Fatalf("FIFO produced %d elements, want %d", len(results), len(expected))
		}

		for i, v := range results {
			if v != expected[i] {
				t.Errorf("FIFO[%d] = %d, want %d", i, v, expected[i])
			}
		}
	})

	t.Run("FullRing", func(t *testing.T) {
		r := &Ring[string]{}
		r.Setup(3)
		r.Push("a")
		r.Push("b")
		r.Push("c")

		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		r2 := &Ring[string]{}
		r2.Setup(5) // Different size is ok
		if err := r2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if r2.Len() != 3 {
			t.Errorf("UnmarshalJSON() length = %d, want 3", r2.Len())
		}
	})

	t.Run("WrappedRing", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(3)
		// Push more than capacity to wrap around
		r.Push(1)
		r.Push(2)
		r.Push(3)
		r.Push(4) // Overwrites 1
		r.Push(5) // Overwrites 2

		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		r2 := &Ring[int]{}
		r2.Setup(3)
		if err := r2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		// Should have the last 3 elements: 3, 4, 5
		expected := []int{3, 4, 5}
		results := make([]int, 0, 3)
		for v := range r2.FIFO() {
			results = append(results, v)
		}

		if len(results) != len(expected) {
			t.Fatalf("FIFO produced %d elements, want %d", len(results), len(expected))
		}

		for i, v := range results {
			if v != expected[i] {
				t.Errorf("FIFO[%d] = %d, want %d", i, v, expected[i])
			}
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(5)
		err := r.UnmarshalJSON([]byte(`[1,2`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for invalid JSON, got nil")
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(5)
		err := r.UnmarshalJSON([]byte(`["string"]`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for type mismatch, got nil")
		}
	})

	t.Run("NotAnArray", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(5)
		err := r.UnmarshalJSON([]byte(`{"key":"value"}`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for object instead of array, got nil")
		}
	})

	t.Run("RingOverflow", func(t *testing.T) {
		// Test that unmarshaling more items than ring capacity works
		r := &Ring[int]{}
		r.Setup(2) // Small ring

		// Unmarshal 5 items - should only keep the last 2
		err := r.UnmarshalJSON([]byte(`[1,2,3,4,5]`))
		if err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if r.Len() != 2 {
			t.Errorf("UnmarshalJSON() length = %d, want 2", r.Len())
		}

		// Should have the last 2 elements: 4, 5
		expected := []int{4, 5}
		results := make([]int, 0, 3)
		for v := range r.FIFO() {
			results = append(results, v)
		}

		if len(results) != len(expected) {
			t.Fatalf("FIFO produced %d elements, want %d", len(results), len(expected))
		}

		for i, v := range results {
			if v != expected[i] {
				t.Errorf("FIFO[%d] = %d, want %d", i, v, expected[i])
			}
		}
	})

	t.Run("RingRoundTrip", func(t *testing.T) {
		r := &Ring[int]{}
		r.Setup(5)
		r.Push(10)
		r.Push(20)
		r.Push(30)

		// Marshal
		data, err := r.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		// Unmarshal
		r2 := &Ring[int]{}
		r2.Setup(5)
		if err := r2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		// Verify same content
		if r.Len() != r2.Len() {
			t.Errorf("Round trip length mismatch: %d vs %d", r.Len(), r2.Len())
		}

		original := irt.Collect(r.FIFO())
		restored := irt.Collect(r2.FIFO())

		if len(original) != len(restored) {
			t.Fatalf("Round trip element count: %d vs %d", len(original), len(restored))
		}

		for i := range original {
			if original[i] != restored[i] {
				t.Errorf("Round trip[%d]: %d vs %d", i, original[i], restored[i])
			}
		}
	})
}
