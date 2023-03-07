// Package pubsub provides a message broker for one-to-many or
// many-to-many message distribution. In addition pubsub includes a
// generic deque and queue implementations suited to concurrent use.
package pubsub

import (
	"context"
	"errors"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/set"
)

// stole this from
// https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel
// with some modifications and additional features.

// Broker is a simple message broker that provides a useable interface
// for distributing messages to an arbitrary group of channels.
type Broker[T any] struct {
	wg        sync.WaitGroup
	publishCh chan T
	subCh     chan chan T
	unsubCh   chan chan T
	opts      BrokerOptions
	stats     chan func(BrokerStats)

	mu    sync.Mutex
	close context.CancelFunc
}

type BrokerStats struct {
	Subscriptions int
	BufferDepth   int
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
	return MakeDistributorBroker(ctx, DistributorChannel(make(chan T)), opts)
}

// MakeDistributorBroker constructs a Broker that uses the provided
// distributor to handle the buffering between the sending half and
// the receiving half.
//
// In general, you should configure the distributor to provide
// whatever buffering requirements you have, and
func MakeDistributorBroker[T any](ctx context.Context, dist Distributor[T], opts BrokerOptions) *Broker[T] {
	b := makeBroker[T](opts)
	ctx, b.close = context.WithCancel(ctx)
	b.startQueueWorkers(ctx, dist)
	return b
}

// NewQueueBroker constructs a broker that uses the queue object to
// buffer incoming requests if subscribers are slow to process
// requests. Queue have a system for sheding load when the queue's
// limits have been exceeded. In general the messages are distributed
// in FIFO order, and Publish calls will drop messages if the queue is
// full.
//
// All brokers respect the BrokerOptions, which control the size of
// the worker pool used to send messages to senders and if the Broker
// should use non-blocking sends. All channels between the broker and
// the subscribers are un-buffered.
func NewQueueBroker[T any](ctx context.Context, queue *Queue[T], opts BrokerOptions) *Broker[T] {
	return MakeDistributorBroker(ctx, DistributorQueue(queue), opts)
}

// NewDequeBroker constructs a broker that uses the queue object to
// buffer incoming requests if subscribers are slow to process
// requests. The semantics of the Deque depends a bit on the
// configuration of it's limits and capacity.
//
// This broker distributes messages in a FIFO order, dropping older
// messages to make room for new messages.
func NewDequeBroker[T any](ctx context.Context, deque *Deque[T], opts BrokerOptions) *Broker[T] {
	return MakeDistributorBroker(ctx, DistributorDeque(deque), opts)
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
func NewLIFOBroker[T any](ctx context.Context, opts BrokerOptions, capacity int) *Broker[T] {
	return MakeDistributorBroker(ctx, DistributorLIFO(fun.Must(NewDeque[T](DequeOptions{Capacity: capacity}))), opts)
}

func makeBroker[T any](opts BrokerOptions) *Broker[T] {
	if opts.BufferSize < 0 {
		opts.BufferSize = 0
	}

	return &Broker[T]{
		opts:      opts,
		publishCh: make(chan T),
		subCh:     make(chan chan T, opts.BufferSize),
		unsubCh:   make(chan chan T, opts.BufferSize),
		stats:     make(chan func(BrokerStats)),
	}
}

func (b *Broker[T]) startQueueWorkers(
	ctx context.Context,
	dist Distributor[T],
) {
	subs := set.Synchronize(set.NewOrdered[chan T]())
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msgCh := <-b.subCh:
				subs.Add(msgCh)
			case msgCh := <-b.unsubCh:
				subs.Delete(msgCh)
			case fn := <-b.stats:
				fn(BrokerStats{
					Subscriptions: subs.Len(),
					BufferDepth:   dist.Len(),
				})
			case msg := <-b.publishCh:
				if err := dist.Send(ctx, msg); err != nil {
					// ignore most push errors: either they're queue full issues, which are the
					// result of user configuration (and we don't log anyway,) or
					// the queue has been closed (return), but otherwise
					// some amount of load shedding is fine here, and
					// we should avoid exiting too soon.
					switch {
					case errors.Is(err, ErrQueueFull) || errors.Is(err, ErrQueueNoCredit):
						continue
					case errors.Is(err, ErrQueueClosed):
						b.close()
						return
					}
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
			for {
				msg, err := dist.Receive(ctx)
				if err != nil {
					return
				}
				b.dispatchMessage(ctx, subs.Iterator(), msg)
			}
		}()
	}
}

func (b *Broker[T]) dispatchMessage(ctx context.Context, iter fun.Iterator[chan T], msg T) {
	// do sendingmsg
	if b.opts.ParallelDispatch {
		wg := &sync.WaitGroup{}
		for iter.Next(ctx) {
			wg.Add(1)
			go func(msg T, ch chan T) {
				defer wg.Done()
				b.sendMsg(ctx, msg, ch)
			}(msg, iter.Value())
		}
		_ = iter.Close()
		fun.Wait(ctx, wg)
	} else {
		for iter.Next(ctx) {
			b.sendMsg(ctx, msg, iter.Value())
		}
		_ = iter.Close()
	}

}

func (b *Broker[T]) Stats(ctx context.Context) BrokerStats {
	signal := make(chan BrokerStats)
	var output BrokerStats
	select {
	case <-ctx.Done():
		return output
	case b.stats <- func(stats BrokerStats) {
		defer close(signal)
		signal <- stats

	}:
	}
	select {
	case <-ctx.Done():
	case output = <-signal:
	}
	return output
}

func (b *Broker[T]) sendMsg(ctx context.Context, m T, ch chan T) {
	select {
	case <-ctx.Done():
	case ch <- m:
	}
}

// Stop cancels the broker, allowing background work to stop.
func (b *Broker[T]) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.close()
}

// Wait blocks until either the context has been canceled, or all work
// has been completed.
func (b *Broker[T]) Wait(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	fun.Wait(ctx, &b.wg)
}

// Subscribe generates a new subscription channel, of the specified
// buffer size. You *must* call Unsubcribe on this channel when you
// are no longer listening to this channel.
//
// Subscription channels are *not* closed and should never be closed
// by the caller.
func (b *Broker[T]) Subscribe(ctx context.Context) chan T {
	if ctx.Err() != nil {
		return nil
	}
	msgCh := make(chan T, b.opts.BufferSize)
	select {
	case <-ctx.Done():
		return nil
	case b.subCh <- msgCh:
		return msgCh
	}
}

// Unsubscribe removes a channel from the broker.
func (b *Broker[T]) Unsubscribe(ctx context.Context, msgCh chan T) {
	if ctx.Err() != nil {
		return
	}

	select {
	case <-ctx.Done():
	case b.unsubCh <- msgCh:
	}
}

// Publish distributes a message to all subscribers.
func (b *Broker[T]) Publish(ctx context.Context, msg T) {
	select {
	case <-ctx.Done():
	case b.publishCh <- msg:
	}
}
