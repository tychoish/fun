package dt

import (
	"slices"
	"sync"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
)

func TestOrderedMap(t *testing.T) {
	t.Run("InsertionOrder", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		// Add items in a specific order
		m.Set("first", 1)
		m.Set("second", 2)
		m.Set("third", 3)
		m.Set("fourth", 4)

		// Verify iteration respects insertion order
		keys := irt.Collect(m.Keys(), 0, 4)
		check.True(t, slices.Equal(keys, []string{"first", "second", "third", "fourth"}))

		values := irt.Collect(m.Values(), 0, 4)
		check.True(t, slices.Equal(values, []int{1, 2, 3, 4}))
	})

	t.Run("UpdatePreservesOrder", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("a", 1)
		m.Set("b", 2)
		m.Set("c", 3)

		// Update middle element
		m.Set("b", 20)

		// Order should be unchanged
		keys := irt.Collect(m.Keys(), 0, 3)
		check.True(t, slices.Equal(keys, []string{"a", "b", "c"}))

		// But value should be updated
		check.Equal(t, m.Get("b"), 20)
	})

	t.Run("StorePreservesOrder", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Store("x", 10)
		m.Store("y", 20)
		m.Store("z", 30)

		// Update using Store
		m.Store("y", 200)

		keys := irt.Collect(m.Keys(), 0, 3)
		check.True(t, slices.Equal(keys, []string{"x", "y", "z"}))
		check.Equal(t, m.Get("y"), 200)
	})

	t.Run("DeletePreservesOrder", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("one", 1)
		m.Set("two", 2)
		m.Set("three", 3)
		m.Set("four", 4)
		m.Set("five", 5)

		// Delete middle element
		m.Delete("three")

		keys := irt.Collect(m.Keys(), 0, 4)
		check.True(t, slices.Equal(keys, []string{"one", "two", "four", "five"}))
		check.Equal(t, m.Len(), 4)
	})

	t.Run("Check", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		check.True(t, !m.Check("missing"))

		m.Set("exists", 42)
		check.True(t, m.Check("exists"))
		check.True(t, !m.Check("missing"))
	})

	t.Run("Get", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		// Get on empty map returns zero value
		check.Equal(t, m.Get("missing"), 0)

		m.Set("key", 99)
		check.Equal(t, m.Get("key"), 99)
		check.Equal(t, m.Get("missing"), 0)
	})

	t.Run("Load", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		val, ok := m.Load("missing")
		check.True(t, !ok)
		check.Equal(t, val, 0)

		m.Set("present", 123)
		val, ok = m.Load("present")
		check.True(t, ok)
		check.Equal(t, val, 123)
	})

	t.Run("SetDefault", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Ensure("key1")
		check.Equal(t, m.Get("key1"), 0) // zero value for int

		m.Set("key2", 42)
		check.Equal(t, m.Get("key2"), 42)
		m.Ensure("key2") // Should not modify existing
		check.Equal(t, m.Get("key2"), 42)

		keys := irt.Collect(m.Keys(), 0, 2)
		check.True(t, slices.Equal(keys, []string{"key1", "key2"}))
	})

	t.Run("Extend", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		// Create a sequence of key-value pairs
		pairs := map[string]int{
			"alpha": 1,
			"beta":  2,
			"gamma": 3,
		}

		m.Extend(irt.Map(pairs))

		check.Equal(t, m.Len(), 3)
		check.True(t, m.Check("alpha"))
		check.True(t, m.Check("beta"))
		check.True(t, m.Check("gamma"))
	})

	t.Run("Iterator", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("a", 10)
		m.Set("b", 20)
		m.Set("c", 30)

		var keys []string
		var values []int

		for k, v := range m.Iterator() {
			keys = append(keys, k)
			values = append(values, v)
		}

		check.True(t, slices.Equal(keys, []string{"a", "b", "c"}))
		check.True(t, slices.Equal(values, []int{10, 20, 30}))
	})

	t.Run("IteratorEarlyExit", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("first", 1)
		m.Set("second", 2)
		m.Set("third", 3)

		count := 0
		for range m.Iterator() {
			count++
			if count == 2 {
				break
			}
		}

		check.Equal(t, count, 2)
	})

	t.Run("Len", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		check.Equal(t, m.Len(), 0)

		m.Set("one", 1)
		check.Equal(t, m.Len(), 1)

		m.Set("two", 2)
		check.Equal(t, m.Len(), 2)

		m.Delete("one")
		check.Equal(t, m.Len(), 1)

		m.Delete("two")
		check.Equal(t, m.Len(), 0)
	})

	t.Run("EmptyMap", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		check.Equal(t, m.Len(), 0)
		check.True(t, !m.Check("anything"))

		keys := irt.Collect(m.Keys(), 0, 0)
		check.Equal(t, len(keys), 0)

		values := irt.Collect(m.Values(), 0, 0)
		check.Equal(t, len(values), 0)
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("exists", 1)

		// Delete non-existent key should not panic
		m.Delete("missing")

		check.Equal(t, m.Len(), 1)
		check.True(t, m.Check("exists"))
	})

	t.Run("LargeMap", func(t *testing.T) {
		m := &OrderedMap[int, string]{}

		// Add many items
		for i := 0; i < 1000; i++ {
			m.Set(i, string(rune('a'+i%26)))
		}

		check.Equal(t, m.Len(), 1000)

		// Verify order is maintained
		keys := irt.Collect(m.Keys(), 0, 1000)
		for i := 0; i < 1000; i++ {
			check.Equal(t, keys[i], i)
		}
	})

	t.Run("Concurrent", func(t *testing.T) {
		m := &OrderedMap[int, int]{}

		// Pre-populate
		for i := 0; i < 100; i++ {
			m.Set(i, i*10)
		}

		var wg sync.WaitGroup

		// Concurrent readers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_ = m.Get(j)
					_ = m.Check(j)
					_, _ = m.Load(j)
				}
			}()
		}

		// Concurrent writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(offset int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					key := 1000 + offset*100 + j
					m.Set(key, key)
				}
			}(i)
		}

		// Concurrent iterators
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range m.Iterator() {
					// Just iterate
				}
			}()
		}

		wg.Wait()

		// Verify map is still consistent
		check.True(t, m.Len() >= 100) // At least original items
	})

	t.Run("IteratorDuringModification", func(t *testing.T) {
		m := &OrderedMap[int, int]{}

		for i := 0; i < 10; i++ {
			m.Set(i, i)
		}

		// Iterator should not panic even if map is modified during iteration
		// (though we can't guarantee what elements will be seen)
		count := 0
		for k := range m.Keys() {
			count++
			if count%2 == 0 {
				// Delete some elements during iteration
				m.Delete(k + 100) // Delete non-existent key
			}
			if count > 20 {
				break // Prevent infinite loop in case of issues
			}
		}
	})

	t.Run("ValuesMatchKeys", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("a", 1)
		m.Set("b", 2)
		m.Set("c", 3)

		keys := irt.Collect(m.Keys(), 0, 3)

		var valuesFromIterator []int
		for _, v := range m.Iterator() {
			valuesFromIterator = append(valuesFromIterator, v)
		}

		irt.Equal(m.Values(), irt.Slice(valuesFromIterator))
		check.Equal(t, len(keys), len(valuesFromIterator))
	})
}

