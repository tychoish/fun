package srv

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
)

const (
	// ErrServiceAlreadyStarted is returend by the Start() method
	// of a service that is already running. These are safe to
	// ignore in many contexts.
	ErrServiceAlreadyStarted ers.Error = ers.Error("service already started")
	// ErrServiceReturned is returned by the Start() method of a
	// service that has already returned.
	ErrServiceReturned ers.Error = ers.Error("service returned")
	// ErrServiceNotStarted is returned by the Wait() method if
	// the Start method has not yet returned.
	ErrServiceNotStarted ers.Error = ers.Error("service not started")
)

// Service defines a background operation. The behavior of the service
// is defined by the Run, Shutdown, and Cleanup functions defined in
// the Service structure, with the service lifecycle managed by the
// context passed to start.
//
// Applications often consist of a number of services, and the Group
// function amalgamates a number of services into a single service,
// while the Orchestrator provides a more dynamic mechanism for
// managing services during an application's lifetime.
//
// There are no special construction requirements for Services and
// implementations must define Run methods, with the other options
// optional. The service can only be run once. There is no particular
// concurency control provided on Services, except via the atomic
// setter on the ErrorHandler. While callers should not modify the
// other attributes of the service, after Start() returns the
// Run/Cleanup/Shutdown functions are not referenced.
type Service struct {
	// Name is a human-readable name for the service, and is used
	// in the String() method, and to annotate errors.
	Name string

	// Run is a blocking function that does the main action of the
	// service. When the Run function returns the shutdown
	// function is called if it exists. The error produced by the
	// Run function is propagated to callers via the results of
	// the Wait method. Panics in the Run method are converted to
	// errors and propagated to the Wait method.
	//
	// Implementations are responsible for returning promptly when
	// the context is canceled.
	Run fnx.Worker

	// Cleanup is optional, but if defined is always called after the
	// Run function returned. Cleanup operations should all return
	// relatively quickly and be used for releasing state rather
	// than doing potentially blocking work.
	Cleanup func() error

	// Shutdown is optional, but provides a hook that
	// implementations can be used to trigger a shutdown when the
	// context passed to Start is canceled but before the Run
	// function returns. The shutdown function, when defined,
	// must return before the Cleanup function runs.
	Shutdown func() error

	Signal fnx.Worker

	// ErrorHandler functions, if specified, are called when the
	// service returns, with the output of the Service's error
	// output. This is always the result of an
	// erc.Collector.Resolve() method, and so contains the
	// aggregated errors and panics collected during the service's
	// execution (e.g. Run, Shutdown, Cleanup), and should be
	// identical to the output of Wait(). Caller's set this value
	// during the execution of the service.
	ErrorHandler adt.Atomic[fn.Handler[error]]

	isRunning  atomic.Bool
	isFinished atomic.Bool
	isStarted  atomic.Bool
	doStart    sync.Once
	cancel     context.CancelFunc
	ec         erc.Collector

	// the mutex just protects the signal. this is a race in some
	// tests, but should generally not be an issue.
	wg fnx.WaitGroup
}

// String implements fmt.Stringer and includes value of s.Name.
func (s *Service) String() string { return fmt.Sprintf("Service<%s>", s.Name) }

// Running returns true as soon as start returns, and returns false
// after close is called.
func (s *Service) Running() bool { return s.isRunning.Load() }

// Start launches the configured service and tracks its lifecycle. If
// the context is canceled, the service returns, and any errors
// collected during the service's lifecycle are returned by the close
// (or wait) methods.
//
// If the service is running or is finished, Start returns the
// appropriate sentinel error.
func (s *Service) Start(ctx context.Context) error {
	if s.isFinished.Load() {
		return ErrServiceReturned
	}

	if s.isStarted.Load() || !s.isRunning.CompareAndSwap(false, true) {
		return ErrServiceAlreadyStarted
	}

	s.doStart.Do(func() {
		defer s.isRunning.Store(true)
		defer s.isStarted.Store(true)
		ec := &s.ec
		ehSignal := make(chan struct{})
		mainSignal := make(chan struct{})
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer ec.Recover()
			<-mainSignal
			<-ehSignal
			eh := s.ErrorHandler.Get()
			if eh != nil {
				defer ec.Recover()
				if err := ec.Resolve(); err != nil {
					eh(err)
				}
			}
		}()

		var shutdown func() error
		if s.Shutdown != nil {
			shutdown = s.Shutdown
		} else {
			shutdown = func() error { return nil }
		}

		if !HasBaseContext(ctx) {
			ctx = SetBaseContext(ctx)
		}

		ctx, s.cancel = context.WithCancel(ctx)
		sig := s.Signal
		if sig == nil {
			sig = fnx.MAKE.ContextChannelWorker(ctx)
		}

		shutdownSignal := make(chan struct{})
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer close(shutdownSignal)
			defer ec.Recover()

			var err error

			select {
			case err = <-sig.Signal(GetBaseContext(ctx)):
			case <-ctx.Done():
			}

			filter := erc.NewFilter().WithoutContext()
			ec.Join(filter.Apply(err), filter.Apply(shutdown()))
		}()

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer close(mainSignal)
			defer s.isRunning.Store(false)
			defer s.isFinished.Store(true)
			if s.Cleanup != nil {
				cleanup := s.Cleanup
				// this catches a panic during shutdown
				defer ec.Recover()
				defer func() { ec.Push(cleanup()) }()
			}
			defer func() { defer close(ehSignal); <-shutdownSignal }()
			// this catches a panic in the execution of
			// the startup/op hook
			defer ec.Recover()

			defer s.cancel()
			ec.Push(s.Run(ctx))
		}()
	})

	return nil
}

// Close forceably shuts down the service, causing the background
// process to terminate and wait to return. If the service hasn't
// started or isn't running this has no effect. Close is safe to call
// multiple times.
func (s *Service) Close() {
	if s.isRunning.Load() {
		ft.CallSafe(s.cancel)
	}
}

// Wait blocks until the service returns or the service's start
// context is canceled. If the service hasn't been started Wait
// returns ErrServiceNotStarted. It is safe to call wait multiple
// times. While wait does not accept a context, it respects the
// context passed to start. Wait returns nil if the service's
// background operation encountered no errors, and otherwise returns
// an erc.ErrorStack object that contains the errors encountered when
// running the service, the shutdown hook, and any panics encountered
// during the service's execution.
func (s *Service) Wait() error { return s.waitFor(context.Background()) }

func (s *Service) waitFor(ctx context.Context) error {
	if s.isFinished.Load() {
		return s.ec.Resolve()
	}

	if !s.isStarted.Load() {
		return fmt.Errorf("%s: %w", s.String(), ErrServiceNotStarted)
	}

	s.wg.Wait(ctx)
	return s.ec.Resolve()
}

// Worker runs the service, starting it if needed and then waiting for
// the service to return. If the service has already finished, the
// fnx.Worker, like Wait(), will return the same aggreagated error
// from the service when called multiple times on a completed service.
//
// Unlike Wait(), the fnx.Worker respects the context and will return
// early if the context is canceled: returning a context cancelation
// error.
func (s *Service) Worker() fnx.Worker {
	return func(ctx context.Context) error {
		_ = s.Start(ctx)
		werr := s.waitFor(ctx)

		if err := ctx.Err(); err != nil {
			return err
		}

		return werr
	}
}
