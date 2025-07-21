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
		check.Equal(t, mp.Stream().Count(ctx), 100)
		check.Equal(t, mp.Keys().Count(ctx), 100)
		check.Equal(t, mp.Values().Count(ctx), 100)
	})
	t.Run("Extend", func(t *testing.T) {
		mp := makeMap(100)

		// noop because same keys
		mp.AppendPairs(mp.Pairs())
		check.Equal(t, mp.Len(), 100)

		// works because different keys, probably
		mp.AppendPairs(makeMap(100).Pairs())
		check.Equal(t, mp.Len(), 200)
	})
	t.Run("Append", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mp := makeMap(100)

		// noop because same keys
		mp.Append(ft.Must(mp.Pairs().Stream().Slice(ctx))...)
		check.Equal(t, mp.Len(), 100)

		// works because different keys, probably
		mp.Append(ft.Must(makeMap(100).Stream().Slice(ctx))...)
		check.Equal(t, mp.Len(), 200)
	})
	t.Run("Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		num := int(time.Microsecond)
		mp := makeMap(num)

		check.True(t, mp.Stream().Count(ctx) == mp.Len())

		iter := mp.Stream()
		check.True(t, iter.Next(ctx))

		cancel()

		check.True(t, !iter.Next(ctx))

		check.True(t, mp.Stream().Count(ctx) < num)
	})
	t.Run("SetDefaultAndCheck", func(t *testing.T) {
		mp := Map[string, bool]{}
		assert.True(t, !mp.Check("hi"))
		assert.True(t, !mp.Check("kip"))
		assert.True(t, !mp.Check("buddy"))
		mp.SetDefault("hi")
		mp.SetDefault("kip")
		mp.SetDefault("buddy")
		assert.True(t, mp.Check("hi"))
		assert.True(t, mp.Check("kip"))
		assert.True(t, mp.Check("buddy"))
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
		orig := Map[string, int]{
			"1": 1,
			"2": 2,
			"3": 3,
		}
		mp := Map[string, int]{}
		assert.Equal(t, len(orig), 3)
		assert.Equal(t, len(mp), 0)
		mp.AppendPairs(orig.Pairs())
		check.Equal(t, mp["1"], 1)
		check.Equal(t, mp["2"], 2)
		check.Equal(t, mp["3"], 3)

	})
	t.Run("MapConverter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		in := map[string]string{
			"hi":  "there",
			"how": "are you doing",
		}

		iter := NewMap(in).Stream()
		seen := 0
		for iter.Next(ctx) {
			item := iter.Value()
			switch item.Key {
			case "hi":
				check.Equal(t, item.Value, "there")
			case "how":
				check.Equal(t, item.Value, "are you doing")
			default:
				t.Errorf("unexpected value: %s", item)
			}
			seen++
		}
		assert.Equal(t, seen, len(in))
	})
	t.Run("Helpers", func(t *testing.T) {
		assert.NotPanic(t, func() { NewSlice([]int{1, 2, 3}) })
		assert.NotPanic(t, func() { NewMap(map[string]int{"a": 1, "b": 2, "c": 3}) })
	})
	t.Run("ConsumeStream", func(t *testing.T) {
		source := makeMap(300)
		target := NewMap(map[string]int{})
		assert.NotError(t, target.AppendStream(source.Stream()).Run(t.Context()))
		assert.Equal(t, source.Len(), target.Len())
		for pair := range source.Stream().Iterator(t.Context()) {
			assert.True(t, target.Check(pair.Key))
			assert.Equal(t, pair.Value, target.Get(pair.Key))
		}
	})
	t.Run("Constructors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("MapKeys", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.Add("big", 42)
			mp.Add("small", 4)
			mp.Add("orange", 400)
			keys := mp.Keys()

			count := 0
			err := keys.ReadAll(func(in string) {
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
		t.Run("MapValues", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.Add("big", 42)
			mp.Add("small", 4)
			mp.Add("orange", 400)

			keys := mp.Values()

			count := 0
			err := keys.ReadAll(func(in int) {
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
			sl := DefaultMap(map[string]int{"one": 1}, 32)
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
	t.Run("Delete", func(t *testing.T) {
		mp := Map[string, int]{}
		mp.Add("hi", 1)
		assert.Equal(t, mp.Len(), 1)
		mp.Delete("boo")
		assert.Equal(t, mp.Len(), 1)
		mp.Delete("hi")
		assert.Equal(t, mp.Len(), 0)
	})

}
