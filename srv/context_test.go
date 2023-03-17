package srv

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/testt"
)

func TestContext(t *testing.T) {
	t.Run("Orchestrator", func(t *testing.T) {
		t.Run("Panics", func(t *testing.T) {
			ec := &erc.Collector{}
			func() {
				defer erc.Recover(ec)
				GetOrchestrator(context.Background())
			}()
			err := ec.Resolve()
			if err == nil {
				t.Error("should have panic'd")
			}
			errStr := err.Error()
			if !strings.Contains(errStr, "orchestrator was not correctly attached") {
				t.Error(errStr)
			}
			if !strings.Contains(errStr, "invariant violation") {
				t.Error(errStr)
			}
			if !strings.Contains(errStr, "panic") {
				t.Error(errStr)
			}
			if !errors.Is(err, fun.ErrInvariantViolation) {
				t.Error(err, errors.Is(err, fun.ErrInvariantViolation))
			}
		})
		t.Run("GetOrchestrator", func(t *testing.T) {
			orc := &Orchestrator{}

			ctx := context.Background()

			nctx := SetOrchestrator(ctx, orc)

			or := GetOrchestrator(nctx)
			if or == nil {
				t.Error("should not be nil")
			}
			if or != orc {
				t.Error("should be the same orchestrator")
			}
		})
		t.Run("SetOrchestrator", func(t *testing.T) {
			orc := &Orchestrator{}

			ctx := context.Background()

			nctx := SetOrchestrator(ctx, orc)
			if nctx == ctx {
				t.Error("should have replaced context")
			}
		})
		t.Run("HasOrchestrator", func(t *testing.T) {
			orc := &Orchestrator{}

			ctx := context.Background()
			assert.True(t, !HasOrchestrator(ctx))

			ctx = SetOrchestrator(ctx, orc)
			assert.True(t, HasOrchestrator(ctx))
		})
	})
	t.Run("BaseContext", func(t *testing.T) {
		t.Run("Root", func(t *testing.T) {
			rctx := context.Background()
			ctx, cancel := context.WithCancel(rctx)
			defer cancel()
			ctx = SetBaseContext(ctx)
			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()

			bctx := GetBaseContext(ctx2)
			if bctx.Err() != nil {
				t.Error("have wrong context")
			}
			cancel()
			if bctx.Err() == nil {
				t.Error("have wrong context")
			}
		})
		t.Run("Tree", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = SetBaseContext(ctx)

			ctxleft, cancelleft := context.WithCancel(ctx)
			defer cancelleft()
			ctxright, cancelright := context.WithCancel(ctx)
			defer cancelright()

			cl2, cl2cancel := context.WithCancel(ctxleft)
			defer cl2cancel()
			cr2, cr2cancel := context.WithCancel(ctxright)
			defer cr2cancel()

			if GetBaseContext(cl2) != GetBaseContext(cr2) {
				t.Fatal("not equal")
			}
		})
	})
	t.Run("Shutdown", func(t *testing.T) {
		t.Run("NormalCase", func(t *testing.T) {
			rctx, cancel0 := context.WithCancel(context.Background())
			defer cancel0()

			ctx, cancel1 := context.WithCancel(rctx)
			defer cancel1()
			ctx = SetShutdownSignal(ctx)

			ctx2, cancel2 := context.WithCancel(ctx)
			defer cancel2()

			ctx3, cancel3 := context.WithCancel(ctx2)
			defer cancel3()

			GetShutdownSignal(ctx2)()
			if ctx3.Err() == nil {
				t.Error("shutdown did not propogate")
			}
			if ctx.Err() == nil {
				t.Error("shutdown acted at wrong level")
			}
			if rctx.Err() != nil {
				t.Error("shutdown should not have reached back to root")
			}
		})
		t.Run("Tree", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ctxleft, cancelleft := context.WithCancel(ctx)
			defer cancelleft()
			ctxleft = SetShutdownSignal(ctxleft)
			ctxright, cancelright := context.WithCancel(ctx)
			defer cancelright()

			cl2, cl2cancel := context.WithCancel(ctxleft)
			defer cl2cancel()
			cr2, cr2cancel := context.WithCancel(ctxright)
			defer cr2cancel()

			cl3, cl3cancel := context.WithCancel(cl2)
			defer cl3cancel()

			GetShutdownSignal(cl2)()
			if cl3.Err() == nil {
				t.Error("should be canceled")
			}
			if cl2.Err() == nil {
				t.Error("should be canceled")
			}
			if ctxright.Err() != nil {
				t.Error("should not be cancelled")
			}
			if cr2.Err() != nil {
				t.Error("should not be cancelled")
			}
			if ctx.Err() != nil {
				t.Error("should not be cancelled")
			}
		})
	})
	t.Run("WithOrchestrator", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctx = WithOrchestrator(ctx)
		orca := GetOrchestrator(ctx)
		if orca == nil {
			t.Error("should have orchestrator")
		}
		if !orca.Service().Running() {
			t.Error("should be running")
		}
	})
	t.Run("MultipleAttchments", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctx = SetShutdownSignal(ctx)
		ctx = SetBaseContext(ctx)
		ctx = WithOrchestrator(ctx)

		if orca := GetOrchestrator(ctx); orca == nil {
			t.Error("should have orchestrator")
		} else {
			var _ *Orchestrator = orca
		}
		if bctx := GetBaseContext(ctx); bctx == nil {
			t.Error("should have base context")
		} else {
			var _ context.Context = bctx
		}
		if shutdown := GetShutdownSignal(ctx); shutdown != nil {
			var _ context.CancelFunc = shutdown
		}
	})
}

