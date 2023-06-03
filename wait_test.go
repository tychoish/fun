package fun

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestWait(t *testing.T) {
	t.Parallel()
	t.Run("WaitGroupEndToEnd", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := &WaitGroup{}

		wg.Add(1)
		check.Equal(t, wg.Num(), 1)

		cancel()
		start := time.Now()
		wg.Wait(ctx)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "canceled cotnext")
		}

		start = time.Now()
		ctxTwo, cancelTwo := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelTwo()
		wg.Wait(ctxTwo)

		if time.Since(start) < 100*time.Millisecond {
			t.Fatal("should have returned after a wait", "timeout", time.Since(start))
		}
		wg.Done()
		check.Equal(t, wg.Num(), 0)
		check.True(t, wg.IsDone())

		start = time.Now()
		ctxThree, cancelThree := context.WithTimeout(context.Background(), time.Second)
		defer cancelThree()
		wg.Wait(ctxThree)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "no pending work")
		}

		wg = &WaitGroup{}
		start = time.Now()
		ctxFour, cancelFour := context.WithTimeout(context.Background(), time.Second)
		defer cancelFour()
		wg.Add(1)
		go func() { time.Sleep(10 * time.Millisecond); wg.Done() }()
		wg.Wait(ctxFour)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 20*time.Millisecond {
			t.Fatal("should have returned after completion", "delayed completion", time.Since(start))
		}
	})
	t.Run("SyncWaitGroupEndToEnd", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := &sync.WaitGroup{}

		wg.Add(1)
		cancel()
		start := time.Now()
		WaitForGroup(wg)(ctx)
		dur := time.Since(start)
		if dur > time.Millisecond {
			t.Fatal("should have returned instantly", "canceled cotnext", dur)
		}

		start = time.Now()
		ctxTwo, cancelTwo := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelTwo()
		WaitForGroup(wg)(ctxTwo)
		if time.Since(start) < 100*time.Millisecond {
			t.Fatal("should have returned after a wait", "timeout", time.Since(start))
		}
		wg.Done()

		start = time.Now()
		ctxThree, cancelThree := context.WithTimeout(context.Background(), time.Second)
		defer cancelThree()
		WaitForGroup(wg)(ctxThree)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "no pending work")
		}

		wg = &sync.WaitGroup{}
		start = time.Now()
		ctxFour, cancelFour := context.WithTimeout(context.Background(), time.Second)
		defer cancelFour()
		wg.Add(1)
		go func() { time.Sleep(10 * time.Millisecond); wg.Done() }()
		WaitForGroup(wg)(ctxFour)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 20*time.Millisecond {
			t.Fatal("should have returned after completion", "delayed completion", time.Since(start))
		}
	})
	t.Run("WaitContext", func(t *testing.T) {
		t.Run("BaseBlocking", func(t *testing.T) {
			bctx := context.Background()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			WaitContext(bctx)(ctx)
			if time.Since(start) > time.Millisecond {
				t.Error("waited too long")
			}
		})
		t.Run("WaitBlocking", func(t *testing.T) {
			wctx := context.Background()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			WaitContext(ctx)(wctx)
			if time.Since(start) > time.Millisecond {
				t.Error("waited too long")
			}
		})
	})
	t.Run("Block", func(t *testing.T) {
		wf := WaitFunc(func(ctx context.Context) {
			if ctx == context.Background() {
				// block runs through wait, so that
				//  any threads spawned in the
				//  WaitFunc are cleaned up when the
				//  main wait function returns.
				t.Error("background context not expected")
			}
			time.Sleep(10 * time.Millisecond)
		})
		start := time.Now()
		wf.Block()
		if time.Since(start) < 10*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("Merge", func(t *testing.T) {
		wfs := make([]WaitFunc, 100)
		for i := 0; i < 100; i++ {
			wfs[i] = func(context.Context) { time.Sleep(10 * time.Millisecond) }
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		start := time.Now()
		WaitMerge(internal.NewSliceIter(wfs))(ctx)
		dur := time.Since(start)
		if dur > 50*time.Millisecond || dur < 10*time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("Wait", func(t *testing.T) {
		ops := make([]int, 100)
		for i := 0; i < len(ops); i++ {
			ops[i] = rand.Int()
		}
		seen := make(map[int]struct{})
		counter := 0
		var of Observer[int] = func(in int) {
			seen[in] = struct{}{}
			counter++
		}

		wf := of.Iterator(internal.NewSliceIter(ops))

		if len(seen) != 0 || counter != 0 {
			t.Error("should be lazy execution", counter, seen)
		}

		if err := wf(testt.Context(t)); err != nil {
			t.Error(err)
		}

		if len(seen) != 100 {
			t.Error(len(seen), seen)
		}
		if counter != 100 {
			t.Error(counter)
		}
	})
	t.Run("WorkerConverter", func(t *testing.T) {
		called := &atomic.Bool{}
		err := WaitFunc(func(context.Context) { called.Store(true) }).Worker()(testt.Context(t))
		assert.NotError(t, err)
		assert.True(t, called.Load())
		assert.Panic(t, func() {
			WaitFunc(func(context.Context) { panic("hi") }).Run(testt.Context(t))
		})
	})
	t.Run("WorkerCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := &atomic.Bool{}
		err := WaitFunc(func(context.Context) { called.Store(true) }).Worker()(ctx)
		assert.Error(t, err)
		assert.True(t, called.Load())
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("Safe", func(t *testing.T) {
		expected := errors.New("safer")
		err := WaitFunc(func(context.Context) { panic(expected) }).
			Safe()(testt.Context(t))
		assert.Error(t, err)
		assert.ErrorIs(t, err, expected)
	})
}
