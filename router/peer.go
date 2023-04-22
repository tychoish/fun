package router

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/seq"
)

// DialerFunc constructs a new connection. These functions can be
// simple constructors, or may actually open a network connection. The
// `T` object is arbitrary, and reflects the configuration of the
// peer. Typically this is a plain data struct.
type DialerFunc[T any] func(context.Context, T) (Connection, error)

type ListenerFunc[T any] func(context.Context) (T, Connection, error)

type RouterConfig struct {
	HeartbeatTimeout time.Duration
	CheckInterval    time.Duration
	MaxPeerAge       time.Duration
	DialAttempts     int
}

type Router[T any] struct {
	Dialer        adt.Atomic[DialerFunc[T]]
	Listner       adt.Atomic[ListenerFunc[T]]
	ErrorObserver adt.Atomic[func(error)]

	self string
	seq  atomic.Int64
	conf RouterConfig

	incoming pubsub.Broker[Envelope]
	outgoing pubsub.Deque[Envelope]
	peerPipe pubsub.Queue[T]

	peers adt.Map[string, Connection]
	store adt.Map[string, peer[T]]

	connErrorPeers pubsub.Deque[disconnectedPeer[T]]
	wg             fun.WaitGroup
}

type disconnectedPeer[T any] struct {
	opts        T
	peer        *peer[T]
	lastFailure time.Time
	count       int

	lastError       error
	checkFailed     bool
	missedHeartbeat bool
}

func NewRouter(conf RouterConfig) *Router[string] {
	return &Router[string]{
		conf: conf,
	}
}
func (r *Router[T]) Start(ctx context.Context) error {
	r.startDialer(ctx)
	r.startListener(ctx)
	return nil
}

func (r *Router[T]) Wait(ctx context.Context) { r.wg.Wait(ctx) }

// internal tracking object for a peer within the router.
type peer[T any] struct {
	id      string
	options T
	conn    Connection

	sequence          int64
	lastObservedReset time.Time

	info        *adt.Synchronized[*ConnectionInfo]
	historic    *seq.List[ConnectionInfo]
	failureInfo *disconnectedPeer[T]
}

// ConnectionInfo reports on the current state of a connection.
type ConnectionInfo struct {
	ID               string
	Connected        bool
	MessagesSent     int
	MessagesRecieved int
	Reconnect        bool
	DialedAt         time.Time
	LastActive       time.Time
	Created          time.Time
	Latency          time.Duration
}

var (
	ErrPeerDropped      = errors.New("peer dropped")
	ErrProtocolConflict = errors.New("protocol conflict")
)

// Connection represents a communication channel between two actors in
// a peer system, typically processes on different machines, though
// local in-process implementations.
//
// Connection handling and retry logic is implementation dependent: in
// general users of Connections should not be concerned with ensuring
// that peers are live and healthy. The Send/Recieve methods will
// ensure Envelopes are delivered and received,
type Connection interface {
	// ID provides a canonical (and hopefully stable) identifier
	// for the peer.
	ID() string
	Check(context.Context) error
	Send(context.Context, Envelope) error
	Recieve(context.Context) (Envelope, error)
	Close() error
}

const TargetNetwork = "__ENTIRE_NETWORK"

