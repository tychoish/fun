package fun

import (
	"encoding/json"
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestPairs(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		ps := MakePairs(map[string]string{"in": "out"})
		ps.Add("in", "in")
		mp := ps.Map()
		assert.Equal(t, len(mp), 1)
		assert.Equal(t, len(ps), 2)
		assert.Equal(t, mp["in"], "out") // first value wins
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
