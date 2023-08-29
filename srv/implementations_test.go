package srv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/testt"
)

func TestHelpers(t *testing.T) {
	t.Parallel()
	t.Run("Wait", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		svc := Wait(fun.VariadicIterator(fun.Operation(func(context.Context) { time.Sleep(50 * time.Millisecond) })))
		start := time.Now()
		if err := svc.Start(ctx); err != nil {
			t.Error(err)
		}

		if err := svc.Wait(); err != nil {
			t.Error(err)
		}

		dur := time.Since(start)
		if dur < 50*time.Millisecond || dur > 100*time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("Process", func(t *testing.T) {
		t.Parallel()
		t.Run("Large", func(t *testing.T) {
			count := atomic.Int64{}
			srv := ProcessIterator(
				makeIterator(100),
				func(_ context.Context, in int) error { count.Add(1); return nil },
				fun.WorkerGroupConfNumWorkers(2),
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
		})
		t.Run("Medium", func(t *testing.T) {
			count := atomic.Int64{}
			srv := ProcessIterator(
				makeIterator(50),
				func(_ context.Context, in int) error {
					time.Sleep(10 * time.Millisecond)
					count.Add(1)
					return nil
				},
				fun.WorkerGroupConfNumWorkers(50),
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			start := time.Now()
			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 50 {
				t.Error(count.Load())
			}
			if time.Since(start) < 10*time.Millisecond || time.Since(start) > 250*time.Millisecond {
				t.Error(time.Since(start))
			}
		})
	})

	t.Run("WorkerPool", func(t *testing.T) {
		t.Parallel()
		t.Run("Small", func(t *testing.T) {
			count := &atomic.Int64{}
			srv := WorkerPool(
				makeQueue(t, 100, count),
				fun.WorkerGroupConfWorkerPerCPU(),
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}

			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
		})
		t.Run("Large", func(t *testing.T) {
			count := &atomic.Int64{}
			srv := WorkerPool(
				makeQueue(t, 100, count),
				fun.WorkerGroupConfWorkerPerCPU(),
			)
			ctx := testt.ContextWithTimeout(t, 500*time.Millisecond)

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
			assert.NotError(t, ctx.Err())
		})
	})

	t.Run("HandlerWorkerPool", func(t *testing.T) {
		t.Parallel()
		t.Run("Small", func(t *testing.T) {
			count := &atomic.Int64{}
			errCount := &atomic.Int64{}
			srv := HandlerWorkerPool(
				makeErroringQueue(t, 100, count),
				func(err error) {
					t.Log(err)
					check.Error(t, err)
					errCount.Add(1)
				},
				fun.WorkerGroupConfNumWorkers(50),
			)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}

			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
			if errCount.Load() != 100 {
				t.Error("did not observe correct errors", errCount.Load())
			}
		})
		t.Run("Large", func(t *testing.T) {
			count := &atomic.Int64{}
			errCount := &atomic.Int64{}
			srv := HandlerWorkerPool(
				makeErroringQueue(t, 100, count),
				func(err error) {
					check.Error(t, err)
					errCount.Add(1)
				},
				fun.WorkerGroupConfNumWorkers(50),
			)
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)

			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 100 {
				t.Error(count.Load())
			}
			if errCount.Load() != 100 {
				t.Error("did not observe correct errors", errCount.Load())
			}
			assert.NotError(t, ctx.Err())
		})
	})

	t.Run("Broker", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		broker := pubsub.NewBroker[int](ctx, pubsub.BrokerOptions{})
		srv := Broker(broker)
		if err := srv.Start(ctx); err != nil {
			t.Fatal(err)
		}
		ch := broker.Subscribe(ctx)
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			num := <-ch
			if num != 42 {
				t.Error(num)
			}
		}()

		broker.Publish(ctx, 42)
		fun.WaitChannel(sig).Run(ctx)
	})
}

