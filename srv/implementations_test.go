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
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/testt"
)

func TestCmd(t *testing.T) {
	t.Parallel()
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("Iteration", i), func(t *testing.T) {
			t.Parallel()
			t.Run("SimpleSleep", func(t *testing.T) {
				cmd := exec.Command("sleep", "1")
				s := Cmd(cmd, 0)
				ctx := testt.Context(t)
				assert.MaxRuntime(t, 1250*time.Millisecond, func() {
					check.NotError(t, s.Start(ctx))
					check.NotError(t, s.Wait())
				})
				assert.True(t, s.isFinished.Load())
			})
			t.Run("QuickReturn", func(t *testing.T) {
				cmd := exec.Command("sleep", ".1")
				s := Cmd(cmd, 0)
				ctx := testt.Context(t)
				check.NotError(t, s.Start(ctx))
				assert.MaxRuntime(t, 15*time.Millisecond, func() {
					s.Close()
					check.Error(t, s.Wait())
				})
				assert.True(t, s.isFinished.Load())
			})
			t.Run("RunningStartedErrors", func(t *testing.T) {
				ctx := testt.Context(t)
				cmd := exec.Command("sleep", "10")
				_ = cmd.Start()
				s := Cmd(cmd, 0)
				check.NotError(t, s.Start(ctx))
				err := s.Wait()
				assert.Error(t, err) // already
				assert.Substring(t, err.Error(), "already started")
			})
			t.Run("TimeoutObserved", func(t *testing.T) {
				ctx := testt.Context(t)
				cmd := exec.Command("sleep", "2")
				s := Cmd(cmd, time.Millisecond)
				check.NotError(t, s.Start(ctx))
				assert.MaxRuntime(t, 10*time.Millisecond, func() {
					s.Close()
					check.Error(t, s.Wait())
				})
			})
			t.Run("ForceSigKILL", func(t *testing.T) {
				ctx := testt.Context(t)
				cmd := exec.Command("bash", "-c", "trap SIGTERM; sleep 15; echo 'woop'")
				out := &bytes.Buffer{}
				cmd.Stdout = out
				cmd.Stderr = out
				s := Cmd(cmd, time.Millisecond)

				check.NotError(t, s.Start(ctx))
				assert.MaxRuntime(t, 20*time.Millisecond, func() {
					s.Close()
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
				if baseRunCounter.Load() > 10 {
					return context.Canceled
				}
				baseRunCounter.Add(1)
				time.Sleep(20 * time.Millisecond)
				return nil
			},
		}
		ctx := testt.ContextWithTimeout(t, 200*time.Millisecond)
		ds := Daemon(baseService, 20*time.Millisecond)
		check.MinRuntime(t, 100*time.Millisecond, func() {
			check.NotError(t, ds.Start(ctx))
			check.NotError(t, ds.Wait())
		})
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, baseRunCounter.Load(), 10)
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
		assert.True(t, len(erc.Unwind(err)) >= 2)
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
		assert.True(t, len(erc.Unwind(err)) >= 2)
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
	t.Run("Basic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := pubsub.NewUnlimitedQueue[fun.WorkerFunc]()

		signal := make(chan struct{})
		count := &atomic.Int64{}
		s := Cleanup(pipe, 10*time.Second)

		assert.NotError(t, s.Start(ctx))

		go func() {
			defer close(signal)
			check.Equal(t, 0, count.Load())
			check.NotError(t, s.Wait())
			check.Equal(t, 100, count.Load())
		}()

		for i := 0; i < 100; i++ {
			check.NotError(t, pipe.Add(func(context.Context) error {
				count.Add(1)
				return nil
			}))
		}
		time.Sleep(time.Millisecond)
		check.True(t, s.Running())
		check.Equal(t, 0, count.Load())
		check.NotError(t, s.Shutdown())
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
		time.Sleep(time.Millisecond)
		check.Equal(t, 0, count.Load())

		GetOrchestrator(ctx).Service().Close()
		<-signal

		check.Equal(t, 100, count.Load())
	})
}
