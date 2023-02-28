package srv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
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
	waiters := fun.Must(pubsub.NewDeque[func() error](pubsub.DequeOptions{Unlimited: true}))
	wg := &sync.WaitGroup{}
	ec := &erc.Collector{}

	return &Service{
		Run: func(ctx context.Context) error {
			for services.Next(ctx) {
				wg.Add(1)
				go func(s *Service) {
					defer erc.Recover(ec)
					defer wg.Done()
					defer func() { fun.Invariant(waiters.PushBack(s.Wait) == nil) }()
					ec.Add(s.Start(ctx))
				}(services.Value())
			}
			fun.Wait(ctx, wg)
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

			wg.Wait()
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
// function returns an error that aggregates all errors encountered by
// the constituent wait functions.
//
// Wait produces a service that fills the same role as the
// fun.WaitMerge function, but that can be more easily integrated into
// existing orchestration tools.
//
// When the service returns all worker Goroutines will have returned.
func Wait(iter fun.Iterator[fun.WaitFunc]) *Service {
	wg := &sync.WaitGroup{}
	ec := &erc.Collector{}
	return &Service{
		Run: func(ctx context.Context) error {
			for iter.Next(ctx) {
				wg.Add(1)

				go func(fn fun.WaitFunc) {
					defer erc.Recover(ec)
					defer wg.Done()
					fn(ctx)
				}(iter.Value())

			}
			ec.Add(iter.Close())
			fun.Wait(ctx, wg)
			return nil
		},
		Cleanup: func() error { wg.Wait(); return ec.Resolve() },
	}
}

// ProcessIterator runs an itertool.ParallelForEach operation as a *Service.
func ProcessIterator[T any](
	iter fun.Iterator[T],
	mapper func(context.Context, T) error,
	opts itertool.Options,
) *Service {
	return &Service{
		Run: func(ctx context.Context) error {
			return itertool.ParallelForEach(ctx, iter, mapper, opts)
		},
	}
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

// RunWaitObserve returns a fun.WaitFunc that runs the service,
// passing the error that the Service's Wait() method to the
// observer.
func RunWaitObserve(observe func(error), s *Service) fun.WaitFunc {
	return func(ctx context.Context) { observe(s.Start(ctx)); observe(s.Wait()) }
}

// RunWaitCollect produces a fun.WaitFunc that runs the service and
// adds any errors produced to the provided collector.
func RunWaitCollect(ec *erc.Collector, s *Service) fun.WaitFunc { return RunWaitObserve(ec.Add, s) }

// RunWait produces a fun.WaitFunc that runs the service, ignoring any
// error from the Service.
func RunWait(s *Service) fun.WaitFunc { return RunWaitObserve(func(error) {}, s) }
