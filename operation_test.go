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
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
)

func TestOperation(t *testing.T) {
	t.Parallel()
	t.Run("WaitGroupEndToEnd", func(t *testing.T) {
		t.Parallel()
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
	t.Run("Constructor", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		op := MakeOperation(func() { count++ })
		assert.Equal(t, count, 0)
		op(ctx)
		assert.Equal(t, count, 1)
		op(nil)
		assert.Equal(t, count, 2)

		op(ctx)
		assert.Equal(t, count, 3)
	})
	t.Run("WithCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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
			assert.MaxRuntime(t, 100*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				wf(ctx)
			})
		})
	})
	t.Run("If", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		called := 0
		wf := Operation(func(_ context.Context) {
			called++
		})

		wf.If(false).Run(ctx)
		check.Zero(t, called)
		wf.If(true).Run(ctx)
		check.Equal(t, 1, called)
		wf.If(true).Run(ctx)
		check.Equal(t, 2, called)
		wf.If(false).Run(ctx)
		check.Equal(t, 2, called)
		wf(ctx)
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		called := 0
		wf := Operation(func(_ context.Context) {
			called++
		})

		wf.When(func() bool { return false }).Run(ctx)
		check.Zero(t, called)
		wf.When(func() bool { return true }).Run(ctx)
		check.Equal(t, 1, called)
		wf.When(func() bool { return true }).Run(ctx)
		check.Equal(t, 2, called)
		wf.When(func() bool { return false }).Run(ctx)
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
			WaitContext(bctx).Run(ctx)
			if time.Since(start) > time.Millisecond {
				t.Error("waited too long")
			}
		})
		t.Run("WaitBlocking", func(t *testing.T) {
			wctx := context.Background()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			WaitContext(ctx).Run(wctx)
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
		wf.Wait()
		if time.Since(start) < 10*time.Millisecond {
			t.Error(time.Since(start))
		}
	})
	t.Run("Merge", func(t *testing.T) {
		wfs := make([]Operation, 100)
		count := &atomic.Int64{}
		for i := 0; i < 100; i++ {
			wfs[i] = func(context.Context) {
				startAt := time.Now()
				defer count.Add(1)

				time.Sleep(10 * time.Millisecond)
				t.Log(time.Since(startAt))
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		start := time.Now()
		t.Log("before exec:", count.Load())
		err := MAKE.OperationPool(SliceStream(wfs)).Run(ctx)
		t.Log("after exec:", count.Load())
		if err != nil {
			t.Error(err)
		}
		dur := time.Since(start)
		if dur > 150*time.Millisecond {
			t.Error(dur)
		}
		if int(dur) < (1000 / runtime.NumCPU()) {
			t.Error(t, dur, 1000/runtime.NumCPU())
		}
	})
	t.Run("Wait", func(t *testing.T) {
		ops := make([]int, 100)
		for i := 0; i < len(ops); i++ {
			ops[i] = rand.Int()
		}
		seen := make(map[int]struct{})
		counter := 0
		var of fn.Handler[int] = func(in int) {
			seen[in] = struct{}{}
			counter++
		}

		wf := SliceStream(ops).ReadAll(of)

		if len(seen) != 0 || counter != 0 {
			t.Error("should be lazy execution", counter, seen)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := wf(ctx); err != nil {
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Operation(func(context.Context) { called.Store(true) }).Worker().Run(ctx)
		assert.NotError(t, err)
		assert.True(t, called.Load())
		assert.Panic(t, func() {
			Operation(func(context.Context) { panic("hi") }).Run(ctx)
		})
	})
	t.Run("WorkerCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := &atomic.Bool{}
		err := Operation(func(context.Context) { called.Store(true) }).Worker().Run(ctx)
		assert.Error(t, err)
		assert.True(t, called.Load())
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("Safe", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		expected := errors.New("safer")
		err := Operation(func(context.Context) { panic(expected) }).
			WithRecover().Run(ctx)
		assert.Error(t, err)
		assert.ErrorIs(t, err, expected)
	})
	t.Run("PreHook", func(t *testing.T) {
		ops := []string{}

		MakeOperation(func() { ops = append(ops, "main") }).
			PreHook(MakeOperation(func() { ops = append(ops, "pre") })).
			Wait()

		// check call order
		check.EqualItems(t, ops, []string{"pre", "main"})
	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			ops := []string{}
			MakeOperation(func() { ops = append(ops, "first") }).
				Join(MakeOperation(func() { ops = append(ops, "second") })).Wait()
			// check call order
			check.EqualItems(t, ops, []string{"first", "second"})
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			ops := []string{}
			MakeOperation(func() { ops = append(ops, "first") }).
				Join(MakeOperation(func() { ops = append(ops, "second") })).Run(ctx)
			// check call order
			check.EqualItems(t, ops, []string{"first"})

		})
	})
	t.Run("Lock", func(t *testing.T) {
		t.Run("NilLockPanics", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			op := Operation(func(context.Context) { count++ })
			check.Panic(t, func() { op.WithLock(nil).Run(ctx) })
			check.Equal(t, 0, count)
		})
		// the rest of the tests are really just "tempt the
		// race detector"
		t.Run("ManagedLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0

			Operation(func(context.Context) { count++ }).
				Lock().
				StartGroup(ctx, 128).
				Run(ctx)

			assert.Equal(t, count, 128)
		})
		t.Run("CustomLock", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			op := Operation(func(context.Context) { count++ })
			mu := &sync.Mutex{}
			op.WithLock(mu).StartGroup(ctx, 128).Run(ctx)

			assert.Equal(t, count, 128)
		})
		t.Run("ForBackground", func(t *testing.T) {
			count := 0
			op := Operation(func(context.Context) {
				count++
			}).Lock()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			jobs := []Operation{}

			ft.CallTimes(128, func() { jobs = append(jobs, op) })

			err := SliceStream(jobs).Parallel(MAKE.OperationHandler(), WorkerGroupConfNumWorkers(4)).Run(ctx)
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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sig := op.Signal(ctx)
		check.Equal(t, count.Load(), 0)
		<-sig
		check.Equal(t, count.Load(), 1)
	})
	t.Run("Future", func(t *testing.T) {
		count := &atomic.Int64{}
		op := Operation(func(context.Context) {
			time.Sleep(20 * time.Millisecond)
			count.Add(1)
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opwait := op.Launch(ctx)

		check.Equal(t, count.Load(), 0)
		time.Sleep(100 * time.Millisecond)
		check.Equal(t, count.Load(), 1)
		runtime.Gosched()
		check.MaxRuntime(t, 10*time.Millisecond, func() {
			runtime.Gosched()
			opwait(ctx)
		})
	})
	t.Run("PreHook", func(t *testing.T) {
		t.Run("Chain", func(t *testing.T) {
			count := 0

			rctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var wf Operation = func(ctx context.Context) {
				check.True(t, count == 1 || count == 4)
				check.Equal(t, rctx, ctx)
				count++
			}

			wf = wf.PreHook(func(ctx context.Context) {
				check.True(t, count == 0 || count == 3)
				check.Equal(t, rctx, ctx)
				count++
			})
			wf(rctx)
			check.Equal(t, 2, count)
			wf = wf.PreHook(func(ctx context.Context) {
				check.Equal(t, count, 2)
				check.Equal(t, rctx, ctx)
				count++
			})
			wf(rctx)
			check.Equal(t, 5, count)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			pf := Operation(func(_ context.Context) {
				assert.Equal(t, count, 1)
				count++
			}).PreHook(func(_ context.Context) { assert.Zero(t, count); count++ })

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			pf(ctx)
			check.Equal(t, 2, count)
		})
	})
	t.Run("PostHook", func(t *testing.T) {
		count := 0
		pf := Operation(func(_ context.Context) {
			assert.Zero(t, count)
			count++
		}).PostHook(func() { assert.Equal(t, count, 1); count++ })

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pf(ctx)
		check.Equal(t, 2, count)
	})
	t.Run("Limit", func(t *testing.T) {
		t.Parallel()
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			var wf Operation = func(context.Context) { count++ }
			wf = wf.Limit(10)
			for i := 0; i < 100; i++ {
				wf(ctx)
			}
			assert.Equal(t, count, 10)
		})
		t.Run("Parallel", func(t *testing.T) {
			ft.CallTimes(32, func() {
				t.Run("Iteration", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					count := &atomic.Int64{}
					var wf Operation = func(context.Context) { count.Add(1) }
					wf = wf.Limit(10)
					wg := &sync.WaitGroup{}
					for i := 0; i < 32; i++ {
						wg.Add(1)
						go func() { defer wg.Done(); wf(ctx) }()
					}
					wg.Wait()
					assert.Equal(t, count.Load(), 10)

				})
			})
		})
	})
	t.Run("Jitter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := &atomic.Int64{}
		var wf Operation = func(context.Context) { count.Add(1) }
		delay := 100 * time.Millisecond
		wf = wf.Jitter(func() time.Duration { return delay })
		start := time.Now()
		wf(ctx)
		dur := time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= 100*time.Millisecond)
		assert.True(t, dur < 200*time.Millisecond)

		delay = 10 * time.Millisecond
		start = time.Now()
		wf(ctx)
		dur = time.Since(start).Truncate(time.Millisecond)

		assert.True(t, dur >= time.Millisecond)
		assert.True(t, dur < 102*time.Millisecond)
	})
	t.Run("Delay", func(t *testing.T) {
		t.Parallel()
		t.Run("Basic", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Operation = func(context.Context) { count.Add(1) }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					wf(ctx)
				}()
			}
			check.Equal(t, 100, wg.Num())
			time.Sleep(125 * time.Millisecond)
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 100)
		})
		t.Run("Canceled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Operation = func(context.Context) { count.Add(1) }
			wf = wf.Delay(100 * time.Millisecond)
			wg := &WaitGroup{}
			wg.Add(100)
			cancel()
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					wf(ctx)
				}()
			}
			time.Sleep(2 * time.Millisecond)
			wg.Wait(context.Background())
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 0)
		})
		t.Run("After", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := &atomic.Int64{}
			var wf Operation = func(context.Context) { count.Add(1) }
			ts := time.Now().Add(100 * time.Millisecond)
			wf = wf.After(ts)
			wg := &WaitGroup{}
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() {
					defer wg.Done()
					start := time.Now()
					defer func() { check.True(t, time.Since(start) > 75*time.Millisecond) }()
					wf(ctx)
				}()
			}
			check.Equal(t, 100, wg.Num())
			time.Sleep(120 * time.Millisecond)
			check.Equal(t, 0, wg.Num())
			check.Equal(t, count.Load(), 100)
		})
	})
	t.Run("Interval", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := &atomic.Int64{}
		var wf Operation = func(context.Context) { count.Add(1); runtime.Gosched() }
		wf = wf.Interval(20 * time.Millisecond)
		check.Equal(t, 0, count.Load())
		sig := wf.Signal(ctx)
		time.Sleep(145 * time.Millisecond)
		runtime.Gosched()
		cancel()
		<-sig
		check.True(t, count.Load() > 5)
		check.True(t, count.Load() < 10)
	})
	t.Run("While", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := &atomic.Int64{}
		var wf Operation = func(context.Context) { count.Add(1); time.Sleep(10 * time.Millisecond) }
		wf.While().Background(ctx)
		time.Sleep(100 * time.Millisecond)
		cancel()
		check.True(t, count.Load() >= 6)
	})
	t.Run("TTL", func(t *testing.T) {
		t.Parallel()
		t.Run("Zero", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			var wf Operation = func(context.Context) { count++ }
			wf = wf.TTL(0)
			for i := 0; i < 100; i++ {
				wf(ctx)
			}
			check.Equal(t, 100, count)
		})
		t.Run("Serial", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			count := 0
			var wf Operation = func(context.Context) { count++ }
			wf = wf.TTL(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				wf(ctx)
			}
			check.Equal(t, 1, count)
			time.Sleep(100 * time.Millisecond)
			for i := 0; i < 100; i++ {
				wf(ctx)
			}
			check.Equal(t, 2, count)
		})
		t.Run("Parallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			wg := &sync.WaitGroup{}

			count := 0
			var wf Operation = func(context.Context) { count++ }
			wf = wf.TTL(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() { defer wg.Done(); wf(ctx) }()
			}
			wg.Wait()
			check.Equal(t, 1, count)
			time.Sleep(100 * time.Millisecond)
			wg.Add(100)
			for i := 0; i < 100; i++ {
				go func() { defer wg.Done(); wf(ctx) }()
			}
			wg.Wait()
			check.Equal(t, 2, count)
		})
	})
	t.Run("Group", func(t *testing.T) {
		count := &atomic.Int64{}
		MakeOperation(func() { count.Add(1) }).Group(32).Run(t.Context())
		assert.Equal(t, count.Load(), 32)
	})

}
