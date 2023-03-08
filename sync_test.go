package fun

import (
	"context"
	"math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
)

func TestAtomic(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		at := NewAtomic(1000)
		assert.Equal(t, at.Get(), 1000)
	})
	t.Run("Zero", func(t *testing.T) {
		at := &Atomic[int]{}
		assert.Equal(t, at.Get(), 0)
	})
	t.Run("RoundTrip", func(t *testing.T) {
		at := &Atomic[int]{}
		at.Set(42)
		assert.Equal(t, at.Get(), 42)
	})
}

func TestWaitGroup(t *testing.T) {
	t.Run("MultipleWaiters", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg := &WaitGroup{}
		wg.Add(100)
		start := time.Now()
		firstCase := make(chan struct{})
		go func() { wg.Wait(ctx) }()

		go func() {
			defer close(firstCase)

			wg.Wait(ctx)
		}()

		runtime.Gosched()

		secondCase := make(chan struct{})
		go func() {
			defer close(secondCase)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			wg.Wait(ctx)
		}()
		runtime.Gosched()

		<-secondCase

		timeoutDur := time.Since(start)
		if timeoutDur > 20*time.Millisecond {
			t.Error("timeout waiter took too long", timeoutDur)
		}
		time.Sleep(10 * time.Millisecond)
		cancel()

		<-firstCase

		blockingDur := time.Since(start)
		if blockingDur-timeoutDur > 20*time.Millisecond {
			t.Error("blocking waiter deadlocked", blockingDur, timeoutDur)
		}
	})
	t.Run("BusyBlocking", func(t *testing.T) {
		wg := &WaitGroup{}
		const num = 100
		wg.Add(100)
		waits := make([]chan struct{}, 100)
		for i := 0; i < num; i++ {
			ch := make(chan struct{})
			waits[i] = ch
			go func(ch chan struct{}) {
				defer close(ch)
				wg.Wait(context.Background())
			}(ch)
		}

		for i := 0; i < num; i++ {
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int63n(100)+1) * time.Millisecond)
			}()
		}
		time.Sleep(200 * time.Millisecond)
		waitStart := time.Now()
		for _, ch := range waits {
			<-ch
		}
		dur := time.Since(waitStart)
		if dur > time.Millisecond {
			t.Error("took too long for waiters to resolve", dur)
		}

	})

}
