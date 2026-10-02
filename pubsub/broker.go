// Package pubsub provides a message broker for one-to-many or
// many-to-many message distribution. In addition pubsub includes a
// generic deque and queue implementations suited to concurrent use.
package pubsub

import (
	"context"
	"errors"
	"io"
	"iter"
	"sync/atomic"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
)

// ErrBrokerClosed is returned by Broker operations that are attempted
// after the broker has been stopped (or its context canceled).
const ErrBrokerClosed = ers.Error("broker is closed")

// stole this from
// https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel
// with some modifications and additional features.

// Broker is a simple message broker that provides a useable interface
// for distributing messages to an arbitrary group of channels.
type Broker[T any] struct {
	wg    fnx.WaitGroup
	ctlCh chan ctlRequest[T]
	opts  BrokerOptions

	close context.CancelFunc
	ctx   context.Context
	sink  func(context.Context, T) error
	count atomic.Uint64
}

// ctlRequest is a subscription change processed in order by the
// control loop. For subscriptions, ack is closed once the subscription
// is visible to dispatch.
type ctlRequest[T any] struct {
	ch    chan T
	ack   chan struct{}
	unsub bool
	stats func(BrokerStats)
}

// BrokerStats is a data struct used to report on the internal state
// of the broker.
type BrokerStats struct {
	Subscriptions int
	BufferDepth   int
	MessageCount  uint64
}

// BrokerOptions configures the semantics of a broker. The zero-values
// produce a blocking unbuffered queue message broker with every
// message distributed to every subscriber. While the default settings
// make it possible for one subscriber to block another subscriber,
// they guarantee that all messages will be delivered.  Buffered
// brokers may lose messages.
type BrokerOptions struct {
	// BufferSize controls the buffer size of the internal broker
	// channels that handle subscription creation and deletion
	// (unsubscribe.) Buffering
	BufferSize int
	// ParallelDispatch, when true, sends each message to
	// subscribers in parallel, and waits for all messages to be
	// delivered before continuing.
	ParallelDispatch bool
	// WorkerPoolSize controls the number of go routines used for
	// sending messages to subscribers, when using Queue-backed
	// brokers. If unset this defaults to 1.
	//
	// When this value is larger than 1, the order of messages
	// observed by individual subscribers will not be consistent.
	WorkerPoolSize int
	// NonBlockingPush, when true, makes Queue-backed brokers
	// (NewQueueBroker) insert messages with Queue.Push rather than
	// Queue.WaitPush: when the queue is full the message is shed
	// (ErrQueueFull) instead of applying back-pressure to
	// publishers. It has no effect on other brokers.
	NonBlockingPush bool
}

// NewBroker constructs with a simple distrubtion scheme: the incoming
// and outgoing messages are not buffered, but the client subscription
// channels are not buffered.
//
// All brokers respect the BrokerOptions, which control how messages
// are set to subscribers. The specific configuration of these
// settings can have profound impacts on the semantics and ordering of
// messages in the broker.
func NewBroker[T any](ctx context.Context, opts BrokerOptions) *Broker[T] {
	ch := make(chan T)
	chw := stw.ChanBlocking(ch)
	return makeInternalBrokerImpl(
		ctx,
		func(ctx context.Context) iter.Seq[T] { return irt.Channel(ctx, ch) },
		chw.Send().Write,
		chw.Len,
		opts,
	)
}

// makeInternalBrokerImpl constructs a Broker that uses the provided
// channel source for message distribution, with sink handling incoming
// published messages and length reporting buffer depth.
//
// In general, you should configure the source channel to provide
// whatever buffering requirements you have.
func makeInternalBrokerImpl[T any](
	ctx context.Context,
	source func(context.Context) iter.Seq[T],
	sink func(context.Context, T) error,
	length func() int,
	opts BrokerOptions,
) *Broker[T] {
	b := makeBroker[T](opts)
	ctx, b.close = context.WithCancel(ctx)
	b.ctx = ctx
	b.startQueueWorkers(ctx, source, sink, length)
	return b
}

