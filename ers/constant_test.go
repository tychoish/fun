package ers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestConstant(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		check.NotError(t, Wrap(nil, "hello"))
		check.NotError(t, Wrapf(nil, "hello %s %s", "args", "argsd"))
		const expected Error = "hello"
		err := Wrap(expected, "hello")
		assert.Equal(t, err.Error(), "hello: hello")
		assert.ErrorIs(t, err, expected)

		err = Wrapf(expected, "hello %s", "world")
		assert.Equal(t, err.Error(), "hello world: hello")
		assert.ErrorIs(t, err, expected)
	})
	t.Run("Is", func(t *testing.T) {
		const ErrForTest Error = "for-test"
		const ErrEmpty Error = ""

		var err error

		assert.True(t, ErrEmpty.Is(nil))
		assert.True(t, Error("").Is(nil))
		assert.True(t, ErrEmpty.Is(err))
		assert.True(t, Error("").Is(err))

		assert.True(t, !ErrForTest.Is(nil))
		assert.True(t, !ErrForTest.Is(err))
		assert.True(t, !Error("hi").Is(nil))
		assert.True(t, !Error("hi").Is(err))

		err = errors.New("for-test")

		assert.True(t, ErrForTest.Is(err))
		assert.True(t, ErrForTest.Is(New("for-test")))
		assert.True(t, ErrForTest.Is(ErrForTest))
		assert.True(t, Error("for-test").Is(err))
		assert.True(t, !Error("").Is(err))

		// use the stdlib helper
		// assert.True(t, errors.Is(err, ErrForTest))
		assert.True(t, !errors.Is(errors.New("fake"), ErrForTest))
		// assert.ErrorIs(t, err, ErrForTest)

	})
	t.Run("As", func(t *testing.T) {

		const ErrForTest Error = "for-test"
		var errValue Error
		var err error = ErrForTest

		assert.Zero(t, errValue)
		check.True(t, errors.As(err, &errValue))
		assert.NotZero(t, errValue)
		assert.True(t, errValue == "for-test")
		const ErrRoot Error = "root-error"

		wrapped := fmt.Errorf("Hello: %w", ErrRoot)

		var errValueTwo Error

		check.True(t, errors.As(wrapped, &errValueTwo))
		check.Equal(t, errValueTwo, "root-error")
		assert.True(t, wrapped.Error() != "root-error")
	})
}