// Envelope represents a message sent between two components (Router
// instances) in a system.
//
// Envelopes primarily consist of two fields: Target,
// and the Payload. Which direct where the message will be delivered
// and it's content.
//
// Additionally, there are "semantics" options which communicates the
// expectations and requirements of the sender: use this if you want
// to send Envelopes that don
//
// The remainder of the fields are
type Envelope struct {
	// Target is the PeerID.
	Target string
	// Payload represents the body of the message, implementations
	// of Connection should take special care to ensure that the
	// payload of the is encoded and decoded with high fidelity.
	Payload Message

	// Broadcast, when true, indicates to the router that there
	Broadcast bool

	// The following fields are set by the router:

	// Error indicates if any errors were observed during the
	// routing of the message. If this message is in response to
	// another message, the error will propogate errors from the
	// peer. When non-nil, these errors may contain several
	// wrapped/aggregated errors.
	//
	// Connection implementations should take care to ensure that
	// these are encoded with sufficent fidelity: though it is
	// possible to encode and decode some errors between peers,
	// for most non-local cases the error objects will differ
	// across the protocol.
	Error error

	// RespondingTo contains the ID of the Payload that this
	// message is in response to.
	ResponingTo string

	// ResponseTimeout indicates a period of time (relative to the
	// sent/recieved at time stamps), after which a sender expects
	// that it will not care about the response. This is
	// *extremely* approximate and is loosely enforced by the
	// router, and responses that exceed this timeout may still be
	// sent or may be processed, depending on the implementation
	// details of the Connection, as well as the router itself.
	ResponseTimeout time.Duration

	// The remainder of the fields are populated (or overridden)
	// by either the sending or receiving peer.

	// Source is the ID of the sending router, and is always
	// overriden by the router before sending the message. Used to
	Source string

	// The timing information captures the reported times of both
	// the source and target, and makes it possible to observe the
	// latency of communication between two actors in the
	// system. Negative latencies are possible because of
	// clock skew.
	SentAt     time.Time
	RecievedAt time.Time

	// Sequence is populated by the *sending* Router, and is
	// always strictly increasing.
	Sequence int64
}

func (r *Router[T]) AddPeer(opt T) error { return r.peerPipe.Add(opt) }

func (r *Router[T]) Send(ep Envelope) error { return r.outgoing.PushBack(ep) }

func (r *Router[T]) Stream(ctx context.Context) fun.Iterator[Envelope] {
	sub := r.incoming.Subscribe(ctx)
	defer r.incoming.Unsubscribe(ctx, sub)
	return itertool.Channel(sub)
}

func (r *Router[T]) Exec(ctx context.Context, ep Envelope) (Envelope, error) {
	ep.Payload.ID = populateID(ep.Payload.ID)
	sub := r.incoming.Subscribe(ctx)
	defer r.incoming.Unsubscribe(ctx, sub)

	if err := r.Send(ep); err != nil {
		return Envelope{}, err
	}

	for {
		rsp, err := fun.ReadOne(ctx, sub)
		if err != nil {
			return Envelope{}, fmt.Errorf("message-id: %s: %w",
				ep.Payload.ID, err)
		}
		if rsp.ResponingTo != ep.Payload.ID {
			continue
		}
	}
}

