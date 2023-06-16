package fun

import (
	"errors"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestOptionProvider(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		conf := &WorkerGroupOptions{ExcludededErrors: []error{ErrLimitExceeded}}
		newc := &WorkerGroupOptions{ContinueOnPanic: true, ExcludededErrors: []error{ErrRecoveredPanic, ErrInvariantViolation}}
		eone := errors.New("cat")
		etwo := errors.New("3")

		testt.Log(t, "before", conf)
		err := ApplyOptions(conf,
			AddExcludeErrors(eone, etwo),
			func(o *WorkerGroupOptions) error {
				return nil
			},
			Set(newc))
		testt.Log(t, "after", conf)
		check.NotError(t, err)
		check.Equal(t, len(conf.ExcludededErrors), 2)
		check.True(t, conf.ContinueOnPanic)
		check.NotContains(t, conf.ExcludededErrors, eone)
		check.NotContains(t, conf.ExcludededErrors, etwo)
	})
	t.Run("Error", func(t *testing.T) {
		of := AddExcludeErrors(ErrRecoveredPanic)
		opt := &WorkerGroupOptions{}
		assert.Equal(t, 0, len(opt.ExcludededErrors))
		err := of(opt)
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "cannot exclude recovered panics")
	})
	t.Run("HandleErrorEdgecases", func(t *testing.T) {
		opt := &WorkerGroupOptions{}
		t.Run("NilError", func(t *testing.T) {
			called := 0
			of := func(err error) { called++ }

			check.True(t, opt.CanContinueOnError(of, nil))
			check.Equal(t, 0, called)
		})
		t.Run("Continue", func(t *testing.T) {
			called := 0
			of := func(err error) { called++ }

			check.True(t, opt.CanContinueOnError(of, ErrIteratorSkip))
			check.Equal(t, 0, called)
		})
	})
}
