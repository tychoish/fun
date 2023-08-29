package srv

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/pubsub"
)

// Orchestrator manages groups of services and makes it possible to
// add services to a running system, but with coordinated shutdown
// mechanisms of normal services.
//
// Use the Service() method to generate a service that will wait for
// new services to added to the orchestrator and start them (if
// needed.)
type Orchestrator struct {
	// Name is used in the string format of the Orchestrator
	// object and also propagated to the name of services.
	Name string
	// mutex protects the struct. while the deque is threadsafe,
	// we want to be able to avoid adding new threads during shutdown
	mtx   sync.Mutex
	input *pubsub.Queue[*Service]
	srv   *Service
}

// String implements fmt.Stringer and returns the type name and
func (or *Orchestrator) String() string { return fmt.Sprintf("Orchestrator<%s>", or.Name) }

func (or *Orchestrator) setup() {
	if or.input == nil {
		or.input = pubsub.NewUnlimitedQueue[*Service]()
	}
	if or.Name == "" {
		or.Name = "orchestrator"
	}
}

// Add sends a *Service to the orchestrator. If the service is
// running, it will be started after all of the other services have
// started. There is no limit on the number of services an
// orchestrator can manage, and you can add services before starting
// the orchestrator's service. Services are started in the order
// they're added.
//
// Nil services are ignored without an error.
//
// Once the orchestrator's service is closed, or its context is
// canceled, all successive calls to Add will return an error.
func (or *Orchestrator) Add(s *Service) error {
	or.mtx.Lock()
	defer or.mtx.Unlock()
	or.setup()

	if s == nil {
		return nil
	}

	return or.input.Add(s)
}

// Start is a convenience function that run's the service's start
// function.
func (or *Orchestrator) Start(ctx context.Context) error { return or.Service().Start(ctx) }

// Wait is a convenience function that blocks until the Orchestrator's
// service completes.
func (or *Orchestrator) Wait() error { return or.Service().Wait() }

// Service returns a service that runs all of the constituent services
// of the orchestrator. The service must be started by the
// caller.
//
// Constituent Services are started in the order they are passed to
// Add.
//
// The orchestrator's service is blocking and will wait until it's
// closed or its context is canceled.
//
// When called more than once, Service will return the same
// object. However, if you call Add or Service to an orchestrator
// whose Service has already completed and return, the Orchestrator
// will reset and a new service will be created. The new service must
// be started, but this can allow the orchestrator to drop all
// references to completed services that would otherwise remain.
func (or *Orchestrator) Service() *Service {
	or.mtx.Lock()
	defer or.mtx.Unlock()
	if or.srv != nil {
		return or.srv
	}

	or.setup()

	or.srv = &Service{
		Name: or.Name,
		Run: func(ctx context.Context) error {
			ec := &erc.Collector{}
			wg := &sync.WaitGroup{}

			for {
				s, ok := or.input.Remove()
				if !ok {
					var err error

					s, err = or.input.Wait(ctx)
					if err != nil {
						// we only want to collect non-context
						// cancelation and non queue-closed errors
						erc.When(ec, (!errors.Is(err, pubsub.ErrQueueClosed) && !ers.IsExpiredContext(err)), err)
						break
					}
				}

				if s.Running() {
					wg.Add(1)
					go func(ss *Service) {
						defer wg.Done()
						ec.Add(ss.waitFor(ctx))
					}(s)
					continue
				}

				if s.isFinished.Load() {
					ec.Add(s.Wait())
					continue
				}

				wg.Add(1)
				go func(ss *Service) {
					defer wg.Done()

					ec.Add(ers.Wrapf(ss.Start(ctx), "problem starting %s", ss.String()))
					ec.Add(ss.Wait())
				}(s)
			}

			wg.Wait()
			return ec.Resolve()
		},
	}

	return or.srv
}
