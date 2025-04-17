// Package itertool provides a set of functional helpers for
// managinging and using fun.Streams, including a parallel
// processing, generators, Map/Reduce, Merge, and other convenient
// tools.
package itertool

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// compile-time assertions that both worker types support the "safe"
// interface needed for the Worker() tool.
var _ interface{ WithRecover() fun.Worker } = new(fun.Worker)
var _ interface{ WithRecover() fun.Worker } = new(fun.Operation)

// WorkerPool takes streams of fun.WorkerPool or fun.Operation lambdas
// and processes them in according to the configuration.
//
// All operations functions are processed using their respective
// Safe() methods, which means that the functions themselves will
// never panic, and the ContinueOnPanic option will not impact the
// outcome of the operation (unless the stream returns a nil
// operation.)
//
// This operation is particularly powerful in combination with the
// stream for a pubsub.Distributor, interfaces which provide
// synchronized, blocking, and destructive (e.g. so completed
// workloads do not remain in memory) containers.
//
// WorkerPool is implemented using fun.Stream[T].ProcessParallel()
func WorkerPool[OP fun.Worker | fun.Operation](
	iter *fun.Stream[OP],
	optp ...fun.OptionProvider[*fun.WorkerGroupConf],
) fun.Worker {
	return iter.Parallel(func(ctx context.Context, op OP) error {
		return any(op).(interface{ WithRecover() fun.Worker }).WithRecover().Run(ctx)
	}, append(optp,
		fun.WorkerGroupConfWithErrorCollector(&erc.Collector{}),
		fun.WorkerGroupConfWorkerPerCPU(),
	)...)
}

// JSON takes a stream of line-oriented JSON and marshals those
// documents into objects in the form of a stream.
func JSON[T any](in io.Reader) *fun.Stream[T] {
	var zero T
	return fun.ConvertStream(fun.MAKE.LinesWithSpaceTrimed(in), fun.MakeConverterErr(func(in string) (out T, err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()
		if err = json.Unmarshal([]byte(in), &out); err != nil {
			return zero, err
		}
		return out, err
	}))
}

// Indexed produces a stream that keeps track of and reports the
// sequence/index id of the item in the iteration sequence.
func Indexed[T any](iter *fun.Stream[T]) *fun.Stream[dt.Pair[int, T]] {
	idx := &atomic.Int64{}
	idx.Store(-1)
	return fun.ConvertStream(iter, fun.MakeConverter(func(in T) dt.Pair[int, T] { return dt.MakePair(int(idx.Add(1)), in) }))
}

// RateLimit wraps a stream with a rate-limiter to ensure that the
// output stream will produce no more than <num> items in any given
// <window>.
func RateLimit[T any](iter *fun.Stream[T], num int, window time.Duration) *fun.Stream[T] {
	fun.Invariant.IsTrue(num > 0, "rate must be greater than zero")

	timer := time.NewTimer(0)
	queue := &dt.List[time.Time]{}

	return fun.NewGenerator(func(ctx context.Context) (zero T, _ error) {
		for {
			now := time.Now()

			if queue.Len() < num {
				queue.PushBack(now)
				return iter.Read(ctx)
			}

			for queue.Len() > 0 && now.After(queue.Front().Value().Add(window)) {
				queue.Front().Drop()
			}

			if queue.Len() < num {
				queue.PushBack(now)
				return iter.Read(ctx)
			}

			sleepUntil := time.Until(queue.Front().Value().Add(window))
			fun.Invariant.IsTrue(sleepUntil >= 0, "the next sleep must be in the future")

			timer.Reset(sleepUntil)
			select {
			case <-timer.C:
				continue
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}
	}).PostHook(func() { timer.Stop() }).Lock().Stream()
}
