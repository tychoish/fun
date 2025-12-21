package adt

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

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
	t.Run("LockerRead", func(t *testing.T) {
		mtx := &sync.RWMutex{}
		var number int64
		wg := &sync.WaitGroup{}
		for range 2 * runtime.NumCPU() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 1000 {
					func() {
						defer WithW(LockW(mtx))
						switch number {
						case 0:
							number = 2
						case 2:
							number = 4
						case 4:
							number = 8
						case 8:
							number = 16
						case 16:
							number = 32
						case 32:
							number = 2
						default:
							panic("should never happen")
						}
						time.Sleep(1 + time.Duration(rand.Int63n(number)))
					}()
				}
			}()
		}

		wg.Wait()

		assert.Zero(t, number%4)
	})
	t.Run("LockR", func(t *testing.T) {
		// straigtht forward, just making sure we don't panic
		rmu := &sync.RWMutex{}
		assert.NotPanic(t, func() { WithR(LockR(rmu)) })
	})
}
