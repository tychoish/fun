package set

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
	return fun.SliceIterator(out)
}

func TestSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for name, builder := range map[string]func() Set[string]{
		"Unordered/Basic":      func() Set[string] { return NewUnordered[string]() },
		"Unordered/BasicLarge": func() Set[string] { return MakeUnordered[string](100) },
		"Ordered/Basic":        func() Set[string] { return MakeOrdered[string]() },
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
				testt.Log(t, rjson)
				nset := builder()
				assert.NotError(t, nset.(json.Unmarshaler).UnmarshalJSON(data))
				check.True(t, Equal(ctx, set, nset))

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

				if !DeleteCheck(set, "abc") {
					t.Error("set item should have been present")
				}

				if DeleteCheck(set, "abc") {
					t.Error("set item should not have been present")
				}
			})
			t.Run("AddCheck", func(t *testing.T) {
				set := builder()

				if AddCheck(set, "abc") {
					t.Error("set item should not have been present")
				}
				if !AddCheck(set, "abc") {
					t.Error("set item should have been present")
				}
			})
			for populatorName, populator := range map[string]func(Set[string]){
				"Three": func(set Set[string]) {
					set.Add("a")
					set.Add("b")
					set.Add("c")
				},
				"Numbers": func(set Set[string]) {
					for i := 0; i < 100; i++ {
						set.Add(fmt.Sprint(i))
					}
				},
				"Populator": func(set Set[string]) {
					Populate(ctx, set, generateIter(ctx, 100))
				},
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
						if !Equal(ctx, set, set2) {
							t.Fatal("sets should be equal")
						}
					})
					t.Run("EqualityCancelation", func(t *testing.T) {
						cctx, ccancel := context.WithCancel(ctx)
						ccancel()

						set := builder()
						populator(set)

						set2 := builder()
						populator(set2)

						if Equal(cctx, set, set2) {
							t.Fatal("context cancellation disrupts iteration")
						}
					})
					t.Run("InqualitySizeSimple", func(t *testing.T) {
						set := builder()
						populator(set)

						set2 := builder()
						set2.Add("foo")
						if Equal(ctx, set, set2) {
							t.Fatal("sets should not be equal")
						}
					})
					t.Run("InqualitySizeComplex", func(t *testing.T) {
						set := builder()
						populator(set)
						count := 0
						iter := set.Iterator()
						collected := []string{}
						for iter.Next(ctx) {
							count++
							if iter.Value() == "" {
								continue
							}
							collected = append(collected, iter.Value())
						}

						if err := iter.Close(); err != nil {
							t.Fatal(err)
						} else if len(collected) == 0 {
							t.Fatal("should have items")
						}

						set2 := builder()
						populator(set2)

						if !Equal(ctx, set, set2) {
							t.Fatal("sets should be equal")
						}

						set2.Delete(collected[1])
						set2.Add("foo")
						if set.Len() != set2.Len() {
							t.Fatal("test bug")
						}
						if Equal(ctx, set, set2) {
							t.Fatal("sets should not be equal")
						}
					})
				})
			}
		})
	}
	t.Run("Pairs/Basic", func(t *testing.T) {
		pairs := fun.Pairs[string, int]{
			{Key: "foo", Value: 42},
			{Key: "bar", Value: 31},
		}

		set := BuildUnorderedFromPairs(pairs)
		if set.Len() != 2 {
			t.Fatal("set conversion didn't work")
		}

		pmap := fun.Pairs[string, int](pairs).Map()
		if v, ok := pmap["foo"]; !ok && v != 42 {
			t.Error("value should exist, and have expected value", ok, v)
		}

		extraPair := fun.Pair[string, int]{Key: "foo", Value: 89}
		pairs = append(pairs, extraPair)
		set = BuildOrderedFromPairs(pairs)
		if set.Len() != 3 {
			t.Fatal("now three items in set")
		}
		osetIter := set.Iterator()
		idx := 0
		seen := 0
		for osetIter.Next(ctx) {
			val := osetIter.Value()
			switch idx {
			case 0:
				seen++
				if val.Key != "foo" || val.Value != 42 {
					t.Errorf("unexpeded @ idx=%d, %+v", idx, val)
				}
			case 1:
				seen++
				if val.Key != "bar" || val.Value != 31 {
					t.Errorf("unexpeded @ idx=%d, %+v", idx, val)
				}
			case 2:
				seen++
				if val.Key != "foo" || val.Value != 89 {
					t.Errorf("unexpeded @ idx=%d, %+v", idx, val)
				}
			default:
				t.Error("unexepected item")
			}
			idx++
		}
		if seen != 3 {
			t.Errorf("saw=%d not 3", seen)
		}

		set = BuildUnorderedFromPairs(pairs)
		if set.Len() != 3 {
			t.Fatal("now three items in set")
		}

		pmap = fun.Pairs[string, int](pairs).Map()
		if len(pmap) != 2 {
			t.Fatal("deduplication of keys", pmap)
		}
		set.Delete(extraPair)

		pairsFromMap := fun.Pairs[string, int]{}
		pairsFromMap.ConsumeMap(map[string]int{"foo": 42, "bar": 31})
		set2 := BuildUnorderedFromPairs(pairsFromMap)
		if !Equal(ctx, set, set2) {
			t.Log(pairsFromMap)
			t.Log(set)
			t.Log(set2)
			t.Error("pairs should be equal")
		}

		fp := fun.Pairs[string, int](pairs)

		fp = fp.Append(fun.Pair[string, int]{Key: "kip", Value: 14})
		newItem := fp[len(fp)-1]
		if newItem.Key != "kip" {
			t.Error("wrong key value", newItem.Key)
		}

		fp.Add("merlin", 14)
		newItem = fp[len(fp)-1]
		if newItem.Key != "merlin" {
			t.Error("wrong key value", newItem.Key)
		}
	})
	t.Run("Pairs/DuplicateKey", func(t *testing.T) {
		pairs := fun.Pairs[string, string]{}
		pairs.Add("aaa", "bbb")
		pairs.Add("aaa", "bbb")
		pairs.Add("aaa", "ccc")

		if len(pairs) != 3 {
			t.Fatal("unexpected pairs value")
		}

		for _, tt := range []struct {
			Name    string
			MakeSet func() Set[fun.Pair[string, string]]
		}{
			{
				Name: "Map",
				MakeSet: func() Set[fun.Pair[string, string]] {
					return BuildUnorderedFromPairs(pairs)
				},
			},
			{
				Name: "Ordered",
				MakeSet: func() Set[fun.Pair[string, string]] {
					return BuildOrderedFromPairs(pairs)
				},
			},
		} {
			t.Run(tt.Name, func(t *testing.T) {
				set := tt.MakeSet()
				if set.Len() != 2 {
					t.Error("set conversion didn't work")
				}
				// if they're comparably equal in the
				// set there's no way to know which
				// index from the pair was chosen.
			})
		}
	})
	t.Run("Unwrap", func(t *testing.T) {
		t.Run("Set", func(t *testing.T) {
			base := MakeUnordered[string](1)
			base.Add("abc")
			wrapped := Synchronize(base)
			maybeBase := wrapped.(interface{ Unwrap() Set[string] }).Unwrap()
			if maybeBase == nil {
				t.Fatal("should not be nil")
			}
		})
		t.Run("SetIter", func(t *testing.T) {
			base := MakeUnordered[string](1)
			base.Add("abc")
			wrapped := Synchronize(base).Iterator()
			maybeBase := fun.Unwrap(wrapped)
			if maybeBase != nil {
				t.Fatal("should not be nil")
			}
		})
	})
	t.Run("JSONCheck", func(t *testing.T) {
		// want to make sure this works normally, without the
		// extra cast, as above.
		set := MakeOrdered[int]()
		err := json.Unmarshal([]byte("[1,2,3]"), &set)
		check.NotError(t, err)
		check.Equal(t, 3, set.Len())
	})

}

func BenchmarkSet(b *testing.B) {
	ctx := context.Background()

	const size = 10000
	b.Run("Append", func(b *testing.B) {
		operation := func(set Set[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * i)
			}
		}
		b.Run("Map", func(b *testing.B) {
			set := NewUnordered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := MakeOrdered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})
	b.Run("Mixed", func(b *testing.B) {
		operation := func(set Set[int]) {
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
			set := NewUnordered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := MakeOrdered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

	b.Run("Deletion", func(b *testing.B) {
		operation := func(set Set[int]) {
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
			set := NewUnordered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := MakeOrdered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

	b.Run("Iteration", func(b *testing.B) {
		operation := func(set Set[int]) {
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
			set := NewUnordered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := MakeOrdered[int]()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

}
