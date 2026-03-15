package adt

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/irt"
)

// siMap is the common interface for LockedMap[string,int] and LockedRWMap[string,int].
type siMap interface { //nolint:interfacebloat
	Len() int
	Check(string) bool
	Get(string) int
	Load(string) (int, bool)
	Delete(string)
	Set(string, int) bool
	Ensure(string) bool
	Store(string, int)
	Extend(iter.Seq2[string, int])
	Iterator() iter.Seq2[string, int]
	Keys() iter.Seq[string]
	Values() iter.Seq[int]
	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

func runLockedMapBasicTests(t *testing.T, newMap func() siMap) {
	t.Helper()

	t.Run("EmptyLen", func(t *testing.T) {
		assert.Equal(t, 0, newMap().Len())
	})

	t.Run("SetAndGet", func(t *testing.T) {
		mp := newMap()
		existed := mp.Set("key", 42)
		assert.True(t, !existed)
		assert.Equal(t, 42, mp.Get("key"))
	})

	t.Run("SetReturnsExisted", func(t *testing.T) {
		mp := newMap()
		assert.True(t, !mp.Set("k", 1))
		assert.True(t, mp.Set("k", 2))
		assert.Equal(t, 2, mp.Get("k"))
	})

	t.Run("GetMissing", func(t *testing.T) {
		mp := newMap()
		assert.Equal(t, 0, mp.Get("missing"))
	})

	t.Run("LoadMissing", func(t *testing.T) {
		mp := newMap()
		v, ok := mp.Load("missing")
		assert.True(t, !ok)
		assert.Equal(t, 0, v)
	})

	t.Run("LoadPresent", func(t *testing.T) {
		mp := newMap()
		mp.Store("k", 99)
		v, ok := mp.Load("k")
		assert.True(t, ok)
		assert.Equal(t, 99, v)
	})

	t.Run("Check", func(t *testing.T) {
		mp := newMap()
		assert.True(t, !mp.Check("absent"))
		mp.Store("present", 1)
		assert.True(t, mp.Check("present"))
		assert.True(t, !mp.Check("absent"))
	})

	t.Run("Delete", func(t *testing.T) {
		mp := newMap()
		mp.Store("k", 7)
		assert.Equal(t, 1, mp.Len())
		mp.Delete("k")
		assert.Equal(t, 0, mp.Len())
		assert.True(t, !mp.Check("k"))
	})

	t.Run("DeleteMissing", func(t *testing.T) {
		mp := newMap()
		mp.Delete("nope") // must not panic
		assert.Equal(t, 0, mp.Len())
	})

	t.Run("Len", func(t *testing.T) {
		mp := newMap()
		for i := range 50 {
			mp.Store(fmt.Sprint(i), i)
		}
		assert.Equal(t, 50, mp.Len())
	})

	t.Run("EnsureSemantics", func(t *testing.T) {
		mp := newMap()
		for i := range 100 {
			mp.Store(fmt.Sprint(i), i+1)
		}
		for i := range 200 {
			mp.Ensure(fmt.Sprint(i))
		}
		assert.Equal(t, 200, mp.Len())
		for i := range 100 {
			v, ok := mp.Load(fmt.Sprint(i))
			assert.True(t, ok)
			assert.Equal(t, i+1, v) // existing values are preserved
		}
		for i := 100; i < 200; i++ {
			v, ok := mp.Load(fmt.Sprint(i))
			assert.True(t, ok)
			assert.Equal(t, 0, v) // new keys get zero value
		}
	})

	t.Run("EnsureReturnValue", func(t *testing.T) {
		mp := newMap()
		assert.True(t, !mp.Ensure("new")) // first call: key did not exist
		assert.True(t, mp.Ensure("new"))  // second call: key already exists
		assert.Equal(t, 0, mp.Get("new"))
	})

	t.Run("Extend", func(t *testing.T) {
		mp := newMap()
		pairs := func(yield func(string, int) bool) {
			for i := range 10 {
				if !yield(fmt.Sprint(i), i*10) {
					return
				}
			}
		}
		mp.Extend(pairs)
		assert.Equal(t, 10, mp.Len())
		for i := range 10 {
			assert.Equal(t, i*10, mp.Get(fmt.Sprint(i)))
		}
	})

	t.Run("Iterator", func(t *testing.T) {
		mp := newMap()
		for i := range 20 {
			mp.Store(fmt.Sprint(i), i)
		}
		assert.Equal(t, 20, mp.Len())

		result := irt.Collect2(mp.Iterator())
		assert.Equal(t, 20, len(result))
		for i := range 20 {
			v, ok := result[fmt.Sprint(i)]
			assert.True(t, ok)
			assert.Equal(t, i, v)
		}
	})

	t.Run("Keys", func(t *testing.T) {
		mp := newMap()
		for i := range 30 {
			mp.Store(fmt.Sprint(i), i)
		}

		seen := map[string]struct{}{}
		for k := range mp.Keys() {
			seen[k] = struct{}{}
		}
		assert.Equal(t, 30, len(seen))
		for i := range 30 {
			_, ok := seen[fmt.Sprint(i)]
			assert.True(t, ok)
		}
	})

	t.Run("Values", func(t *testing.T) {
		mp := newMap()
		for i := range 25 {
			mp.Store(fmt.Sprint(i), 0)
		}

		count := 0
		for v := range mp.Values() {
			count++
			assert.Equal(t, 0, v)
		}
		assert.Equal(t, 25, count)
	})
}

func runLockedMapJSONTests(t *testing.T, newMap func() siMap) {
	t.Helper()

	t.Run("HappyPath", func(t *testing.T) {
		mp := newMap()
		mp.Store("foo", 100)
		js, err := mp.MarshalJSON()
		assert.NotError(t, err)
		assert.Equal(t, string(js), `{"foo":100}`)

		nmp := newMap()
		err = nmp.UnmarshalJSON(js)
		assert.NotError(t, err)
		assert.Equal(t, 1, nmp.Len())
		assert.Equal(t, 100, nmp.Get("foo"))
	})

	t.Run("Impossible", func(t *testing.T) {
		mp := newMap()
		err := mp.UnmarshalJSON([]byte(`{"foo": []}`))
		assert.Error(t, err)
	})

	t.Run("RoundTrip", func(t *testing.T) {
		mp := newMap()
		for i := range 5 {
			mp.Store(fmt.Sprint(i), i*2)
		}
		js, err := json.Marshal(mp)
		assert.NotError(t, err)

		nmp := newMap()
		assert.NotError(t, json.Unmarshal(js, nmp))
		assert.Equal(t, 5, nmp.Len())
		for i := range 5 {
			assert.Equal(t, i*2, nmp.Get(fmt.Sprint(i)))
		}
	})
}

func runLockedMapConcurrentTests(t *testing.T, newMap func() siMap) {
	t.Helper()

	t.Run("ConcurrentStoreItems", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mp := newMap()
		passed := &atomic.Bool{}
		wg := &sync.WaitGroup{}
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ctx.Err() == nil && !passed.Load() {
					for i := range 100 {
						mp.Set(fmt.Sprint(i), rand.Int())
					}
					runtime.Gosched()
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ctx.Err() == nil && !passed.Load() {
					if mp.Len() == 100 {
						passed.Store(true)
						cancel()
						return
					}
					runtime.Gosched()
				}
			}()
		}
		wg.Wait()
		assert.True(t, passed.Load())
	})

	t.Run("ConcurrentDeleteAfterPopulate", func(t *testing.T) {
		t.Parallel()
		const n = 200
		mp := newMap()
		for i := range n {
			mp.Store(fmt.Sprint(i), i)
		}
		assert.Equal(t, n, mp.Len())

		wg := &sync.WaitGroup{}
		for range runtime.NumCPU() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range n {
					mp.Delete(fmt.Sprint(i))
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, 0, mp.Len())
	})

	t.Run("ConcurrentStoreAndDelete", func(t *testing.T) {
		t.Parallel()
		const (
			workers = 8
			keys    = 50
			rounds  = 500
		)
		mp := newMap()
		wg := &sync.WaitGroup{}
		for i := range workers {
			wg.Add(1)
			if i%2 == 0 {
				go func() {
					defer wg.Done()
					for r := range rounds {
						mp.Store(fmt.Sprint(r%keys), r)
					}
				}()
			} else {
				go func() {
					defer wg.Done()
					for r := range rounds {
						mp.Delete(fmt.Sprint(r % keys))
					}
				}()
			}
		}
		wg.Wait()
		assert.True(t, mp.Len() <= keys)
	})

	t.Run("ConcurrentIterator", func(t *testing.T) {
		t.Parallel()
		mp := newMap()
		for i := range 100 {
			mp.Store(fmt.Sprint(i), i)
		}

		wg := &sync.WaitGroup{}
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				count := 0
				for range mp.Iterator() {
					count++
				}
				assert.Equal(t, 100, count)
			}()
		}
		wg.Wait()
	})
}

func TestLockedMap(t *testing.T) {
	t.Parallel()
	newMap := func() siMap { return &LockedMap[string, int]{} }

	t.Run("BasicOperations", func(t *testing.T) { runLockedMapBasicTests(t, newMap) })
	t.Run("JSON", func(t *testing.T) { runLockedMapJSONTests(t, newMap) })
	t.Run("Concurrent", func(t *testing.T) { runLockedMapConcurrentTests(t, newMap) })
}

func TestLockedRWMap(t *testing.T) {
	t.Parallel()
	newMap := func() siMap { return &LockedRWMap[string, int]{} }

	t.Run("BasicOperations", func(t *testing.T) { runLockedMapBasicTests(t, newMap) })
	t.Run("JSON", func(t *testing.T) { runLockedMapJSONTests(t, newMap) })
	t.Run("Concurrent", func(t *testing.T) { runLockedMapConcurrentTests(t, newMap) })
}
