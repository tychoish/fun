package pubsub

import (
	"context"
	"iter"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
)

// RateLimit wraps a iterator with a rate-limiter to ensure that the
// output stream will produce no more than <num> items in any given
// <window>.
func RateLimit[T any](ctx context.Context, seq iter.Seq[T], num int, window time.Duration) iter.Seq[T] {
	fun.Invariant.Ok(num > 0, "rate must be greater than zero")

	timer := time.NewTimer(0)
	queue := &dt.List[time.Time]{}

	return func(yield func(T) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		send := func(now time.Time) bool {
			queue.PushBack(now)
			if val, ok := next(); !ok || !yield(val) {
				return false
			}
			return true
		}

		for {
			now := time.Now()

			for queue.Len() > 0 && now.After(queue.Front().Value().Add(window)) {
				queue.PopFront()
			}

			if queue.Len() < num {
				if !send(now) {
					return
				}
				continue
			}

			sleepUntil := time.Until(queue.Front().Value().Add(window))
			fun.Invariant.Ok(sleepUntil >= 0, "the next sleep must be in the future")

			timer.Reset(sleepUntil)
			select {
			case <-timer.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}
}
