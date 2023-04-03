package adt

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/testt"
)

type poolTestType struct {
	value int
}

func TestPool(t *testing.T) {
	t.Parallel()
	t.Run("PanicsWithoutConstructor", func(t *testing.T) {
		assert.Panic(t, func() {
			p := &Pool[*poolTestType]{}
			_ = p.Get()
		})

	})

	t.Run("Init", func(t *testing.T) {
		p := &Pool[*poolTestType]{}
		p.Constructor.Set(func() *poolTestType { return &poolTestType{} })
		ptt := p.Make()
		assert.Equal(t, ptt.value, 0)
		pgt := p.Get()
		assert.Equal(t, *ptt, *pgt)
	})
	t.Run("ArePooled", func(t *testing.T) {
		t.Parallel()
		p := &Pool[*poolTestType]{}
		p.Constructor.Set(func() *poolTestType { return &poolTestType{} })

		seen := false

		for i := 0; i < 1000; i++ {
			pgt := p.Get()
			if pgt.value == 100 {
				seen = true
				break
			}
			pgt.value = 100
			p.Put(pgt)
		}

		for i := 0; i < 100000; i++ {
			ppg := p.Make()
			if ppg.value == 100 || seen {
				seen = true
				break
			}
			time.Sleep(time.Microsecond)
		}
		assert.True(t, seen)
	})
	t.Run("CleanupHook", func(t *testing.T) {
		t.Parallel()
		p := &Pool[*poolTestType]{}
		p.Constructor.Set(func() *poolTestType { return &poolTestType{} })
		called := false
		p.SetCleanupHook(func(in *poolTestType) *poolTestType {
			called = true
			assert.NotZero(t, in)
			in.value = 42
			return in
		})
		for i := 0; i < 100000; i++ {
			ppg := p.Get()
			ppg.value = 9001
			p.Put(ppg)
		}
		seen := false
		for i := 0; i < 100000; i++ {
			ppg := p.Get()
			assert.NotEqual(t, ppg.value, 9001)
			if ppg.value == 42 {
				seen = true
				break
			}
		}
		t.Log(seen, called)
		assert.True(t, seen)
		assert.True(t, called)
	})
	t.Run("MakeMagic", func(t *testing.T) {
		t.Parallel()
		p := &Pool[*poolTestType]{}
		p.Constructor.Set(func() *poolTestType { return &poolTestType{} })
		ctx, cancel := context.WithCancel(testt.ContextWithTimeout(t, 5*time.Second))
		defer cancel()
		wg := &sync.WaitGroup{}
		seen := &atomic.Bool{}
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					if seen.Load() || ctx.Err() != nil {
						return
					}

					func() {
						ppg := p.Make()
						ppg.value = 420
					}()
					runtime.GC()
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()

				for {
					for {
						if seen.Load() || ctx.Err() != nil {
							return
						}

						ppg := p.Get()
						if ppg.value == 420 {
							seen.Store(true)
							cancel()
							return
						}

						runtime.GC()
					}

				}

			}()
		}
		<-ctx.Done()
		assert.True(t, seen.Load())
	})
}
