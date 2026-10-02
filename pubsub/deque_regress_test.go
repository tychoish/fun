package pubsub

import (
	"context"
	"syscall"
	"testing"
	"time"
)

func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func TestDequeIdleWaitersDoNotSpin(t *testing.T) {
	for _, full := range []bool{false, true} {
		name := "PopEmpty"
		if full {
			name = "PushFull"
		}
		t.Run(name, func(t *testing.T) {
			dq, err := NewDeque[int](DequeOptions{Capacity: 1})
			if err != nil {
				t.Fatal(err)
			}
			if full {
				if err := dq.PushBack(1); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			start := cpuTime()
			done := make(chan struct{})
			for i := 0; i < 4; i++ {
				go func() {
					if full {
						_ = dq.WaitPushBack(ctx, 2)
					} else {
						_, _ = dq.WaitPopFront(ctx)
					}
					done <- struct{}{}
				}()
			}
			for i := 0; i < 4; i++ {
				<-done
			}
			if used := cpuTime() - start; used > 100*time.Millisecond {
				t.Fatalf("idle waiters burned %s of CPU in 300ms", used)
			}
		})
	}
}
