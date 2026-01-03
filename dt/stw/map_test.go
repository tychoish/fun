package stw

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/irt"
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
		mp := makeMap(100)
		check.Equal(t, len(mp), 100)
		check.Equal(t, mp.Len(), 100)
	})
	t.Run("SetDefaultAndCheck", func(t *testing.T) {
		mp := Map[string, bool]{}
		assert.True(t, !mp.Check("hi"))
		assert.True(t, !mp.Check("kip"))
		assert.True(t, !mp.Check("buddy"))
		mp.Ensure("hi")
		mp.Ensure("kip")
		mp.Ensure("buddy")
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

		mp.Ensure("foo")
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
		mp.Extend(orig.Iterator())
		check.Equal(t, mp["1"], 1)
		check.Equal(t, mp["2"], 2)
		check.Equal(t, mp["3"], 3)
	})
	t.Run("MapConverter", func(t *testing.T) {
		in := map[string]string{
			"hi":  "there",
			"how": "are you doing",
		}

		mp := NewMap(in)
		seen := 0
		for key, value := range mp.Iterator() {
			switch key {
			case "hi":
				check.Equal(t, value, "there")
			case "how":
				check.Equal(t, value, "are you doing")
			default:
				t.Errorf("unexpected pair: %s, %s", key, value)
			}
			seen++
		}
		assert.Equal(t, seen, len(in))
	})
	t.Run("Helpers", func(t *testing.T) {
		assert.NotPanic(t, func() { NewSlice([]int{1, 2, 3}) })
		assert.NotPanic(t, func() { NewMap(map[string]int{"a": 1, "b": 2, "c": 3}) })
	})
	t.Run("Constructors", func(t *testing.T) {
		t.Run("MapKeys", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.Store("big", 42)
			mp.Store("small", 4)
			mp.Store("orange", 400)
			keys := mp.Keys()

			count := 0
			report := irt.Apply(keys, func(in string) {
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
			assert.Equal(t, 3, report)
		})
		t.Run("MapValues", func(t *testing.T) {
			mp := Map[string, int]{}
			mp.Set("big", 42)
			mp.Set("small", 4)
			mp.Set("orange", 400)

			keys := mp.Values()

			count := 0
			report := irt.Apply(keys, func(in int) {
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
			assert.Equal(t, 3, report)
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
		mp.Set("hi", 1)
		assert.Equal(t, mp.Len(), 1)
		mp.Delete("boo")
		assert.Equal(t, mp.Len(), 1)
		mp.Delete("hi")
		assert.Equal(t, mp.Len(), 0)
	})
}
