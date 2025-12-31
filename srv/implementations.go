package srv

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/opt"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/wpa"
)

// Group makes it possible to have a collection of services, provided
// via a stream, that behave as a single service. All services are
// started concurrently (and without order) and shutdown concurrently
// (and without order). The Service produced by group, has the same
// semantics as any other Service.
//
// Use Group(itertool.Slice([]*Service)) to produce a group from a
// slice of *Services,.
func Group(services iter.Seq[*Service]) *Service {
	waiters := pubsub.NewUnlimitedQueue[func() error]()
	wg := &fnx.WaitGroup{}
	ec := &erc.Collector{}

	return &Service{
		Run: func(ctx context.Context) error {
			for srvc := range services {
				wg.Add(1)
				go func(s *Service) {
					defer ec.Recover()
					defer wg.Done()
					defer func() { fun.Invariant.Ok(waiters.Add(s.Wait) == nil) }()
					ec.Push(s.Start(ctx))
				}(srvc)
			}
			wg.Wait(ctx)
			ec.Push(waiters.Close())
			return nil
		},
		Cleanup: func() error {
			defer ec.Recover()
			// because we know that the implementation of the
			// waiters stream won't block in this context, it's
			// safe to call it with a background context, though
			// it's worth being careful here
			ctx := context.Background()

			// we're calling each service's wait() here, which
			// might be a recipe for deadlocks, but it gives us
			// the chance to collect all errors from the contained
			// services. This will cause our "group service" to
			// have the same semantics as a single service, however.

			for value := range waiters.IteratorWait(ctx) {
				wg.Add(1)
				go func(wait func() error) {
					defer ec.Recover()
					defer wg.Done()
					ec.Push(wait())
				}(value)
			}

			wg.Operation().Wait()
			return ec.Resolve()
		},
	}
}

// HTTP wraps an http.Server object in a Service, both for common use
// and convenience, and as an example for implementing arbitrary
// services.
//
// If the http.Server's BaseContext method is not set, it is
// overridden with a function that provides a copy of the context
// passed to the service's Start method: this ensures that requests
// have access to the same underlying context as the service.
func HTTP(name string, shutdownTimeout time.Duration, hs *http.Server) *Service {
	return &Service{
		Name: name,
		Run: func(ctx context.Context) error {
			if hs.BaseContext == nil {
				hs.BaseContext = func(net.Listener) context.Context { return ctx }
			}

			if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
		Shutdown: func() error {
			sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			return hs.Shutdown(sctx)
		},
	}
}

// Wait creates a service that runs until *both* all wait functions
// have returned *and* the stream is exhausted. The Service's wait
// function returns an error that aggregates all errors (e.g. panics)
// encountered by the constituent wait functions.
//
// Wait produces a service that fills the same role as the
// fun.WaitMerge function, but that can be more easily integrated into
// existing orchestration tools.
//
// When the service returns all worker Goroutines as well as the input
// worker will have returned. Use a blocking pubsub stream to
// dispatch wait functions throughout the lifecycle of your program.
func Wait(seq iter.Seq[fnx.Operation]) *Service {
	wg := &sync.WaitGroup{}
	ec := &erc.Collector{}
	return &Service{
		Run: func(ctx context.Context) error {
			for value := range seq {
				wg.Add(1)
				go func(fn fnx.Operation) {
					defer ec.Recover()
					defer wg.Done()
					fn(ctx)
				}(value)
			}

			wg.Wait()
			return nil
		},
		Cleanup: func() error { wg.Wait(); return ec.Resolve() },
	}
}

// ProcessStream runs an itertool.ParallelForEach operation as a
// *Service. For a long running service, use a stream that is
// blocking (e.g. based on a pubsub queue/deque or a channel.)
func ProcessStream[T any](
	seq iter.Seq[T],
	processor fnx.Handler[T],
	optp ...opt.Provider[*wpa.WorkerGroupConf],
) *Service {
	st := fun.IteratorStream(seq)
	return &Service{
		Run:      st.Parallel(processor, optp...),
		Shutdown: st.Close,
	}
}

// Cleanup provides a service which services the provided queue of
// worker functions and runs the shutdown functions during service
// shutdown (e.g. either after Close() is called or when the context
// is canceled.) The timeout, when non-zero, is passed to the clean up
// operation. Cleanup functions are dispatched in parallel.
func Cleanup(pipe *pubsub.Queue[fnx.Worker], timeout time.Duration) *Service {
	closer, waitForSignal := fnx.MAKE.Signal()

	return &Service{
		Run:      waitForSignal.WithErrorFilter(erc.NewFilter().WithoutContext()),
		Shutdown: func() error { closer(); ; return pipe.Close() },
		Cleanup: func() error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			if err := fun.InterfaceStream(pipe.Distributor()).Parallel(
				func(ctx context.Context, wf fnx.Worker) error { return wf.Run(ctx) },
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfContinueOnPanic(),
				wpa.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx); err != nil {
				return fmt.Errorf("hit timeout [%d], %w", timeout, err)
			}
			return nil
		},
	}
}

// WorkerPool wraps a pubsub.Queue of functions that represent units
// of work in an worker pool. The pool follows the semantics
// configured by the itertool.Options, with regards to error handling,
// panic handling, and parallelism. Errors are collected and
// propogated to the service's ywait function.
func WorkerPool(workQueue *pubsub.Queue[fnx.Worker], optp ...opt.Provider[*wpa.WorkerGroupConf]) *Service {
	return &Service{
		Run: fun.InterfaceStream(workQueue.Distributor()).Parallel(
			func(ctx context.Context, fn fnx.Worker) error {
				return fn.Run(ctx)
			},
			optp...,
		),
		// TODO: have a shutdown methot that will block till
		// the queue shutsdown.
		Shutdown: workQueue.Close,
	}
}

