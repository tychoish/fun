package risky

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestOperations(t *testing.T) {
	t.Run("Ignore", func(t *testing.T) {
		t.Run("SwallowError", func(t *testing.T) {
			called := &atomic.Bool{}
			Ignore(func(in string) error {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return errors.New("exists")
			}, "hello world")
			assert.True(t, called.Load())
		})
		t.Run("IndifferentError", func(t *testing.T) {
			called := &atomic.Bool{}
			Ignore(func(in string) error {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return nil
			}, "hello world")
			assert.True(t, called.Load())
		})
		t.Run("Panic", func(t *testing.T) {
			called := &atomic.Bool{}
			Ignore(func(in string) error {
				called.Store(true)
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
		})

	})
	t.Run("IgnoreMust", func(t *testing.T) {
		t.Run("SwallowError", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return 100, errors.New("exists")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Equal(t, 100, output)
		})
		t.Run("IndifferentError", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return 100, nil
			}, "hello world")
			assert.True(t, called.Load())
			assert.Equal(t, 100, output)
		})
		t.Run("IndifferentValue", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (*int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return nil, errors.New("hello")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Zero(t, output)
		})
		t.Run("Panic", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Zero(t, output)
		})
		t.Run("PanicDefault", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (val int, _ error) {
				defer func() { val = 100 }()
				called.Store(true)
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
			// defers that don't recover aren't run
			assert.Zero(t, output)
		})
		t.Run("DefersRun", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (val int, _ error) {
				defer func() { val = 100 }()
				defer func() { called.Store(true) }()
				defer func() { _ = recover() }()
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Equal(t, 100, output)
		})
	})
	t.Run("Check", func(t *testing.T) {
		value, ok := Check(100, errors.New("foo"))
		check.True(t, !ok)
		check.Equal(t, 100, value)

		value, ok = Check(100, nil)
		check.True(t, ok)
		check.Equal(t, 100, value)
	})
	t.Run("Force", func(t *testing.T) {
		value := Force(100, errors.New("foo"))
		check.Equal(t, 100, value)

		value = Force(100, nil)
		check.Equal(t, 100, value)
	})
	t.Run("Block", func(t *testing.T) {
		out := Block(func(ctx context.Context) int {
			check.True(t, ctx != nil)
			check.NotError(t, ctx.Err())
			return 42
		})
		assert.Equal(t, out, 42)
	})
	t.Run("ForceOp", func(t *testing.T) {
		assert.Zero(t, ForceOp(func() (out int, err error) {
			out = 100
			panic("hi")
		}))
		assert.Equal(t, 100, ForceOp(func() (int, error) {
			return 100, nil
		}))
		assert.Equal(t, 100, ForceOp(func() (int, error) {
			return 100, errors.New("foo")
		}))
	})
	t.Run("BlockForceOp", func(t *testing.T) {
		assert.Zero(t, BlockForceOp(func(ctx context.Context) (out int, err error) {
			out = 100
			panic("hi")
		}))
		assert.Equal(t, 100, BlockForceOp(func(ctx context.Context) (int, error) {
			check.True(t, ctx != nil)
			check.NotError(t, ctx.Err())
			return 100, nil
		}))
		assert.Equal(t, 100, BlockForceOp(func(ctx context.Context) (int, error) {
			return 100, errors.New("foo")
		}))
	})
	t.Run("Try", func(t *testing.T) {
		assert.Equal(t, 100, Try(func(i int) (int, error) { return 0, errors.New("foo") }, 100))
		assert.Equal(t, 42, Try(func(i int) (int, error) { check.Equal(t, i, 100); return 42, nil }, 100))
	})
	t.Run("Cast", func(t *testing.T) {
		assert.NotPanic(t, func() { Cast[int](100) })
		assert.Panic(t, func() { Cast[int]("hello") })
		assert.Equal(t, Cast[int](100), 100)
		assert.Equal(t, Cast[string]("bond"), "bond")
	})
}

func TestApply(t *testing.T) {
	primes := []int{1, 3, 5, 7, 9, 11, 17, 19}
	magnitutde := Apply(func(in int) int { return in * 10 }, primes)
	assert.Equal(t, len(primes), len(magnitutde))
	assert.Equal(t, len(primes), 8)

	for idx := range primes {
		assert.Zero(t, magnitutde[idx]%primes[idx])
		assert.NotEqual(t, magnitutde[idx], primes[idx])
		assert.Equal(t, magnitutde[idx]/10, primes[idx])
	}
}