func TestCmd(t *testing.T) {
	t.Parallel()
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint("Iteration", i), func(t *testing.T) {
			t.Parallel()
			t.Run("SimpleSleep", func(t *testing.T) {
				t.Parallel()

				ctx := testt.Context(t)
				cmd := exec.CommandContext(ctx, "sleep", "1")
				s := Cmd(cmd, 0)
				assert.MaxRuntime(t, 1250*time.Millisecond, func() {
					check.NotError(t, s.Start(ctx))
					check.NotError(t, s.Wait())
				})
				assert.True(t, s.isFinished.Load())
			})
			t.Run("QuickReturn", func(t *testing.T) {
				t.Parallel()

				cmd := exec.Command("sleep", ".1")
				s := Cmd(cmd, 0)
				ctx := testt.Context(t)
				check.NotError(t, s.Start(ctx))
				assert.MaxRuntime(t, 100*time.Millisecond, func() {
					s.Close()
					check.Error(t, s.Wait())
				})
				assert.True(t, s.isFinished.Load())
			})
			t.Run("RunningStartedErrors", func(t *testing.T) {
				t.Parallel()
				ctx := testt.Context(t)
				cmd := exec.CommandContext(ctx, "sleep", "10")
				_ = cmd.Start()
				s := Cmd(cmd, 0)
				check.NotError(t, s.Start(ctx))
				err := s.Wait()
				assert.Error(t, err) // already
				assert.Substring(t, err.Error(), "already started")
			})
			t.Run("TimeoutObserved", func(t *testing.T) {
				t.Parallel()
				ctx := testt.Context(t)
				cmd := exec.CommandContext(ctx, "sleep", "2")
				s := Cmd(cmd, 10*time.Millisecond)
				check.NotError(t, s.Start(ctx))
				assert.MaxRuntime(t, 100*time.Millisecond, func() {
					s.Close()
					check.Error(t, s.Wait())
				})
			})
			t.Run("ForceSigKILL", func(t *testing.T) {
				t.Parallel()
				ctx := testt.Context(t)
				cmd := exec.CommandContext(ctx, "bash", "-c", "trap SIGTERM; sleep 15; echo 'woop'")
				out := &bytes.Buffer{}
				cmd.Stdout = out
				cmd.Stderr = out
				s := Cmd(cmd, 10*time.Millisecond)

				check.NotError(t, s.Start(ctx))
				runtime.Gosched()
				assert.MaxRuntime(t, 500*time.Millisecond, func() {
					s.Close()
					runtime.Gosched()
					err := s.Wait()
					check.Error(t, err)
					testt.Log(t, err)
				})
				testt.Log(t, out.String())
			})
		})
	}
}

func TestDaemon(t *testing.T) {
	t.Parallel()

	t.Run("OnlyRun", func(t *testing.T) {
		baseRunCounter := &atomic.Int64{}
		baseService := &Service{
			Run: func(context.Context) error {
				for {
					cur := baseRunCounter.Load()
					if cur >= 20 {
						return context.Canceled
					}
					if baseRunCounter.CompareAndSwap(cur, cur+1) {
						time.Sleep(time.Millisecond)
						return nil
					}
				}
			},
		}
		ctx := testt.ContextWithTimeout(t, 500*time.Millisecond)
		ds := Daemon(baseService, 10*time.Millisecond)
		check.MinRuntime(t, 100*time.Millisecond, func() {
			check.NotError(t, ds.Start(ctx))
			check.NotError(t, ds.Wait())
		})
		assert.Equal(t, baseRunCounter.Load(), 20)
	})
	t.Run("WithCleanupShutdown", func(t *testing.T) {
		baseRunCounter := &atomic.Int64{}
		baseCleanupCalled := &atomic.Bool{}
		baseShutdownCalled := &atomic.Bool{}
		baseService := &Service{
			Cleanup:  func() error { baseCleanupCalled.Store(true); return nil },
			Shutdown: func() error { baseShutdownCalled.Store(true); return nil },
			Run: func(context.Context) error {
				baseRunCounter.Add(1)
				time.Sleep(10 * time.Millisecond)
				if baseRunCounter.Load() > 10 {
					return context.Canceled
				}
				return nil
			},
		}
		ctx := testt.ContextWithTimeout(t, 110*time.Millisecond)
		ds := Daemon(baseService, 10*time.Millisecond)
		check.MinRuntime(t, 100*time.Millisecond, func() {
			check.NotError(t, ds.Start(ctx))
			check.NotError(t, ds.Wait())
		})
		assert.True(t, baseCleanupCalled.Load())
		assert.True(t, baseShutdownCalled.Load())
	})

	t.Run("CloseTriggers", func(t *testing.T) {
		ctx := testt.Context(t)
		baseRunCounter := &atomic.Int64{}
		baseService := &Service{
			Run: func(ctx context.Context) error {
				baseRunCounter.Add(1)

				time.Sleep(time.Millisecond)
				return errors.New("kip")
			},
		}
		ds := Daemon(baseService, 10*time.Millisecond)
		var err error
		check.MaxRuntime(t, 20*time.Millisecond, func() {
			check.NotError(t, ds.Start(ctx))
			time.Sleep(5 * time.Millisecond)
			runtime.Gosched()
			ds.Close()
			err = ds.Wait()
			check.Error(t, err)
		})
		assert.True(t, baseRunCounter.Load() >= 2)
		assert.True(t, len(ers.Unwind(err)) >= 2)
		assert.Substring(t, err.Error(), "kip")
	})
	t.Run("ShutdownTriggers", func(t *testing.T) {
		ctx := testt.Context(t)
		baseRunCounter := &atomic.Int64{}
		baseService := &Service{
			Run: func(ctx context.Context) error {
				baseRunCounter.Add(1)

				time.Sleep(time.Millisecond)
				return errors.New("kip")
			},
		}
		ds := Daemon(baseService, 10*time.Millisecond)
		var err error
		check.MaxRuntime(t, 20*time.Millisecond, func() {
			check.NotError(t, ds.Start(ctx))
			time.Sleep(5 * time.Millisecond)
			runtime.Gosched()
			check.NotError(t, ds.Shutdown())
			err = ds.Wait()
			check.Error(t, err)
		})
		assert.True(t, baseRunCounter.Load() >= 2)
		assert.True(t, len(ers.Unwind(err)) >= 2)
		assert.Substring(t, err.Error(), "kip")
	})
	t.Run("CancelationTriggersAbort", func(t *testing.T) {
		ctx, cancel := context.WithCancel(testt.Context(t))
		baseRunCounter := &atomic.Int64{}
		baseService := &Service{
			Run: func(ctx context.Context) error {
				baseRunCounter.Add(1)
				time.Sleep(2 * time.Millisecond)
				return nil
			},
		}
		ds := Daemon(baseService, time.Second)
		ds.Shutdown = func() error { return nil }

		check.MaxRuntime(t, 20*time.Millisecond, func() {
			check.NotError(t, ds.Start(ctx))
			time.Sleep(5 * time.Millisecond)
			runtime.Gosched()
			cancel()

			check.NotError(t, ds.Wait())
		})
		assert.True(t, baseRunCounter.Load() >= 1)
	})
}

