package adt

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestMap(t *testing.T) {
	t.Parallel()
	t.Run("StoreItems", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mp := &Map[string, int]{}
		passed := &atomic.Bool{}
		wg := &fun.WaitGroup{}
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if ctx.Err() != nil || passed.Load() {
						return
					}

					for i := 0; i < 100; i++ {
						mp.Store(fmt.Sprint(i), rand.Int())
					}
					runtime.Gosched()
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if ctx.Err() != nil || passed.Load() {
						return
					}
					size := mp.Len()

					if size == 100 {
						passed.Store(true)
						cancel()
						return
					}
				}
			}()
		}
		wg.Wait(ctx)
		t.Log(mp.Len())
		assert.True(t, passed.Load())
	})
	t.Run("DeleteItems", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mp := &Map[string, int]{}
		passed := &atomic.Bool{}
		wg := &fun.WaitGroup{}
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if ctx.Err() != nil || passed.Load() {
						return
					}

					for i := 0; i < 300; i++ {
						mp.Store(fmt.Sprint(i), rand.Int())
					}
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(100 * time.Millisecond)
				for {
					if ctx.Err() != nil || passed.Load() {
						return
					}
					if mp.Len() == 0 {
						continue
					}
					for i := 0; i < 500; i++ {
						mp.Delete(fmt.Sprint(i))
					}
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					time.Sleep(time.Microsecond)
					if mp.Len() > 0 {
						break
					}
				}

				for {
					if ctx.Err() != nil || passed.Load() {
						return
					}
					if mp.Len() == 0 {
						passed.Store(true)
						cancel()
						return
					}
				}
			}()
		}
		wg.Wait(ctx)
		t.Log(mp.Len())
		assert.True(t, passed.Load())
	})
	t.Run("EnsureSemantics", func(t *testing.T) {
		mp := &Map[int, int]{}
		mp.Default.SetConstructor(func() int { return 42 })
		for i := 0; i < 100; i++ {
			mp.Set(MapItem[int, int]{Key: i, Value: rand.Int() + 43})
		}

		for i := 0; i < 200; i++ {
			mp.Ensure(i)
		}
		assert.Equal(t, 200, mp.Len())
		iter := mp.Iterator()
		ctx := testt.Context(t)
		count := 0
		for iter.Next(ctx) {
			count++
			pair := iter.Value()
			if pair.Key < 100 {
				assert.True(t, pair.Value >= 43)
				continue
			}
			assert.True(t, pair.Value == 42)
		}
		assert.NotError(t, iter.Close())
		assert.Equal(t, count, 200)
	})
	t.Run("Iterator", func(t *testing.T) {
		mp := &Map[int, int]{}
		mp.Default.SetConstructor(func() int { return 42 })
		for i := 0; i < 200; i++ {
			mp.Ensure(i)
		}
		assert.Equal(t, 200, mp.Len())
		iter := mp.Iterator()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		for iter.Next(ctx) {
			count++
			pair := iter.Value()
			assert.True(t, pair.Value == 42)
			if count == 100 {
				cancel()
			}
		}
		assert.NotError(t, iter.Close())
		assert.Equal(t, count, 100)
	})
	t.Run("Contains", func(t *testing.T) {
		mp := &Map[string, int]{}
		mp.Store("foo", 100)
		mp.Store("bar", 42)
		assert.Equal(t, 2, mp.Len())
		assert.True(t, mp.Contains("foo"))
		assert.True(t, mp.Contains("bar"))
		assert.True(t, !mp.Contains("baz"))
		assert.Equal(t, 2, mp.Len())
	})
	t.Run("Get", func(t *testing.T) {
		mp := &Map[string, int]{}
		assert.Zero(t, mp.Len())
		val := mp.Get("foo")
		assert.Zero(t, val)
		assert.Equal(t, 1, mp.Len())
		mp.Default.SetConstructor(func() int { return 42 })
		_ = mp.Default.Get()
		_ = mp.Default.Get()
		_ = mp.Default.Get()
		val = mp.Get("bar")
		assert.Equal(t, 42, val)
		val = mp.Get("foo")
		assert.Zero(t, val)
	})
	t.Run("Join", func(t *testing.T) {
		t.Run("Disjoint", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			mp.Store("bar", 100)
			mp2 := &Map[string, int]{}
			mp2.Store("foofoo", 100)
			mp2.Store("barfoo", 100)
			assert.Equal(t, 2, mp.Len())
			assert.Equal(t, 2, mp2.Len())

			// the op:
			mp.Join(mp2)

			assert.Equal(t, 2, mp2.Len())
			assert.Equal(t, 4, mp.Len())
		})
		t.Run("Overlapping", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			mp.Store("bar", 100)
			mp2 := &Map[string, int]{}
			mp2.Store("foo", 500)
			mp2.Store("baz", 100)
			assert.Equal(t, 2, mp.Len())
			assert.Equal(t, 2, mp2.Len())

			// the op:
			mp.Join(mp2)

			assert.Equal(t, 2, mp2.Len())
			assert.Equal(t, 3, mp.Len())
			assert.Equal(t, mp.Get("foo"), 500)
		})
	})
	t.Run("StoreFrom", func(t *testing.T) {
		t.Run("Disjoint", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			mp.Store("bar", 100)
			mp2 := &Map[string, int]{}
			mp2.Store("foofoo", 100)
			mp2.Store("barfoo", 100)
			assert.Equal(t, 2, mp.Len())
			assert.Equal(t, 2, mp2.Len())

			// the op:
			mp.StoreFrom(testt.Context(t), mp2.Iterator())

			assert.Equal(t, 2, mp2.Len())
			assert.Equal(t, 4, mp.Len())
		})
		t.Run("Overlapping", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			mp.Store("bar", 100)
			mp2 := &Map[string, int]{}
			mp2.Store("foo", 500)
			mp2.Store("baz", 100)
			assert.Equal(t, 2, mp.Len())
			assert.Equal(t, 2, mp2.Len())

			// the op:
			mp.StoreFrom(testt.Context(t), mp2.Iterator())

			assert.Equal(t, 2, mp2.Len())
			assert.Equal(t, 3, mp.Len())
			assert.Equal(t, mp.Get("foo"), 500)
		})
	})
	t.Run("Populate", func(t *testing.T) {
		t.Run("Disjoint", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			mp.Store("bar", 100)
			mp2 := map[string]int{
				"foofoo": 100,
				"barfoo": 100,
			}
			assert.Equal(t, 2, mp.Len())
			assert.Equal(t, 2, len(mp2))
			mp.Populate(mp2)
			assert.Equal(t, 4, mp.Len())
			assert.Equal(t, 2, len(mp2))
		})
		t.Run("Overlapping", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			mp.Store("bar", 100)
			mp2 := map[string]int{
				"foo": 500,
				"baz": 100,
			}
			assert.Equal(t, 2, mp.Len())
			assert.Equal(t, 2, len(mp2))
			mp.Populate(mp2)
			assert.Equal(t, 2, len(mp2))
			assert.Equal(t, 3, mp.Len())
			assert.Equal(t, mp.Get("foo"), 500)
		})
	})
	t.Run("Append", func(t *testing.T) {
		mp := &Map[string, int]{}
		assert.Equal(t, 0, mp.Len())
		mp.Append(NewMapItem("foo", 400))
		assert.Equal(t, 1, mp.Len())
		mp.Append(NewMapItem("foo", 42), NewMapItem("bar", 3))
		assert.Equal(t, 2, mp.Len())
		assert.Equal(t, 42, mp.Get("foo"))
		mp.Append(NewMapItem("foofoo", 42), NewMapItem("baz", 3))
		assert.Equal(t, 4, mp.Len())
	})
	t.Run("Export", func(t *testing.T) {
		mp := &Map[string, int]{}
		mp.Store("foo", 100)
		xp := mp.Export()
		assert.Equal(t, len(xp), 1)
		assert.Equal(t, xp["foo"], 100)
	})
	t.Run("JSON", func(t *testing.T) {
		t.Run("HappyPath", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Store("foo", 100)
			js, err := json.Marshal(mp)
			assert.NotError(t, err)
			assert.Equal(t, string(js), `{"foo":100}`)
			nmp := &Map[string, int]{}
			err = json.Unmarshal(js, nmp)
			assert.NotError(t, err)
			assert.Equal(t, nmp.Len(), 1)
			assert.Equal(t, nmp.Get("foo"), 100)
		})
		t.Run("Impossible", func(t *testing.T) {
			mp := &Map[string, int]{}
			err := json.Unmarshal([]byte(`{"foo": []}`), mp)
			assert.Error(t, err)
		})
	})

	t.Run("Ensure", func(t *testing.T) {
		mp := &Map[string, int]{}
		ok := mp.EnsureSet(MapItem[string, int]{Key: "hi", Value: 100})
		check.True(t, ok)
		ok = mp.EnsureSet(MapItem[string, int]{Key: "hi", Value: 100})
		check.True(t, !ok)
		ok = mp.EnsureSet(MapItem[string, int]{Key: "hi", Value: 10})
		check.True(t, !ok)
	})

}
