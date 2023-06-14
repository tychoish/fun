package itertool

import (
	"errors"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestOptionProvider(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		conf := &Options{ExcludededErrors: []error{fun.ErrLimitExceeded}}
		newc := &Options{ContinueOnPanic: true, ExcludededErrors: []error{fun.ErrRecoveredPanic, fun.ErrInvariantViolation}}
		eone := errors.New("cat")
		etwo := errors.New("3")

		testt.Log(t, "before", conf)
		err := Apply(conf,
			AddExcludeErrors(eone, etwo),
			func(o *Options) error {
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
		of := AddExcludeErrors(fun.ErrRecoveredPanic)
		opt := &Options{}
		assert.Equal(t, 0, len(opt.ExcludededErrors))
		err := of(opt)
		assert.Error(t, err)
		assert.Substring(t, err.Error(), "cannot exclude recovered panics")
	})
}
