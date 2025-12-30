package fnx

import (
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

func TestOptionProvider(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		conf := &WorkerGroupConf{ExcludedErrors: []error{ers.ErrLimitExceeded}}
		newc := &WorkerGroupConf{ContinueOnPanic: true, ExcludedErrors: []error{ers.ErrRecoveredPanic, ers.ErrInvariantViolation}}
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
		check.NotError(t, WorkerGroupConfWithErrorCollector(&erc.Collector{})(opt))
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
	t.Run("HandleErrorEdgecases", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		t.Run("NilError", func(t *testing.T) {
			opt.ErrorCollector = &erc.Collector{}
			check.True(t, opt.CanContinueOnError(nil))
			check.Equal(t, 0, opt.ErrorCollector.Len())
		})
		t.Run("Termintaing", func(t *testing.T) {
			opt.ErrorCollector = &erc.Collector{}
			check.True(t, ft.Not(opt.CanContinueOnError(io.EOF)))
		})

		t.Run("Continue", func(t *testing.T) {
			opt.ErrorCollector = &erc.Collector{}
			check.True(t, opt.CanContinueOnError(ers.ErrCurrentOpSkip))
			check.Equal(t, 0, opt.ErrorCollector.Len())
		})
	})
}
