package adt

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestLocked(t *testing.T) {
	t.Run("Stress", func(t *testing.T) {
		const num = 30
		val := NewSynchronized([]int{})
		seed := make([]int, num)
		for idx := range seed {
			seed[idx] = num
		}
		val.Set(seed)
		second := val.Get()

		assert.Equal(t, len(seed), len(second))
		for idx := range seed {
			assert.Equal(t, seed[idx], second[idx])
		}

		wg := &sync.WaitGroup{}
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				count := 0
				for {
					if count > 500 {
						return
					}

					count++
					time.Sleep(time.Millisecond)
					val.With(func(in []int) {
						check.True(t, len(in) == 30)
						check.NotEqual(t, in[id], 0)
						in[id] = rand.Intn(30*100) + 1
					})
				}
			}(i)
		}

		wg.Wait()
	})
	t.Run("Swaps", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			item := &Synchronized[int]{}
			assert.Equal(t, item.Get(), 0)
			item.Set(42)
			assert.Equal(t, item.Get(), 42)
			assert.Equal(t, item.String(), "42")
			prev := item.Swap(100)
			assert.Equal(t, prev, 42)
			assert.Equal(t, item.Get(), 100)
		})
		t.Run("Compare", func(t *testing.T) {
			item := &Synchronized[int]{}
			assert.True(t, CompareAndSwap[int](item, 0, 100))
			assert.Equal(t, item.Get(), 100)
			assert.True(t, !CompareAndSwap[int](item, 0, 42))
			assert.True(t, CompareAndSwap[int](item, 100, 42))
			assert.Equal(t, item.Get(), 42)
		})
	})
	t.Run("WithLockDemo", func(t *testing.T) {
		// uncomment and run this test to watch:
		// t.Fail()
		with := func(mtx *sync.Mutex) { t.Log("2:before unlock"); mtx.Unlock(); t.Log("3:after unlock") }
		lock := func(mtx *sync.Mutex) *sync.Mutex { mtx.Lock(); t.Log("1:lock"); return mtx }
		mtx := &sync.Mutex{}
		t.Log("before")
		with(lock(mtx))
		t.Log("middle")
		a := lock(mtx)
		with(a)
		t.Log("end; defering")
		defer with(lock(mtx))
		t.Log("defered")
	})
	t.Run("Accessors", func(t *testing.T) {
		t.Run("Mutex", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var getter fun.Future[int]
			var setter fun.Handler[int]
			getCalled := &atomic.Bool{}
			setCalled := &atomic.Bool{}

			getter = func() int { getCalled.Store(true); return 42 }
			setter = func(in int) { setCalled.Store(true); check.Equal(t, in, 267); time.Sleep(100 * time.Millisecond) }
			mget, mset := AccessorsWithLock(getter, setter)
			start := time.Now()
			sw := mset.Worker(267).Launch(ctx)
			runtime.Gosched()
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				check.MinRuntime(t, 100*time.Millisecond, func() { check.Equal(t, 42, mget()) })
				check.NotError(t, sw(ctx))
				if time.Since(start) < 100*time.Millisecond {
					t.Log(time.Since(start))
				}
			}()

			<-sig
			check.True(t, getCalled.Load())
			check.True(t, setCalled.Load())
		})
		t.Run("RWMutex", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var getter fun.Future[int]
			var setter fun.Handler[int]
			getCalled := &atomic.Bool{}
			setCalled := &atomic.Bool{}

			getter = func() int { getCalled.Store(true); return 42 }
			setter = func(in int) { setCalled.Store(true); check.Equal(t, in, 267); time.Sleep(100 * time.Millisecond) }
			mget, mset := AccessorsWithReadLock(getter, setter)
			start := time.Now()
			sw := mset.Worker(267).Launch(ctx)
			runtime.Gosched()
			wg := &sync.WaitGroup{}
			wg.Add(2)
			go func() {
				defer wg.Done()
				check.NotError(t, sw(ctx))
				if time.Since(start) < 100*time.Millisecond {
					t.Log(time.Since(start))
				}
			}()
			go func() {
				defer wg.Done()
				check.Equal(t, mget(), 42)
				if time.Since(start) < 100*time.Millisecond {
					t.Log(time.Since(start))
				}
			}()

			wg.Wait()
			check.True(t, getCalled.Load())
			check.True(t, setCalled.Load())

		})
	})

}
