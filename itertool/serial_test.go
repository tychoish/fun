package itertool

import (
	"context"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
)

func TestContains(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Exists", func(t *testing.T) {
		assert.True(t, Contains[int](ctx, 1, fun.SliceIterator([]int{12, 3, 44, 1})))
	})
	t.Run("NotExists", func(t *testing.T) {
		assert.True(t, !Contains[int](ctx, 1, fun.SliceIterator([]int{12, 3, 44})))
	})
}

func TestUniq(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sl := []int{1, 1, 2, 3, 5, 8, 9, 5}
	assert.Equal(t, fun.SliceIterator(sl).Count(ctx), 8)

	assert.Equal(t, Uniq(fun.SliceIterator(sl)).Count(ctx), 6)
}

func TestChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	num := []int{1, 2, 3, 5, 7, 9, 11, 13, 17, 19}
	iter := Chain[int](fun.SliceIterator(num), fun.SliceIterator(num))
	n := iter.Count(ctx)
	assert.Equal(t, len(num)*2, n)

	iter = Chain[int](fun.SliceIterator(num), fun.SliceIterator(num), fun.SliceIterator(num), fun.SliceIterator(num))
	cancel()
	n = iter.Count(ctx)
	assert.Equal(t, n, 0)
}

func TestDropZeros(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	all := make([]string, 100)
	n := fun.SliceIterator(all).Count(ctx)
	assert.Equal(t, 100, n)
	n = DropZeroValues[string](fun.SliceIterator(all)).Count(ctx)
	assert.Equal(t, 0, n)

	DropZeroValues[string](fun.SliceIterator(all)).Observe(ctx, func(in string) { assert.Zero(t, in) })

	all[45] = "49"
	n = DropZeroValues[string](fun.SliceIterator(all)).Count(ctx)
	assert.Equal(t, 1, n)
}
