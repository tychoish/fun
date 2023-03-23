package pubsub

import (
	"context"
	"errors"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
)

type Message struct {
	Payload          any
	ResponseExpected bool

	Version int
	Schema  string
	ID      string
}

type Response struct {
	Payload any
	Error   error
	Version int
	Schema  string
	ID      string
}

type RegistryKey struct {
	Version int
	Schema  string
}

type Middleware func(context.Context, *Message) error
type Interceptor func(context.Context, *Response) error
type Handler func(context.Context, Message) (Response, error)

type Router struct {
	incoming *adt.SyncMap[RegistryKey, adt.Sequence[Middleware]]
	handlers *adt.SyncMap[RegistryKey, Handler]
	outgoing *adt.SyncMap[RegistryKey, adt.Sequence[Interceptor]]
	broker   *pubsub.Broker[Response]
}

func (r *Router) RegisterMiddleware(key RegistryKey, it Middleware)   { r.incoming.Get(key).Add(it) }
func (r *Router) RegisterInterceptor(key RegistryKey, it Interceptor) { r.outgoing.Get(key).Add(it) }
func (r *Router) RegisterHandler(key RegistryKey, hfn Handler)        { r.handlers.Store(key, hfn) }
func (r *Router) Stream(ctx context.Context) <-chan Response          { return r.broker.Subscribe(ctx) }
func (r *Router) Broadcast(ctx context.Context, m Message) error      { return nil }

func (r *Router) Exec(ctx context.Context, m Message) (*Response, bool) {
	sub := r.broker.Subscribe(ctx)
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

	_ = r.Broadcast(ctx, m)

	select {
	case <-ctx.Done():
		return nil, false
	case r := <-out:
		return &r, true
	}
}

var ErrNoHandler = errors.New("no handler defined")

func (r *Router) dispatchIncomming(ctx context.Context, incoming <-chan Message) {
	outQueue := pubsub.NewUnlimitedQueue[Response]()
	handleQueue := fun.Must(pubsub.NewDeque[Message](pubsub.DequeOptions{Unlimited: true}))

	// TODO run these worker pools in the background
	// TODO properly size worker pools
	_ = itertool.WorkerPool(ctx,
		itertool.Transform(ctx, outQueue.Iterator(), func(rr Response) fun.WorkerFunc {
			return func(ctx context.Context) error {
				if icetp, ok := r.outgoing.Load(RegistryKey{Version: rr.Version, Schema: rr.Schema}); ok {
					iter := icetp.Iterator()
					for iter.Next(ctx) {
						if err := iter.Value()(ctx, &rr); err != nil {
							rr.Error = erc.Merge(rr.Error, err)
							break
						}
					}
					if err := iter.Close(); err != nil {
						rr.Error = erc.Merge(rr.Error, err)
					}
				}

				r.broker.Publish(ctx, rr)
				return ctx.Err()
			}
		}),
		itertool.Options{NumWorkers: 8},
	)
	_ = itertool.WorkerPool(ctx,
		itertool.Transform(ctx, handleQueue.IteratorBlocking(), func(req Message) fun.WorkerFunc {
			return func(ctx context.Context) error {
				if mw, ok := r.incoming.Load(RegistryKey{Version: req.Version, Schema: req.Schema}); ok {
					fun.Observe(ctx, mw.Iterator(), func(m Middleware) { m(ctx, &req) })
				}
				var resp Response
				var err error
				if h, ok := r.handlers.Load(RegistryKey{Version: req.Version, Schema: req.Schema}); ok {
					resp, err = h(ctx, req)
				} else {
					err = ErrNoHandler
				}

				// if err is nil, Wrapf returns nil
				resp.Error = erc.Wrapf(err, "name=%q version=%d", req.Version, req.Version)

				return outQueue.Add(resp)
			}
		}),
		itertool.Options{NumWorkers: 8},
	)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-incoming:
			if err := handleQueue.WaitPushBack(ctx, msg); err != nil {
				return
			}
			continue
		}
	}
}
