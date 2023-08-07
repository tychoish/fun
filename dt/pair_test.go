package dt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestPairs(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		ps := Mapify(map[string]string{"in": "out"}).Pairs()
		ps.Add("in", "in").Add("in", "what").Add("in", "out")
		ps.AddPair(MakePair("in", "in"))
		mp := ps.Map()
		assert.Equal(t, len(mp), 1)
		assert.Equal(t, ps.Len(), 5)
		assert.Equal(t, mp["in"], "out") // first value wins
	})
	t.Run("IterationOrder", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ps := Pairs[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
		}

		assert.Equal(t, ps.Len(), 128)
		keys := ps.Keys()
		values := ps.Values()
		idx := 0
		for keys.Next(ctx) && values.Next(ctx) {
			check.Equal(t, idx, keys.Value())
			check.Equal(t, idx, values.Value())
			idx++
		}
		check.Equal(t, idx, 128)
	})
	t.Run("Extend", func(t *testing.T) {
		ps := &Pairs[string, string]{}
		ps.Extend(MakePairs(MakePair("one", "1"), MakePair("two", "2")))
		assert.Equal(t, ps.Len(), 2)
		assert.Equal(t, ps.ll.Front().Value(), MakePair("one", "1"))
	})
	t.Run("Consume", func(t *testing.T) {
		t.Run("Prime", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps := Pairs[int, int]{}
			sp := Pairs[int, int]{}
			for i := 0; i < 128; i++ {
				ps.Add(i, i)
				sp.Add(i, i)
			}
			assert.Equal(t, ps.Len(), 128)
			assert.NotError(t, ps.Consume(ctx, sp.Iterator()))
			assert.Equal(t, ps.Len(), 256)
			assert.Equal(t, len(ps.Map()), 128)
		})
		t.Run("Slice", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.ConsumeSlice([]int{1, 2, 3}, func(in int) string { return fmt.Sprint(in) })

			ps := p.Slice()
			for idx := range ps {
				check.Equal(t, ps[idx].Key, fmt.Sprint(idx+1))
				check.Equal(t, ps[idx].Value, idx+1)
			}
		})
		t.Run("List", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.ConsumeSlice([]int{1, 2, 3}, func(in int) string { return fmt.Sprint(in) })

			pl := p.List()
			check.NotEqual(t, pl, p.ll)
			idx := 1
			for item := pl.Front(); item.Ok(); item = item.Next() {
				check.Equal(t, item.Value().Value, idx)
				check.Equal(t, item.Value().Key, fmt.Sprint(idx))
				idx++
			}
			check.Equal(t, 4, idx)
		})
		t.Run("Values", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			p := Pairs[string, int]{}
			err := p.ConsumeValues(
				ctx,
				Sliceify([]int{1, 2, 3}).Iterator(),
				func(in int) string { return fmt.Sprint(in) },
			)
			assert.NotError(t, err)
			assert.Equal(t, p.Len(), 3)
			ps := p.Slice()
			for idx := range ps {
				check.Equal(t, ps[idx].Key, fmt.Sprint(idx+1))
				check.Equal(t, ps[idx].Value, idx+1)
			}
		})
		t.Run("Map", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.ConsumeMap(map[string]int{
				"1": 1,
				"2": 2,
				"3": 3,
			})
			assert.Equal(t, p.Len(), 3)
		})

	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Encode", func(t *testing.T) {
			ps := Mapify(map[string]string{"in": "out"}).Pairs()
			out, err := json.Marshal(ps)
			assert.NotError(t, err)
			assert.Equal(t, string(out), `{"in":"out"}`)
		})
		t.Run("EncodeLong", func(t *testing.T) {
			ps := MakePairs(MakePair("in", "out"), MakePair("out", "in"))
			out, err := json.Marshal(ps)
			assert.NotError(t, err)
			assert.Equal(t, string(out), `{"in":"out","out":"in"}`)
		})
		t.Run("Decode", func(t *testing.T) {
			ps := Pairs[string, string]{}
			err := json.Unmarshal([]byte(`{"in":"out"}`), &ps)
			assert.NotError(t, err)
			assert.Equal(t, 1, ps.Len())
			psl := ps.Slice()
			assert.Equal(t, "out", psl[0].Value)
			assert.Equal(t, "in", psl[0].Key)
		})
		t.Run("DecodeError", func(t *testing.T) {
			ps := Pairs[string, string]{}
			err := json.Unmarshal([]byte(`{"in":1}`), &ps)
			assert.Error(t, err)
			assert.Equal(t, 0, ps.Len())
		})
		t.Run("ImpossibleValue", func(t *testing.T) {
			ps := MakePairs[string, context.CancelFunc](Pair[string, context.CancelFunc]{"hi", func() {}})
			_, err := ps.MarshalJSON()
			assert.Error(t, err)
		})
		t.Run("ImpossibleKey", func(t *testing.T) {
			ps := MakePairs[badKey, string](Pair[badKey, string]{"hi", "hi"})
			_, err := ps.MarshalJSON()
			assert.Error(t, err)
		})
	})
	t.Run("Sorts", func(t *testing.T) {
		cmp := func(a, b Pair[int, int]) bool {
			return a.Key < b.Key // && a.Value < b.Value
		}
		t.Run("Quick", func(t *testing.T) {
			list := sortFixture(t, 100)
			ps := &Pairs[int, int]{ll: list}
			check.True(t, ft.Not(list.IsSorted(cmp)))
			ps.SortQuick(cmp)
			check.True(t, list.IsSorted(cmp))
		})
		t.Run("Merge", func(t *testing.T) {
			list := sortFixture(t, 100)
			ps := &Pairs[int, int]{ll: list}
			check.True(t, ft.Not(list.IsSorted(cmp)))
			ps.SortMerge(cmp)
			check.True(t, list.IsSorted(cmp))
		})
	})
}

type badKey string

func (badKey) MarshalJSON() ([]byte, error) { return nil, errors.New("cannot marshal") }

func sortFixture(t *testing.T, size int) *List[Pair[int, int]] {
	nums := rand.Perm(size)

	out := &List[Pair[int, int]]{}
	for _, num := range nums {
		out.PushBack(MakePair(num, num))
	}
	assert.Equal(t, size, out.Len())
	return out
}
