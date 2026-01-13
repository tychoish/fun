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

	timer := time.NewTimer(0)
	queue := NewUnlimitedQueue[time.Time]()

	type state int

	const (
		stateComplete       state = iota // early return
		stateRateExceded                 // try prune
		stateOverCapacity                // after prune: wait sleep
		stateRetrySend                   // after prune: retry send
		stateSendSuccessful              // under threshold
	)

	return func(yield func(T) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		send := func(now time.Time) state {
			if queue.Len() < num && queue.Push(now) == nil {
				if val, ok := next(); !ok || !yield(val) {
					return stateComplete
				}
				return stateSendSuccessful
			}
			return stateOverCapacity
		}

		prune := func(now time.Time) state {
			ok := true
			for ok && queue.Len() >= 0 && now.Sub(queue.getFront()) > window {
				_, ok = queue.Pop()
			}
			if queue.Len() >= num {
				return stateOverCapacity
			}
			return stateRetrySend
		}

		for ctx.Err() == nil {
			now := time.Now()

			switch send(now) {
			case stateComplete:
				return
			case stateRetrySend, stateSendSuccessful:
				continue
			case stateOverCapacity:
				switch prune(now) {
				case stateRetrySend:
					continue
				case stateOverCapacity:
					timer.Reset(max(time.Millisecond, time.Until(queue.getFront().Add(window))))
					select {
					case <-timer.C:
						continue
					case <-ctx.Done():
						return
					}
				}

			}
		}
	}
}
