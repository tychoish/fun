package ers

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestConstant(t *testing.T) {
	check.NotError(t, Wrap(nil, "hello"))
	check.NotError(t, Wrapf(nil, "hello %s %s", "args", "argsd"))
	const expected Error = "hello"
	err := Wrap(expected, "hello")
	assert.Equal(t, err.Error(), "hello: hello")
	assert.ErrorIs(t, err, expected)

	err = Wrapf(expected, "hello %s", "world")
	assert.Equal(t, err.Error(), "hello world: hello")
	assert.ErrorIs(t, err, expected)
}