// NewQueueBroker constructs a broker that uses the queue object to
// buffer incoming requests if subscribers are slow to process
// requests. Messages are distributed in FIFO order and are removed
// from the queue as they are dispatched.
//
// By default Publish and Send block (back-pressure) while the queue
// is full, matching Queue.WaitPush, and all messages are delivered.
// Set BrokerOptions.NonBlockingPush to use Queue.Push instead: when
// the queue is full the message is dropped (ErrQueueFull).
//
// All brokers respect the BrokerOptions, which control the size of
// the worker pool used to send messages to senders. All channels
// between the broker and the subscribers are un-buffered.
func NewQueueBroker[T any](ctx context.Context, queue *Queue[T], opts BrokerOptions) *Broker[T] {
	sink := queue.WaitPush
	if opts.NonBlockingPush {
		sink = func(_ context.Context, msg T) error { return queue.Push(msg) }
	}
	return makeInternalBrokerImpl(ctx, queue.IteratorWaitPop, sink, queue.Len, opts)
}

// NewDequeBroker constructs a broker that uses the queue object to
// buffer incoming requests if subscribers are slow to process
// requests. The semantics of the Deque depends a bit on the
// configuration of it's limits and capacity.
//
// This broker distributes messages in a FIFO order, dropping older
// messages to make room for new messages.
func NewDequeBroker[T any](ctx context.Context, deque *Deque[T], opts BrokerOptions) *Broker[T] {
	return makeInternalBrokerImpl(ctx, deque.IteratorWaitPopBack, deque.WaitPushFront, deque.Len, opts)
}

// NewLIFOBroker constructs a broker that uses the queue object to
// buffer incoming requests if subscribers are slow to process
// requests. The semantics of the Deque depends a bit on the
// configuration of it's limits and capacity.
//
// This broker distributes messages in a LIFO order, dropping older
// messages to make room for new messages. The capacity of the queue
// is fixed, and must be a positive integer greater than 0,
// NewLIFOBroker will panic if the capcity is less than or equal to 0.
func NewLIFOBroker[T any](ctx context.Context, deque *Deque[T], opts BrokerOptions) *Broker[T] {
	return makeInternalBrokerImpl(ctx, deque.IteratorWaitPopBack, deque.WaitPushBack, deque.Len, opts)
}

func makeBroker[T any](opts BrokerOptions) *Broker[T] {
	if opts.BufferSize < 0 {
		opts.BufferSize = 0
	}

	return &Broker[T]{
		opts:  opts,
		ctlCh: make(chan ctlRequest[T], opts.BufferSize),
	}
}

func (b *Broker[T]) startQueueWorkers(
	ctx context.Context,
	source func(context.Context) iter.Seq[T],
	sink func(context.Context, T) error,
	length func() int,
) {
	subs := &adt.SyncMap[chan T, chan struct{}]{}
	b.sink = sink
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-b.ctlCh:
				switch {
				case req.stats != nil:
					// ordered behind earlier subscribe/unsubscribe requests.
					req.stats(BrokerStats{
						Subscriptions: subs.Len(),
						BufferDepth:   length(),
						MessageCount:  b.count.Load(),
					})
				case req.unsub:
					// closing done releases any sender blocked
					// on this subscriber.
					if done, ok := subs.Load(req.ch); ok {
						subs.Delete(req.ch)
						close(done)
					}
				case !subs.Check(req.ch):
					subs.Store(req.ch, make(chan struct{}))
					close(req.ack)
				default:
					close(req.ack)
				}
			}
		}
	}()

	numWorkers := b.opts.WorkerPoolSize
	if numWorkers <= 0 {
		numWorkers = 1
	}

	for i := 0; i < numWorkers; i++ {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			for msg := range source(ctx) {
				b.dispatchMessage(ctx, subs.Iterator(), msg)
			}
		}()
	}
}

// dispatchMessage delivers msg to every subscriber, iterating the live
// subscription set (not a snapshot, so no per-message allocation).
//
// Because the set is live, a subscriber that registers while msg is
// being dispatched may also receive msg, even though it was published
// before that Subscribe call. Subscribers must tolerate one such
// message at the start of a subscription.
//
// Delivery is subject to head-of-line blocking: with sequential
// dispatch a slow subscriber delays every subscriber after it, and with
// ParallelDispatch each message still waits for its slowest subscriber
// before the next is dispatched. Use buffered subscriptions (BufferSize)
// or Unsubscribe slow consumers to bound the delay.
func (b *Broker[T]) dispatchMessage(ctx context.Context, seq iter.Seq2[chan T, chan struct{}], msg T) {
	// do sendingmsg
	if b.opts.ParallelDispatch {
		wg := &fnx.WaitGroup{}
		for value, done := range seq {
			wg.Add(1)
			go func(msg T, ch chan T, done chan struct{}) {
				defer wg.Done()
				b.sendMsg(ctx, msg, ch, done)
			}(msg, value, done)
		}
		wg.Wait(ctx)
	} else {
		for value, done := range seq {
			b.sendMsg(ctx, msg, value, done)
		}
	}
}

