package adt

import (
	"cmp"
	"encoding/json"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
)

func TestOrderedSet(t *testing.T) {
	t.Run("EmptyIteration", func(t *testing.T) {
		set := &OrderedSet[string]{}
		ct := 0
		assert.NotPanic(t, func() {
			for range set.Iterator() {
				ct++
			}
		})
		assert.Zero(t, ct)
	})

	t.Run("Initialization", func(t *testing.T) {
		set := &OrderedSet[string]{}
		if set.Len() != 0 {
			t.Fatal("initialized non-empty set")
		}
	})

	t.Run("InsertionOrder", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("xyz")
		set.Add("lmn")
		set.Add("abc")

		sl := irt.Collect(set.Iterator())

		check.Equal(t, "xyz", sl[0])
		check.Equal(t, "lmn", sl[1])
		check.Equal(t, "abc", sl[2])
	})

	t.Run("Extend", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Extend(irt.Args("xyz", "lmn", "abc"))

		sl := irt.Collect(set.Iterator())

		check.Equal(t, "xyz", sl[0])
		check.Equal(t, "lmn", sl[1])
		check.Equal(t, "abc", sl[2])
	})

	t.Run("JSON", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("hello")
		set.Add("buddy")
		set.Add("hello")
		set.Add("kip")
		check.Equal(t, set.Len(), 3) // hello is a dupe

		data, err := json.Marshal(set)
		check.NotError(t, err)

		// Should preserve insertion order
		expected := `["hello","buddy","kip"]`
		check.Equal(t, expected, string(data))

		nset := &OrderedSet[string]{}
		assert.NotError(t, nset.UnmarshalJSON(data))
	})

	t.Run("Recall", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("abc")
		if set.Len() != 1 {
			t.Error("should have added one")
		}
		if !set.Check("abc") {
			t.Error("should recall item")
		}
		set.Add("abc")
		if set.Len() != 1 {
			t.Error("should have only added key once")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("abc")

		if !set.Check("abc") {
			t.Error("should recall item")
		}

		set.Delete("abc")

		if set.Check("abc") {
			t.Error("should have deleted item")
		}
	})

	t.Run("DeletePreservesOrder", func(t *testing.T) {
		set := &OrderedSet[int]{}
		set.Add(1)
		set.Add(2)
		set.Add(3)
		set.Add(4)

		set.Delete(2)

		sl := irt.Collect(set.Iterator())
		check.Equal(t, 1, sl[0])
		check.Equal(t, 3, sl[1])
		check.Equal(t, 4, sl[2])
	})

	t.Run("DeleteCheck", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("abc")

		if !set.Delete("abc") {
			t.Error("set item should have been present")
		}

		if set.Delete("abc") {
			t.Error("set item should not have been present")
		}
	})

	t.Run("AddCheck", func(t *testing.T) {
		set := &OrderedSet[string]{}

		if set.Add("abc") {
			t.Error("set item should not have been present")
		}
		if !set.Add("abc") {
			t.Error("set item should have been present")
		}
	})

	t.Run("SortQuick", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("xyz")
		set.Add("lmn")
		set.Add("abc")
		set.Add("opq")
		set.Add("def")
		set.Add("ghi")

		sl := irt.Collect(set.Iterator())

		if sl[0] == "abc" && sl[1] == "def" && sl[2] == "ghi" {
			t.Fatal("should not be sorted initially")
		}

		set.SortQuick(cmp.Compare)

		sorted := irt.Collect(set.Iterator())

		check.Equal(t, "abc", sorted[0])
		check.Equal(t, "def", sorted[1])
		check.Equal(t, "ghi", sorted[2])
		check.Equal(t, "lmn", sorted[3])
		check.Equal(t, "opq", sorted[4])
		check.Equal(t, "xyz", sorted[5])
	})

	t.Run("SortMerge", func(t *testing.T) {
		set := &OrderedSet[string]{}
		set.Add("xyz")
		set.Add("lmn")
		set.Add("abc")
		set.Add("opq")
		set.Add("def")
		set.Add("ghi")

		sl := irt.Collect(set.Iterator())

		if sl[0] == "abc" && sl[1] == "def" && sl[2] == "ghi" {
			t.Fatal("should not be sorted initially")
		}

		set.SortMerge(cmp.Compare)

		sorted := irt.Collect(set.Iterator())

		check.Equal(t, "abc", sorted[0])
		check.Equal(t, "def", sorted[1])
		check.Equal(t, "ghi", sorted[2])
		check.Equal(t, "lmn", sorted[3])
		check.Equal(t, "opq", sorted[4])
		check.Equal(t, "xyz", sorted[5])
	})

	t.Run("JSONCheck", func(t *testing.T) {
		set := &OrderedSet[int]{}
		err := json.Unmarshal([]byte("[1,2,3]"), &set)
		check.NotError(t, err)
		check.Equal(t, 3, set.Len())
	})
}

func BenchmarkOrderedSet(b *testing.B) {
	const size = 10000
	b.Run("Append", func(b *testing.B) {
		operation := func(set *OrderedSet[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * i)
			}
		}
		set := &OrderedSet[int]{}
		for j := 0; j < b.N; j++ {
			operation(set)
		}
	})

	b.Run("Mixed", func(b *testing.B) {
		operation := func(set *OrderedSet[int]) {
			var last int
			for i := 0; i < size; i++ {
				val := i * i * size
				set.Add(val)
				if i%3 == 0 {
					set.Delete(last)
				}
				last = val
			}
		}
		set := &OrderedSet[int]{}
		for j := 0; j < b.N; j++ {
			operation(set)
		}
	})

	b.Run("Deletion", func(b *testing.B) {
		operation := func(set *OrderedSet[int]) {
			for i := 0; i < size; i++ {
				set.Add(i)
				set.Add(i * size)
			}
			for i := 0; i < size; i++ {
				set.Delete(i)
				set.Delete(i + 1)
				if i%3 == 0 {
					set.Delete(i * size)
				}
			}
		}
		set := &OrderedSet[int]{}
		for j := 0; j < b.N; j++ {
			operation(set)
		}
	})

	b.Run("Iteration", func(b *testing.B) {
		operation := func(set *OrderedSet[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * size)
			}
			count := 0
			for range set.Iterator() {
				count++
			}
			check.Equal(b, size, count)
		}
		set := &OrderedSet[int]{}
		for j := 0; j < b.N; j++ {
			operation(set)
		}
	})
}
