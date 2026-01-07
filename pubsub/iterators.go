package pubsub

import (
	"context"
	"iter"
	"time"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
)

// RateLimit wraps a iterator with a rate-limiter to ensure that the
// output iterator will produce no more than <num> items in any given
// <window>.
func RateLimit[T any](ctx context.Context, seq iter.Seq[T], num int, window time.Duration) iter.Seq[T] {
	erc.InvariantOk(num > 0, "rate must be greater than zero")

	timer := time.NewTimer(0)
	queue := &dt.List[time.Time]{}

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
			if queue.Len() < num {
				queue.PushBack(now)
				if val, ok := next(); !ok || !yield(val) {
					return stateComplete
				}
				return stateSendSuccessful
			}
			return stateRateExceded
		}

		prune := func(now time.Time) state {
			for queue.Len() > 0 && num >= queue.Len() && now.Sub(queue.Front().Value()) > window {
				queue.PopFront()
			}
			if queue.Len() >= num {
				return stateOverCapacity
			}
			return stateRetrySend
		}

		for ctx.Err() == nil {
			now := time.Now()
			s := send(now)

		RETRY:
			switch s {
			case stateComplete:
				return
			case stateSendSuccessful:
				continue
			case stateRateExceded:
				s = prune(now)
				goto RETRY
			case stateRetrySend:
				s = send(now)
				goto RETRY
			case stateOverCapacity:
				sleepUntil := max(time.Nanosecond, time.Until(queue.Front().Value().Add(window)))
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
}
