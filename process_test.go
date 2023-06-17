package fun

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/testt"
)

func TestProcess(t *testing.T) {
	t.Run("Risky", func(t *testing.T) {
		called := 0
		pf := BlockingProcessor(func(in int) error {
			check.Equal(t, in, 42)
			called++
			return nil
		})
		check.NotError(t, pf.Block(42))
		pf.Ignore(testt.Context(t), 42)
		pf.Force(42)
		check.Equal(t, called, 3)
	})
	t.Run("Run", func(t *testing.T) {
		called := 0
		pf := BlockingProcessor(func(in int) error {
			check.Equal(t, in, 42)
			called++
			return nil
		})

		//nolint:staticcheck
		check.Panic(t, func() { _ = pf.Run(nil, 42) })
		check.NotPanic(t, func() { check.NotError(t, pf(nil, 42)) })
	})
}
