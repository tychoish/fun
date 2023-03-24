package router

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
)

type Config struct {
	Name    string
	Workers int
}

type Dispatcher struct {
	// ErrorObserver defaults to a noop, but is called whenever a
	// middlware or handler returns an error, and makes it
	// possible to inject behavior to handle or collect errors.
	ErrorObserver fun.Atomic[func(error)]

	middleware  *adt.SyncMap[Protocol, *pubsub.Deque[Middleware]]
	interceptor *adt.SyncMap[Protocol, *pubsub.Deque[Interceptor]]
	handlers    *adt.SyncMap[Protocol, Handler]
	broker      *pubsub.Broker[Response]
	pipe        *pubsub.Deque[Message]
	conf        Config
	services    srv.Orchestrator
}

func NewDispatcher(ctx context.Context, conf Config) (*Dispatcher, error) {
	broker := pubsub.NewBroker[Response](ctx, pubsub.BrokerOptions{WorkerPoolSize: conf.Workers, ParallelDispatch: true})
	pipe, err := pubsub.NewDeque[Message](pubsub.DequeOptions{Capacity: conf.Workers})
	if err != nil {
		return nil, err
	}

	r := &Dispatcher{
		middleware: &adt.SyncMap[Protocol, *pubsub.Deque[Middleware]]{},
		handlers:   &adt.SyncMap[Protocol, Handler]{},
		broker:     broker,
		pipe:       pipe,
		conf:       conf,
	}
	r.ErrorObserver.Set(func(error) {})
	r.handlers.DefaultConstructor.Constructor.Set(func() Handler {
		return func(_ context.Context, m Message) (Response, error) {
			return Response{ID: m.ID, Error: ErrNoHandler}, ErrNoHandler
		}
	})

	r.middleware.DefaultConstructor.Constructor.Set(func() *pubsub.Deque[Middleware] {
		return fun.Must(pubsub.NewDeque[Middleware](pubsub.DequeOptions{Unlimited: true}))
	})

	r.interceptor.DefaultConstructor.Constructor.Set(func() *pubsub.Deque[Interceptor] {
		return fun.Must(pubsub.NewDeque[Interceptor](pubsub.DequeOptions{Unlimited: true}))
	})

	if err := r.registerServices(ctx); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Dispatcher) Service() *srv.Service { return r.services.Service() }

func (r *Dispatcher) AddMiddleware(key Protocol, it Middleware) error {
	return r.middleware.Get(key).PushBack(it)
}

func (r *Dispatcher) AddInterceptor(key Protocol, it Interceptor) error {
	return r.interceptor.Get(key).PushBack(it)
}

func populateID(id string) string {
	if id != "" {
		return id
	}
	return GenerateID()
}

func (r *Dispatcher) RegisterHandler(key Protocol, hfn Handler)  { r.handlers.Store(key, hfn) }
func (r *Dispatcher) Stream(ctx context.Context) <-chan Response { return r.broker.Subscribe(ctx) }

func (r *Dispatcher) Broadcast(ctx context.Context, m Message) error {
	if m.Protocol.IsZero() {
		return ErrNoProtocol
	}

	m.ID = populateID(m.ID)
	return r.pipe.WaitPushBack(ctx, m)
}

func (r *Dispatcher) Exec(ctx context.Context, m Message) (*Response, bool) {
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

	if err := r.Broadcast(ctx, m); err != nil {
		return nil, false
	}

	val, err := fun.ReadOne(ctx, out)
	if err != nil {
		return nil, false
	}
	return &val, true
}

func (r *Dispatcher) doIntercept(ctx context.Context, key Protocol, rr *Response) (err error) {
	defer func() { err = erc.Merge(erc.RecoverError(), err) }()

	if icept, ok := r.interceptor.Load(key); ok {
		iter := icept.Iterator()
		for iter.Next(ctx) {
			if err = iter.Value()(ctx, rr); err != nil {
				return err
			}
		}
		if err = iter.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Dispatcher) doMiddleware(ctx context.Context, key Protocol, req *Message) (err error) {
	defer func() { err = erc.Merge(erc.RecoverError(), err) }()

	if mw, ok := r.middleware.Load(key); ok {
		iter := mw.Iterator()
		for iter.Next(ctx) {
			if err := iter.Value()(ctx, req); err != nil {
				return err
			}
		}
		if err := iter.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Dispatcher) doHandler(ctx context.Context, key Protocol, req Message) (resp Response) {
	defer func() { resp.Error = erc.Merge(erc.RecoverError(), resp.Error) }()

	h, ok := r.handlers.Load(key)
	if !ok {
		resp.Error = ErrNoHandler
		return
	}

	var err error
	resp, err = h(ctx, req)
	if resp.Error != nil && !errors.Is(resp.Error, err) {
		resp.Error = erc.Merge(err, resp.Error)
	}

	return
}

func (r *Dispatcher) registerServices(ctx context.Context) error {
	ec := &erc.Collector{}

	r.services.Name = fmt.Sprintf("Router<%s>", r.conf.Name)
	ec.Add(r.services.Add(srv.Broker(r.broker)))
	ec.Add(r.services.Add(&srv.Service{
		Run:     func(ctx context.Context) error { <-ctx.Done(); return nil },
		Cleanup: r.pipe.Close,
	}))

	// TODO config object to control the size of the worker pools
	// and the buffering of the pipes between things
	outQueue := pubsub.NewUnlimitedQueue[Response]()

	// process outgoing message filters
	ec.Add(r.services.Add(&srv.Service{
		Shutdown: outQueue.Close,
		Run: func(ctx context.Context) error {
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
		},
	}))

	// process middleware and actual handler
	ec.Add(r.services.Add(&srv.Service{
		Shutdown: r.pipe.Close,
		Run: func(ctx context.Context) error {
			return itertool.WorkerPool(ctx,
				itertool.Transform(ctx, r.pipe.IteratorBlocking(), func(req Message) fun.WorkerFunc {
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
