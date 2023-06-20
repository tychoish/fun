package fun

import (
	"context"
	"fmt"
	"io"
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

func TestProducer(t *testing.T) {
	t.Parallel()
	t.Run("WithCancel", func(t *testing.T) {
		ctx := testt.Context(t)
		wf, cancel := Producer[int](func(ctx context.Context) (int, error) {
			timer := time.NewTimer(time.Hour)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				check.ErrorIs(t, ctx.Err(), context.Canceled)
				return 42, nil
			case <-timer.C:
				t.Error("should not have reached this timeout")
			}
			return -1, ers.Error("unreachable")
		}).WithCancel()
		assert.MinRuntime(t, 40*time.Millisecond, func() {
			assert.MaxRuntime(t, 75*time.Millisecond, func() {
				go func() { time.Sleep(60 * time.Millisecond); cancel() }()
				time.Sleep(time.Millisecond)
				out, err := wf(ctx)
				check.Equal(t, out, 42)
				check.NotError(t, err)
			})
		})
	})
	t.Run("Once", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Producer[int](func(ctx context.Context) (int, error) {
			called++
			return 42, nil
		}).Once()
		for i := 0; i < 1024; i++ {
			val, err := pf(ctx)
			assert.NotError(t, err)
			assert.Equal(t, val, 42)
		}
		check.Equal(t, called, 1)
	})
	t.Run("If", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Producer[int](func(ctx context.Context) (int, error) {
			called++
			return 42, nil
		})

		check.Equal(t, 0, pf.If(false).Must(ctx))
		check.Equal(t, 0, called)
		check.Equal(t, 42, pf.If(true).Must(ctx))
		check.Equal(t, 1, called)
		check.Equal(t, 42, pf.If(true).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 0, pf.If(false).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 42, pf.Must(ctx))
		check.Equal(t, 3, called)
	})
	t.Run("When", func(t *testing.T) {
		ctx := testt.Context(t)
		called := 0
		pf := Producer[int](func(ctx context.Context) (int, error) {
			called++
			return 42, nil
		})

		check.Equal(t, 0, pf.When(func() bool { return false }).Must(ctx))
		check.Equal(t, 0, called)
		check.Equal(t, 42, pf.When(func() bool { return true }).Must(ctx))
		check.Equal(t, 1, called)
		check.Equal(t, 42, pf.When(func() bool { return true }).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 0, pf.When(func() bool { return false }).Must(ctx))
		check.Equal(t, 2, called)
		check.Equal(t, 42, pf.Must(ctx))
		check.Equal(t, 3, called)
	})
	t.Run("Constructor", func(t *testing.T) {
		t.Run("Value", func(t *testing.T) {
			ctx := testt.Context(t)
			pf := ValueProducer(42)
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.NotError(t, err)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Static", func(t *testing.T) {
			ctx := testt.Context(t)
			root := ers.Error(t.Name())
			pf := StaticProducer(42, root)
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.ErrorIs(t, err, root)
				assert.Equal(t, v, 42)
			}
		})
		t.Run("Future", func(t *testing.T) {
			ctx := testt.Context(t)
			ch := make(chan *string, 2)
			prod := MakeFuture(ch)
			ch <- ft.Ptr("hi")
			ch <- ft.Ptr("hi")
			check.NotZero(t, prod.Must(ctx))
		})
		t.Run("Blocking", func(t *testing.T) {
			ctx := testt.Context(t)
			root := ers.Error(t.Name())
			pf := BlockingProducer(func() (int, error) { return 42, root })
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.ErrorIs(t, err, root)
				assert.Equal(t, v, 42)
			}
		})

		t.Run("Consistent", func(t *testing.T) {
			ctx := testt.Context(t)
			pf := ConsistentProducer(func() int { return 42 })
			for i := 0; i < 1024; i++ {
				v, err := pf(ctx)
				assert.Equal(t, v, 42)
				assert.NotError(t, err)
			}
		})
	})
	t.Run("Lock", func(t *testing.T) {
		t.Run("NilLockPanics", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			op := Producer[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			})
			check.Panic(t, func() {
				v, err := op.WithLock(nil)(ctx)
				check.Equal(t, v, 0)
				check.Error(t, err)
			})
			check.Equal(t, count, 0)
		})
		// the rest of the tests are really just "tempt the
		// race detector"
		t.Run("ManagedLock", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			op := Producer[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			})

			opct := &atomic.Int64{}
			wg := &WaitGroup{}
			future := op.Lock().Worker(
				func(in int) { opct.Add(1); check.Equal(t, in, 42) },
			).StartGroup(ctx, 128)

			wg.Wait(ctx)
			check.NotError(t, future(ctx))
			check.Equal(t, 128, opct.Load())
			assert.Equal(t, count, 128)
		})
		t.Run("CustomLock", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			op := Producer[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			})
			wg := &WaitGroup{}
			mu := &sync.Mutex{}
			opct := &atomic.Int64{}
			op.WithLock(mu).Operation(
				func(in int) { opct.Add(1); check.Equal(t, in, 42) },
				func(err error) { opct.Add(1); check.NotError(t, err) },
			).StartGroup(ctx, wg, 128)
			wg.Wait(ctx)
			check.Equal(t, 2*128, opct.Load())
			assert.Equal(t, count, 128)
		})
		t.Run("ForBackground", func(t *testing.T) {
			count := 0
			op := Producer[int](func(context.Context) (int, error) {
				count++
				return 42, nil
			}).Lock()
			ctx := testt.Context(t)

			obct := 0
			obv := Observer[int](func(in int) { obct++; check.Equal(t, in, 42) }).Lock()
			jobs := []Worker{}

			ft.DoTimes(128, func() { jobs = append(jobs, op.Background(ctx, obv)) })

			err := SliceIterator(jobs).ProcessParallel(ctx, HF.ProcessWorker(), NumWorkers(4))
			assert.NotError(t, err)
			check.Equal(t, count, 128)
			check.Equal(t, obct, 128)
		})
	})
	t.Run("ScopedContext", func(t *testing.T) {
		ctx := testt.Context(t)
		sig := make(chan struct{})
		ops := []string{}

		var pf Producer[int] = func(ctx context.Context) (int, error) {
			go func() { defer close(sig); <-ctx.Done(); ops = append(ops, "inner") }()
			ops = append(ops, "outer")
			time.Sleep(10 * time.Millisecond)
			return 42, nil
		}
		check.MinRuntime(t, 10*time.Millisecond, func() {
			val, err := pf.Run(ctx)
			check.NotError(t, err)
			check.Equal(t, val, 42)

			<-sig
		})
		check.EqualItems(t, ops, []string{"outer", "inner"})
	})
	t.Run("CheckBlock", func(t *testing.T) {
		count := 0
		var err error
		var pf Producer[int] = func(ctx context.Context) (int, error) {
			count++
			return 42, err
		}
		val, ok := pf.CheckBlock()
		check.True(t, ok)
		check.Equal(t, 42, val)
		err = ers.New("check should fail")

		val, ok = pf.CheckBlock()
		check.True(t, !ok)
		check.Equal(t, 42, val)

	})
	t.Run("WithoutErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		count := 0
		var err error
		var pf Producer[int] = func(_ context.Context) (int, error) { count++; return 42, err }

		checkOutput := func(in int, err error) error { check.Equal(t, 42, in); return err }

		err = ers.Error("hello")
		pf = pf.WithoutErrors(io.EOF)
		assert.Equal(t, count, 0)
		assert.Error(t, checkOutput(pf(ctx)))
		assert.Equal(t, count, 1)
		err = io.EOF
		assert.NotError(t, checkOutput(pf(ctx)))
		assert.Equal(t, count, 2)
		err = context.Canceled
		assert.Error(t, checkOutput(pf(ctx)))
		assert.Equal(t, count, 3)
	})
	t.Run("Force", func(t *testing.T) {
		count := 0
		var err error = io.EOF
		var pf Producer[int] = func(_ context.Context) (int, error) { count++; return 42, err }
		var out int

		assert.Panic(t, func() { out = pf.Force() })
		assert.Equal(t, out, 0)
		assert.Equal(t, count, 1)
		err = nil
		assert.Equal(t, 42, pf.Force())
	})
	t.Run("Block", func(t *testing.T) {
		count := 0
		var pf Producer[int] = func(_ context.Context) (int, error) { count++; return 42, io.EOF }
		out, err := pf.Block()
		assert.ErrorIs(t, err, io.EOF)
		assert.Equal(t, out, 42)
		assert.Equal(t, count, 1)
	})
	t.Run("Chain", func(t *testing.T) {
		t.Run("Exhausted", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			var pf Producer[int] = func(_ context.Context) (int, error) { count++; return -1, io.EOF }
			pf = pf.Join(pf)
			out, err := pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
			assert.Equal(t, 0, out)

			assert.Equal(t, 2, count)

			out, err = pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
			assert.Equal(t, 0, out)
		})
		t.Run("FirstContinues", func(t *testing.T) {
			ctx := testt.Context(t)
			counter := &atomic.Int64{}

			pf := producerContinuesOnce(42, counter)
			pf = pf.Join(producerContinuesOnce(42, counter))
			check.Equal(t, counter.Load(), 0)

			val, err := pf(ctx)
			assert.NotError(t, err)
			assert.Equal(t, val, 0)
			check.Equal(t, counter.Load(), 2)

			val, err = pf(ctx)
			assert.Error(t, err)
			assert.Equal(t, val, 0)
			check.Equal(t, counter.Load(), 4)
		})
		t.Run("SecondContinues", func(t *testing.T) {
			ctx := testt.Context(t)
			counter := &atomic.Int64{}
			pf := Producer[int](func(ctx context.Context) (int, error) {
				return -1, io.EOF
			}).Join(producerContinuesOnce(42, counter))
			out, err := pf(ctx)
			assert.NotError(t, err)
			assert.Zero(t, out)
			assert.Equal(t, counter.Load(), 2)
		})
		t.Run("ErrorFirstCanceled", func(t *testing.T) {
			ctx := testt.Context(t)
			count := 0
			pf := Producer[int](func(ctx context.Context) (int, error) {
				return -1, context.Canceled
			}).Join(func(ctx context.Context) (int, error) { count++; return 300, nil })
			out, err := pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Zero(t, count)

			// should repeat the second time
			out, err = pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Zero(t, count)
		})
		t.Run("SecondCancled", func(t *testing.T) {
			ctx := testt.Context(t)
			counter := 0
			pf := Producer[int](func(ctx context.Context) (int, error) {
				return -1, io.EOF
			}).Join(func(ctx context.Context) (int, error) { counter++; return 300, context.Canceled })
			out, err := pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Equal(t, counter, 1)

			// should repeat the second time
			out, err = pf(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, out, 0)
			assert.Equal(t, counter, 1)
		})

	})

}

func producerContinuesOnce[T any](out T, counter *atomic.Int64) Producer[T] {
	once := &sync.Once{}
	var zero T
	return func(ctx context.Context) (_ T, err error) {
		once.Do(func() {
			out = zero
			err = ErrIteratorSkip
			fmt.Println("pow")
		})
		if counter.Add(1) > 2 {
			return zero, io.EOF
		}

		return out, err
	}

}
