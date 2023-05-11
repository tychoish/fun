package fun

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/tychoish/fun/assert/check"
)

func makeMap(size int) Map[string, int] {
	out := Mapify(make(map[string]int, size))
	for i := 0; len(out) < size; i++ {

		out[fmt.Sprint(rand.Intn((1+i)*1000))] = i
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

		mp.Merge(mp)
		check.Equal(t, mp.Len(), 100)

		mp.Merge(makeMap(100))
		check.Equal(t, mp.Len(), 200)

	})
	t.Run("Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		num := int(time.Microsecond)
		mp := makeMap(num)

		go cancel()

		check.True(t, Count(ctx, mp.Iterator()) < num)
	})

}
