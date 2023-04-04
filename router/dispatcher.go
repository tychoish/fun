package router

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
)

type Config struct {
	Name    string
	Buffer  int
	Workers int
}

// Dispatcher provides a basic message dispatching system for
// dynamically messages of different types. Unlike the pubsub.Broker
// which simply dispatches messages of one type to many subscribers,
// the dispatcher manages the processing of manages with its
// Handlers, Middleware, and Interceptors, while also providing for a
// request-and-response usage pattern.
//
// Dispatchers must be constructed by the NewDispatcher() function.
type Dispatcher struct {
	// ErrorObserver defaults to a noop, but is called whenever a
	// middlware or handler returns an error, and makes it
	// possible to inject behavior to handle or collect errors.
	//
	// Callers can modify the value at any time, and the
	// NewDispatcher constructor creates a noop observer.
	ErrorObserver adt.Atomic[func(error)]

	services    srv.Orchestrator
	middleware  adt.Map[Protocol, *pubsub.Deque[Middleware]]
	interceptor adt.Map[Protocol, *pubsub.Deque[Interceptor]]
	handlers    adt.Map[Protocol, Handler]

	isRunning atomic.Bool
	// the broker and pipe are constructed when the service is
	// initialized.
	broker *pubsub.Broker[Response]
	pipe   *pubsub.Deque[Message]
	conf   Config
}

// NewDispatcher builds a new dispatching service. The configuration
// may specify a number of workers. Once constructed, callers are
// responsible for registering handlers to process messages of
// specific types. Callers may also choose to register middleware (request
// pre-processing) and interceptors (response post-processing) to
// process requests and responses (serialization, encoding, authentication,
// metrics) before the handlers.
//
// The constructor does not start any services: once registered, use
// the Service() method to access a service that manages the work of
// the dispatcher. Once the service has started and the Running()
// method returns true you can use the service.
func NewDispatcher(conf Config) (*Dispatcher, error) {
	if conf.Workers < 1 {
		return nil, errors.New("must specify one or more workers")
	}

	pipe, err := pubsub.NewDeque[Message](pubsub.DequeOptions{Capacity: conf.Workers})
	if err != nil {
		return nil, err
	}

	r := &Dispatcher{
		pipe: pipe,
		conf: conf,
	}
	r.services.Name = conf.Name
	r.ErrorObserver.Set(func(error) {})
	r.handlers.Default.SetConstructor(func() Handler {
		return func(_ context.Context, m Message) (Response, error) {
			return Response{ID: m.ID, Error: ErrNoHandler}, ErrNoHandler
		}
	})

	r.middleware.Default.SetConstructor(func() *pubsub.Deque[Middleware] {
		return fun.Must(pubsub.NewDeque[Middleware](pubsub.DequeOptions{Unlimited: true}))
	})

	r.interceptor.Default.SetConstructor(func() *pubsub.Deque[Interceptor] {
		return fun.Must(pubsub.NewDeque[Interceptor](pubsub.DequeOptions{Unlimited: true}))
	})

	if err := r.registerServices(); err != nil {
		return nil, err
	}

	return r, nil
}

// Service returns access to a srv.Service object that is responsible
// for tracking the lifecycle of all background processes responsible
// for handling the work of the Dispatcher. Until the dispatcher
// starts, there are no background threads running and using the
// dispatcher will raise panics.
//
// Once initialized you must start the service *and* the Running()
// method must return true before you can use the service.
func (r *Dispatcher) Service() *srv.Service { return r.services.Service() }
func (r *Dispatcher) Running() bool         { return r.Service().Running() && r.isRunning.Load() }

// Ready produces a fun.WaitFunc that blocks until the dispatcher's
// service is ready and running (or the context passed to the wait
// function is canceled.)
func (r *Dispatcher) Ready() fun.WaitFunc {
	return func(ctx context.Context) {
		if r.isRunning.Load() {
			return
		}

		timer := time.NewTimer(randomInterval(5))
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if r.isRunning.Load() {
					return
				}
				timer.Reset(randomInterval(5))
			}
		}
	}
}

func (r *Dispatcher) AddMiddleware(key Protocol, it Middleware) error {
	return r.middleware.Get(key).PushBack(it)
}

func (r *Dispatcher) AddInterceptor(key Protocol, it Interceptor) error {
	return r.interceptor.Get(key).PushBack(it)
}

func (r *Dispatcher) RegisterHandler(key Protocol, hfn Handler) { r.handlers.Store(key, hfn) }

func (r *Dispatcher) Stream(ctx context.Context) <-chan Response {
	return r.watch(ctx, false, Protocol{})
}

func (r *Dispatcher) Subscribe(ctx context.Context, p Protocol) <-chan Response {
	return r.watch(ctx, true, p)
}

func (r *Dispatcher) watch(ctx context.Context, shouldFilter bool, key Protocol) <-chan Response {
	out := make(chan Response)
	go func() {
		defer close(out)
		sub := r.broker.Subscribe(ctx)
		defer r.broker.Unsubscribe(ctx, sub)

		for {
			select {
			case <-ctx.Done():
				return
			case r := <-sub:
				if shouldFilter && r.Protocol != key {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- r:
				}
			}
		}
	}()
	return out
}

func (r *Dispatcher) Broadcast(ctx context.Context, m Message) error {
	if m.Protocol.IsZero() {
		return ErrNoProtocol
	}

	m.ID = populateID(m.ID)
	return r.pipe.WaitPushBack(ctx, m)
}

