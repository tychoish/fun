package fun

import (
	"context"
	"errors"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestOptionProvider(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		conf := &WorkerGroupConf{ExcludedErrors: []error{ers.ErrLimitExceeded}}
		newc := &WorkerGroupConf{ContinueOnPanic: true, ExcludedErrors: []error{ErrRecoveredPanic, ers.ErrInvariantViolation}}
		eone := errors.New("cat")
		etwo := errors.New("3")

		err := WorkerGroupConfAddExcludeErrors(eone, etwo).Join(
			func(_ *WorkerGroupConf) error {
				return nil
			},
			WorkerGroupConfSet(newc),
		).Apply(conf)

		check.NotError(t, err)
		check.Equal(t, len(conf.ExcludedErrors), 2)
		check.True(t, conf.ContinueOnPanic)
		check.NotContains(t, conf.ExcludedErrors, eone)
		check.NotContains(t, conf.ExcludedErrors, etwo)
	})
	t.Run("SkippsNil", func(t *testing.T) {
		assert.NotError(t, WorkerGroupConfAddExcludeErrors(nil).Join(nil, nil, nil).Apply(&WorkerGroupConf{}))
	})
	t.Run("Error", func(t *testing.T) {
		of := WorkerGroupConfAddExcludeErrors(ers.ErrRecoveredPanic)
		opt := &WorkerGroupConf{}
		assert.Equal(t, 0, len(opt.ExcludedErrors))
		err := of(opt)
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "cannot exclude recovered panics")
	})
	t.Run("Collector", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		check.Error(t, WorkerGroupConfWithErrorCollector(nil)(opt))
		check.NotError(t, WorkerGroupConfWithErrorCollector(&Collector{})(opt))
	})
	t.Run("Build", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		n, err := WorkerGroupConfNumWorkers(42).Build(opt)
		assert.NotError(t, err)
		assert.True(t, n != nil)
		assert.Equal(t, n.NumWorkers, 42)

		n, err = OptionProvider[*WorkerGroupConf](func(_ *WorkerGroupConf) error { return errors.New("hi") }).Build(opt)
		assert.Error(t, err)
		assert.True(t, n == nil)

	})
	t.Run("ErrorHandler", func(t *testing.T) {
		t.Run("Configuration", func(t *testing.T) {
			opt := &WorkerGroupConf{}
			check.True(t, opt.ErrorHandler == nil)
			check.True(t, opt.ErrorResolver == nil)
			check.NotError(t, WorkerGroupConfErrorCollectorPair(MAKE.ErrorCollector())(opt))
			check.True(t, opt.ErrorHandler != nil)
			check.True(t, opt.ErrorResolver != nil)
			check.NotError(t, opt.Validate())
		})
		t.Run("CollectionRoundTrip", func(t *testing.T) {
			opt := &WorkerGroupConf{}
			check.NotError(t, WorkerGroupConfErrorCollectorPair(MAKE.ErrorCollector())(opt))
			check.Equal(t, len(ers.Unwind(opt.ErrorResolver())), 0)
			opt.ErrorHandler(ers.New("hello"))
			check.Equal(t, len(ers.Unwind(opt.ErrorResolver())), 1)
		})
		t.Run("LoadTest", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			opt := &WorkerGroupConf{}
			check.NotError(t, WorkerGroupConfErrorCollectorPair(MAKE.ErrorCollector())(opt))

			wg := &WaitGroup{}
			wg.Group(128, func(_ context.Context) {
				opt.ErrorHandler(ers.New("hello"))
			}).Run(ctx)
			wg.Operation().Wait()
			check.Equal(t, len(ers.Unwind(opt.ErrorResolver())), 128)
		})
	})
	t.Run("HandleErrorEdgecases", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		t.Run("NilError", func(t *testing.T) {
			called := 0
			opt.ErrorHandler = func(_ error) { called++ }

			check.True(t, opt.CanContinueOnError(nil))
			check.Equal(t, 0, called)
		})
		t.Run("Continue", func(t *testing.T) {
			called := 0
			opt.ErrorHandler = func(_ error) { called++ }
			check.True(t, opt.CanContinueOnError(ErrStreamContinue))
			check.Equal(t, 0, called)
		})
	})
}
