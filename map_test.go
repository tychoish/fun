package fun

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
)

func makeMap(size int) Map[string, int] {
	out := Mapify(make(map[string]int, size))
	for i := 0; len(out) < size; i++ {

		out[fmt.Sprint(rand.Intn((1+i)*100000))] = i
	}
	return Mapify(out)
}

func TestMap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("ExpectedSize", func(t *testing.T) {
		mp := makeMap(100)
		check.Equal(t, len(mp), 100)
		check.Equal(t, mp.Len(), 100)
		check.Equal(t, len(mp.Pairs()), 100)
		check.Equal(t, Count(ctx, mp.Iterator()), 100)
		check.Equal(t, Count(ctx, mp.Keys()), 100)
		check.Equal(t, Count(ctx, mp.Values()), 100)
	})
	t.Run("Extend", func(t *testing.T) {
		mp := makeMap(100)

		// noop because same keys
		mp.Extend(mp.Pairs())
		check.Equal(t, mp.Len(), 100)

		// works because different keys, probably
		mp.Extend(makeMap(100).Pairs())
		check.Equal(t, mp.Len(), 200)
	})
	t.Run("Append", func(t *testing.T) {
		mp := makeMap(100)

		// noop because same keys
		mp.Append(mp.Pairs()...)
		check.Equal(t, mp.Len(), 100)

		// works because different keys, probably
		mp.Append(makeMap(100).Pairs()...)
		check.Equal(t, mp.Len(), 200)
	})
	t.Run("Merge", func(t *testing.T) {
		mp := makeMap(100)

		mp.ConsumeMap(mp)
		check.Equal(t, mp.Len(), 100)

		mp.ConsumeMap(makeMap(100))
		check.Equal(t, mp.Len(), 200)

	})
	t.Run("Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		num := int(time.Microsecond)
		mp := makeMap(num)

		cancel()

		check.True(t, Count(ctx, mp.Iterator()) < num)
	})
	t.Run("SetDefaultAndCheck", func(t *testing.T) {
		mp := Map[string, bool]{}
		assert.True(t, !mp.Check("hi"))
		assert.True(t, !mp.Check("kip"))
		assert.True(t, !mp.Check("merlin"))
		mp.SetDefault("hi")
		mp.SetDefault("kip")
		mp.SetDefault("merlin")
		assert.True(t, mp.Check("hi"))
		assert.True(t, mp.Check("kip"))
		assert.True(t, mp.Check("merlin"))
	})
	t.Run("GetAndLoad", func(t *testing.T) {
		mp := Map[string, bool]{}
		assert.True(t, !mp.Get("foo"))
		assert.Equal(t, mp.Len(), 0)
		v, ok := mp.Load("foo")
		assert.True(t, !v)
		assert.True(t, !ok)

		mp.SetDefault("foo")
		assert.True(t, !mp.Get("foo"))

		v, ok = mp.Load("foo")
		assert.True(t, !v)
		assert.True(t, ok)
	})

	t.Run("Consume", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.ConsumeSlice([]int{1, 2, 3}, func(in int) string { return fmt.Sprint(in) })
			check.Equal(t, mp["1"], 1)
			check.Equal(t, mp["2"], 2)
			check.Equal(t, mp["3"], 3)
		})
		t.Run("Prine", func(t *testing.T) {
			orig := Map[string, int]{
				"1": 1,
				"2": 2,
				"3": 3,
			}
			mp := Map[string, int]{}
			iter := orig.Iterator()
			assert.Equal(t, len(orig), 3)
			assert.Equal(t, len(mp), 0)
			mp.Consume(ctx, iter)
			check.Equal(t, mp["1"], 1)
			check.Equal(t, mp["2"], 2)
			check.Equal(t, mp["3"], 3)

		})
		t.Run("Values", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.ConsumeValues(ctx,
				internal.NewSliceIter([]int{1, 2, 3}),
				func(in int) string { return fmt.Sprint(in) },
			)
			check.Equal(t, mp["1"], 1)
			check.Equal(t, mp["2"], 2)
			check.Equal(t, mp["3"], 3)

		})
	})

}
