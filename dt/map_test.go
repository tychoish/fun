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
	out := NewMap(make(map[string]int, size))
	for i := 0; len(out) < size; i++ {
		out[fmt.Sprint(rand.Intn((1+i)*1000000))] = i
	}
	return NewMap(out)
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
			orig := Map[string, int]{
				"1": 1,
				"2": 2,
				"3": 3,
			}
			mp := Map[string, int]{}
			assert.Equal(t, len(orig), 3)
			assert.Equal(t, len(mp), 0)
			mp.ConsumePairs(orig.Pairs())
			check.Equal(t, mp["1"], 1)
			check.Equal(t, mp["2"], 2)
			check.Equal(t, mp["3"], 3)

		})
		t.Run("Values", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mp := Map[string, int]{}
			err := mp.ConsumeValues(
				NewSlice([]int{1, 2, 3}).Iterator(),
				func(in int) string { return fmt.Sprint(in) },
			).Run(ctx)
			check.NotError(t, err)
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

		iter := NewMap(in).Iterator()
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
	t.Run("Helpers", func(t *testing.T) {
		assert.NotPanic(t, func() { Sliceify([]int{1, 2, 3}) })
		assert.NotPanic(t, func() { Mapify(map[string]int{"a": 1, "b": 2, "c": 3}) })
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
			err := keys.Observe(func(in string) {
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
			}).Run(ctx)
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
			err := keys.Observe(func(in int) {
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
			}).Run(ctx)
			assert.Equal(t, 3, count)
			assert.NotError(t, err)
		})
	})
	t.Run("Default", func(t *testing.T) {
		t.Run("EndToEnd", func(t *testing.T) {

			var nmp map[string]int
			mp := map[string]int{"one": 1}

			check.True(t, (nmp == nil) != (mp == nil))
			check.True(t, nmp == nil)
			check.True(t, mp != nil)

			check.Equal(t, len(mp), 1)
			check.Equal(t, len(nmp), 0)

			nvone := DefaultMap(nmp, 12)
			check.True(t, nvone != nil)
			check.Equal(t, len(nvone), 0)

			nmp = DefaultMap(mp, 12)
			check.Equal(t, nmp["one"], mp["one"])

		})
		t.Run("Passthrough", func(t *testing.T) {
			sl := DefaultMap[string, int](map[string]int{"one": 1}, 32)
			check.Equal(t, len(sl), 1)
			check.Equal(t, sl["one"], 1)
		})

		t.Run("ZeroLength", func(t *testing.T) {
			sl := DefaultMap[string, int](nil)
			check.Equal(t, len(sl), 0)
		})
		t.Run("LengthOnly", func(t *testing.T) {
			sl := DefaultMap[string, int](nil, 32)
			check.Equal(t, len(sl), 0)

		})
		t.Run("ExtraPanic", func(t *testing.T) {
			check.Panic(t, func() {
				_ = DefaultMap[string, int](nil, 16, -1)
			})
		})
	})
}
