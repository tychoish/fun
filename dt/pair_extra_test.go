package dt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestPairExtra(t *testing.T) {
	t.Run("Consume", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.PushMany(MakePair("1", 1), MakePair("2", 2), MakePair("3", 3))

			ps := p.Slice()
			for idx := range ps {
				check.Equal(t, ps[idx].Key, fmt.Sprint(idx+1))
				check.Equal(t, ps[idx].Value, idx+1)
			}
		})
		t.Run("List", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.PushMany(MakePair("1", 1), MakePair("2", 2), MakePair("3", 3))

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
		t.Run("Copy", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.PushMany(MakePair("1", 1), MakePair("2", 2), MakePair("3", 3))

			pl := p.Copy()
			check.NotEqual(t, pl, p)
			idx := 1
			for item := pl.ll.Front(); item.Ok(); item = item.Next() {
				check.Equal(t, item.Value().Value, idx)
				check.Equal(t, item.Value().Key, fmt.Sprint(idx))
				idx++
			}
			check.Equal(t, 4, idx)
		})
		t.Run("Map", func(t *testing.T) {
			p := &Pairs[string, int]{}
			p.AppendMap(map[string]int{
				"1": 1,
				"2": 2,
				"3": 3,
			})
			assert.Equal(t, p.Len(), 3)
		})
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("Encode", func(t *testing.T) {
			ps := NewMap(map[string]string{"in": "out"}).Pairs()
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
		t.Run("Empty", func(t *testing.T) {
			ps := Pairs[string, string]{}
			out, err := ps.MarshalJSON()
			assert.NotError(t, err)
			assert.Equal(t, string(out), "{}")
		})
		t.Run("ImpossibleValue", func(t *testing.T) {
			ps := MakePairs(Pair[string, context.CancelFunc]{"hi", func() {}})
			_, err := ps.MarshalJSON()
			assert.Error(t, err)
		})
		t.Run("ImpossibleKey", func(t *testing.T) {
			ps := MakePairs(Pair[badKey, string]{"hi", "hi"})
			_, err := ps.MarshalJSON()
			assert.Error(t, err)
		})
	})
	t.Run("FunctionalStreams", func(t *testing.T) {
		ctx := t.Context()

		num := 1000
		seen := &Set[int]{}

		ps, err := ConsumePairs(randNumPairListFixture(t, num).StreamPopFront()).Read(ctx)
		assert.NotError(t, err)
		err = ps.Stream().ReadAll(func(p Pair[int, int]) {
			check.Equal(t, p.Key, p.Value)
			check.True(t, ft.Not(seen.Check(p.Key)))
			seen.Add(p.Key)
		}).Run(ctx)
		assert.NotError(t, err)
		check.Equal(t, seen.Len(), num)
	})
	t.Run("Consume", func(t *testing.T) {
		ctx := t.Context()

		ps := Pairs[int, int]{}
		sp := Pairs[int, int]{}
		for i := 0; i < 128; i++ {
			ps.Add(i, i)
			sp.Add(i, i)
		}
		assert.Equal(t, ps.Len(), 128)
		assert.NotError(t, ps.AppendStream(sp.Stream()).Run(ctx))
		assert.Equal(t, ps.Len(), 256)
		mp := ps.Map()
		assert.Equal(t, len(mp), 128)
	})
}

type badKey string

func (badKey) MarshalJSON() ([]byte, error) { return nil, errors.New("cannot marshal") }
