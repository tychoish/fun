package fun

import (
	"context"
	"sync"
)

// stole this from
// https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel
// with some modifications and additional features.

// Broker is a simple message broker that provides a useable interface
// for distributing messages of a given type to many channels.
type Broker[T any] struct {
	wg        WaitGroup
	publishCh chan T
	subCh     chan chan T
	unsubCh   chan chan T
	opts      BrokerOptions

	mu    sync.Mutex
	close context.CancelFunc
}

// BrokerOptions configures the semantics of a broker. The zero-values
// produce a blocking unbuffered queue message broker with every
// message distributed to every subscriber. While the default settings
// make it possible for one subscriber to block another subscriber,
// they guarantee that all messages will be delivered. NonBlocking and
// Buffered brokers may lose messages.
type BrokerOptions struct {
	// NonBlockingSubscriptions, when true, allows the broker to
	// skip sending messags to subscribers with filled queues.
	NonBlockingSubscriptions bool
	// BufferSize controls the buffer size of all internal broker
	// channels.
	BufferSize int
	// ParallelDispatch, when true, sends each message to
	// subscribers in parallel, and (pending the behavior of
	// NonBlockingSubscriptions) waits for all messages to be
	// delivered before continuing.
	ParallelDispatch bool
}

// NewBroker constructs a new message broker. The default options are
// ideal for most use cases.
func NewBroker[T any](opts BrokerOptions) *Broker[T] {
	return &Broker[T]{
		opts:      opts,
		publishCh: make(chan T, opts.BufferSize),
		subCh:     make(chan chan T, opts.BufferSize),
		unsubCh:   make(chan chan T, opts.BufferSize),
	}
}

// Start initiates the background worker for the Broker, ignoring
// subsequent calls.
func (b *Broker[T]) Start(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.close != nil {
		return
	}

	ctx, b.close = context.WithCancel(ctx)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		subs := map[chan T]struct{}{}
		for {
			select {
			case <-ctx.Done():
				return
			case msgCh := <-b.subCh:
				subs[msgCh] = struct{}{}
			case msgCh := <-b.unsubCh:
				delete(subs, msgCh)
			case msg := <-b.publishCh:
				// do sending
				if b.opts.ParallelDispatch {
					wg := &WaitGroup{}
					for msgCh := range subs {
						wg.Add(1)
						go func(msg T, ch chan T) {
							defer wg.Done()
							b.send(ctx, msg, ch)
						}(msg, msgCh)
					}
					wg.Wait(ctx)
				} else {
					for ch := range subs {
						b.send(ctx, msg, ch)
					}
				}
			}
		}
	}()
}

func (b *Broker[T]) send(ctx context.Context, m T, ch chan T) {
	if b.opts.NonBlockingSubscriptions {
		select {
		case <-ctx.Done():
		case ch <- m:
		default:
		}
	} else {
		select {
		case <-ctx.Done():
		case ch <- m:
		}
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

	b.wg.Wait(ctx)
}

// Subscribe generates a new subscription channel, of the specified
// buffer size. You *must* call Unsubcribe on this channel when you
// are no longer listening to this channel.
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
