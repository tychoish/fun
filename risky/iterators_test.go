package risky

import (
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
)

func TestStream(t *testing.T) {
	t.Run("Slice", func(t *testing.T) {
		out := make([]int, 100)
		for idx := range out {
			out[idx] = idx + rand.Intn(10*idx+1)
		}
		iter := fun.SliceStream(out)
		cpy := Slice(iter)
		assert.EqualItems(t, out, cpy)
	})
}
