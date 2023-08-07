package dt

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func makeMap(size int) Map[string, int] {
	out := Mapify(make(map[string]int, size))
	for i := 0; len(out) < size; i++ {
		out[fmt.Sprint(rand.Intn((1+i)*100000))] = i
	}
	return Mapify(out)
}

func TestMap(t *testing.T) {
	t.Run("ExpectedSize", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mp := makeMap(100)
		check.Equal(t, len(mp), 100)
		check.Equal(t, mp.Len(), 100)
		check.Equal(t, mp.Pairs().Len(), 100)
		check.Equal(t, mp.Iterator().Count(ctx), 100)
		check.Equal(t, mp.Keys().Count(ctx), 100)
		check.Equal(t, mp.Values().Count(ctx), 100)
		check.Equal(t, MapIterator(mp).Count(ctx), 100)
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mp := makeMap(100)

		// noop because same keys
		mp.Append(ft.Must(mp.Pairs().Iterator().Slice(ctx))...)
		check.Equal(t, mp.Len(), 100)

		// works because different keys, probably
		mp.Append(ft.Must(makeMap(100).Iterator().Slice(ctx))...)
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

		check.True(t, mp.Iterator().Count(ctx) == mp.Len())

		iter := mp.Iterator()
		check.True(t, iter.Next(ctx))

		cancel()

		check.True(t, !iter.Next(ctx))

		check.True(t, mp.Iterator().Count(ctx) < num)
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mp := Map[string, int]{}
			mp.ConsumeValues(ctx,
				Sliceify([]int{1, 2, 3}).Iterator(),
				func(in int) string { return fmt.Sprint(in) },
			)
			check.Equal(t, mp["1"], 1)
			check.Equal(t, mp["2"], 2)
			check.Equal(t, mp["3"], 3)

		})
	})
	t.Run("MapConverter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		in := map[string]string{
			"hi":  "there",
			"how": "are you doing",
		}
		iter := Mapify(in).Iterator()
		seen := 0
		for iter.Next(ctx) {
			item := iter.Value()
			switch {
			case item.Key == "hi":
				check.Equal(t, item.Value, "there")
			case item.Key == "how":
				check.Equal(t, item.Value, "are you doing")
			default:
				t.Errorf("unexpected value: %s", item)
			}
			seen++
		}
		assert.Equal(t, seen, len(in))
	})
	t.Run("Constructors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("MapKeys", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.Add("big", 42)
			mp.Add("small", 4)
			mp.Add("orange", 400)
			keys := MapKeys(mp)

			count := 0
			err := keys.Observe(ctx, func(in string) {
				switch in {
				case "big":
					count++
				case "small":
					count++
				case "orange":
					count++
				default:
					t.Error("unexpected", in)
				}
			})
			assert.Equal(t, 3, count)
			assert.NotError(t, err)
		})
		t.Run("MapKeys", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.Add("big", 42)
			mp.Add("small", 4)
			mp.Add("orange", 400)

			keys := MapValues(mp)

			count := 0
			err := keys.Observe(ctx, func(in int) {
				switch in {
				case 42:
					count++
				case 4:
					count++
				case 400:
					count++
				default:
					t.Error("unexpected", in)
				}
			})
			assert.Equal(t, 3, count)
			assert.NotError(t, err)
		})
	})

}
