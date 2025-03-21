package dt

import (
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestRing(t *testing.T) {
	t.Run("Smoke", func(t *testing.T) {
		ring := &Ring[int]{}

		t.Run("InsertOne", func(t *testing.T) {
			assert.Equal(t, ring.Len(), 0)
			ring.Push(42)
			assert.Equal(t, ring.Len(), 1)
		})
		t.Run("InsertSecond", func(t *testing.T) {
			ring.Push(84)
			assert.Equal(t, ring.Len(), 2)
		})
		t.Run("FIFO", func(t *testing.T) {
			items, err := ring.FIFO().Slice(t.Context())
			assert.NotError(t, err)
			t.Log(items)
			assert.Equal(t, len(items), ring.Len())
			// ordering:
			assert.Equal(t, items[0], 42)
			assert.Equal(t, items[1], 84)
		})
		t.Run("LIFO", func(t *testing.T) {
			items, err := ring.FIFO().Slice(t.Context())
			assert.NotError(t, err)
			t.Log(items)
			assert.Equal(t, len(items), ring.Len())
			// ordering:
			assert.Equal(t, items[0], 84)
			assert.Equal(t, items[1], 42)
		})

	})

}
