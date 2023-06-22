package fun

import (
	"errors"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

func TestOptionProvider(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		conf := &WorkerGroupConf{ExcludededErrors: []error{ers.ErrLimitExceeded}}
		newc := &WorkerGroupConf{ContinueOnPanic: true, ExcludededErrors: []error{ErrRecoveredPanic, ErrInvariantViolation}}
		eone := errors.New("cat")
		etwo := errors.New("3")

		testt.Log(t, "before", conf)
		err := WorkerGroupConfAddExcludeErrors(eone, etwo).Join(
			func(o *WorkerGroupConf) error {
				return nil
			},
			WorkerGroupConfSet(newc),
		).Apply(conf)

		testt.Log(t, "after", conf)
		check.NotError(t, err)
		check.Equal(t, len(conf.ExcludededErrors), 2)
		check.True(t, conf.ContinueOnPanic)
		check.NotContains(t, conf.ExcludededErrors, eone)
		check.NotContains(t, conf.ExcludededErrors, etwo)
	})
	t.Run("SkippsNil", func(t *testing.T) {
		assert.NotError(t, WorkerGroupConfAddExcludeErrors(nil).Join(nil, nil, nil).Apply(&WorkerGroupConf{}))
	})
	t.Run("Error", func(t *testing.T) {
		of := WorkerGroupConfAddExcludeErrors(ErrRecoveredPanic)
		opt := &WorkerGroupConf{}
		assert.Equal(t, 0, len(opt.ExcludededErrors))
		err := of(opt)
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "cannot exclude recovered panics")
	})
	t.Run("Collector", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		check.Error(t, WorkerGroupConfWithErrorCollector(nil)(opt))
		check.NotError(t, WorkerGroupConfWithErrorCollector(&Collector{})(opt))
	})
	t.Run("HandleErrorEdgecases", func(t *testing.T) {
		opt := &WorkerGroupConf{}
		t.Run("NilError", func(t *testing.T) {
			called := 0
			opt.ErrorObserver = func(err error) { called++ }

			check.True(t, opt.CanContinueOnError(nil))
			check.Equal(t, 0, called)
		})
		t.Run("Continue", func(t *testing.T) {
			called := 0
			opt.ErrorObserver = func(err error) { called++ }
			check.True(t, opt.CanContinueOnError(ErrIteratorSkip))
			check.Equal(t, 0, called)
		})
	})
}
