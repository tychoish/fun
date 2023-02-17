package srv

import (
	"context"
	"fmt"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
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
	// mutex protects the struct. while the deque is threadsafe,
	// we want to be able to avoid adding new threads during shutdown
	mtx  sync.Mutex
	pipe *pubsub.Deque[*Service]
	srv  *Service
}

func (or *Orchestrator) setup() {
	if or.srv != nil {
		if !or.srv.Running() && or.srv.isFinished.Load() {
			or.srv = nil
			or.pipe = nil
		}
	}

	if or.pipe == nil {
		or.pipe = fun.Must(pubsub.NewDeque[*Service](pubsub.DequeOptions{Unlimited: true}))
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

	return or.pipe.PushBack(s)
}

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
		Run: func(ctx context.Context) error {
			ec := &erc.Collector{}

			// this means that we'll just block when we've
			// gotten to the end until something new is
			// pushed or the context is canceled.
			iter := or.pipe.IteratorBlocking()

			wg := &sync.WaitGroup{}
			seen := 0
			for iter.Next(ctx) {
				seen++
				s := iter.Value()
				if s.Running() {
					continue
				}
				wg.Add(1)
				go func(ss *Service) {
					defer wg.Done()
					defer erc.Recover(ec)

					// note: we could easily
					// choose to ignore this
					// error.
					if err := ss.Start(ctx); err != nil {
						ec.Add(fmt.Errorf("problem starting %s: %w", ss.String(), err))
					}
				}(s)
			}

			fun.Wait(ctx, wg)

			ec.Add(iter.Close())

			return ec.Resolve()
		},
		Cleanup: func() error {
			ec := &erc.Collector{}

			or.mtx.Lock()
			defer or.mtx.Unlock()

			defer erc.Recover(ec)
			defer func() { ec.Add(or.pipe.Close()) }()

			// this has to be a background context,
			// because any context that we would have at
			// this point would not be live. However, we
			// know that the non-blocking iterator won't
			// cause a hang and we want to get through all
			// services' waiters.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			iter := or.pipe.Iterator()
			wg := &sync.WaitGroup{}

			for iter.Next(ctx) {
				s := iter.Value()
				if s != nil {
					wg.Add(1)
					go func() {
						defer erc.Recover(ec)
						defer wg.Done()
						ec.Add(s.Wait())
					}()
				}
			}
			ec.Add(iter.Close())
			wg.Wait()
			return ec.Resolve()
		},
	}

	return or.srv
}