func TestCleanup(t *testing.T) {
	t.Parallel()
	t.Run("Basic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := pubsub.NewUnlimitedQueue[fun.Worker]()

		signal := make(chan struct{})
		count := &atomic.Int64{}
		s := Cleanup(pipe, 10*time.Second)

		assert.NotError(t, s.Start(ctx))

		check.Equal(t, 0, count.Load())
		for i := 0; i < 100; i++ {
			check.NotError(t, pipe.Add(func(context.Context) error {
				count.Add(1)
				return nil
			}))
		}
		check.Equal(t, 0, count.Load())
		go func() {
			defer close(signal)
			check.Equal(t, 0, count.Load())
			check.NotError(t, s.Wait())
			check.Equal(t, 100, count.Load())
		}()

		time.Sleep(100 * time.Millisecond)

		check.True(t, s.Running())
		check.Equal(t, 0, count.Load())
		check.NotError(t, s.Shutdown())
		check.NotError(t, s.Wait())
		<-signal
		check.Equal(t, 100, count.Load())
	})
	t.Run("Context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ctx = WithCleanup(ctx)
		count := &atomic.Int64{}

		signal := make(chan struct{})
		go func() {
			defer close(signal)
			check.Equal(t, 0, count.Load())
			check.True(t, GetOrchestrator(ctx).Service().Running())
			check.NotError(t, GetOrchestrator(ctx).Wait())
			check.Equal(t, 100, count.Load())
		}()

		called := 0
		for i := 0; i < 100; i++ {
			check.NotPanic(t, func() {
				called++
				AddCleanup(ctx, func(context.Context) error {
					count.Add(1)
					return nil
				})
			})
		}
		check.True(t, HasCleanup(ctx))
		check.Equal(t, 100, called)
		time.Sleep(30 * time.Millisecond)
		check.Equal(t, 0, count.Load())

		GetOrchestrator(ctx).Service().Close()
		<-signal

		check.Equal(t, 100, count.Load())
	})
	t.Run("ContextCleanupError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = WithCleanup(ctx)
		err := errors.New("kip")

		AddCleanupError(ctx, err)

		orch := GetOrchestrator(ctx)
		srv := orch.Service()
		time.Sleep(10 * time.Millisecond)
		srv.Close()
		out := srv.Wait()
		assert.True(t, out != nil)
		assert.ErrorIs(t, out, err)
	})
}
