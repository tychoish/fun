package router

import (
	"context"
	"errors"
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

type Router[T any] struct {
	Dialer        adt.Atomic[DialerFunc[T]]
	ErrorObserver adt.Atomic[func(error)]

	self string
	seq  atomic.Int64

	incoming pubsub.Broker[Envelope]
	outgoing pubsub.Deque[Envelope]
	peerPipe pubsub.Queue[T]

	peers adt.Map[string, Connection]
	store adt.Map[string, peer[T]]
}

func NewRouter() *Router[string] {
	r := &Router[string]{}
	r.reactor(context.TODO())
	return r
}

// internal tracking object for a peer within the router.
type peer[T any] struct {
	id      string
	options T
	conn    Connection

	sequence          int64
	lastObservedReset time.Time

	info adt.Synchronized[*ConnectionInfo]

	// cap this.
	historic *seq.List[ConnectionInfo]
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

var ErrPeerDropped = errors.New("peer dropped")

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
	// for most non-local cases
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

func (r *Router[T]) AddPeer(opt T) error    { return r.peerPipe.Add(opt) }
func (r *Router[T]) Send(ep Envelope) error { return r.outgoing.PushBack(ep) }

func (r *Router[T]) Stream(ctx context.Context) fun.Iterator[Envelope] {
	sub := r.incoming.Subscribe(ctx)
	defer r.incoming.Unsubscribe(ctx, sub)
	return itertool.Channel(sub)
}

// func (r *Router[T]) Exec(ctx context.Context, ep Envelope) (Envelope, error) {
// 	sub := r.incoming.Subscribe(ctx)
// 	defer r.incoming.Unsubscribe(ctx, sub)
// 	for {
// 		select {
// 		case <-ctx.Done():
// 		}
// 	}

// }

func (r *Router[T]) reactor(ctx context.Context) {
	iter := r.peerPipe.Iterator()
	for iter.Next(ctx) {
		dialer := r.Dialer.Get()

		opts := iter.Value()
		conn, err := dialer(ctx, opts)
		if err != nil {

			// TODO:
			//  - retry
			//  - backoff
			//  - record keeping
			continue
		}
		id := conn.ID()
		p := peer[T]{
			options: opts,
			id:      id,
			conn:    conn,
		}
		connectedAt := time.Now()
		p.info.Set(&ConnectionInfo{
			ID:        id,
			Connected: true,
			Created:   connectedAt,
			DialedAt:  connectedAt,
		})

		// when ensure store returns true, we allready had a
		// peer here
		if !r.store.EnsureStore(id, p) {
			// TODO: make sure here that the peer's info
			// is related? we might not want to do
			p = r.store.Get(id)
		}

		if previous, ok := r.peers.Swap(id, conn); ok {
			// TODO handle this error? maybe?
			_ = previous.Close()
		}

		// TODO: worker pool...

		// TODO: figure out how to keep track of
		// incoming/outgoing connections.

		// RECIEVE MESSAGES
		go func() {
			defer func() {
				p.info.With(func(info *ConnectionInfo) {
					info.Connected = false
					r.ErrorObserver.Get()(conn.Close())
				})
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
		go func() {
			defer func() {
				p.info.With(func(info *ConnectionInfo) {
					r.ErrorObserver.Get()(conn.Close())
					info.Connected = false
				})
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

		go func() {
			cycle := time.NewTicker(time.Second)
			for {
				select {
				case <-ctx.Done():
					return
				case <-cycle.C:
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
}
