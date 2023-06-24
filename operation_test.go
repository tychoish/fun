package fun

import (
	"context"
	"errors"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/testt"
)

func TestOperation(t *testing.T) {
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
	t.Run("Constructor", func(t *testing.T) {
		ctx := testt.Context(t)
		count := 0
		op := BlockingOperation(func() { count++ })
		assert.Equal(t, count, 0)
		op(ctx)
		assert.Equal(t, count, 1)
		op(nil)
		assert.Equal(t, count, 2)

		ctx, cancel := context.WithCancel(ctx)
		cancel()
		op(ctx)
		assert.Equal(t, count, 3)
	})
	t.Run("WithCancel", func(t *testing.T) {
		ctx := testt.Context(t)
		wf, cancel := Operation(func(ctx context.Context) {
			timer := time.NewTimer(time.Hour)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				check.ErrorIs(t, ctx.Err(), context.Canceled)
				return
			case <-timer.C:
				t.Error("should not have reached this timeout")
			}
		}).WithCancel()
		assert.MinRuntime(t, 40*time.Millisecond, func() {
			assert.MaxRuntime(t, 75*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				wf(ctx)
			})
		})
	})
	t.Run("If", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		wf := Operation(func(ctx context.Context) {
			called++
		})

		wf.If(false)(ctx)
		check.Zero(t, called)
		wf.If(true)(ctx)
		check.Equal(t, 1, called)
		wf.If(true)(ctx)
		check.Equal(t, 2, called)
		wf.If(false)(ctx)
		check.Equal(t, 2, called)
		wf(ctx)
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		wf := Operation(func(ctx context.Context) {
			called++
		})

		wf.When(func() bool { return false })(ctx)
		check.Zero(t, called)
		wf.When(func() bool { return true })(ctx)
		check.Equal(t, 1, called)
		wf.When(func() bool { return true })(ctx)
		check.Equal(t, 2, called)
		wf.When(func() bool { return false })(ctx)
		check.Equal(t, 2, called)
		wf(ctx)
		check.Equal(t, 3, called)
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
		wf := Operation(func(ctx context.Context) {
			if ctx != context.Background() {
				t.Error("background context expected")
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
		wfs := make([]Operation, 100)
		for i := 0; i < 100; i++ {
			wfs[i] = func(context.Context) { time.Sleep(10 * time.Millisecond) }
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		start := time.Now()
		WaitMerge(SliceIterator(wfs))(ctx)
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

		wf := of.Iterator(SliceIterator(ops))

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
		err := Operation(func(context.Context) { called.Store(true) }).Worker()(testt.Context(t))
		assert.NotError(t, err)
		assert.True(t, called.Load())
		assert.Panic(t, func() {
			Operation(func(context.Context) { panic("hi") }).Run(testt.Context(t))
		})
	})
	t.Run("WorkerCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := &atomic.Bool{}
		err := Operation(func(context.Context) { called.Store(true) }).Worker()(ctx)
		assert.Error(t, err)
		assert.True(t, called.Load())
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("Safe", func(t *testing.T) {
		expected := errors.New("safer")
		err := Operation(func(context.Context) { panic(expected) }).
			Safe()(testt.Context(t))
		assert.Error(t, err)
		assert.ErrorIs(t, err, expected)
	})
	t.Run("PreHook", func(t *testing.T) {
		ops := []string{}

		BlockingOperation(func() { ops = append(ops, "main") }).
			PreHook(BlockingOperation(func() { ops = append(ops, "pre") })).
			Block()

		// check call order
		check.EqualItems(t, ops, []string{"pre", "main"})
	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			ops := []string{}
			BlockingOperation(func() { ops = append(ops, "first") }).
				Join(BlockingOperation(func() { ops = append(ops, "second") })).Block()
			// check call order
			check.EqualItems(t, ops, []string{"first", "second"})
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			ops := []string{}
			BlockingOperation(func() { ops = append(ops, "first") }).
				Join(BlockingOperation(func() { ops = append(ops, "second") }))(ctx)
			// check call order
			check.EqualItems(t, ops, []string{"first"})

		})
	})
	t.Run("Lock", func(t *testing.T) {
		t.Run("NilLockPanics", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			op := Operation(func(context.Context) { count++ })
			check.Panic(t, func() { op.WithLock(nil)(ctx) })
			check.Equal(t, 0, count)
		})
		// the rest of the tests are really just "tempt the
		// race detector"
		t.Run("ManagedLock", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			op := Operation(func(context.Context) { count++ })
			wg := &WaitGroup{}
			op.Lock().StartGroup(ctx, wg, 128)
			wg.Wait(ctx)
			assert.Equal(t, count, 128)
		})
		t.Run("CustomLock", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			op := Operation(func(context.Context) { count++ })
			wg := &WaitGroup{}
			mu := &sync.Mutex{}
			op.WithLock(mu).StartGroup(ctx, wg, 128)
			wg.Wait(ctx)
			assert.Equal(t, count, 128)
		})
		t.Run("ForBackground", func(t *testing.T) {
			count := 0
			op := Operation(func(context.Context) {
				count++
			}).Lock()
			ctx := testt.Context(t)

			jobs := []Operation{}

			ft.DoTimes(128, func() { jobs = append(jobs, op) })

			err := SliceIterator(jobs).ProcessParallel(ctx, HF.ProcessOperation(), WorkerGroupConfNumWorkers(4))
			assert.NotError(t, err)
			check.Equal(t, count, 128)
		})
	})
	t.Run("Signal", func(t *testing.T) {
		count := &atomic.Int64{}
		op := Operation(func(context.Context) {
			time.Sleep(10 * time.Millisecond)
			count.Add(1)
		})
		ctx := testt.Context(t)
		sig := op.Signal(ctx)
		check.Equal(t, count.Load(), 0)
		<-sig
		check.Equal(t, count.Load(), 1)
	})
	t.Run("Future", func(t *testing.T) {
		count := &atomic.Int64{}
		op := Operation(func(context.Context) {
			time.Sleep(10 * time.Millisecond)
			count.Add(1)
		})
		ctx := testt.Context(t)
		opwait := op.Future(ctx)
		check.Equal(t, count.Load(), 0)
		time.Sleep(12 * time.Millisecond)
		runtime.Gosched()
		time.Sleep(12 * time.Millisecond)
		check.Equal(t, count.Load(), 1)
		check.MaxRuntime(t, 5*time.Millisecond, func() {
			opwait(ctx)
		})
	})
	t.Run("PreHook", func(t *testing.T) {
		t.Run("Chain", func(t *testing.T) {
			count := 0
			rctx := testt.Context(t)
			var wf Operaiton = func(ctx context.Context) error {
				testt.Log(t, count)
				check.True(t, count == 1 || count == 4)
				check.Equal(t, rctx, ctx)
				count++
				return nil
			}

			wf = wf.PreHook(func(ctx context.Context) {
				testt.Log(t, count)
				check.True(t, count == 0 || count == 3)
				check.Equal(t, rctx, ctx)
				count++
			})
			check.NotError(t, wf(rctx))
			check.Equal(t, 2, count)
			wf = wf.PreHook(func(ctx context.Context) {
				testt.Log(t, count)
				check.Equal(t, count, 2)
				check.Equal(t, rctx, ctx)
				count++
			})
			check.NotError(t, wf(rctx))
			check.Equal(t, 5, count)
		})

		t.Run("WithPanic", func(t *testing.T) {
			count := 0
			pf := Operation(func(ctx context.Context) {
				assert.Equal(t, count, 1)
				count++
			}).PreHook(func(ctx context.Context) { assert.Zero(t, count); count++; panic(root) })
			ctx := testt.Context(t)
			pf(ctx)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Operation(func(ctx context.Context) {
				assert.Equal(t, count, 1)
				count++
			}).PreHook(func(ctx context.Context) { assert.Zero(t, count); count++ })
			ctx := testt.Context(t)
			err := pf(ctx)
			check.Equal(t, 2, count)
		})
	})
	t.Run("PostHook", func(t *testing.T) {
		t.Run("WithPanic", func(t *testing.T) {
			root := ers.Error(t.Name())
			count := 0
			pf := Operation(func(ctx context.Context) {
				assert.Zero(t, count)
				count++
			}).PostHook(func() { assert.Equal(t, count, 1); count++; panic(root) })
			ctx := testt.Context(t)
			pf(ctx)
			check.Equal(t, 2, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Operation(func(ctx context.Context) {
				assert.Zero(t, count)
				count++
			}).PostHook(func() { assert.Equal(t, count, 1); count++ })
			ctx := testt.Context(t)
			pf(ctx)
			check.Equal(t, 2, count)
		})
	})

}
