package dt

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestPairs(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		ps := NewMap(map[string]string{"in": "out"}).Pairs()
		ps.Add("in", "in")
		ps.Add("in", "what")
		ps.Add("in", "out")
		ps.Push(MakePair("in", "in"))
		mp := Map[string, string]{}
		mp.ConsumePairs(ps)
		assert.Equal(t, len(mp), 1)
		assert.Equal(t, ps.Len(), 5)
		assert.Equal(t, mp["in"], "in") // first value wins
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ps := Pairs[int, int]{}
		sp := Pairs[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
			sp.Add(i, i)
		}
		assert.Equal(t, ps.Len(), 128)
		assert.NotError(t, ps.Consume(sp.Iterator()).Run(ctx))
		assert.Equal(t, ps.Len(), 256)
		mp := Map[int, int]{}
		mp.ConsumePairs(&ps)
		assert.Equal(t, len(mp), 128)
	})
	t.Run("ConsumePairs", func(t *testing.T) {
		t.Run("Error", func(t *testing.T) {
			expected := errors.New("hi")
			iter := fun.StaticProducer(MakePair("1", 1), expected).Iterator()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps, err := ConsumePairs(iter).Resolve(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, expected)
			assert.True(t, ps == nil)
		})
		t.Run("Happy", func(t *testing.T) {
			iter := NewSlice[Pair[string, int]]([]Pair[string, int]{
				MakePair("1", 1), MakePair("2", 2),
				MakePair("3", 3), MakePair("4", 4),
				MakePair("5", 5), MakePair("6", 6),
			}).Iterator()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps, err := ConsumePairs(iter).Resolve(ctx)
			check.NotError(t, err)
			assert.True(t, ps != nil)
			check.Equal(t, ps.Len(), 6)
		})

	})
	t.Run("Sorts", func(t *testing.T) {
		cmp := func(a, b Pair[int, int]) bool {
			return a.Key < b.Key // && a.Value < b.Value
		}
		t.Run("Quick", func(t *testing.T) {
			list := randNumPairListFixture(t, 100)
			ps := &Pairs[int, int]{ll: list}
			check.True(t, ft.Not(list.IsSorted(cmp)))
			ps.SortQuick(cmp)
			check.True(t, list.IsSorted(cmp))
		})
		t.Run("Merge", func(t *testing.T) {
			list := randNumPairListFixture(t, 100)
			ps := &Pairs[int, int]{ll: list}
			check.True(t, ft.Not(list.IsSorted(cmp)))
			ps.SortMerge(cmp)
			check.True(t, list.IsSorted(cmp))
		})
	})
}

func randNumPairListFixture(t *testing.T, size int) *List[Pair[int, int]] {
	t.Helper()
	nums := rand.Perm(size)

	out := &List[Pair[int, int]]{}
	for _, num := range nums {
		out.PushBack(MakePair(num, num))
	}
	assert.Equal(t, size, out.Len())
	return out
}
