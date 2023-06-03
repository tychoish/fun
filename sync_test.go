package fun

import (
	"context"
	"math/rand"
	"runtime"
	"testing"
	"time"
)

func TestWaitGroup(t *testing.T) {
	t.Parallel()
	t.Run("MultipleWaiters", func(t *testing.T) {
		t.Parallel()
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
		if timeoutDur > 50*time.Millisecond {
			t.Error("timeout waiter took too long", timeoutDur)
		}
		time.Sleep(10 * time.Millisecond)
		cancel()

		<-firstCase

		blockingDur := time.Since(start)
		if blockingDur-timeoutDur > 50*time.Millisecond {
			t.Error("blocking waiter deadlocked", blockingDur, timeoutDur)
		}
	})
	t.Run("BusyBlocking", func(t *testing.T) {
		t.Parallel()

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
			if i%10 == 0 {
				runtime.Gosched()
			}
		}
		time.Sleep(101 * time.Millisecond)
		waitStart := time.Now()
		for _, ch := range waits {
			<-ch
		}
		dur := time.Since(waitStart)
		if dur > 20*time.Millisecond {
			t.Error("took too long for waiters to resolve", dur)
		}
	})
	t.Run("BusyBlockingMixed", func(t *testing.T) {
		t.Parallel()

		wg := &WaitGroup{}
		const num = 100
		wg.Add(100)
		waits := make([]chan struct{}, 100)
		for i := 0; i < num; i++ {
			ch := make(chan struct{})
			waits[i] = ch
			if i%4 == 0 {
				go func(ch chan struct{}) {
					defer close(ch)
					_ = wg.Worker().Block()
				}(ch)
			} else if i%2 == 0 {
				go func(ch chan struct{}) {
					defer close(ch)
					wg.WaitFunc().Block()
				}(ch)
			} else {
				go func(ch chan struct{}, num int) {
					defer close(ch)
					ctx, cancel := context.WithTimeout(
						context.Background(),
						time.Duration(num*2)*time.Millisecond,
					)
					defer cancel()
					wg.Wait(ctx)
				}(ch, i)
			}
		}

		for i := 0; i < num; i++ {
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.Int63n(100)+1) * time.Millisecond)
			}()
			if i%10 == 0 {
				runtime.Gosched()
			}
		}
		time.Sleep(101 * time.Millisecond)
		waitStart := time.Now()
		for _, ch := range waits {
			<-ch
		}
		dur := time.Since(waitStart)
		if dur > 25*time.Millisecond {
			t.Error("took too long for waiters to resolve", dur)
		}
	})

}
