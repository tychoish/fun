package ft

import (
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func TestErrorHandlingHelpers(t *testing.T) {
	t.Run("WrapCheck", func(t *testing.T) {
		t.Run("Passthrough", func(t *testing.T) {
			out, ok := Do2(WrapCheck(99, nil))
			check.Equal(t, out, 99)
			check.True(t, ok)
		})
		t.Run("Zero", func(t *testing.T) {
			out, ok := Do2(WrapCheck(99, io.EOF))
			check.Equal(t, out, 0)
			check.True(t, !ok)
		})
	})
}
