package srv

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/erc"
)

var (
	ErrServiceAlreadyStarted = errors.New("service already started")
	ErrServiceReturned       = errors.New("service returned")
	ErrServiceNotStarted     = errors.New("service not started")
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
// optional. The service can only be run once; however, callers
// should avoid mutating the service after calling Start.
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
	Run func(context.Context) error

	// Cleanup is optional, but if defined is always called after the
	// service returned. Cleanup operations should all return
	// relatively quickly and be used for releasing state rather
	// than doing potentially blocking work.
	Cleanup func() error

	// Shutdown is optional, but provides a hook that
	// implementations can be used to trigger a shutdown while a
	// Service is running.
	Shutdown func() error

	isRunning  atomic.Bool
	isFinished atomic.Bool
	doStart    sync.Once
	cancel     context.CancelFunc
	ec         erc.Collector

	// the mutex just protects the signal. this is a race in some
	// tests, but should generally not be an issue.
	mtx sync.Mutex
	sig chan struct{}
}

// String implements fmt.Stringer and includes value of s.Name.
func (s *Service) String() string { return fmt.Sprintf("Service<%s>", s.Name) }

// Running returns true as soon as start returns, and returns false
// after close is called.
func (s *Service) Running() bool { return s.isRunning.Load() }

// Start launches the configured service and tracks  its lifecycle. If
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

	if s.isRunning.Swap(true) {
		return ErrServiceAlreadyStarted
	}

	s.doStart.Do(func() {
		ec := &s.ec
		shutdownSignal := make(chan struct{})

		s.setSignal()
		ctx, s.cancel = context.WithCancel(ctx)

		// set running to true here so that close is always safe
		s.isRunning.Store(true)

		if s.Shutdown != nil {
			// capture it just to be safe.
			shutdown := s.Shutdown
			go func() {
				defer close(shutdownSignal)
				defer erc.Recover(ec)
				<-ctx.Done()
				s.ec.Add(shutdown())
			}()
		} else {
			close(shutdownSignal)
		}

		go func() {
			defer s.fireSignal()
			defer s.isRunning.Store(false)
			defer s.isFinished.Store(true)

			if s.Cleanup != nil {
				cleanup := s.Cleanup
				// this catches a panic during shutdown
				defer erc.Recover(ec)
				defer func() { ec.Add(cleanup()) }()
			}
			defer func() { <-shutdownSignal }()
			// this catches a panic in the execution of
			// the startup/op hook
			defer erc.Recover(ec)

			ec.Add(s.Run(ctx))
			s.cancel()
		}()
	})

	return nil
}

// Close forceably shuts down the service, causing the background
// process to terminate and wait to return. If the service hasn't
// started or isn't running this has no effect. Close is safe to call
// multiple times.
func (s *Service) Close() {
	if s.isRunning.Load() && s.cancel != nil {
		s.cancel()
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
func (s *Service) Wait() error {
	s.mtx.Lock()
	if s.sig == nil {
		s.mtx.Unlock()
		return ErrServiceNotStarted
	}
	s.mtx.Unlock()

	<-s.sig
	return s.ec.Resolve()
}

func (s *Service) setSignal() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.sig = make(chan struct{})
}

func (s *Service) fireSignal() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	close(s.sig)
}
