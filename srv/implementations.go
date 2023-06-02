package srv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/seq"
)

// Group makes it possible to have a collection of services, provided
// via an iterator, that behave as a single service. All services are
// started concurrently (and without order) and shutdown concurrently
// (and without order). The Service produced by group, has the same
// semantics as any other Service.
//
// Use Group(itertool.Slice([]*Service)) to produce a group from a
// slice of *Services,
func Group(services fun.Iterator[*Service]) *Service {
	waiters := pubsub.NewUnlimitedQueue[func() error]()
	wg := &fun.WaitGroup{}
	ec := &erc.Collector{}

	return &Service{
		Run: func(ctx context.Context) error {
			for services.Next(ctx) {
				wg.Add(1)
				go func(s *Service) {
					defer erc.Recover(ec)
					defer wg.Done()
					defer func() { fun.Invariant(waiters.Add(s.Wait) == nil) }()
					ec.Add(s.Start(ctx))
				}(services.Value())
			}
			wg.Wait(ctx)
			ec.Add(waiters.Close())
			return nil
		},
		Cleanup: func() error {
			defer erc.Recover(ec)
			// we're calling each service's wait() here, which
			// might be a recipe for deadlocks, but it gives us
			// the chance to collect all errors from the contained
			// services. This will cause our "group service" to
			// have the same semantics as a single service, however.
			iter := waiters.Iterator()

			// because we know that the implementation of the
			// waiters iterator won't block in this context, it's
			// safe to call it with a background context, though
			// it's worth being careful here
			for iter.Next(internal.BackgroundContext) {
				wg.Add(1)
				go func(wait func() error) {
					defer erc.Recover(ec)
					defer wg.Done()
					ec.Add(wait())
				}(iter.Value())
			}

			wg.Wait(internal.BackgroundContext)
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
// have returned *and* the iterator is exhausted. The Service's wait
// function returns an error that aggregates all errors (e.g. panics)
// encountered by the constituent wait functions.
//
// Wait produces a service that fills the same role as the
// fun.WaitMerge function, but that can be more easily integrated into
// existing orchestration tools.
//
// When the service returns all worker Goroutines as well as the input
// worker will have returned. Use a blocking pubsub iterator to
// dispatch wait functions throughout the lifecycle of your program.
func Wait(iter fun.Iterator[fun.WaitFunc]) *Service {
	wg := &sync.WaitGroup{}
	ec := &erc.Collector{}
	return &Service{
		Run: func(ctx context.Context) error {
			for {
				value, err := fun.IterateOne(ctx, iter)
				if err != nil {
					break
				}

				wg.Add(1)
				go func(fn fun.WaitFunc) {
					defer erc.Recover(ec)
					defer wg.Done()
					fn(ctx)
				}(value)
			}

			ec.Add(iter.Close())
			wg.Wait()
			return nil
		},
		Cleanup:  func() error { wg.Wait(); return ec.Resolve() },
		Shutdown: iter.Close,
	}
}

// ProcessIterator runs an itertool.ParallelForEach operation as a
// *Service. For a long running service, use an iterator that is
// blocking (e.g. based on a pubsub queue/deque or a channel.)
func ProcessIterator[T any](
	iter fun.Iterator[T],
	mapper fun.Processor[T],
	opts itertool.Options,
) *Service {
	return &Service{
		Run: func(ctx context.Context) error {
			return itertool.ParallelForEach(ctx, iter, mapper, opts)
		},
		Shutdown: iter.Close,
	}
}

// Cleanup provides a service which services the provided queue of
// worker functions and runs the shutdown functions during service
// shutdown (e.g. either after Close() is called or when the context
// is canceled.) The timeout, when non-zero, is passed to the clean up
// operation. Cleanup functions are dispatched in parallel.
func Cleanup(pipe *pubsub.Queue[fun.Worker], timeout time.Duration) *Service {
	// assume some reasonable defaults.
	opts := itertool.Options{
		ContinueOnPanic: true,
		ContinueOnError: true,
		NumWorkers:      runtime.NumCPU(),
	}

	// copy the values out of the pipe so that we don't end up
	// deadlocking or missing jobs. on shutdown.
	cache := &seq.List[fun.Worker]{}

	return &Service{
		Run: func(ctx context.Context) error {
			iter := pipe.Distributor().Iterator()

			for {
				item, err := fun.IterateOne(ctx, iter)
				if err != nil {
					return nil
				}
				cache.PushBack(item)
			}
		},
		Shutdown: func() error { return pipe.Close() },
		Cleanup: func() error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			ec := &erc.Collector{}

			ec.Add(itertool.ParallelForEach(ctx, cache.PopValues(),
				func(ctx context.Context, wf fun.Worker) error {
					ec.Add(wf.Safe()(ctx))
					return nil
				},
				opts))

			return ec.Resolve()
		},
	}
}

// WorkerPool wraps a pubsub.Queue of functions that represent units
// of work in an worker pool. The pool follows the semantics
// configured by the itertool.Options, with regards to error handling,
// panic handling, and parallelism. Errors are collected and
// propogated to the service's ywait function.
func WorkerPool(workQueue *pubsub.Queue[fun.Worker], opts itertool.Options) *Service {
	return &Service{
		Run: func(ctx context.Context) error {
			return itertool.ParallelForEach(ctx,
				workQueue.Distributor().Iterator(),
				func(ctx context.Context, fn fun.Worker) error {
					return fn.Run(ctx)
				},
				opts,
			)
		},
		Shutdown: workQueue.Close,
	}
}

// ObserverWorkerPool has similar semantics and use to the WorkerPool,
// but rather than aggregating errors, all errors are passed to the
// observer function, which is responsible for ignoring or processing
// the errors. The worker pool will respect continue/abort on error or
// panics as expected.
//
// Observer pools may be more operable if your workers generate many
// errors, and/or your process is long lived.
//
// The service itself may have execution or shutdown related errors,
// particularly if there is an invariant violation or panic during
// service execution which will be propagated to the return value of
// the service's Wait method; but all errors that occur during the
// execution of the workload will be observed (including panics) as well.
func ObserverWorkerPool(
	workQueue *pubsub.Queue[fun.Worker],
	observer fun.Observer[error],
	opts itertool.Options,
) *Service {
	s := &Service{
		Run: func(ctx context.Context) error {
			return itertool.ParallelForEach(ctx,
				workQueue.Distributor().Iterator(),
				func(ctx context.Context, fn fun.Worker) error {
					observer(fn.Run(ctx))
					return nil
				},
				opts,
			)
		},
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
	fun.Invariant(c != nil, "exec.Cmd must be non-nil")

	started := make(chan struct{})
	wg := &fun.WaitGroup{}

	return &Service{
		Run: func(ctx context.Context) error {
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
			ctx, cancel := context.WithTimeout(internal.BackgroundContext, shutdownTimeout)
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
			fun.WaitFunc(wg.Wait).Block()
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
				if erc.ContextExpired(err) || ctx.Err() != nil {
					return nil
				}
				ec.Add(err)

				timer.Reset(maxOfDur(0, minInterval-time.Since(start)))

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

func maxOfDur(a, b time.Duration) time.Duration {
	if a >= b {
		return a
	}
	return b
}
