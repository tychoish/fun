package dt

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
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
			assert.Equal(t, len(items), ring.Len())
			// ordering:
			check.Equal(t, items[0], 42)
			check.Equal(t, items[1], 84)
		})
		t.Run("LIFO", func(t *testing.T) {
			items, err := ring.LIFO().Slice(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, len(items), ring.Len())
			// ordering:
			assert.Equal(t, items[0], 84)
			assert.Equal(t, items[1], 42)
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			items, err := ring.LIFO().Slice(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, len(items), 0)
		})
	})
	t.Run("Zero", func(t *testing.T) {
		ring := &Ring[int]{}

		assert.Zero(t, ring.Head())
		assert.Zero(t, ring.Tail())
		assert.Equal(t, ring.Cap(), 1024)
		assert.Equal(t, 0, ring.Len())
	})

	t.Run("Setup", func(t *testing.T) {
		ring := &Ring[int]{}
		ring.Setup(2048)

		assert.Zero(t, ring.Head())
		assert.Zero(t, ring.Tail())
		assert.Equal(t, ring.Cap(), 2048)
		assert.Equal(t, 0, ring.Len())
	})
	t.Run("Overload", func(t *testing.T) {
		t.Run("Limit", func(t *testing.T) {
			ring := &Ring[int]{}
			ring.Setup(8)

			for range 16 {
				ring.Push(2)
			}

			assert.Equal(t, ring.Len(), 8)
			assert.Equal(t, ring.Cap(), 8)
			assert.Equal(t, ring.Total(), 16)
			assert.Equal(t, sum(ft.Must(ring.FIFO().Slice(t.Context()))), 2*8)
		})

		t.Run("Order", func(t *testing.T) {
			t.Run("FIFO", func(t *testing.T) {
				ring := &Ring[int]{}
				ring.Setup(5)

				for range 5 {
					for v := range 5 {
						ring.Push(v)
					}
				}

				assert.Equal(t, ring.Len(), 5)
				assert.Equal(t, ring.Cap(), 5)
				assert.Equal(t, ring.Total(), 25)

				fifo := ft.Must(ring.FIFO().Slice(t.Context()))
				expected := []int{0, 1, 2, 3, 4}
				for idx := range fifo {
					check.Equal(t, fifo[idx], expected[idx])
				}

			})
			t.Run("LIFO", func(t *testing.T) {
				ring := &Ring[int]{}
				ring.Setup(5)

				for range 5 {
					for v := range 5 {
						ring.Push(v)
					}
				}

				assert.Equal(t, ring.Len(), 5)
				assert.Equal(t, ring.Cap(), 5)
				assert.Equal(t, ring.Total(), 25)

				lifo := ft.Must(ring.LIFO().Slice(t.Context()))
				expected := []int{4, 3, 2, 1, 0}
				for idx := range lifo {
					check.Equal(t, lifo[idx], expected[idx])
				}
			})
		})
	})
	t.Run("Pop", func(t *testing.T) {
		ring := &Ring[int]{}

		for idx := range 5 {
			ring.Push(idx)
		}
		assert.Equal(t, ring.Len(), 5)

		assert.Equal(t, ring.Total(), 5)
		for idx := range 5 {
			v := ring.Pop()
			assert.NotNil(t, v)
			check.Equal(t, *v, idx)
		}
		assert.Equal(t, ring.Len(), 0)
		assert.Equal(t, ring.Total(), 5)
	})
	t.Run("OverPop", func(t *testing.T) {
		ring := &Ring[int]{}
		assert.Nil(t, ring.Pop())
	})
	t.Run("HeadAndTail", func(t *testing.T) {
		ring := &Ring[int]{}

		for idx := range 5 {
			ring.Push(idx)
			check.Equal(t, ring.Tail(), idx)
		}

		check.Equal(t, ring.Head(), 0)
		check.Equal(t, ring.Tail(), 4)
	})

}

func sum(in []int) int {
	out := 0
	for _, v := range in {
		out += v
	}
	return out
}
