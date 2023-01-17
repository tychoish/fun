package fun

import (
	"context"
	"fmt"
	"testing"
)

func TestSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for name, builder := range map[string]func() Set[string]{
		"Unordered/Basic":          func() Set[string] { return NewSet[string]() },
		"Unordered/BasicLarge":     func() Set[string] { return MakeSet[string](100) },
		"Unordered/SyncBasic":      func() Set[string] { return MakeSynchronizedSet(NewSet[string]()) },
		"Unordered/SyncBasicLarge": func() Set[string] { return MakeSynchronizedSet(MakeSet[string](100)) },
		"Ordered/Basic":            func() Set[string] { return NewOrderedSet[string]() },
		"Ordered/BasicLarge":       func() Set[string] { return MakeOrderedSet[string](100) },
		"Ordered/SyncBasic":        func() Set[string] { return MakeSynchronizedSet(NewOrderedSet[string]()) },
		"Ordered/SyncBasicLarge":   func() Set[string] { return MakeSynchronizedSet(MakeOrderedSet[string](100)) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("Initialization", func(t *testing.T) {
				set := builder()
				if set.Len() != 0 {
					t.Fatal("initalized non-empty set")
				}
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

				if !SetDeleteCheck(set, "abc") {
					t.Error("set item should have been present")
				}

				if SetDeleteCheck(set, "abc") {
					t.Error("set item should not have been present")
				}
			})
			t.Run("AddCheck", func(t *testing.T) {
				set := builder()

				if SetAddCheck(set, "abc") {
					t.Error("set item should not have been present")
				}
				if !SetAddCheck(set, "abc") {
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
							t.Fatal("size should not change")
						}
					})
					t.Run("Equality", func(t *testing.T) {
						set := builder()
						populator(set)

						set2 := builder()
						populator(set2)
						if !SetEqual(set, set2) {
							t.Fatal("sets should be equal")
						}
					})
					t.Run("InqualitySizeSimple", func(t *testing.T) {
						set := builder()
						populator(set)

						set2 := builder()
						set2.Add("foo")
						if SetEqual(set, set2) {
							t.Fatal("sets should not be equal")
						}
					})
					t.Run("InqualitySizeComplex", func(t *testing.T) {
						set := builder()
						populator(set)
						elems, err := IteratorCollect(ctx, set.Iterator(ctx))
						if err != nil {
							t.Fatal(err)
						} else if len(elems) == 0 {
							t.Fatal("should have items")
						}

						set2 := builder()
						populator(set2)

						if !SetEqual(set, set2) {
							t.Fatal("sets should be equal")
						}

						set2.Delete(elems[1])
						set2.Add("foo")
						if set.Len() != set2.Len() {
							t.Fatal("test bug")
						}
						if SetEqual(set, set2) {
							t.Fatal("sets should not be equal")
						}
					})
				})
			}
		})
	}
	t.Run("Pairs/Basic", func(t *testing.T) {
		pairs := Pairs[string, int]{
			{"foo", 42},
			{"bar", 31},
		}

		set := pairs.Set()
		if set.Len() != 2 {
			t.Fatal("set conversion didn't work")
		}

		pmap := pairs.Map()
		if v, ok := pmap["foo"]; !ok && v != 42 {
			t.Error("value should exist, and have expected value", ok, v)
		}

		extraPair := Pair[string, int]{"foo", 89}
		pairs = append(pairs, extraPair)
		set = pairs.OrderedSet()
		if set.Len() != 3 {
			t.Fatal("now three items in set")
		}
		osetIter := set.Iterator(ctx)
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

		set = pairs.Set()
		if set.Len() != 3 {
			t.Fatal("now three items in set")
		}

		pmap = pairs.Map()
		if len(pmap) != 2 {
			t.Fatal("deduplication of keys", pmap)
		}
		set.Delete(extraPair)

		pairsFromMap := MakePairs(map[string]int{"foo": 42, "bar": 31})
		set2 := pairsFromMap.Set()
		if !SetEqualCtx(ctx, set, set2) {
			t.Log(pairsFromMap)
			t.Log(set)
			t.Log(set2)
			t.Error("pairs should be equal")
		}

		pairs = pairs.Append(Pair[string, int]{"kip", 14})
		newItem := pairs[len(pairs)-1]
		if newItem.Key != "kip" {
			t.Error("wrong key value", newItem.Key)
		}

		pairs.Add("merlin", 14)
		newItem = pairs[len(pairs)-1]
		if newItem.Key != "merlin" {
			t.Error("wrong key value", newItem.Key)
		}
	})
	t.Run("Pairs/DuplicateKey", func(t *testing.T) {
		pairs := Pairs[string, string]{}
		pairs.Add("aaa", "bbb")
		pairs.Add("aaa", "bbb")
		pairs.Add("aaa", "ccc")

		if len(pairs) != 3 {
			t.Fatal("unexpected pairs value")
		}

		for _, tt := range []struct {
			Name    string
			MakeSet func() Set[Pair[string, string]]
		}{
			{
				Name:    "Map",
				MakeSet: pairs.Set,
			},
			{
				Name:    "Ordered",
				MakeSet: pairs.OrderedSet,
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
	t.Run("OrderedSetCleanup", func(t *testing.T) {
		set := NewOrderedSet[int]()
		for i := range make([]int, 300) {
			set.Add(i)
		}

		os := set.(*orderedSetImpl[int])
		for i := range make([]int, 300) {
			if i < 100 {
				if len(os.elems) != 300 {
					t.Error("lazy delete", len(os.elems))
				}
			}
			if i%2 == 0 || i%3 == 0 {
				set.Delete(i)
			}
		}
		if l := len(os.set); l != 100 {
			t.Fatal("unexpected size", l)
		}
		if l := len(os.elems); l != 100 {
			t.Fatal("unexpected size", l)
		}
		if os.deletedCount != 0 {
			t.Fatal("unexpected delete count", os.deletedCount)
		}

	})
}
