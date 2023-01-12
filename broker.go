package fun

import (
	"context"
	"sync"
)

// stole this from https://stackoverflow.com/questions/36417199/how-to-broadcast-message-using-channel

type Broker[T any] struct {
	wg        WaitGroup
	publishCh chan T
	subCh     chan chan T
	unsubCh   chan chan T
	opts      BrokerOptions

	mu    sync.Mutex
	close context.CancelFunc
}

// BrokerOptions
type BrokerOptions struct {
	NonBlockingSubscriptions bool
	BufferSize               int
	ParallelDispatch         bool
}

func NewBroker[T any](opts BrokerOptions) *Broker[T] {
	return &Broker[T]{
		opts:      opts,
		publishCh: make(chan T, opts.BufferSize),
		subCh:     make(chan chan T, opts.BufferSize),
		unsubCh:   make(chan chan T, opts.BufferSize),
	}
}

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

func (b *Broker[T]) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.close()
}

func (b *Broker[T]) Wait(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.wg.Wait(ctx)
}

func (b *Broker[T]) Subscribe(ctx context.Context) chan T {
	msgCh := make(chan T)
	select {
	case <-ctx.Done():
		return nil
	case b.subCh <- msgCh:
		return msgCh
	}
}

func (b *Broker[T]) Unsubscribe(ctx context.Context, msgCh chan T) {
	select {
	case <-ctx.Done():
	case b.unsubCh <- msgCh:
	}
}

func (b *Broker[T]) Publish(ctx context.Context, msg T) {
	select {
	case <-ctx.Done():
	case b.publishCh <- msg:
	}
}
