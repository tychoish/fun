// GENERATED FILE FROM PAIR IMPLEMENTATION
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

// ConsumeTuples creates a *Tuples[K,V] object from a stream of
// Tuple[K,V] objects.
func ConsumeTuples[K any, V any](iter *fun.Stream[Tuple[K, V]]) fun.Generator[*Tuples[K, V]] {
	return func(ctx context.Context) (*Tuples[K, V], error) {
		p := &Tuples[K, V]{}
		if err := p.AppendStream(iter).Run(ctx); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func TestTuples(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		ps := MakeTuples(MakeTuple("in", "out"))
		ps.Add("in", "in")
		ps.Add("in", "what")
		ps.Add("in", "out")
		ps.Push(MakeTuple("in", "in"))
		mp := Map[string, string]{}
		mp.ExtendWithTuples(ps)
		assert.Equal(t, len(mp), 1)
		assert.Equal(t, ps.Len(), 5)
		assert.Equal(t, mp["in"], "in") // first value wins
	})
	t.Run("IterationOrder", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ps := Tuples[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
		}

		assert.Equal(t, ps.Len(), 128)
		keys := ps.Ones()
		values := ps.Twos()
		idx := 0
		for keys.Next(ctx) && values.Next(ctx) {
			check.Equal(t, idx, keys.Value())
			check.Equal(t, idx, values.Value())
			idx++
		}
		check.Equal(t, idx, 128)
	})
	t.Run("Seq2Iteration", func(t *testing.T) {
		ps := Tuples[int, int]{}
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
		ps := Tuples[int, int]{}
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
		ps := &Tuples[string, string]{}
		ps.AppendTuples(MakeTuples(MakeTuple("one", "1"), MakeTuple("two", "2")))
		assert.Equal(t, ps.Len(), 2)
		assert.Equal(t, ps.ll.Front().Value(), MakeTuple("one", "1"))
	})
	t.Run("Consume", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ps := Tuples[int, int]{}
		sp := Tuples[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
			sp.Add(i, i)
		}
		assert.Equal(t, ps.Len(), 128)
		assert.NotError(t, ps.AppendStream(sp.Stream()).Run(ctx))
		assert.Equal(t, ps.Len(), 256)
		mp := Map[int, int]{}
		mp.ExtendWithTuples(&ps)
		assert.Equal(t, len(mp), 128)
	})
	t.Run("ConsumeTuples", func(t *testing.T) {
		t.Run("Error", func(t *testing.T) {
			expected := errors.New("hi")
			iter := fun.MakeStream(fun.StaticGenerator(MakeTuple("1", 1), expected))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps, err := ConsumeTuples(iter).Read(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, expected)
			assert.True(t, ps == nil)
		})
		t.Run("Happy", func(t *testing.T) {
			iter := NewSlice[Tuple[string, int]]([]Tuple[string, int]{
				MakeTuple("1", 1), MakeTuple("2", 2),
				MakeTuple("3", 3), MakeTuple("4", 4),
				MakeTuple("5", 5), MakeTuple("6", 6),
			}).Stream()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ps, err := ConsumeTuples(iter).Read(ctx)
			check.NotError(t, err)
			assert.True(t, ps != nil)
			check.Equal(t, ps.Len(), 6)
		})
	})
	t.Run("Sorts", func(t *testing.T) {
		cmp := func(a, b Tuple[int, int]) bool {
			return a.One < b.One // && a.Value < b.Value
		}
		t.Run("Quick", func(t *testing.T) {
			list := randNumTupleListFixture(t, 100)
			ps := &Tuples[int, int]{ll: list}
			check.True(t, ft.Not(list.IsSorted(cmp)))
			ps.SortQuick(cmp)
			check.True(t, list.IsSorted(cmp))
		})
		t.Run("Merge", func(t *testing.T) {
			list := randNumTupleListFixture(t, 100)
			ps := &Tuples[int, int]{ll: list}
			check.True(t, ft.Not(list.IsSorted(cmp)))
			ps.SortMerge(cmp)
			check.True(t, list.IsSorted(cmp))
		})
	})
}

func randNumTupleListFixture(t *testing.T, size int) *List[Tuple[int, int]] {
	t.Helper()
	nums := rand.Perm(size)

	out := &List[Tuple[int, int]]{}
	for _, num := range nums {
		out.PushBack(MakeTuple(num, num))
	}
	assert.Equal(t, size, out.Len())
	return out
}