func (r *Dispatcher) Exec(ctx context.Context, m Message) (*Response, bool) {
	if m.Protocol.IsZero() {
		return nil, false
	}

	m.ID = populateID(m.ID)

	sub := r.broker.Subscribe(ctx)
	defer r.broker.Unsubscribe(ctx, sub)

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	out := make(chan Response)

	// start a goroutine to listen for responses before sending
	// the message.
	go func(id string) {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case resp := <-sub:
				if resp.ID != id {
					continue
				}

				select {
				case <-ctx.Done():
				case out <- resp:
				}

				return
			}
		}
	}(m.ID)

	if err := r.pipe.WaitPushBack(ctx, m); err != nil {
		return nil, false
	}

	val, err := fun.ReadOne(ctx, out)
	if err != nil {
		return nil, false
	}
	return &val, true
}

func (r *Dispatcher) doIntercept(ctx context.Context, key Protocol, rr *Response) (err error) {
	ec := &erc.Collector{}
	defer func() { err = ec.Resolve() }()
	defer erc.Recover(ec)

	if icept, ok := r.interceptor.Load(key); ok {
		iter := icept.Iterator()
		for iter.Next(ctx) {
			if err = iter.Value()(ctx, rr); err != nil {
				ec.Add(err)
				break
			}
		}
		ec.Add(iter.Close())
	}

	return nil
}

func (r *Dispatcher) doMiddleware(ctx context.Context, key Protocol, req *Message) (err error) {
	ec := &erc.Collector{}
	defer func() { err = ec.Resolve() }()
	defer erc.Recover(ec)

	if mw, ok := r.middleware.Load(key); ok {
		iter := mw.Iterator()
		for iter.Next(ctx) {
			if err := iter.Value()(ctx, req); err != nil {
				ec.Add(err)
				break
			}
		}
		ec.Add(iter.Close())
	}
	return nil
}

func (r *Dispatcher) doHandler(ctx context.Context, key Protocol, req Message) (resp Response) {
	ec := &erc.Collector{}
	defer func() { resp.Error = ec.Resolve() }()
	defer erc.Recover(ec)

	h, ok := r.handlers.Load(key)
	erc.When(ec, !ok, ErrNoHandler)

	if ok {
		var err error
		resp, err = h(ctx, req)
		ec.Add(err)
		ec.Add(resp.Error)
	}
	return
}

func (r *Dispatcher) registerServices() error {
	ec := &erc.Collector{}

	// we need to be able to signal from the broker service, which
	// starts first that the broker is initialized in the
	// dispatcher struct, this channel does that.
	brokerSetup := make(chan struct{})

	ec.Add(r.services.Add(&srv.Service{
		Run: func(ctx context.Context) error {
			r.broker = pubsub.NewBroker[Response](ctx, pubsub.BrokerOptions{WorkerPoolSize: r.conf.Workers, ParallelDispatch: true})
			r.isRunning.Store(true)
			close(brokerSetup)
			r.broker.Wait(ctx)
			return nil
		},
		Shutdown: func() error { r.broker.Stop(); return nil },
	}))

	ec.Add(r.services.Add(&srv.Service{
		Run:     func(ctx context.Context) error { <-ctx.Done(); return nil },
		Cleanup: r.pipe.Close,
	}))

	var outQueue *pubsub.Queue[Response]
	if r.conf.Buffer <= 0 {
		outQueue = pubsub.NewUnlimitedQueue[Response]()
	} else {
		outQueue = erc.Collect[*pubsub.Queue[Response]](ec)(pubsub.NewQueue[Response](pubsub.QueueOptions{
			SoftQuota:   r.conf.Buffer,
			HardLimit:   r.conf.Buffer + r.conf.Workers,
			BurstCredit: float64(r.conf.Workers + 1),
		}))
	}

	// process outgoing message filters
	ec.Add(r.services.Add(&srv.Service{
		Shutdown: outQueue.Close,
		Run: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			case <-brokerSetup:
				return itertool.WorkerPool(ctx,
					itertool.Transform(ctx, outQueue.Iterator(), func(rr Response) fun.WorkerFunc {
						return func(ctx context.Context) error {
							if err := r.doIntercept(ctx, Protocol{}, &rr); err != nil {
								rr.Error = err
								r.broker.Publish(ctx, rr)
								return ctx.Err()
							}
							if err := r.doIntercept(ctx, rr.Protocol, &rr); err != nil {
								rr.Error = erc.Merge(err, rr.Error)
							}

							r.broker.Publish(ctx, rr)
							return ctx.Err()
						}
					}),
					itertool.Options{NumWorkers: r.conf.Workers},
				)
			}
		},
	}))

	// process middleware and actual handler
	ec.Add(r.services.Add(&srv.Service{
		Shutdown: r.pipe.Close,
		Run: func(ctx context.Context) error {
			return itertool.WorkerPool(ctx,
				// convert an iterator of messages to an iterator of workers:
				itertool.Transform(ctx, r.pipe.IteratorBlocking(), func(req Message) fun.WorkerFunc {
					// each worker processes the message through its middlewares
					// (in the pool) and then passes the message off to the dispatcher/outgoing pool
					return func(ctx context.Context) error {
						var resp Response
						var err error

						id := req.ID
						protocol := req.Protocol

						if err = r.doMiddleware(ctx, Protocol{}, &req); err != nil {
							resp.Error = err
						} else if err = r.doMiddleware(ctx, protocol, &req); err != nil {
							resp.Error = err
						} else {
							resp = r.doHandler(ctx, protocol, req)
						}

						resp.ID = id

						if resp.Protocol.IsZero() {
							resp.Protocol = protocol
						}

						if err != nil {
							resp.Error = &Error{Err: err, ID: id, Protocol: protocol}
							r.ErrorObserver.Get()(resp.Error)
						}
						return outQueue.Add(resp)
					}
				}),
				itertool.Options{NumWorkers: r.conf.Workers},
			)
		},
	}))

	return ec.Resolve()
}