// Stats provides introspection into the current state of the broker.
//
// Stats has no error return: if ctx is canceled or the broker has been
// stopped it returns promptly with the zero BrokerStats.
func (b *Broker[T]) Stats(ctx context.Context) BrokerStats {
	signal := make(chan BrokerStats, 1)
	var output BrokerStats
	select {
	case <-ctx.Done():
		return output
	case <-b.ctx.Done():
		return output
	case b.ctlCh <- ctlRequest[T]{stats: func(stats BrokerStats) { signal <- stats }}:
	}

	select {
	case <-ctx.Done():
	case <-b.ctx.Done():
	case output = <-signal:
	}
	return output
}

func (b *Broker[T]) sendMsg(ctx context.Context, m T, ch chan T, done <-chan struct{}) {
	select {
	case <-ctx.Done():
	case <-done:
	case ch <- m:
	}
}

// Stop cancels the broker, allowing background work to stop.
func (b *Broker[T]) Stop() {
	b.close()
}

// Wait blocks until either the context has been canceled, or all work
// has been completed.
func (b *Broker[T]) Wait(ctx context.Context) {
	b.wg.Wait(ctx)
}

// Subscribe generates a new subscription channel, of the specified
// buffer size. You *must* call Unsubcribe on this channel when you
// are no longer listening to this channel.
//
// Subscribe waits until the subscription is visible to dispatch, so a
// Send or Publish that follows a successful Subscribe is delivered to
// the returned channel.
//
// Subscription channels are *not* closed and should never be closed
// by the caller. Closing a subscription channel will cause an
// unhandled panic.
//
// Subscribe returns ErrBrokerClosed if the broker has been stopped,
// and the context's error if ctx is canceled first.
func (b *Broker[T]) Subscribe(ctx context.Context) (chan T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.ctx.Err() != nil {
		return nil, ErrBrokerClosed
	}
	msgCh := make(chan T, b.opts.BufferSize)
	req := ctlRequest[T]{ch: msgCh, ack: make(chan struct{})}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.ctx.Done():
		return nil, ErrBrokerClosed
	case b.ctlCh <- req:
	}
	// wait for registration so that a Publish that follows Subscribe
	// is delivered to this subscriber.
	select {
	case <-ctx.Done():
		// the request is queued and will be processed; queue the
		// matching removal behind it so the subscription isn't leaked.
		_ = b.Unsubscribe(context.Background(), msgCh)
		return nil, ctx.Err()
	case <-b.ctx.Done():
		return nil, ErrBrokerClosed
	case <-req.ack:
		return msgCh, nil
	}
}

// Unsubscribe removes a channel from the broker. It returns
// ErrBrokerClosed if the broker has been stopped (all subscriptions
// are then moot), and the context's error if ctx is canceled before
// the request is accepted.
func (b *Broker[T]) Unsubscribe(ctx context.Context, msgCh chan T) error {
	if b.ctx.Err() != nil {
		return ErrBrokerClosed
	}
	select {
	case b.ctlCh <- ctlRequest[T]{ch: msgCh, unsub: true}:
		return nil
	case <-b.ctx.Done():
		return ErrBrokerClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Publish distributes a message to all subscribers. It is equivalent
// to Send, and returns ErrBrokerClosed after the broker has been
// stopped.
func (b *Broker[T]) Publish(ctx context.Context, msg T) error { return b.Send(ctx, msg) }

// Send distributes a message to all subscribers. The message is
// handed to the broker's sink on the calling goroutine, so
// back-pressure (and, for queue-backed brokers with NonBlockingPush,
// ErrQueueFull) is reported to the caller rather than stalling the
// broker's control loop.
//
// Send does not wait for subscribers: it returns once the sink has
// accepted the message. After the broker has been stopped, Send
// returns ErrBrokerClosed.
func (b *Broker[T]) Send(ctx context.Context, msg T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.ctx.Err() != nil {
		return ErrBrokerClosed
	}

	// abort a blocked sink if the broker stops.
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(b.ctx, cancel)()

	b.count.Add(1)
	err := b.sink(sctx, msg)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrQueueClosed) || errors.Is(err, io.EOF):
		b.close()
		return errors.Join(ErrBrokerClosed, err)
	case ctx.Err() == nil && b.ctx.Err() != nil:
		return errors.Join(ErrBrokerClosed, err)
	default:
		return err
	}
}
