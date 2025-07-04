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
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/testt"
)

type poolTestType struct{ value int }

func TestPool(t *testing.T) {
	t.Parallel()
	t.Run("PanicsWithoutConstructor", func(t *testing.T) {
		t.Skip("default constructor now provided, maybe questionable")
		assert.Panic(t, func() {
			p := &Pool[*poolTestType]{}
			_ = p.Get()
		})

	})

	t.Run("Init", func(t *testing.T) {
		p := &Pool[*poolTestType]{}
		p.SetConstructor(func() *poolTestType { return &poolTestType{} })
		ptt := p.Make()
		assert.Equal(t, ptt.value, 0)
		pgt := p.Get()
		assert.Equal(t, *ptt, *pgt)
	})
	t.Run("ArePooled", func(t *testing.T) {
		t.Parallel()
		p := &Pool[*poolTestType]{}
		p.SetConstructor(func() *poolTestType { return &poolTestType{} })

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
		called := &atomic.Bool{}
		seen := &atomic.Bool{}

		p.SetConstructor(func() *poolTestType { return &poolTestType{} })
		p.SetCleanupHook(func(in *poolTestType) *poolTestType {
			called.Store(true)
			check.NotZero(t, in)
			in.value = 42
			return in
		})

		wg := &fun.WaitGroup{}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for gr := 0; gr < 32; gr++ {
			wg.Add(2)

			go func() {
				defer wg.Done()
				for {
					if ctx.Err() != nil {
						return
					}
					ppg := p.Get()
					ppg.value = 9001
					p.Put(ppg)
				}
			}()
			go func() {
				defer wg.Done()
				for {
					ppg := p.Get()
					check.NotEqual(t, ppg.value, 9001)
					switch ppg.value {
					case 9001:
						t.Error("cleanup hook failed")
					case 42:
						seen.Store(true)
						cancel()
						return
					}
				}
			}()
		}
		wg.Wait(ctx)
		check.True(t, seen.Load())
		check.True(t, called.Load())
		testt.Log(t, "seen =", seen.Load(), "called = ", called.Load())
	})
	t.Run("MakeMagic", func(t *testing.T) {
		t.Parallel()
		p := &Pool[*poolTestType]{}
		p.SetConstructor(func() *poolTestType { return &poolTestType{} })
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
	t.Run("Finalize", func(t *testing.T) {
		p := &Pool[int]{}

		ft.CallTimes(100, func() { p.SetConstructor(rand.Int) })
		p.SetConstructor(nil)
		assert.NotNil(t, p.constructor.Load())
		p.SetCleanupHook(nil)
		assert.NotNil(t, p.hook.Load())

		ft.CallTimes(100, p.FinalizeSetup)
		check.Panic(t, func() { p.SetConstructor(rand.Int) })
		check.Panic(t, func() { p.SetCleanupHook(func(int) int { return 0 }) })
		check.Panic(t, func() { p.SetConstructor(nil) })
		check.Panic(t, func() { p.SetCleanupHook(nil) })
	})
	t.Run("MakeBytesBufferPool", func(t *testing.T) {
		bp := MakeBytesBufferPool(1024)
		buf := bp.Get()
		check.Equal(t, buf.Cap(), 1024)

		var passed bool
		var count int64
		for start := time.Now(); time.Since(start) < 2*time.Second; {
			count++
			ft.CallTimes(2048, func() { buf.WriteByte('!') })
			bp.Put(buf)
			buf = bp.Get()
			check.Equal(t, buf.Len(), 0)
			if buf.Cap() >= 2048 {
				passed = true
				break
			}
		}
		testt.Log(t, "attempts:", count)
		check.True(t, passed)

	})
	t.Run("BytesBuffer", func(t *testing.T) {
		t.Run("Resizes", func(t *testing.T) {
			for name, bufpool := range map[string]*Pool[dt.Slice[byte]]{
				"Small":   MakeBufferPool(0, 32),
				"Default": DefaultBufferPool(),
			} {
				t.Run(name, func(t *testing.T) {
					buf := bufpool.Get()
					check.Equal(t, cap(buf), 0)
					check.Equal(t, len(buf), 0)
					bufpool.Put(buf)

					var passed bool
					var count int64
					for start := time.Now(); time.Since(start) < 2*time.Second; {
						count++
						buf = bufpool.Get()
						assert.True(t, cap(buf) <= 32)
						assert.Zero(t, len(buf))

						if cap(buf) >= 16 {
							passed = true
							break
						}

						for len(buf) < 32 {
							buf = append(buf, '~')
						}
						bufpool.Put(buf)
					}

					testt.Log(t, "attempts:", count)
					check.True(t, passed)
				})
			}

		})
		t.Run("OversizeSafely", func(t *testing.T) {
			// hard to introspect if they're actually in
			// the pool. Not panicing is good.
			check.NotPanic(t, func() {
				p := MakeBufferPool(0, 4)
				buf := p.Get()
				for len(buf) < 32 {
					buf = append(buf, '~')
				}
				p.Put(buf)
			})
		})
		t.Run("PanicsForZeroMax", func(t *testing.T) {
			check.Panic(t, func() { MakeBufferPool(0, 0) })
			check.Panic(t, func() { MakeBufferPool(-100, -2) })
		})
		t.Run("BoundsAreFlooredAtZero", func(t *testing.T) {
			check.NotPanic(t, func() { MakeBufferPool(-100, 100) })
			check.NotPanic(t, func() { MakeBufferPool(100, -100) })
		})
		t.Run("FlipsIfNeeded", func(t *testing.T) {
			for name, bufpool := range map[string]*Pool[dt.Slice[byte]]{
				"MinMax": MakeBufferPool(32, 64),
				"MaxMin": MakeBufferPool(64, 32),
			} {
				t.Run(name, func(t *testing.T) {
					buf := bufpool.Get()
					check.Equal(t, cap(buf), 32)
					check.Equal(t, len(buf), 0)
				})
			}

		})

	})

}
