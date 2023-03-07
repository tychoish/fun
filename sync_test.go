package fun

import (
	"context"
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
	if timeoutDur > 11*time.Millisecond {
		t.Error("timeout waiter deadlocked", timeoutDur)
	}
	time.Sleep(10 * time.Millisecond)
	cancel()

	<-firstCase

	blockingDur := time.Since(start)
	if blockingDur-timeoutDur > 11*time.Millisecond {
		t.Error("blocking waiter deadlocked", blockingDur, timeoutDur)
	}
}
