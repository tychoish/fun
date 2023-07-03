package ensure_test

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ensure"
	"github.com/tychoish/fun/ensure/is"
)

func TestEsnure(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		ensure.That(is.EqualTo(1, 1)).Verbose().Log("one").Run(t)
	})
	t.Run("Failing", func(t *testing.T) {
		check.Failing(t, func(t *testing.T) { ensure.That(is.EqualTo(1, 2)).Run(t) })
	})
	t.Run("Subtest", func(t *testing.T) {
		ensure.That(is.EqualTo(1, 1)).Verbose().Subtest("action").Log("one").Run(t)
	})
}