// HandlerWorkerPool has similar semantics and use to the WorkerPool,
// but rather than aggregating errors, all errors are passed to the
// observer function, which is responsible for ignoring or processing
// the errors. The worker pool will respect continue/abort on error or
// panics as expected.
//
// Handler pools may be more operable if your workers generate many
// errors, and/or your process is long lived.
//
// The service itself may have execution or shutdown related errors,
// particularly if there is an invariant violation or panic during
// service execution which will be propagated to the return value of
// the service's Wait method; but all errors that occur during the
// execution of the workload will be observed (including panics) as well.
func HandlerWorkerPool(
	workQueue *pubsub.Queue[fnx.Worker],
	observer fn.Handler[error],
	optp ...opt.Provider[*wpa.WorkerGroupConf],
) *Service {
	s := &Service{
		Run: fun.InterfaceStream(workQueue.Distributor()).Parallel(
			func(ctx context.Context, fn fnx.Worker) error {
				observer(fn.Run(ctx))
				return nil
			},
			optp...,
		),
		Shutdown: workQueue.Close,
	}
	s.ErrorHandler.Set(observer)
	return s
}

// Broker creates a Service implementation that wraps a
// pubsub.Broker[T] implementation for integration into service
// orchestration frameworks.
func Broker[T any](broker *pubsub.Broker[T]) *Service {
	return &Service{
		Run:      func(ctx context.Context) error { broker.Wait(ctx); return nil },
		Shutdown: func() error { broker.Stop(); return nil },
	}
}

// Cmd wraps a exec.Command execution that **has not started** into a
// service. If the command fails, the service returns.
//
// When the service is closed or the context is canceled, if the
// command has not returned, the process is sent SIGTERM. If, after
// the shutdownTimeout duration, the service has not returned, the
// process is sent SIGKILL. In all cases, the service will not return
// until the underlying command has returned, potentially blocking
// until the command returns.
func Cmd(c *exec.Cmd, shutdownTimeout time.Duration) *Service {
	fun.Invariant.Ok(c != nil, "exec.Cmd must be non-nil")

	started := make(chan struct{})
	wg := &fnx.WaitGroup{}

	return &Service{
		Run: func(_ context.Context) error {
			wg.Add(1)
			defer wg.Done()

			if err := c.Start(); err != nil {
				close(started)
				return err
			}
			close(started)

			return c.Wait()
		},
		Shutdown: func() error {
			<-started
			ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			if wg.IsDone() || sendSignal(c, syscall.SIGTERM) {
				// any of the cases that could cause
				// us to fail to signal mean the
				// process is dead.
				return nil
			}

			<-ctx.Done()

			if wg.IsDone() {
				return nil
			}
			sendSignal(c, syscall.SIGKILL)

			return nil
		},
		Cleanup: func() error {
			wg.Operation().Wait()
			return nil
		},
	}
}

func sendSignal(c *exec.Cmd, sig os.Signal) bool { err := c.Process.Signal(sig); return err != nil }

// Daemon produces a service that wraps another service, restarting
// the base service in the case of errors or early termination. The
// interval governs how long. If the base service's run function
// returns an error that is either context.Canceled or
// context.DeadlineExceeded, the context passed to the daemon service
// is canceled, or the new services' Close() method is called, then
// the service will return. In all other cases the service will
// restart.
//
// All errors encountered, *except* errors that occur after the
// context has been canceled *or* that are rooted in context
// cancellation errors are collected and aggregated to the Daemon
// services Wait() response.
//
// The input services Run/Shutdown/Cleanup/ErrorHandler are captured
// when the Daemon service is created, and modifications to the base
// service are not reflected in the daemon service. The base Service's
// Run function is passed a context that is always canceled after that
// instance of the Run invocation returns to give each invocation of
// the daemon a chance to release its specific resources.
//
// The minInterval value ensures that services don't crash in a
// tight loop: if the time between starting the input service and the
// next loop is less than the minInterval value, then the Daemon
// service will wait until at least that interval has passed from the
// last time the service started.
func Daemon(s *Service, minInterval time.Duration) *Service {
	baseRun := s.Run
	baseShutdown := s.Shutdown
	baseCleanup := s.Cleanup
	shouldShutdown := make(chan struct{})

	out := &Service{
		Shutdown: func() error {
			defer close(shouldShutdown)
			if baseShutdown != nil {
				return baseShutdown()
			}
			return nil
		},
		Cleanup: func() error {
			if baseCleanup != nil {
				return baseCleanup()
			}
			return nil
		},
		Run: func(ctx context.Context) (re error) {
			ec := &erc.Collector{}
			defer func() { re = ec.Resolve() }()
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				start := time.Now()
				err := func() error {
					tctx, tcancel := context.WithCancel(ctx)
					defer tcancel()
					return baseRun(tctx)
				}()
				if ers.IsExpiredContext(err) || ctx.Err() != nil {
					return nil
				}
				ec.Push(err)

				timer.Reset(max(0, minInterval-time.Since(start)))

				select {
				case <-shouldShutdown:
					return
				case <-ctx.Done():
					return
				case <-timer.C:
					continue
				}
			}
		},
	}
	out.ErrorHandler.Set(s.ErrorHandler.Get())

	return out
}
