package fun

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
)

func TestWait(t *testing.T) {
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

		ctxTwo, cancelTwo := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelTwo()
		start = time.Now()
		wg.Wait(ctxTwo)
		if time.Since(start) < 100*time.Millisecond {
			t.Fatal("should have returned after a wait", "timeout", time.Since(start))
		}
		wg.Done()
		check.Equal(t, wg.Num(), 0)
		check.True(t, wg.IsDone())

		ctxThree, cancelThree := context.WithTimeout(context.Background(), time.Second)
		defer cancelThree()
		start = time.Now()
		wg.Wait(ctxThree)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "no pending work")
		}

		wg = &WaitGroup{}
		ctxFour, cancelFour := context.WithTimeout(context.Background(), time.Second)
		defer cancelFour()
		start = time.Now()
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
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "canceled cotnext")
		}

		ctxTwo, cancelTwo := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelTwo()
		start = time.Now()
		WaitForGroup(wg)(ctxTwo)
		if time.Since(start) < 100*time.Millisecond {
			t.Fatal("should have returned after a wait", "timeout", time.Since(start))
		}
		wg.Done()

		ctxThree, cancelThree := context.WithTimeout(context.Background(), time.Second)
		defer cancelThree()
		start = time.Now()
		WaitForGroup(wg)(ctxThree)
		if time.Since(start) > time.Millisecond {
			t.Fatal("should have returned instantly", "no pending work")
		}

		wg = &sync.WaitGroup{}
		ctxFour, cancelFour := context.WithTimeout(context.Background(), time.Second)
		defer cancelFour()
		start = time.Now()
		wg.Add(1)
		go func() { time.Sleep(10 * time.Millisecond); wg.Done() }()
		WaitForGroup(wg)(ctxFour)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 20*time.Millisecond {
			t.Fatal("should have returned after completion", "delayed completion", time.Since(start))
		}
	})
	t.Run("WaitObserveAll", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan int, 2)
		ch <- 42
		ch <- 42
		close(ch)
		var sum int

		WaitObserveAll(func(in int) { sum += in }, ch)(ctx)
		if sum != 84 {
			t.Error("unexpected total", sum)
		}
	})
	t.Run("WaitObserveOne", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := make(chan int, 2)
		ch <- 42
		ch <- 42
		close(ch)
		var sum int

		WaitObserve(func(in int) { sum += in }, ch)(ctx)
		if sum != 42 {
			t.Error("unexpected total", sum)
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
			if ctx != context.Background() {
				t.Error("context not expected")
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
			wfs[i] = func(context.Context) { time.Sleep(5 * time.Millisecond) }
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		start := time.Now()
		WaitMerge(ctx, internal.NewSliceIter(wfs))(ctx)
		if time.Since(start) > 12*time.Millisecond || time.Since(start) < 5*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("Blocking", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			start := time.Now()
			WaitBlocking(func() { time.Sleep(10 * time.Millisecond) }).Block()
			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Second {
				t.Error(time.Since(start))
			}
		})
		t.Run("Context", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			start := time.Now()
			WaitBlocking(func() { time.Sleep(10 * time.Millisecond) })(ctx)
			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Second {
				t.Error(time.Since(start))
			}
		})
		t.Run("Observe", func(t *testing.T) {
			var seen int
			start := time.Now()
			WaitBlockingObserve(
				func(in int) { seen = in },
				func() int { time.Sleep(10 * time.Millisecond); return 42 },
			).Block()

			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 11*time.Second {
				t.Error(time.Since(start))
			}
			if seen != 42 {
				t.Error(seen)
			}
		})
	})
	t.Run("Timeout", func(t *testing.T) {
		wf := WaitFunc(func(ctx context.Context) {
			timer := time.NewTimer(time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
			case <-timer.C:
			}
		})
		start := time.Now()
		wf.WithTimeout(10 * time.Millisecond)
		if time.Since(start) < 10*time.Millisecond || time.Since(start) > 15*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
}