func TestShutdownManager(t *testing.T) {
	t.Run("Initializer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		assert.True(t, !HasOrchestrator(ctx))
		assert.True(t, !HasShutdownSignal(ctx))
		assert.True(t, !HasBaseContext(ctx))
		assert.True(t, !HasShutdownManager(ctx))

		// setup shutdown manager
		ctx = WithShutdownManager(ctx)

		// we have to set some
		assert.True(t, HasShutdownSignal(ctx))
		assert.True(t, HasShutdownManager(ctx))
		assert.True(t, HasOrchestrator(ctx))
		assert.True(t, HasBaseContext(ctx))
	})
	t.Run("WithShutdownManager", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = WithShutdownManager(ctx)
		assert.True(t, HasShutdownManager(ctx))

		called := &atomic.Bool{}
		AddToShutdownManager(ctx, func(ctx context.Context) {
			called.Store(true)
		})

		orca := GetOrchestrator(ctx)
		s := orca.Service()
		time.Sleep(100 * time.Millisecond)
		assert.True(t, s.Running())
		GetShutdownSignal(ctx)()
		time.Sleep(200 * time.Millisecond)
		assert.True(t, s.isStarted.Load())
		assert.NotError(t, s.Wait())

		assert.True(t, called.Load())
	})
	t.Run("AddToQueue", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = WithShutdownManager(ctx)

		called := &atomic.Bool{}
		AddToShutdownManager(ctx, func(ctx context.Context) {
			called.Store(true)
		})

		time.Sleep(10 * time.Millisecond)
		GetShutdownSignal(ctx)()
		time.Sleep(10 * time.Millisecond)

		if err := GetOrchestrator(ctx).Wait(); err != nil {
			t.Error(err)
		}

		if !called.Load() {
			t.Error("should have been called")
		}
	})
	t.Run("Integration", func(t *testing.T) {
		t.Parallel()
		ctx := WithShutdownManager(testt.Context(t))
		const num = 1
		count := &atomic.Int64{}
		for i := 0; i < num; i++ {
			AddToShutdownManager(ctx, func(ctx context.Context) {
				count.Add(1)
			})
		}

		orca := GetOrchestrator(ctx)
		s := orca.Service()
		s.Name = "hi"

		assert.True(t, s.Running())
		assert.True(t, s.isStarted.Load())
		GetShutdownSignal(ctx)()
		assert.Error(t, ctx.Err())
		// err := s.Wait()
		// check.NotError(t, err)
		defer (func() { t.Log("end count", count.Load()) })()
		time.Sleep(200 * time.Millisecond)
		assert.True(t, !s.Running())
		assert.Equal(t, count.Load(), num)
	})

	t.Run("ContextsAreAStack", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		bctx := SetShutdownSignal(ctx)
		ctx = WithOrchestrator(bctx)
		ctx = SetShutdownSignal(ctx)

		assert.NotError(t, ctx.Err())
		GetShutdownSignal(ctx)()
		assert.Error(t, ctx.Err())

		assert.NotError(t, bctx.Err())
	})
}

