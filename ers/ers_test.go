package ers

import (
	"context"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestErrors(t *testing.T) {
	t.Run("Predicates", func(t *testing.T) {
		assert.True(t, IsTerminating(io.EOF))
		assert.True(t, IsTerminating(context.Canceled))
		assert.True(t, IsTerminating(context.DeadlineExceeded))
		assert.True(t, !IsTerminating(Error("hello")))
		assert.True(t, IsTerminating(Merge(Error("beep"), io.EOF)))
		assert.True(t, IsTerminating(Merge(Error("beep"), context.Canceled)))
		assert.True(t, IsTerminating(Merge(Error("beep"), context.DeadlineExceeded)))
	})

}
