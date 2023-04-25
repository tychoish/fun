package adt

import (
	"math/rand"
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

		wg.Done()
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
}
