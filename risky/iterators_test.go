package risky

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/internal"
)

type TestReadoneableImpl struct {
	fun.Iterator[string]
	once bool
}

func (iter *TestReadoneableImpl) ReadOne(ctx context.Context) (string, error) {
	if !iter.once {
		iter.once = true
		return "sparta", nil
	}
	return "", io.EOF
}

func TestIterator(t *testing.T) {
	t.Run("IterateOne", func(t *testing.T) {
		t.Run("First", func(t *testing.T) {
			it, err := IterateOne[int](internal.NewSliceIter([]int{101, 2, 34, 56}))
			assert.NotError(t, err)
			assert.Equal(t, 101, it)
		})

		t.Run("Empty", func(t *testing.T) {
			it, err := IterateOne[int](internal.NewSliceIter([]int{}))
			assert.Zero(t, it)
			assert.ErrorIs(t, err, io.EOF)
		})

		t.Run("ReadOneable", func(t *testing.T) {
			input := &TestReadoneableImpl{
				Iterator: internal.NewSliceIter([]string{
					fmt.Sprint(10),
					fmt.Sprint(10),
					fmt.Sprint(20),
					fmt.Sprint(2),
				}),
			}
			val, err := IterateOne[string](input)
			assert.NotError(t, err)
			assert.Equal(t, "sparta", val)

			val, err = IterateOne[string](input)
			assert.ErrorIs(t, err, io.EOF)
			assert.Zero(t, val)
		})
	})
	t.Run("Slice", func(t *testing.T) {
		out := make([]int, 100)
		for idx := range out {
			out[idx] = idx + rand.Intn(10*idx+1)
		}
		iter := internal.NewSliceIter(out)
		cpy := Slice[int](iter)
		assert.EqualItems(t, out, cpy)
	})
}
