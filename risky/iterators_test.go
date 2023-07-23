package risky

import (
	"math/rand"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/testt"
)

func TestIterator(t *testing.T) {
	t.Run("Slice", func(t *testing.T) {
		out := make([]int, 100)
		for idx := range out {
			out[idx] = idx + rand.Intn(10*idx+1)
		}
		iter := dt.Sliceify(out).Iterator()
		cpy := Slice(iter)
		assert.EqualItems(t, out, cpy)
	})
	t.Run("List", func(t *testing.T) {
		out := make([]int, 100)
		for idx := range out {
			out[idx] = idx + rand.Intn(10*idx+1)
		}
		iter := dt.Sliceify(out).Iterator()
		cpy := List(iter)
		testt.Log(t, len(out), cpy.Len())

		cpyiter := cpy.Iterator()
		cidx := 0
		ctx := testt.Context(t)
		for cpyiter.Next(ctx) {
			testt.Log(t, cidx, out[cidx], cpyiter.Value())
			assert.Equal(t, out[cidx], cpyiter.Value())
			cidx++
		}
		assert.Equal(t, cidx, 100)
	})
}
