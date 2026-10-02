package pubsub

import (
	"context"
	"iter"
	"time"

	"github.com/tychoish/fun/erc"
)

// RateLimit wraps a iterator with a rate-limiter to ensure that the
// output iterator will produce no more than <num> items in any given
// <window>.
func RateLimit[T any](ctx context.Context, seq iter.Seq[T], num int, window time.Duration) iter.Seq[T] {
	erc.InvariantOk(num > 0, "rate must be greater than zero")

	return func(yield func(T) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		// send times within the current window, oldest first. This is
		// state of one iteration: re-iterating starts with a clean slate.
		sent := make([]time.Time, 0, num)

		for ctx.Err() == nil {
			// pull first so that exhausting the input never waits
			// out a window.
			val, ok := next()
			if !ok {
				return
			}

			for {
				now := time.Now()
				for len(sent) > 0 && now.Sub(sent[0]) > window {
					sent = sent[1:]
				}
				if len(sent) < num {
					break
				}

				timer := time.NewTimer(max(time.Millisecond, time.Until(sent[0].Add(window))))
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return
				}
			}

			sent = append(sent, time.Now())
			if !yield(val) {
				return
			}
		}
	}
}
