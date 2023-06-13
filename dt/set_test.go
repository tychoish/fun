package dt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func generateIter(ctx context.Context, size int) *fun.Iterator[string] {
	out := make([]string, size)
	for i := 0; i < size; i++ {
		out[i] = fmt.Sprintf("iter=%d", i)

	}
	return Sliceify(out).Iterator()
}

func TestSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
					iter := set.Iterator()
					for iter.Next(ctx) {
						ct++
					}
					check.NotError(t, iter.Close())
				})
				assert.Zero(t, ct)
			})

			t.Run("Initialization", func(t *testing.T) {
				set := builder()
				if set.Len() != 0 {
					t.Fatal("initalized non-empty set")
				}
			})
			t.Run("JSON", func(t *testing.T) {
				set := builder()
				set.Add("hello")
				set.Add("merlin")
				set.Add("hello")
				set.Add("kip")
				check.Equal(t, set.Len(), 3) // hello is a dupe

				data, err := json.Marshal(set)
				check.NotError(t, err)
				iter := set.Iterator()
				count := 0
				rjson := string(data)
				for iter.Next(ctx) {
					count++
					item := iter.Value()
					switch {
					case strings.Contains(rjson, "hello"):
						continue
					case strings.Contains(rjson, "merlin"):
						continue
					case strings.Contains(rjson, "kip"):
						continue
					default:
						t.Errorf("unexpeced item %q<%d>", item, count)
					}
				}
				check.Equal(t, count, set.Len())
				testt.Log(t, "rjson", rjson)
				testt.Log(t, "set", set)
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
				// 	Populate(ctx, set, generateIter(ctx, 100))
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
					// t.Run("Equality", func(t *testing.T) {
					// 	set := builder()
					// 	populator(set)

					// 	set2 := builder()
					// 	populator(set2)
					// 	if !Equal(ctx, set, set2) {
					// 		t.Fatal("sets should be equal")
					// 	}
					// })
					// t.Run("EqualityCancelation", func(t *testing.T) {
					// 	cctx, ccancel := context.WithCancel(ctx)
					// 	ccancel()

					// 	set := builder()
					// 	populator(set)

					// 	set2 := builder()
					// 	populator(set2)

					// 	if Equal(cctx, set, set2) {
					// 		t.Fatal("context cancellation disrupts iteration")
					// 	}
					// })
					// t.Run("InqualitySizeSimple", func(t *testing.T) {
					// 	set := builder()
					// 	populator(set)

					// 	set2 := builder()
					// 	set2.Add("foo")
					// 	if Equal(ctx, set, set2) {
					// 		t.Fatal("sets should not be equal")
					// 	}
					// })
					// t.Run("InqualitySizeComplex", func(t *testing.T) {
					// 	set := builder()
					// 	populator(set)
					// 	count := 0
					// 	iter := set.Iterator()
					// 	collected := []string{}
					// 	for iter.Next(ctx) {
					// 		count++
					// 		if iter.Value() == "" {
					// 			continue
					// 		}
					// 		collected = append(collected, iter.Value())
					// 	}

					// 	if err := iter.Close(); err != nil {
					// 		t.Fatal(err)
					// 	} else if len(collected) == 0 {
					// 		t.Fatal("should have items")
					// 	}

					// 	set2 := builder()
					// 	populator(set2)

					// 	if !Equal(ctx, set, set2) {
					// 		t.Fatal("sets should be equal")
					// 	}

					// 	set2.Delete(collected[1])
					// 	set2.Add("foo")
					// 	if set.Len() != set2.Len() {
					// 		t.Fatal("test bug")
					// 	}
					// 	if Equal(ctx, set, set2) {
					// 		t.Fatal("sets should not be equal")
					// 	}
					// })
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

}

func BenchmarkSet(b *testing.B) {
	ctx := context.Background()

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
			iter := set.Iterator()
			for iter.Next(ctx) {
				set.Check(iter.Value())
				set.Check(iter.Value())
			}
			iter.Close()
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
