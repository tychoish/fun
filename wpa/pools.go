package wpa

import (
	"context"
	"errors"
	"iter"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
)

type Job interface {
	fnx.Worker | fnx.Operation
	WithRecover() fnx.Worker
}

func Run[T Job](seq iter.Seq[T]) fnx.Worker {
	return func(ctx context.Context) error {
		for job := range seq {
			err := job.WithRecover().Run(ctx)
			switch {
			case err == nil:
				continue
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			case ers.IsExpiredContext(err):
				return err
			case ers.IsTerminating(err):
				return nil
			default:
				return err
			}
		}
		return nil
	}
}

func RunAll[T Job](seq iter.Seq[T]) fnx.Worker {
	return func(ctx context.Context) error {
		ec := &erc.Collector{}
		for job := range seq {
			job.WithRecover().Operation(ec.Push).Run(ctx)
		}
		return ec.Resolve()
	}
}

func RunWithPool[T Job](seq iter.Seq[T], opts ...fnx.OptionProvider[*WorkerGroupConf]) fnx.Worker {
	return func(ctx context.Context) error {
		conf := &WorkerGroupConf{}
		if err := fnx.JoinOptionProviders(opts...).Apply(conf); err != nil {
			return err
		}

		wg := &fnx.WaitGroup{}

		jobs := irt.Convert(seq, func(wf T) fnx.Worker { return wf.WithRecover().WithErrorFilter(conf.Filter) })
		for shard := range irt.Shard(ctx, conf.NumWorkers, jobs) {
			wg.Launch(ctx, Run(shard).
				WithRecover().
				Ignore())
		}

		wg.Wait(ctx)
		return conf.ErrorCollector.Resolve()
	}
}
