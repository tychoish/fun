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
	t.Run("Seq2Iteration", func(t *testing.T) {
		ps := Pairs[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
		}

		assert.Equal(t, ps.Len(), 128)

		var idx int
		for k, v := range ps.Iterator2() {
			check.Equal(t, idx, k)
			check.Equal(t, idx, v)
			idx++
		}

		check.Equal(t, idx, 128)
	})
	t.Run("Seq2IterationAborted", func(t *testing.T) {
		ps := Pairs[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
		}

		assert.Equal(t, ps.Len(), 128)

		var idx int
		for k, v := range ps.Iterator2() {
			check.Equal(t, idx, k)
			check.Equal(t, idx, v)
			idx++
			if idx == 64 {
				break
			}
		}

		check.Equal(t, idx, 64)
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
		assert.NotError(t, ps.Consume(sp.Stream()).Run(ctx))
		assert.Equal(t, ps.Len(), 256)
		mp := Map[int, int]{}
		mp.ConsumePairs(&ps)
		assert.Equal(t, len(mp), 128)
	})
	t.Run("ConsumePairs", func(t *testing.T) {
		t.Run("Error", func(t *testing.T) {
			expected := errors.New("hi")
			iter := fun.StaticGenerator(MakePair("1", 1), expected).Stream()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps, err := ConsumePairs(iter).Read(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, expected)
			assert.True(t, ps == nil)
		})
		t.Run("Happy", func(t *testing.T) {
			iter := NewSlice[Pair[string, int]]([]Pair[string, int]{
				MakePair("1", 1), MakePair("2", 2),
				MakePair("3", 3), MakePair("4", 4),
				MakePair("5", 5), MakePair("6", 6),
			}).Stream()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps, err := ConsumePairs(iter).Read(ctx)
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
	t.Run("Methods", func(t *testing.T) {
		t.Run("Getters", func(t *testing.T) {
			p := MakePair("key", 42)
			check.Equal(t, p.GetKey(), "key")
			check.Equal(t, p.GetValue(), 42)
		})
		t.Run("Get", func(t *testing.T) {
			p := MakePair("key", 42)
			k, v := p.Get()

			check.Equal(t, k, "key")
			check.Equal(t, v, 42)
		})
		t.Run("Set", func(t *testing.T) {
			p := MakePair("", -1)
			k, v := p.Get()
			check.NotEqual(t, k, "key")
			check.NotEqual(t, v, 42)

			p.Set("key", 42)

			k, v = p.Get()
			check.Equal(t, k, "key")
			check.Equal(t, v, 42)
		})
		t.Run("Setters", func(t *testing.T) {
			p := MakePair("", -1)
			k, v := p.Get()
			check.NotEqual(t, k, "key")
			check.NotEqual(t, v, 42)

			p.SetKey("key")
			p.SetValue(42)

			k, v = p.Get()
			check.Equal(t, k, "key")
			check.Equal(t, v, 42)
		})
		t.Run("Chains", func(t *testing.T) {
			p := &Pair[string, int]{Key: "key", Value: 42}

			// this is true because pointers are compared
			// by value
			assert.Equal(t, p, p.Set("key3", 428))
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
