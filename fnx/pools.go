package fnx

import (
	"context"
	"errors"
	"iter"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
)

func handleWorkerError(err error) (bool, error) {
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, ers.ErrCurrentOpSkip):
		return false, nil
	case ers.IsExpiredContext(err):
		return true, err
	case ers.IsTerminating(err):
		return true, nil
	default:
		return true, err
	}
}

func RunAllWorkers(seq iter.Seq[Worker]) Worker {
	return func(ctx context.Context) error {
		for job := range seq {
			if abort, err := handleWorkerError(job.WithRecover().Run(ctx)); abort {
				return err
			}
		}
		return nil
	}
}

func RunAllOperations(seq iter.Seq[Operation]) Worker {
	return func(ctx context.Context) error {
		for job := range seq {
			if abort, err := handleWorkerError(job.WithRecover().Run(ctx)); abort {
				return err
			}
		}
		return nil
	}
}

func PoolWorkers(seq iter.Seq[Worker], opts ...OptionProvider[*WorkerGroupConf]) Worker {
	return func(ctx context.Context) error {
		conf := &WorkerGroupConf{}
		if err := JoinOptionProviders(opts...).Apply(conf); err != nil {
			return err
		}

		wg := &WaitGroup{}

		for shard := range irt.Shard(ctx, conf.NumWorkers, irt.Convert(seq, conf.workersForPool())) {
			wg.Launch(ctx, RunAllWorkers(shard).
				WithRecover().
				Ignore())
		}

		wg.Wait(ctx)
		return conf.ErrorCollector.Resolve()
	}
}
