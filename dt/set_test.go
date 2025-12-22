package dt

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/irt"
)

func TestSet(t *testing.T) {
	for name, builder := range map[string]func() *Set[string]{
		"Unordered/Basic": func() *Set[string] { s := &Set[string]{}; return s },
		"Ordered/Basic":   func() *Set[string] { s := &Set[string]{}; s.Order(); return s },
		"Unordered/Sync":  func() *Set[string] { s := &Set[string]{}; s.Synchronize(); return s },
		"Ordered/Sync":    func() *Set[string] { s := &Set[string]{}; s.Order(); s.Synchronize(); return s },
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

				// nil mutexes should always panic
				check.Panic(t, func() { set.WithLock(nil) })

				// some are pre-configured with locks.
				if set.mtx.Get() == nil {
					// first attept should pass
					check.NotPanic(t, func() { set.WithLock(&sync.Mutex{}) })
					// second set should panic
					check.Panic(t, func() { set.WithLock(&sync.Mutex{}) })
				}
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
				check.True(t, set.Equal(nset))
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

						sl := set.Slice()

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortQuick(cmp.LessThanNative[string])

						sl = set.Slice()

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

						sl := set.Slice()

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortQuick(cmp.LessThanNative[string])

						sll := set.Slice()

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

						sl := set.Slice()

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortMerge(cmp.LessThanNative[string])

						sll := set.Slice()

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

						sl := set.Slice()

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortMerge(cmp.LessThanNative[string])

						sll := set.Slice()

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
				})
			})
			for populatorName, populator := range map[string]func(*Set[string]){
				"Three": func(set *Set[string]) {
					set.Add("a")
					set.Add("b")
					set.Add("c")
				},
				"Numbers": func(set *Set[string]) {
					for i := 0; i < 100; i++ {
						set.Add(fmt.Sprint(i))
					}
				},
				// "Populator": func(set Set[string]) {
				//	Populate(ctx, set, generateIter(ctx, 100))
				// },
			} {
				t.Run(populatorName, func(t *testing.T) {
					t.Run("Uniqueness", func(t *testing.T) {
						set := builder()
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
						set := builder()
						populator(set)

						set2 := builder()
						populator(set2)
						if !set.Equal(set2) {
							t.Fatal("sets should be equal")
						}
					})
					t.Run("InqualitySizeSimple", func(t *testing.T) {
						set := builder()
						populator(set)

						set2 := builder()
						set2.Add("foo")
						if set.Equal(set2) {
							t.Fatal("sets should not be equal")
						}
					})
					t.Run("InqualitySizeComplex", func(t *testing.T) {
						set := builder()
						populator(set)

						collected := irt.Collect(irt.RemoveZeros(set.Iterator()))

						set2 := builder()
						populator(set2)

						if !set.Equal(set2) {
							t.Fatal("sets should be equal")
						}

						set2.Delete(collected[1])
						set2.Add("foo")
						if set.Len() != set2.Len() {
							t.Fatal("test bug")
						}

						if set.Equal(set2) {
							t.Fatal("sets should not be equal")
						}
					})
				})
			}
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
					set := NewSetFromSlice(ls)
					if set.Len() != 100 {
						return
					}
					set2 := NewSetFromSlice(ls)
					if set2.Len() != 100 {
						return
					}
					if !set.Equal(set2) {
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
	t.Run("List", func(t *testing.T) {
		t.Run("Ordered", func(t *testing.T) {
			ls := &List[int]{}

			st := &Set[int]{}
			st.Order()

			for v := range 42 {
				ls.PushBack(v)
				st.Add(v)
			}

			assert.Equal(t, ls.Len(), 42)
			assert.Equal(t, st.Len(), ls.Len())

			setls := st.List()

			count := 0
			for elemL, elemS := ls.Front(), setls.Front(); elemL.Ok() && elemS.Ok(); elemL, elemS = elemL.Next(), elemS.Next() {
				count++
				check.Equal(t, elemL.Value(), elemS.Value())
			}
			assert.Equal(t, count, 42)
		})
		t.Run("Undordered", func(t *testing.T) {
			ls := &List[int]{}

			st := &Set[int]{}

			for v := range 42 {
				ls.PushBack(v)
				st.Add(v)
			}

			setls := st.List()

			assert.Equal(t, ls.Len(), 42)
			assert.Equal(t, st.Len(), ls.Len())
			assert.Equal(t, st.Len(), setls.Len())

			checker := Map[int, int]{}
			for v := range setls.IteratorFront() {
				if checker.Check(v) {
					t.Errorf("duplicate value, %d", v)
				}
				checker.SetDefault(v)
				check.True(t, v <= 42)
			}
			assert.Equal(t, 42, len(checker))
		})
	})
	t.Run("Slice", func(t *testing.T) {
		t.Run("Ordered", func(t *testing.T) {
			in := []int{1, 2, 3, 4, 5, 6}
			st := &Set[int]{}
			st.Order()

			st.AppendStream(irt.Slice(in))
			assert.Equal(t, st.Len(), 6)
			assert.True(t, slices.Equal(in, st.Slice()))
		})
		t.Run("Unordered", func(t *testing.T) {
			in := []int{1, 2, 3, 4, 5, 6}
			st := &Set[int]{}
			st.AppendStream(irt.Slice(in))
			assert.Equal(t, st.Len(), 6)
			out := st.Slice()
			assert.Equal(t, len(out), 6)
			for v := range slices.Values(out) {
				check.True(t, v <= 6 && v >= 1)
			}
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
