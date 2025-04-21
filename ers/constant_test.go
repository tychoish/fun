package ers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestConstant(t *testing.T) {
	t.Run("Is", func(t *testing.T) {
		const ErrForTest Error = "for-test"
		const ErrEmpty Error = ""

		var err error

		check.True(t, ErrEmpty.Is(nil))
		check.True(t, Error("").Is(nil))
		check.True(t, ErrEmpty.Is(err))
		check.True(t, Error("").Is(err))

		check.True(t, !ErrForTest.Is(nil))
		check.True(t, !ErrForTest.Is(err))
		check.True(t, !Error("hi").Is(nil))
		check.True(t, !Error("hi").Is(err))

		err = New("for-test")
		check.True(t, ErrForTest.Is(ErrForTest))
		check.True(t, !Error("").Is(err))

		// use the stdlib helper
		t.Log(err, ErrForTest, errors.Is(err, ErrForTest))
		check.True(t, Is(err, error(ErrForTest)))
		check.True(t, !errors.Is(errors.New("fake"), ErrForTest))
		check.ErrorIs(t, err, error(ErrForTest))
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
