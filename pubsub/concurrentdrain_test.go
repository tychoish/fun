package pubsub

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConcurrentDrainKeepsDraining(t *testing.T) {
	for name, mk := range drainTargets(t) {
		t.Run(name, func(t *testing.T) {
			d := mk()
			if err := d.PushBack(1); err != nil {
				t.Fatal(err)
			}

			ctxA, cancelA := context.WithCancel(context.Background())
			defer cancelA()
			ctxB, cancelB := context.WithCancel(context.Background())
			defer cancelB()

			doneA := make(chan error, 1)
			doneB := make(chan error, 1)
			go func() { doneA <- d.Drain(ctxA) }()
			go func() { doneB <- d.Drain(ctxB) }()

			// wait for both drainers to be running.
			deadline := time.Now().Add(5 * time.Second)
			for !errors.Is(d.PushBack(2), ErrQueueDraining) {
				if time.Now().After(deadline) {
					t.Fatal("never entered draining")
				}
				time.Sleep(time.Millisecond)
			}
			time.Sleep(50 * time.Millisecond)

			cancelA()
			select {
			case <-doneA:
			case <-doneB:
			case <-time.After(5 * time.Second):
				t.Fatal("no drain returned")
			}
			time.Sleep(20 * time.Millisecond)

			// one drain is still running, so pushes must still fail.
			if err := d.PushBack(3); !errors.Is(err, ErrQueueDraining) {
				t.Fatalf("expected ErrQueueDraining while a drain is outstanding, got %v", err)
			}
			cancelB()
		})
	}
}
