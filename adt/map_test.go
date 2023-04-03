package adt

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
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
		mp.DefaultConstructor.Constructor.Set(func() int { return 42 })
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
		mp.DefaultConstructor.Constructor.Set(func() int { return 42 })
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

}
