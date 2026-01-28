package dt

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
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

	t.Run("JSON", func(t *testing.T) {
		t.Run("RoundTrip", func(t *testing.T) {
			set := &OrderedSet[string]{}
			set.Add("hello")
			set.Add("buddy")
			set.Add("hello") // duplicate
			set.Add("kip")
			check.Equal(t, set.Len(), 3) // hello is a dupe

			data, err := json.Marshal(set)
			check.NotError(t, err)

			// Should preserve insertion order
			expected := `["hello","buddy","kip"]`
			check.Equal(t, expected, string(data))

			nset := &OrderedSet[string]{}
			assert.NotError(t, nset.UnmarshalJSON(data))
			check.True(t, set.Equal(nset))
		})
		t.Run("EmptySet", func(t *testing.T) {
			set := &OrderedSet[int]{}
			data, err := set.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(data), "[]")

			nset := &OrderedSet[int]{}
			err = nset.UnmarshalJSON(data)
			check.NotError(t, err)
			check.Equal(t, nset.Len(), 0)
		})
		t.Run("DuplicatesInJSON", func(t *testing.T) {
			set := &OrderedSet[int]{}
			err := set.UnmarshalJSON([]byte("[1,2,3,2,1]"))
			check.NotError(t, err)
			// Should only have unique values
			check.Equal(t, set.Len(), 3)
			check.True(t, set.Check(1))
			check.True(t, set.Check(2))
			check.True(t, set.Check(3))
		})
		t.Run("LargeSet", func(t *testing.T) {
			set := &OrderedSet[int]{}
			for i := 0; i < 100; i++ {
				set.Add(i)
			}

			data, err := set.MarshalJSON()
			check.NotError(t, err)

			nset := &OrderedSet[int]{}
			err = nset.UnmarshalJSON(data)
			check.NotError(t, err)
			check.Equal(t, nset.Len(), 100)
			check.True(t, set.Equal(nset))
		})
		t.Run("AppendDuringUnmarshal", func(t *testing.T) {
			set := &OrderedSet[int]{}
			set.Add(1)
			set.Add(2)

			err := set.UnmarshalJSON([]byte("[3,4,5]"))
			check.NotError(t, err)
			check.Equal(t, set.Len(), 5)

			// Verify all values present
			for i := 1; i <= 5; i++ {
				if !set.Check(i) {
					t.Errorf("set missing value %d", i)
				}
			}
		})
		t.Run("MultipleRoundTrips", func(t *testing.T) {
			set := &OrderedSet[int]{}
			set.Add(10)
			set.Add(20)
			set.Add(30)

			data1, err := set.MarshalJSON()
			check.NotError(t, err)

			set2 := &OrderedSet[int]{}
			err = set2.UnmarshalJSON(data1)
			check.NotError(t, err)

			data2, err := set2.MarshalJSON()
			check.NotError(t, err)

			check.Equal(t, string(data1), string(data2))
		})
		t.Run("TypeMismatch", func(t *testing.T) {
			set := &OrderedSet[int]{}
			set.Add(1)
			set.Add(2)

			data, err := set.MarshalJSON()
			check.NotError(t, err)

			sset := &OrderedSet[string]{}
			err = sset.UnmarshalJSON(data)
			check.Error(t, err)
		})
		t.Run("InvalidJSON", func(t *testing.T) {
			set := &OrderedSet[int]{}
			err := set.UnmarshalJSON([]byte("{invalid}"))
			check.Error(t, err)
		})
		t.Run("NilInput", func(t *testing.T) {
			set := &OrderedSet[int]{}
			err := set.UnmarshalJSON(nil)
			check.Error(t, err)
		})
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

	for populatorName, populator := range map[string]func(*OrderedSet[string]){
		"Three": func(set *OrderedSet[string]) {
			set.Add("a")
			set.Add("b")
			set.Add("c")
		},
		"Numbers": func(set *OrderedSet[string]) {
			for i := 0; i < 100; i++ {
				set.Add(fmt.Sprint(i))
			}
		},
	} {
		t.Run(populatorName, func(t *testing.T) {
			t.Run("Uniqueness", func(t *testing.T) {
				set := &OrderedSet[string]{}
				populator(set)

				if set.Len() == 0 {
					t.Fatal("populator did not work")
				}

				size := set.Len()
				populator(set)
				if set.Len() != size {
					t.Fatal("size should not change", size, set.Len())
				}
			})
			t.Run("Equality", func(t *testing.T) {
				set := &OrderedSet[string]{}
				populator(set)

				set2 := &OrderedSet[string]{}
				populator(set2)
				if !set.Equal(set2) {
					t.Fatal("sets should be equal")
				}
			})
		})
	}

	t.Run("JSONCheck", func(t *testing.T) {
		set := &OrderedSet[int]{}
		err := json.Unmarshal([]byte("[1,2,3]"), &set)
		check.NotError(t, err)
		check.Equal(t, 3, set.Len())

		// Verify order is preserved
		sl := irt.Collect(set.Iterator())
		check.Equal(t, 1, sl[0])
		check.Equal(t, 2, sl[1])
		check.Equal(t, 3, sl[2])
	})

	t.Run("List", func(t *testing.T) {
		t.Run("Ordered", func(t *testing.T) {
			ls := &List[int]{}

			st := &OrderedSet[int]{}

			for v := range 42 {
				ls.PushBack(v)
				st.Add(v)
			}

			assert.Equal(t, ls.Len(), 42)
			assert.Equal(t, st.Len(), ls.Len())

			setls := &List[int]{}
			setls.Extend(st.Iterator())

			count := 0
			for elemL, elemS := ls.Front(), setls.Front(); elemL.Ok() && elemS.Ok(); elemL, elemS = elemL.Next(), elemS.Next() {
				count++
				check.Equal(t, elemL.Value(), elemS.Value())
			}
			assert.Equal(t, count, 42)
		})
	})

	t.Run("Slice", func(t *testing.T) {
		t.Run("Ordered", func(t *testing.T) {
			in := []int{1, 2, 3, 4, 5, 6}
			set := &OrderedSet[int]{}
			set.Extend(irt.Slice(in))
			assert.Equal(t, set.Len(), 6)
			assert.True(t, slices.Equal(in, irt.Collect(set.Iterator())))
		})
	})

	t.Run("Equal", func(t *testing.T) {
		t.Run("BothEmpty", func(t *testing.T) {
			set1 := &OrderedSet[int]{}
			set2 := &OrderedSet[int]{}
			assert.True(t, set1.Equal(set2))
			assert.True(t, set2.Equal(set1))
		})

		t.Run("Reflexive", func(t *testing.T) {
			set := &OrderedSet[int]{}
			set.Add(1)
			set.Add(2)
			set.Add(3)
			assert.True(t, set.Equal(set))
		})

		t.Run("DifferentLengths", func(t *testing.T) {
			set1 := &OrderedSet[int]{}
			set1.Add(1)
			set1.Add(2)

			set2 := &OrderedSet[int]{}
			set2.Add(1)
			set2.Add(2)
			set2.Add(3)

			assert.True(t, !set1.Equal(set2))
			assert.True(t, !set2.Equal(set1))
		})

		t.Run("SameItemsSameOrder", func(t *testing.T) {
			set1 := &OrderedSet[string]{}
			set1.Add("a")
			set1.Add("b")
			set1.Add("c")

			set2 := &OrderedSet[string]{}
			set2.Add("a")
			set2.Add("b")
			set2.Add("c")

			assert.True(t, set1.Equal(set2))
			assert.True(t, set2.Equal(set1))
		})

		t.Run("SameItemsDifferentOrder", func(t *testing.T) {
			set1 := &OrderedSet[string]{}
			set1.Add("a")
			set1.Add("b")
			set1.Add("c")

			set2 := &OrderedSet[string]{}
			set2.Add("c")
			set2.Add("b")
			set2.Add("a")

			assert.True(t, !set1.Equal(set2))
			assert.True(t, !set2.Equal(set1))
		})

		t.Run("SingleItem", func(t *testing.T) {
			set1 := &OrderedSet[int]{}
			set1.Add(42)

			set2 := &OrderedSet[int]{}
			set2.Add(42)

			assert.True(t, set1.Equal(set2))
			assert.True(t, set2.Equal(set1))
		})

		t.Run("EmptyVsNonEmpty", func(t *testing.T) {
			set1 := &OrderedSet[int]{}

			set2 := &OrderedSet[int]{}
			set2.Add(1)

			assert.True(t, !set1.Equal(set2))
			assert.True(t, !set2.Equal(set1))
		})

		t.Run("LargeSetSameOrder", func(t *testing.T) {
			set1 := &OrderedSet[int]{}
			set2 := &OrderedSet[int]{}

			// Add 1000 items in same order
			for i := 0; i < 1000; i++ {
				set1.Add(i)
				set2.Add(i)
			}

			assert.True(t, set1.Equal(set2))
			assert.True(t, set2.Equal(set1))
		})

		t.Run("LargeSetDifferentOrder", func(t *testing.T) {
			set1 := &OrderedSet[int]{}
			set2 := &OrderedSet[int]{}

			// Add 1000 items in different orders
			for i := 0; i < 1000; i++ {
				set1.Add(i)
			}
			for i := 999; i >= 0; i-- {
				set2.Add(i)
			}

			assert.True(t, !set1.Equal(set2))
			assert.True(t, !set2.Equal(set1))
		})

		t.Run("AfterDeletion", func(t *testing.T) {
			set1 := &OrderedSet[int]{}
			set1.Add(1)
			set1.Add(2)
			set1.Add(3)
			set1.Delete(2)

			set2 := &OrderedSet[int]{}
			set2.Add(1)
			set2.Add(3)

			assert.True(t, set1.Equal(set2))
			assert.True(t, set2.Equal(set1))
		})

		t.Run("StringSetOrdered", func(t *testing.T) {
			set1 := &OrderedSet[string]{}
			set1.Add("hello")
			set1.Add("world")

			set2 := &OrderedSet[string]{}
			set2.Add("hello")
			set2.Add("world")

			assert.True(t, set1.Equal(set2))

			set3 := &OrderedSet[string]{}
			set3.Add("world")
			set3.Add("hello")

			assert.True(t, !set1.Equal(set3))
		})
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
