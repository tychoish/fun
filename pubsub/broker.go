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
// for distributing messages of a given type to many channels.
type Broker[T any] struct {
	wg        sync.WaitGroup
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
	// skip sending messags to subscriber channels are filled. In
	// this system, some subscribers will miss some messages based
	// on their own processing time.
	NonBlockingSubscriptions bool
	// BufferSize controls the buffer size of all internal broker
	// channels and channel subscriptions. When using a queue or
	// deque backed broker, the buffer size is only used for the
	// subscription channels.
	BufferSize int
	// ParallelDispatch, when true, sends each message to
	// subscribers in parallel, and (pending the behavior of
	// NonBlockingSubscriptions) waits for all messages to be
	// delivered before continuing.
	ParallelDispatch bool
	// QueueOptions, when set, instructs the Broker to dispatch
	// messages to subscribers via the queue implementation. The
	// queue provides a "load shedding" buffer between publishers
	// (so that they're not blocked) and the subscribers. If the
	// queue is full, new messages are dropped for all subscribers.
	//
	// When using the queue, messages are processed in a FIFO
	// order, and if the queue is full, incoming messages are
	// dropped until the queue empties.
	QueueOptions *QueueOptions
	// DequeOptions, when set, is similar to the Queue, messages
	// are processed in FIFO order, and uses the same
	// "load-shedding" semantics; however, when the queue is at
	// capacity, the oldest messages are dropped.
	DequeOptions *DequeOptions
	// WorkerPoolSize controls the number of go routines used for
	// sending messages to subscribers, when using Queue-backed
	// brokers. If unset this defaults to 1.
	//
	// When this value is larger than 1, the order of messages
	// observed by individual subscribers will not be consistent.
	WorkerPoolSize int
}

func (opts *BrokerOptions) Validate() error {
	if opts.DequeOptions != nil && opts.QueueOptions != nil {
		return errors.New("brokers may not have both queue and deque options")
	}
	if opts.QueueOptions != nil {
		if err := opts.QueueOptions.Validate(); err != nil {
			return err
		}
	}
	if opts.DequeOptions != nil {
		if err := opts.DequeOptions.Validate(); err != nil {
			return err
		}
	}

	if opts.BufferSize < 0 {
		opts.BufferSize = 0
	}

	return nil
}

// NewBroker constructs a new message broker. The default options are
// ideal for most use cases.
func NewBroker[T any](opts BrokerOptions) (*Broker[T], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	var bufSize int
	if opts.QueueOptions == nil {
		bufSize = opts.BufferSize
	}

	return &Broker[T]{
		opts:      opts,
		publishCh: make(chan T, bufSize),
		subCh:     make(chan chan T, bufSize),
		unsubCh:   make(chan chan T, bufSize),
	}, nil
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

	switch {
	case b.opts.QueueOptions != nil:
		b.startQueueWorkers(ctx)
	case b.opts.DequeOptions != nil:
		b.startDequeWorkers(ctx)
	default:
		b.startDefaultWorkers(ctx)
	}
}

func (b *Broker[T]) startDefaultWorkers(ctx context.Context) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		subs := set.Synchronize(set.NewOrdered[chan T]())
		for {
			select {
			case <-ctx.Done():
				return
			case msgCh := <-b.subCh:
				subs.Add(msgCh)
			case msgCh := <-b.unsubCh:
				subs.Delete(msgCh)
			case msg := <-b.publishCh:
				// do sending
				b.dispatchMessage(ctx, subs.Iterator(ctx), msg)
			}
		}
	}()
}

func (b *Broker[T]) startQueueWorkers(ctx context.Context) {
	b.wg.Add(2)

	queue := fun.Must(NewQueue[T](*b.opts.QueueOptions))
	subs := set.Synchronize(set.NewOrdered[chan T]())
	go func() { defer b.wg.Done(); <-ctx.Done(); _ = queue.Close() }()
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
			case msg := <-b.publishCh:
				if err := queue.Add(msg); err != nil {
					switch {
					case errors.Is(err, ErrQueueNoCredit) || errors.Is(err, ErrQueueFull):
						// we should drop messages while the queue is full.
						continue
					default:
						// likely impossible
						// queue encounters other error
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
				msg, ok := queue.Remove()
				if ok {
					b.dispatchMessage(ctx, subs.Iterator(ctx), msg)
					continue
				}
				msg, err := queue.Wait(ctx)
				if err != nil {
					return
				}
				b.dispatchMessage(ctx, subs.Iterator(ctx), msg)
			}
		}()
	}

}

func (b *Broker[T]) startDequeWorkers(ctx context.Context) {
	b.wg.Add(2)

	deque := fun.Must(NewDeque[T](*b.opts.DequeOptions))
	subs := set.Synchronize(set.NewOrdered[chan T]())
	go func() { defer b.wg.Done(); <-ctx.Done(); _ = deque.Close() }()
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
			case msg := <-b.publishCh:
				if err := deque.ForcePushBack(msg); err != nil {
					switch {
					case errors.Is(err, ErrQueueFull) || errors.Is(err, ErrQueueNoCredit):
						continue
					default:
						// likely impossible
						// queue encounters other error
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
				msg, ok := deque.WaitPopFront(ctx)
				if ok {
					b.dispatchMessage(ctx, subs.Iterator(ctx), msg)
					continue
				}
				if ctx.Err() != nil {
					return
				}
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
		_ = iter.Close(ctx)
		fun.Wait(ctx, wg)
	} else {
		for iter.Next(ctx) {
			b.sendMsg(ctx, msg, iter.Value())
		}
		_ = iter.Close(ctx)
	}

}

func (b *Broker[T]) sendMsg(ctx context.Context, m T, ch chan T) {
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
