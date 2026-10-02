package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"
)

type drainer interface {
	PushBack(int) error
	Drain(context.Context) error
	Shutdown(context.Context) error
	Close() error
}

type queueAdapter struct{ *Queue[int] }

func (q queueAdapter) PushBack(i int) error { return q.Push(i) }

func drainTargets(t *testing.T) map[string]func() drainer {
	return map[string]func() drainer{
		"Deque": func() drainer { return NewUnlimitedDeque[int]() },
		"Queue": func() drainer { return queueAdapter{NewUnlimitedQueue[int]()} },
	}
}

func TestCloseDuringDrainReturns(t *testing.T) {
	for name, mk := range drainTargets(t) {
		for opname, op := range map[string]func(drainer, context.Context) error{
			"Drain":    drainer.Drain,
			"Shutdown": drainer.Shutdown,
		} {
			t.Run(name+"/"+opname, func(t *testing.T) {
				d := mk()
				if err := d.PushBack(1); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				done := make(chan error, 1)
				go func() { done <- op(d, ctx) }()
				time.Sleep(20 * time.Millisecond)
				_ = d.Close()

				select {
				case err := <-done:
					if !errors.Is(err, ErrQueueClosed) {
						t.Fatalf("expected ErrQueueClosed, got %v", err)
					}
				case <-time.After(time.Second):
					t.Fatal("drain did not return after Close")
				}
			})
		}
	}
}
