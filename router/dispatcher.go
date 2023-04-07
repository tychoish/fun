package router

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
)

// Config describes the behavior of the Dispatcher.
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

	middleware  adt.Map[Protocol, *pubsub.Deque[Middleware]]
	interceptor adt.Map[Protocol, *pubsub.Deque[Interceptor]]
	handlers    adt.Map[Protocol, Handler]

	isRunning    chan struct{}
	orchestrator srv.Orchestrator

	// the broker and pipe are constructed when the service is
	// initialized.
	broker   *pubsub.Broker[Response]
	pipe     *pubsub.Deque[Message]
	outgoing *pubsub.Queue[Response]
	conf     Config
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
func NewDispatcher(conf Config) *Dispatcher {
	if conf.Workers < 1 {
		conf.Workers = 1
	}

	r := &Dispatcher{
		conf:      conf,
		isRunning: make(chan struct{}),
	}

	r.orchestrator.Name = conf.Name
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

	// impossible for this to error, because the capacity is
	// always greater than 1, and this will always vialidate
	r.pipe = fun.Must(pubsub.NewDeque[Message](pubsub.DequeOptions{Capacity: conf.Workers}))

	if r.conf.Buffer <= 0 {
		r.outgoing = pubsub.NewUnlimitedQueue[Response]()
	} else {
		r.outgoing = fun.Must(pubsub.NewQueue[Response](pubsub.QueueOptions{
			SoftQuota:   r.conf.Buffer,
			HardLimit:   r.conf.Buffer + r.conf.Workers,
			BurstCredit: float64(r.conf.Workers + 1),
		}))
	}

	// there's no real way to make adding these services error:
	// registerServices() is only called in the constructor, the
	// orchestrator is configured such that the only way for Add
	// to error is if the incoming Service buffer is full (it's
	// unlimited), or if something closes the buffer (nothing
	// does,) and even if orchestrators were closable (or closed
	// when their services shutdown; which might be a reasonable
	// change,) because this is called exactly once when the
	// Dispatcher is created, nothing can error

	fun.InvariantMust(r.registerBrokerService())
	fun.InvariantMust(r.registerPipeShutdownService())
	fun.InvariantMust(r.registerProcessResponseService())
	fun.InvariantMust(r.registerMiddlwareService())

	return r
}

// Service returns access to a srv.Service object that is responsible
// for tracking the lifecycle of all background processes responsible
// for handling the work of the Dispatcher. Until the dispatcher
// starts, there are no background threads running and using the
// dispatcher will raise panics.
//
// Once initialized you must start the service *and* the Running()
// method must return true before you can use the service.
func (r *Dispatcher) Service() *srv.Service { return r.orchestrator.Service() }

// Ready produces a fun.WaitFunc that blocks until the dispatcher's
// service is ready and running (or the context passed to the wait
// function is canceled.)
func (r *Dispatcher) Ready() fun.WaitFunc {
	return func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.isRunning:
				return
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

// Broadcast sends a message without waiting for a response. Responses
// will be sent to subscribers.
func (r *Dispatcher) Broadcast(ctx context.Context, m Message) error {
	if m.Protocol.IsZero() {
		return ErrNoProtocol
	}

	m.ID = populateID(m.ID)
	return r.pipe.WaitPushBack(ctx, m)
}

// Exec passes a Message to the dispatcher which processes the message
// and returns a response. The error reflects errors processing the
// message (e.g. an invalid message, canceled context, or the
// dispatcher shutting down.) The response contains its own error that
// may be non-nil if the dispatcher or any of the handlers encountered
// an error when processing the message.
func (r *Dispatcher) Exec(ctx context.Context, m Message) (*Response, error) {
	if m.Protocol.IsZero() {
		return nil, ErrNoProtocol
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
		return nil, err
	}

	val, err := fun.ReadOne(ctx, out)
	if err != nil {
		return nil, err
	}
	return &val, nil
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

func (r *Dispatcher) registerBrokerService() error {
	return r.orchestrator.Add(&srv.Service{
		Run: func(ctx context.Context) error {
			r.broker = pubsub.NewBroker[Response](ctx, pubsub.BrokerOptions{WorkerPoolSize: r.conf.Workers, ParallelDispatch: true})
			close(r.isRunning)
			r.broker.Wait(ctx)
			return nil
		},
		Shutdown: func() error { r.broker.Stop(); return nil },
	})
}

func (r *Dispatcher) registerPipeShutdownService() error {
	return r.orchestrator.Add(&srv.Service{
		Run:     func(ctx context.Context) error { <-ctx.Done(); return nil },
		Cleanup: r.pipe.Close,
	})
}

func (r *Dispatcher) registerProcessResponseService() error {
	// process outgoing message filters
	return r.orchestrator.Add(&srv.Service{
		Shutdown: r.outgoing.Close,
		Run: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			case <-r.isRunning:
				return itertool.WorkerPool(ctx,
					itertool.Transform(ctx, r.outgoing.Iterator(), func(rr Response) fun.WorkerFunc {
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
	})
}

func (r *Dispatcher) registerMiddlwareService() error {
	return r.orchestrator.Add(&srv.Service{
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
						return r.outgoing.Add(resp)
					}
				}),
				itertool.Options{NumWorkers: r.conf.Workers},
			)
		},
	})
}
