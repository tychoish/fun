package dt

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestTupleExtra(t *testing.T) {
	t.Run("JSON", func(t *testing.T) {
		t.Run("Encode", func(t *testing.T) {
			ps := NewMap(map[string]string{"in": "out"}).Tuples()
			out, err := json.Marshal(ps)
			check.NotError(t, err)
			check.Equal(t, string(out), `[["in","out"]]`)
		})
		t.Run("EncodingEdges", func(t *testing.T) {
			ps := MakeTuples(MakeTuple("in", "out"), MakeTuple("second", ""))
			out, err := json.Marshal(ps)
			check.NotError(t, err)
			check.Equal(t, string(out), `[["in","out"],["second",""]]`)
		})
		t.Run("DecodingEdges", func(t *testing.T) {
			t.Run("MissingElement", func(t *testing.T) {
				ps := &Tuples[string, string]{}
				err := json.Unmarshal([]byte(`[["in","out"],["second"]]`), ps)
				check.NotError(t, err)
				assert.Equal(t, ps.Len(), 2)
				sl := ps.Slice()
				check.Equal(t, sl[1].One, "second")
				check.Equal(t, sl[1].Two, "")
			})
			t.Run("ExtraElement", func(t *testing.T) {
				ps := &Tuples[string, string]{}
				err := json.Unmarshal([]byte(`[["in","out","beyond"]]`), ps)
				check.Error(t, err)
			})
		})
		t.Run("EncodeLong", func(t *testing.T) {
			ps := MakeTuples(MakeTuple("in", "out"), MakeTuple("out", "in"))
			out, err := json.Marshal(ps)
			check.NotError(t, err)
			check.Equal(t, string(out), `[["in","out"],["out","in"]]`)
		})
		t.Run("Decode", func(t *testing.T) {
			ps := &Tuples[string, string]{}
			err := json.Unmarshal([]byte(`[["in","out"]]`), &ps)
			check.NotError(t, err)
			psl := ps.Slice()
			check.Equal(t, 1, len(psl))
			assert.True(t, len(psl) >= 1) // avoid panic
			check.Equal(t, "in", psl[0].One)
			check.Equal(t, "out", psl[0].Two)
		})
		t.Run("DecodeError", func(t *testing.T) {
			ps := &Tuples[string, int]{}
			err := json.Unmarshal([]byte(`[["in","out"]]`), &ps)
			check.Error(t, err)
			check.Equal(t, 0, ps.Len())
			if t.Failed() {
				t.Log(ps.ll.Front().Value())
			}
		})
		t.Run("Empty", func(t *testing.T) {
			ps := Tuples[string, string]{}
			out, err := ps.MarshalJSON()
			check.NotError(t, err)
			check.Equal(t, string(out), "[]")
		})

		t.Run("ImpossibleTwo", func(t *testing.T) {
			ps := MakeTuples[string, context.CancelFunc](Tuple[string, context.CancelFunc]{"hi", func() {}})
			_, err := ps.MarshalJSON()
			check.Error(t, err)
			check.Error(t, ps.UnmarshalJSON([]byte(`["hihi",4]`)))
		})
		t.Run("ImpossibleOne", func(t *testing.T) {
			ps := MakeTuples[context.CancelFunc, string](Tuple[context.CancelFunc, string]{func() {}, "hi"})
			_, err := ps.MarshalJSON()
			check.Error(t, err)
			check.Error(t, ps.UnmarshalJSON([]byte(`[5,"hihi"]`)))
		})
		t.Run("BadType", func(t *testing.T) {
			ps := MakeTuples[badKey, string](Tuple[badKey, string]{"hi", "hi"})
			_, err := ps.MarshalJSON()
			check.Error(t, err)
		})
		t.Run("BadTargetType", func(t *testing.T) {
			ps := MakeTuples[badKey, string](Tuple[badKey, string]{})
			check.Error(t, ps.UnmarshalJSON([]byte(`["hi","hi"]`)))
		})
		t.Run("TooManyItems", func(t *testing.T) {
			ps := Tuples[string, string]{}
			check.Error(t, ps.UnmarshalJSON([]byte(`["hi","hihi","hihihi"]`)))
		})
		t.Run("BadTypes", func(t *testing.T) {
			t.Run("One", func(t *testing.T) {
				ps := Tuple[func() int, int]{}
				check.Error(t, ps.UnmarshalJSON([]byte(`[1,1]`)))
			})
			t.Run("Two", func(t *testing.T) {
				ps := Tuple[int, func() int]{}
				check.Error(t, ps.UnmarshalJSON([]byte(`[1,1]`)))
			})
		})
		t.Run("InvalidDecodingInput", func(t *testing.T) {
			ps := Tuples[string, string]{}
			check.Error(t, ps.UnmarshalJSON([]byte(`"hi","hihi","hihihi"`)))
			check.Error(t, ps.UnmarshalJSON([]byte(`8`)))
			check.Error(t, ps.UnmarshalJSON([]byte(`{"one":2}`)))
		})
	})
	t.Run("Copy", func(t *testing.T) {
		tp := &Tuples[string, int]{}
		tp.Add("one", 1).Add("two", 2).Add("three", 3)
		tpc := tp.Copy()
		check.True(t, reflect.DeepEqual(tp.ll, tpc.ll))

		elem := tp.ll.Front()
		elem.item.Two = 0
		elem.next.item.Two = 1
		elem.next.next.item.Two = 2

		check.True(t, ft.Not(reflect.DeepEqual(tp.ll, tpc.ll)))

	})

}
