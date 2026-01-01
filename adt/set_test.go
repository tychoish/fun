package adt

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

func TestSet(t *testing.T) {
	for name, builder := range map[string]func() *Set[string]{
		"Unordered/Basic": func() *Set[string] { s := &Set[string]{}; return s },
		"Ordered/Basic":   func() *Set[string] { s := &Set[string]{}; s.Order(); return s },
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("EmptyIteraton", func(t *testing.T) {
				set := builder()
				ct := 0
				assert.NotPanic(t, func() {
					for range set.Iterator() {
						ct++
					}
				})
				assert.Zero(t, ct)
			})

			t.Run("Initialization", func(t *testing.T) {
				set := builder()
				if set.Len() != 0 {
					t.Fatal("initialized non-empty set")
				}

				// safe to set order more than once
				check.NotPanic(t, set.Order)
				check.NotPanic(t, set.Order)
			})
			t.Run("JSON", func(t *testing.T) {
				set := builder()
				set.Add("hello")
				set.Add("buddy")
				set.Add("hello")
				set.Add("kip")
				check.Equal(t, set.Len(), 3) // hello is a dupe

				data, err := json.Marshal(set)
				check.NotError(t, err)
				count := 0
				rjson := string(data)
				for item := range set.Iterator() {
					count++
					switch {
					case strings.Contains(rjson, "hello"):
						continue
					case strings.Contains(rjson, "buddy"):
						continue
					case strings.Contains(rjson, "kip"):
						continue
					default:
						t.Errorf("unexpeced item %q<%d>", item, count)
					}
				}
				check.Equal(t, count, set.Len())
				nset := builder()
				assert.NotError(t, nset.UnmarshalJSON(data))
			})
			t.Run("Recall", func(t *testing.T) {
				set := builder()
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
				set := builder()
				set.Add("abc")

				if !set.Check("abc") {
					t.Error("should recall item")
				}

				set.Delete("abc")

				if set.Check("abc") {
					t.Error("should have deleted item")
				}
			})
			t.Run("DeleteCheck", func(t *testing.T) {
				set := builder()
				set.Add("abc")

				if !set.DeleteCheck("abc") {
					t.Error("set item should have been present")
				}

				if set.DeleteCheck("abc") {
					t.Error("set item should not have been present")
				}
			})
			t.Run("AddCheck", func(t *testing.T) {
				set := builder()

				if set.AddCheck("abc") {
					t.Error("set item should not have been present")
				}
				if !set.AddCheck("abc") {
					t.Error("set item should have been present")
				}
			})
			t.Run("Sort", func(t *testing.T) {
				t.Run("Quick", func(t *testing.T) {
					t.Parallel()
					t.Run("UnorderedToStart", func(t *testing.T) {
						set := builder()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortQuick(cmp.LessThanNative[string])

						sl = irt.Collect(set.Iterator())

						check.Equal(t, "abc", sl[0])
						check.Equal(t, "def", sl[1])
						check.Equal(t, "ghi", sl[2])
						check.Equal(t, "lmn", sl[3])
						check.Equal(t, "opq", sl[4])
						check.Equal(t, "xyz", sl[5])
					})
					t.Run("OrderedStart", func(t *testing.T) {
						set := builder()
						set.Order()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")
						testt.Log(t, set.Len())

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortQuick(cmp.LessThanNative[string])

						sll := irt.Collect(set.Iterator())
						testt.Log(t, set.Len(), len(sl), sll)

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
				})
				t.Run("Merge", func(t *testing.T) {
					t.Parallel()
					t.Run("UnorderedToStart", func(t *testing.T) {
						set := builder()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")

						testt.Log(t, set.Len())

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortMerge(cmp.LessThanNative[string])

						sll := irt.Collect(set.Iterator())

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
					t.Run("OrderedStart", func(t *testing.T) {
						set := builder()
						set.Order()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortMerge(cmp.LessThanNative[string])

						sll := irt.Collect(set.Iterator())

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
				})
			})
		})
	}
	t.Run("JSONCheck", func(t *testing.T) {
		// want to make sure this works normally, without the
		// extra cast, as above.
		set := &Set[int]{}
		set.Order()
		err := json.Unmarshal([]byte("[1,2,3]"), &set)
		check.NotError(t, err)
		check.Equal(t, 3, set.Len())
	})
	t.Run("Constructors", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			var passed bool
			for attempt := 0; attempt < 10; attempt++ {
				func() {
					ls := make([]uint64, 0, 100)
					for i := 0; i < 100; i++ {
						ls = append(ls, rand.Uint64())
					}
					set := MakeSet(irt.Slice(ls))
					if set.Len() != 100 {
						return
					}
					set2 := MakeSet(irt.Slice(ls))
					if set2.Len() != 100 {
						return
					}
					if !irt.Equal(set.Iterator(), set2.Iterator()) {
						return
					}

					passed = true
				}()
				if passed {
					return
				}
			}
			t.Fatal("test could not pass")
		})
	})
}

func BenchmarkSet(b *testing.B) {
	const size = 10000
	b.Run("Append", func(b *testing.B) {
		operation := func(set *Set[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * i)
			}
		}
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})
	b.Run("Mixed", func(b *testing.B) {
		operation := func(set *Set[int]) {
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
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

	b.Run("Deletion", func(b *testing.B) {
		operation := func(set *Set[int]) {
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
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

	b.Run("Iteration", func(b *testing.B) {
		operation := func(set *Set[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * size)
			}
			count := 0
			for range set.Iterator() {
				count++
			}
			check.Equal(b, size, count)
		}
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()

			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})
}