func TestOrderedMapComplexTypes(t *testing.T) {
	t.Run("StructValues", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		m := &OrderedMap[string, Person]{}

		m.Set("alice", Person{"Alice", 30})
		m.Set("bob", Person{"Bob", 25})
		m.Set("charlie", Person{"Charlie", 35})

		bob, ok := m.Load("bob")
		check.True(t, ok)
		check.Equal(t, bob.Name, "Bob")
		check.Equal(t, bob.Age, 25)

		keys := irt.Collect(m.Keys(), 0, 3)
		check.True(t, slices.Equal(keys, []string{"alice", "bob", "charlie"}))
	})

	t.Run("PointerValues", func(t *testing.T) {
		m := &OrderedMap[int, *string]{}

		s1 := "first"
		s2 := "second"

		m.Set(1, &s1)
		m.Set(2, &s2)

		val, ok := m.Load(1)
		check.True(t, ok)
		check.Equal(t, *val, "first")

		// Update through pointer
		*val = "modified"
		val2, _ := m.Load(1)
		check.Equal(t, *val2, "modified")
	})

	t.Run("SliceValues", func(t *testing.T) {
		m := &OrderedMap[string, []int]{}

		m.Set("primes", []int{2, 3, 5, 7})
		m.Set("evens", []int{2, 4, 6, 8})

		primes := m.Get("primes")
		check.True(t, slices.Equal(primes, []int{2, 3, 5, 7}))
	})
}

func TestOrderedMapEdgeCases(t *testing.T) {
	t.Run("SingleElement", func(t *testing.T) {
		m := &OrderedMap[string, int]{}
		m.Set("only", 42)

		check.Equal(t, m.Len(), 1)
		check.Equal(t, m.Get("only"), 42)

		keys := irt.Collect(m.Keys(), 0, 1)
		check.True(t, slices.Equal(keys, []string{"only"}))

		m.Delete("only")
		check.Equal(t, m.Len(), 0)
	})

	t.Run("AddDeleteAdd", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("key", 1)
		m.Delete("key")
		m.Set("key", 2)

		check.Equal(t, m.Get("key"), 2)
		check.Equal(t, m.Len(), 1)
	})

	t.Run("MultipleDeletes", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("a", 1)
		m.Set("b", 2)
		m.Set("c", 3)
		m.Set("d", 4)

		m.Delete("b")
		m.Delete("d")

		keys := irt.Collect(m.Keys(), 0, 2)
		check.True(t, slices.Equal(keys, []string{"a", "c"}))
	})

	t.Run("ZeroValues", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		m.Set("zero", 0)

		check.True(t, m.Check("zero"))
		check.Equal(t, m.Get("zero"), 0)

		val, ok := m.Load("zero")
		check.True(t, ok)
		check.Equal(t, val, 0)
	})
}

func BenchmarkOrderedMap(b *testing.B) {
	b.Run("Add", func(b *testing.B) {
		m := &OrderedMap[int, int]{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m.Set(i, i*2)
		}
	})

	b.Run("Get", func(b *testing.B) {
		m := &OrderedMap[int, int]{}
		for i := 0; i < 1000; i++ {
			m.Set(i, i*2)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = m.Get(i % 1000)
		}
	})

	b.Run("Iterator", func(b *testing.B) {
		m := &OrderedMap[int, int]{}
		for i := 0; i < 1000; i++ {
			m.Set(i, i*2)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for range m.Iterator() {
			}
		}
	})

	b.Run("Delete", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			m := &OrderedMap[int, int]{}
			for j := 0; j < 1000; j++ {
				m.Set(j, j*2)
			}
			b.StartTimer()

			for j := 0; j < 1000; j++ {
				m.Delete(j)
			}
		}
	})
}