func (r *Router[T]) createNewPeer(ctx context.Context, opts T, conn Connection) {
	var cancelPeer context.CancelFunc
	ctx, cancelPeer = context.WithCancel(ctx)

	connectedAt := time.Now()
	id := conn.ID()

	p := peer[T]{
		options: opts,
		id:      id,
		conn:    conn,
		info: adt.NewSynchronized(&ConnectionInfo{
			ID:        id,
			Connected: true,
			Created:   connectedAt,
			DialedAt:  connectedAt,
		}),
	}

	// when ensure store returns true, we allready had a
	// peer with the same address, let's use that one
	if !r.store.EnsureStore(id, p) {
		// TODO: make sure here that the peer's info
		// is related? we might not want to do
		p = r.store.Get(id)
	}

	if previous, ok := r.peers.Swap(id, conn); ok {
		// TODO handle this error? maybe?
		_ = previous.Close()
	}

	// RECIEVE MESSAGES
	r.wg.Add(1)
	go func() {
		defer func() {
			p.info.With(func(info *ConnectionInfo) {
				info.Connected = false
				r.ErrorObserver.Get()(conn.Close())
			})
			r.wg.Done()
		}()
		for {
			ep, err := conn.Recieve(ctx)

			if errors.Is(err, io.EOF) || erc.ContextExpired(err) {
				return
			}
			if err != nil {
				r.ErrorObserver.Get()(err)
				continue
			}
			now := time.Now()
			p.info.With(func(info *ConnectionInfo) {
				ep.RecievedAt = now
				info.LastActive = now
				info.MessagesRecieved++
				info.Connected = true
			})

			if ep.Target != r.self || ep.Target != TargetNetwork {
				// make an error here but drop it
				continue
			}

			if ep.Sequence < p.sequence {
				p.lastObservedReset = time.Now()
				p.sequence = ep.Sequence
			}

			r.incoming.Publish(ctx, ep)
		}
	}()

	// SEND MESSAGES
	r.wg.Add(1)
	go func() {
		defer func() {
			p.info.With(func(info *ConnectionInfo) {
				r.ErrorObserver.Get()(conn.Close())
				info.Connected = false
			})
			r.wg.Done()
		}()

		iter := r.outgoing.Iterator()
		defer func() { r.ErrorObserver.Get()(iter.Close()) }()

		for iter.Next(ctx) {
			ep := iter.Value()
			ep.Sequence = r.seq.Add(1)
			ep.SentAt = time.Now()
			ep.Source = r.self

			if err := conn.Send(ctx, ep); err != nil {
				r.ErrorObserver.Get()(err)
				return
			}
			p.info.With(func(info *ConnectionInfo) {
				info.LastActive = time.Now()
				info.MessagesSent++
				info.Connected = true
			})
		}
	}()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		// TODO make all timeouts and intervals options (at
		// least for testing)
		statCycle := time.NewTicker(10 * time.Second)
		defer statCycle.Stop()

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeat.C:
				info := p.info.Get()
				now := time.Now()
				if info.LastActive.Sub(now) < 30*time.Second {
					continue
				}
				// TODO make heartbeat protocol public
				rsp, err := r.Exec(ctx, Envelope{
					Target: p.id,
					Payload: Message{
						Protocol: Protocol{
							Schema:  "heartbeat-sent",
							Version: 0,
						},
						Payload: now,
					},
				})
				if err == nil {
					if rsp.Payload.Protocol.Schema != "heartbeat-sent" || rsp.Payload.Protocol.Version != 0 {
						err = ErrProtocolConflict
					}
				}
				if err := erc.Merge(err, rsp.Error); err != nil {
					info := p.failureInfo
					if info == nil {
						info = &disconnectedPeer[T]{
							opts:        opts,
							lastFailure: time.Now(),
							peer:        &p,
						}
						p.failureInfo = info
					}

					info.missedHeartbeat = true
					info.lastError = err

					_ = r.connErrorPeers.PushBack(*info)
					p.info.With(func(info *ConnectionInfo) {
						info.Connected = false
					})

					cancelPeer()
					return
				}
			case <-statCycle.C:
				if err := p.conn.Check(ctx); err != nil {
					info := p.failureInfo
					if info == nil {
						info = &disconnectedPeer[T]{
							opts:        opts,
							lastFailure: time.Now(),
							peer:        &p,
						}
						p.failureInfo = info
					}

					info.checkFailed = true
					_ = r.connErrorPeers.PushBack(*info)
					p.info.With(func(info *ConnectionInfo) {
						info.Connected = false
					})

					cancelPeer()
					return
				}
			case <-statCycle.C:
				p.info.With(func(info *ConnectionInfo) {
					p.historic.PushBack(*info)
					for p.historic.Len() > 360 {
						_ = p.historic.PopFront()
					}
				})
			}
		}
	}()
}

func (r *Router[T]) dialNewPeer(ctx context.Context, opts T) {
	dialer := r.Dialer.Get()

	conn, err := dialer(ctx, opts)
	if err != nil {
		_ = r.connErrorPeers.PushBack(disconnectedPeer[T]{
			opts:        opts,
			lastFailure: time.Now(),
			count:       1,
		})
		return
	}
	r.createNewPeer(ctx, opts, conn)
}

func (r *Router[T]) startDialer(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		iter := r.peerPipe.Iterator()
		for iter.Next(ctx) {
			r.dialNewPeer(ctx, iter.Value())
		}
	}()
}

func (r *Router[T]) startListener(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			opts, conn, err := r.Listner.Get()(ctx)
			if errors.Is(err, io.EOF) || erc.ContextExpired(err) {
				return
			} else if err != nil {
				r.ErrorObserver.Get()(err)
				continue
			}

			r.createNewPeer(ctx, opts, conn)
		}
	}()
}
