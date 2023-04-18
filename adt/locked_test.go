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
}
