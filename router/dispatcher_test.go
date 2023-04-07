package router

import (
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestDispater(t *testing.T) {
	t.Run("DispatcherConstructorDoesNotPanic", func(t *testing.T) {
		assert.NotPanic(t, func() { NewDispatcher(Config{}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -1, Buffer: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Buffer: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Buffer: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 100, Buffer: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 1, Buffer: 100}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 100, Buffer: -1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -1, Buffer: 100}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: -100, Buffer: 1}) })
		assert.NotPanic(t, func() { NewDispatcher(Config{Workers: 1, Buffer: -100}) })
	})

}
