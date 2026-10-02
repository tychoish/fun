package srv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/fun/wpa"
)

func TestHelpers(t *testing.T) {
	t.Parallel()
	t.Run("Wait", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		svc := Wait(irt.One(fnx.Operation(func(context.Context) { time.Sleep(50 * time.Millisecond) })))
		start := time.Now()
		if err := svc.Start(ctx); err != nil {
			t.Error(err)
		}

		if err := svc.Wait(); err != nil {
			t.Error(err)
		}

		dur := time.Since(start)
		if dur < 50*time.Millisecond {
			t.Error(dur)
		}
	})
	t.Run("Process", func(t *testing.T) {
		t.Parallel()
		t.Run("Large", func(t *testing.T) {
			count := atomic.Int64{}
			srv := Handler(
				makeSeq(100),
				func(_ context.Context, _ int) error { count.Add(1); return nil },
				wpa.WorkerGroupConfNumWorkers(2),
			)
			ctx := t.Context()

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
			srv := Handler(
				makeSeq(50),
				func(_ context.Context, _ int) error {
					time.Sleep(10 * time.Millisecond)
					count.Add(1)
					return nil
				},
				wpa.WorkerGroupConfNumWorkers(50),
			)
			ctx := t.Context()

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
			if time.Since(start) < 10*time.Millisecond {
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
				wpa.WorkerGroupConfWorkerPerCPU(),
			)
			ctx := t.Context()

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
				wpa.WorkerGroupConfWorkerPerCPU(),
			)
			ctx := testt.ContextWithTimeout(t, time.Minute)

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
		t.Run("ShutdownDrainsQueue", func(t *testing.T) {
			count := &atomic.Int64{}
			queue := pubsub.NewUnlimitedQueue[fnx.Worker]()

			// Jobs block on gate so the test controls when the queue can drain.
			gate := make(chan struct{})
			started := make(chan struct{}, 80)
			released := &atomic.Bool{}

			// Add jobs to the queue without closing it
			for range 50 {
				assert.NotError(t, queue.Push(func(_ context.Context) error {
					started <- struct{}{}
					<-gate
					count.Add(1)
					return nil
				}))
			}

			srv := WorkerPool(queue, wpa.WorkerGroupConfNumWorkers(5))

			ctx := t.Context()

			// Start the service
			if err := srv.Start(ctx); err != nil {
				t.Fatal(err)
			}

			// Wait until workers are processing
			<-started

			// Add more jobs after service started
			for range 30 {
				assert.NotError(t, queue.Push(func(_ context.Context) error {
					started <- struct{}{}
					<-gate
					count.Add(1)
					return nil
				}))
			}

			// Verify jobs are in flight but not all completed yet
			check.True(t, count.Load() < 80)

			// Call shutdown - this should drain the queue, which cannot
			// finish until the gate is released.
			go func() {
				runtime.Gosched()
				released.Store(true)
				close(gate)
			}()
			if err := srv.Shutdown(); err != nil {
				t.Fatal(err)
			}
			// Shutdown must have waited for the blocked jobs to drain.
			check.True(t, released.Load())

			// Wait for service to complete
			if err := srv.Wait(); err != nil {
				t.Fatal(err)
			}

			// Verify all 80 jobs were processed
			check.Equal(t, int64(80), count.Load())

			// Queue should be empty and closed
			check.Equal(t, 0, queue.Len())
		})
	})

	t.Run("HandlerWorkerPool", func(t *testing.T) {
		t.Parallel()
		t.Run("Small", func(t *testing.T) {
			count := &atomic.Int64{}
			errCount := &atomic.Int64{}
			srv := WorkerPoolWithErrorHandler(
				makeErroringQueue(t, 100, count),
				func(err error) {
					t.Log(err)
					check.Error(t, err)
					errCount.Add(1)
				},
				wpa.WorkerGroupConfNumWorkers(50),
			)
			ctx := t.Context()

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
			srv := WorkerPoolWithErrorHandler(
				makeErroringQueue(t, 100, count),
				func(err error) {
					check.Error(t, err)
					errCount.Add(1)
				},
				wpa.WorkerGroupConfNumWorkers(50),
			)
			ctx := testt.ContextWithTimeout(t, time.Minute)

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
		ctx := t.Context()

		broker := pubsub.NewBroker[int](ctx, pubsub.BrokerOptions{})
		srv := Broker(broker)
		if err := srv.Start(ctx); err != nil {
			t.Fatal(err)
		}
		ch, err := broker.Subscribe(ctx)
		check.NotError(t, err)
		sig := make(chan struct{})
		go func() {
			defer close(sig)
			num := <-ch
			if num != 42 {
				t.Error(num)
			}
		}()

		check.NotError(t, broker.Send(ctx, 42))
		fnx.WaitChannel(sig).Run(ctx)
	})
}

func TestCmd(t *testing.T) {
	t.Run("Short", func(t *testing.T) {
		t.Run("SimpleSleep", func(t *testing.T) {
			ctx := testt.Context(t)
			cmd := exec.CommandContext(ctx, "sleep", ".5")
			s := Cmd(cmd, 0)
			assert.MinRuntime(t, 500*time.Millisecond, func() {
				check.NotError(t, s.Start(ctx))
				check.NotError(t, s.Wait())
			})
			assert.True(t, s.isFinished.Load())
		})
		t.Run("QuickReturn", func(t *testing.T) {
			cmd := exec.Command("sleep", "60")
			s := Cmd(cmd, 0)
			ctx := testt.Context(t)
			check.NotError(t, s.Start(ctx))
			// Close must interrupt the process rather than wait it out.
			assert.MaxRuntime(t, 30*time.Second, func() {
				s.Close()
				check.Error(t, s.Wait())
			})
			assert.True(t, s.isFinished.Load())
		})
		t.Run("TimeoutObserved", func(t *testing.T) {
			ctx := testt.Context(t)
			cmd := exec.CommandContext(ctx, "sleep", "60")
			s := Cmd(cmd, 10*time.Millisecond)
			check.NotError(t, s.Start(ctx))
			// Close must interrupt the process rather than wait it out.
			assert.MaxRuntime(t, 30*time.Second, func() {
				s.Close()
				check.Error(t, s.Wait())
			})
		})
	})

	t.Run("RunningStartedErrors", func(t *testing.T) {
		ctx := testt.Context(t)
		cmd := exec.CommandContext(ctx, "sleep", "10")
		_ = cmd.Start()
		s := Cmd(cmd, 0)
		check.NotError(t, s.Start(ctx))
		err := s.Wait()
		assert.Error(t, err) // already
		assert.Substring(t, err.Error(), "already started")
	})
	t.Run("Termination", func(t *testing.T) {
		t.Run("SIGTERM", func(t *testing.T) {
			ctx := testt.Context(t)
			ctx = SetBaseContext(ctx)
			// exec so that the signaled process is the sleeper itself: an
			// orphaned child would keep the output pipe open and block Wait.
			ready := filepath.Join(t.TempDir(), "ready")
			cmd := exec.CommandContext(ctx, "bash", "-c", "touch "+ready+"; exec sleep 60")
			out := &bytes.Buffer{}
			cmd.Stdout = out
			cmd.Stderr = out
			s := Cmd(cmd, 100*time.Millisecond)
			check.NotError(t, s.Start(ctx))
			waitForFile(t, ready)
			s.Shutdown()

			assert.MaxRuntime(t, 30*time.Second, func() {
				err := s.Wait()
				check.Error(t, err)
				testt.Log(t, err)
			})
			testt.Log(t, out.String())
		})
		t.Run("ForceSigKILL", func(t *testing.T) {
			ctx := testt.Context(t)
			ctx = SetBaseContext(ctx)
			// SIGTERM is ignored (and the ignore is inherited across exec) once
			// the marker exists, so only SIGKILL can stop the process.
			ready := filepath.Join(t.TempDir(), "ready")
			cmd := exec.CommandContext(ctx, "bash", "-c", "trap '' TERM; touch "+ready+"; exec sleep 60")
			out := &bytes.Buffer{}
			cmd.Stdout = out
			cmd.Stderr = out
			s := Cmd(cmd, 100*time.Millisecond)
			check.NotError(t, s.Start(ctx))
			waitForFile(t, ready)
			s.Shutdown()

			assert.MaxRuntime(t, 30*time.Second, func() {
				err := s.Wait()
				check.Error(t, err)
				testt.Log(t, err)
			})
			testt.Log(t, out.String())
		})
	})
}

// waitForFile blocks until the path exists, so tests can tell that a
// subprocess has finished its setup before they signal it.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("subprocess never became ready")
		}
		time.Sleep(time.Millisecond)
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
		ctx := testt.ContextWithTimeout(t, time.Minute)
		ctx = SetBaseContext(ctx)

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
		ctx := testt.ContextWithTimeout(t, time.Minute)
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
		third := make(chan struct{})
		baseService := &Service{
			Run: func(_ context.Context) error {
				// the third run starts only after two errors were recorded.
				if baseRunCounter.Add(1) == 3 {
					close(third)
				}
				time.Sleep(time.Millisecond)
				return errors.New("kip")
			},
		}
		ds := Daemon(baseService, 5*time.Millisecond)
		var err error
		check.NotError(t, ds.Start(ctx))
		<-third
		ds.Close()
		err = ds.Wait()
		check.Error(t, err)
		baseRunCount := baseRunCounter.Load()
		testt.Log(t, "baseRunCounter", baseRunCount)
		testt.Log(t, err)
		assert.True(t, baseRunCount >= 2)
		errs := len(ers.Unwind(err))
		testt.Log(t, err == nil, errs)
		assert.True(t, errs >= 2)
		assert.Substring(t, err.Error(), "kip")
	})
	t.Run("ShutdownTriggers", func(t *testing.T) {
		ctx := testt.Context(t)
		baseRunCounter := &atomic.Int64{}
		third := make(chan struct{})
		baseService := &Service{
			Run: func(_ context.Context) error {
				// the third run starts only after two errors were recorded.
				if baseRunCounter.Add(1) == 3 {
					close(third)
				}
				time.Sleep(time.Millisecond)
				return errors.New("kip")
			},
		}
		ds := Daemon(baseService, 5*time.Millisecond)
		var err error
		check.NotError(t, ds.Start(ctx))
		<-third
		check.NotError(t, ds.Shutdown())
		err = ds.Wait()
		check.Error(t, err)
		baseRunCount := baseRunCounter.Load()
		testt.Log(t, "errs", err == nil, err)
		testt.Log(t, "baseRunCounter", baseRunCount)
		assert.True(t, baseRunCount >= 2)
		assert.True(t, len(ers.Unwind(err)) >= 2)
		assert.Substring(t, err.Error(), "kip")
	})
	t.Run("CancelationTriggersAbort", func(t *testing.T) {
		ctx, cancel := context.WithCancel(testt.Context(t))
		baseRunCounter := &atomic.Int64{}
		first := make(chan struct{})
		once := &sync.Once{}
		baseService := &Service{
			Run: func(_ context.Context) error {
				defer once.Do(func() { close(first) })
				baseRunCounter.Add(1)
				time.Sleep(2 * time.Millisecond)
				return nil
			},
		}
		ds := Daemon(baseService, time.Second)
		ds.Shutdown = func() error { return nil }

		check.NotError(t, ds.Start(ctx))
		<-first
		cancel()

		check.NotError(t, ds.Wait())
		assert.True(t, baseRunCounter.Load() >= 1)
	})
}

func TestCleanup(t *testing.T) {
	t.Parallel()
	t.Run("Basic", func(t *testing.T) {
		var cancel context.CancelFunc
		ctx := context.Background()

		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		ctx = SetBaseContext(ctx)

		pipe := pubsub.NewUnlimitedQueue[fnx.Worker]()

		signal := make(chan struct{})
		count := &atomic.Int64{}
		s := Cleanup(pipe, 10*time.Second)

		assert.NotError(t, s.Start(ctx))

		check.Equal(t, 0, count.Load())
		for range 100 {
			check.NotError(t, pipe.Push(func(context.Context) error {
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
		ctx := t.Context()

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
		for range 100 {
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
		ctx := t.Context()
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
