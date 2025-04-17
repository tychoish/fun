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
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/testt"
)

// When implementing the Map[K,V] type, I wrote a number of
// convenience functions for adding and modifying the map. As I was
// reviewing and documenting the code, these function, which are
// ergonomic are a bit confusing, and may hide that they're not
// strictly speaking atomic (e.g. the underlying map can be modified
// during the execution which may lead to unexpected semantics, but
// will never trigger the race detector.)
//
// If these operations were complicated or gnarly, I might have left
// them with sizeable disclaimers, but they're largely one-liners that
// would be simple to write for most users.
//
// Given that I'd already written tests, and they're not absurd, I
// thought it would be easier to leave them here in the tests with
// this note in case my (or someone elses!) thinking evolves.

// Join adds the keys of the input map to the current map. This uses a
// Range function from the input map, and may have inconsistent
// results if the input map is mutated during the operation. Keys and
// values from the input map will replace keys and values from the
// output map.
func (mp *Map[K, V]) Join(in *Map[K, V]) {
	in.Range(func(k K, v V) bool { mp.Store(k, v); return true })
}

// Append adds a sequence of pairs to the map.
func (mp *Map[K, V]) Append(its ...dt.Pair[K, V]) { mp.Extend(its) }

// Extend adds a slice of pairs to the map.
func (mp *Map[K, V]) Extend(its []dt.Pair[K, V]) {
	for _, it := range its {
		mp.Store(it.Key, it.Value)
	}
}

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
				for ctx.Err() == nil && !passed.Load() {
					for i := 0; i < 100; i++ {
						mp.Store(fmt.Sprint(i), rand.Int())
					}
					runtime.Gosched()
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ctx.Err() == nil && !passed.Load() {
					size := mp.Len()
					if size == 100 {
						passed.Store(true)
						cancel()
						return
					}
					runtime.Gosched()
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
					for i := 0; i < 300; i++ {
						mp.Delete(fmt.Sprint(i))
					}
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					time.Sleep(time.Millisecond)
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
			mp.Set(dt.Pair[int, int]{Key: i, Value: rand.Int() + 43})
		}

		for i := 0; i < 200; i++ {
			mp.Ensure(i)
		}
		assert.Equal(t, 200, mp.Len())
		iter := mp.Stream()
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
	t.Run("Stream", func(t *testing.T) {
		mp := &Map[int, int]{}
		mp.Default.SetConstructor(func() int { return 42 })
		for i := 0; i < 200; i++ {
			mp.Ensure(i)
		}
		assert.Equal(t, 200, mp.Len())
		iter := mp.Stream()

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
		assert.True(t, mp.Check("foo"))
		assert.True(t, mp.Check("bar"))
		assert.True(t, !mp.Check("baz"))
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
	t.Run("Append", func(t *testing.T) {
		mp := &Map[string, int]{}
		assert.Equal(t, 0, mp.Len())
		mp.Append(dt.MakePair("foo", 400))
		assert.Equal(t, 1, mp.Len())
		mp.Append(dt.MakePair("foo", 42), dt.MakePair("bar", 3))
		assert.Equal(t, 2, mp.Len())
		assert.Equal(t, 42, mp.Get("foo"))
		mp.Append(dt.MakePair("foofoo", 42), dt.MakePair("baz", 3))
		assert.Equal(t, 4, mp.Len())
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
		ok := mp.EnsureSet(dt.Pair[string, int]{Key: "hi", Value: 100})
		check.True(t, ok)
		ok = mp.EnsureSet(dt.Pair[string, int]{Key: "hi", Value: 100})
		check.True(t, !ok)
		ok = mp.EnsureSet(dt.Pair[string, int]{Key: "hi", Value: 10})
		check.True(t, !ok)
	})
	t.Run("Streams", func(t *testing.T) {
		t.Run("Keys", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Default.SetConstructor(func() int { return 38 })
			for i := 0; i < 100; i++ {
				mp.Ensure(fmt.Sprint(i))
			}

			ctx := testt.Context(t)
			assert.Equal(t, mp.Len(), 100)
			iter := mp.Keys()
			count := 0
			seen := map[string]struct{}{}
			for iter.Next(ctx) {
				count++
				seen[iter.Value()] = struct{}{}
			}
			assert.Equal(t, count, 100)
			assert.Equal(t, len(seen), 100)
		})
		t.Run("Values", func(t *testing.T) {
			mp := &Map[string, int]{}
			mp.Default.SetConstructor(func() int { return 38 })
			for i := 0; i < 100; i++ {
				mp.Ensure(fmt.Sprint(i))
			}

			ctx := testt.Context(t)
			assert.Equal(t, mp.Len(), 100)
			iter := mp.Values()
			count := 0
			for iter.Next(ctx) {
				count++
				assert.Equal(t, iter.Value(), 38)
			}
			assert.Equal(t, count, 100)
		})
	})
}