func TestWorkerPool(t *testing.T) {
	t.Parallel()
	t.Run("Example", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = WithWorkerPool(ctx, "kip", itertool.Options{NumWorkers: 4})
		assert.True(t, HasOrchestrator(ctx))
		called := &atomic.Bool{}
		sig := make(chan struct{})
		err := AddToWorkerPool(ctx, "kip", func(context.Context) error {
			defer close(sig)
			called.Store(true)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		<-sig
		assert.True(t, called.Load())
		t.Run("MultipleDistinguishablePools", func(t *testing.T) {
			err = AddToWorkerPool(ctx, "merlin", func(context.Context) error { return nil })
			assert.Error(t, err)
		})
	})
	t.Run("NegativeWorkersWork", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = WithWorkerPool(ctx, "kip", itertool.Options{NumWorkers: -4})
		assert.True(t, HasOrchestrator(ctx))
		called := &atomic.Bool{}
		sig := make(chan struct{})
		err := AddToWorkerPool(ctx, "kip", func(context.Context) error {
			defer close(sig)
			called.Store(true)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		<-sig
		assert.True(t, called.Load())
	})
	t.Run("UnsetAddToWorkerErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := AddToWorkerPool(ctx, "kip", func(context.Context) error {
			return errors.New("should not be called")
		})
		if err == nil {
			t.Fatal(err)
		}
		check.Substring(t, err.Error(), "kip")
	})
	t.Run("ClosedQueueError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		queue := pubsub.NewUnlimitedQueue[fun.WorkerFunc]()
		ctx = WithWorkerPool(ctx, "merlin", itertool.Options{})
		ctx = SetWorkerPool(ctx, "kip", queue, itertool.Options{})

		if err := queue.Close(); err != nil {
			t.Error(err)
		}

		err := AddToWorkerPool(ctx, "kip", func(context.Context) error {
			return errors.New("shouldn't run")
		})
		if err == nil {
			t.Fatal("should have an error")
		}
		check.ErrorIs(t, err, pubsub.ErrQueueClosed)

		t.Run("MultiplePools", func(t *testing.T) {
			err := AddToWorkerPool(ctx, "merlin", func(context.Context) error {
				return errors.New("shouldn't run")
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("MultiplePools", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wpCt := &atomic.Int64{}
		ctx = SetShutdownSignal(ctx)
		ctx = WithWorkerPool(ctx, "merlin", itertool.Options{NumWorkers: 50})
		obCt := &atomic.Int64{}
		expected := errors.New("kip")
		ctx = WithObserverWorkerPool(ctx, "kip", func(err error) {
			check.ErrorIs(t, err, expected)
			obCt.Add(1)
		}, itertool.Options{NumWorkers: 50})
		for i := 0; i < 100; i++ {
			err := AddToWorkerPool(ctx, "merlin", func(context.Context) error { wpCt.Add(1); return nil })
			assert.NotError(t, err)
			err = AddToWorkerPool(ctx, "kip", func(context.Context) error { return expected })
			assert.NotError(t, err)
		}
		time.Sleep(250 * time.Millisecond)
		svc := GetOrchestrator(ctx).Service()
		svc.Close()
		check.NotError(t, svc.Wait())
		if obCt.Load() != 100 {
			t.Error(obCt.Load())
		}

		if wpCt.Load() != 100 {
			t.Error(wpCt.Load())
		}
	})
}
