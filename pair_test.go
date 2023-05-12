package fun

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestPairs(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		ps := MakePairs(map[string]string{"in": "out"})
		ps.Add("in", "in")
		ps.AddPair(MakePair("in", "in"))
		mp := ps.Map()
		assert.Equal(t, len(mp), 1)
		assert.Equal(t, len(ps), 3)
		assert.Equal(t, mp["in"], "out") // first value wins
	})
	t.Run("IterationOrder", func(t *testing.T) {
		ctx := testt.Context(t)

		ps := Pairs[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
		}

		assert.Equal(t, len(ps), 128)
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
	t.Run("Consume", func(t *testing.T) {
		t.Run("Prime", func(t *testing.T) {
			ctx := testt.Context(t)
			ps := Pairs[int, int]{}
			sp := Pairs[int, int]{}
			for i := 0; i < 128; i++ {
				ps.Add(i, i)
				sp.Add(i, i)
			}
			assert.Equal(t, len(ps), 128)
			ps.Consume(ctx, sp.Iterator())
			assert.Equal(t, len(ps), 256)
			assert.Equal(t, len(ps.Map()), 128)
		})
		t.Run("Slice", func(t *testing.T) {
			p := Pairs[string, int]{}
			p.ConsumeSlice([]int{1, 2, 3}, func(in int) string { return fmt.Sprint(in) })
			for idx := range p {
				check.Equal(t, p[idx].Key, fmt.Sprint(idx+1))
				check.Equal(t, p[idx].Value, idx+1)
			}
		})
		t.Run("Values", func(t *testing.T) {
			p := Pairs[string, int]{}
			p.ConsumeValues(
				testt.Context(t),
				internal.NewSliceIter([]int{1, 2, 3}),
				func(in int) string { return fmt.Sprint(in) },
			)
			assert.Equal(t, len(p), 3)
			for idx := range p {
				check.Equal(t, p[idx].Key, fmt.Sprint(idx+1))
				check.Equal(t, p[idx].Value, idx+1)
			}
		})
		t.Run("Map", func(t *testing.T) {
			p := Pairs[string, int]{}
			p.ConsumeMap(map[string]int{
				"1": 1,
				"2": 2,
				"3": 3,
			})
			assert.Equal(t, len(p), 3)
		})

	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Encode", func(t *testing.T) {
			ps := MakePairs(map[string]string{"in": "out"})
			out, err := json.Marshal(ps)
			assert.NotError(t, err)
			assert.Equal(t, string(out), `{"in":"out"}`)
		})
		t.Run("Decode", func(t *testing.T) {
			ps := Pairs[string, string]{}
			err := json.Unmarshal([]byte(`{"in":"out"}`), &ps)
			assert.NotError(t, err)
			assert.Equal(t, 1, len(ps))
			assert.Equal(t, "out", ps[0].Value)
			assert.Equal(t, "in", ps[0].Key)
		})
		t.Run("DecodeError", func(t *testing.T) {
			ps := Pairs[string, string]{}
			err := json.Unmarshal([]byte(`{"in":1}`), &ps)
			assert.Error(t, err)
			assert.Equal(t, 0, len(ps))
		})
	})
}
