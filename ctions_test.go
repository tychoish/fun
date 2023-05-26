package fun

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestWhen(t *testing.T) {
	t.Run("Do", func(t *testing.T) {
		out := WhenDo(true, func() int { return 100 })
		check.Equal(t, out, 100)

		out = WhenDo(false, func() int { return 100 })
		check.Equal(t, out, 0)
	})
	t.Run("Call", func(t *testing.T) {
		called := false
		WhenCall(true, func() { called = true })
		check.True(t, called)

		called = false
		WhenCall(false, func() { called = true })
		check.True(t, !called)
	})
}

func TestContains(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		assert.True(t, Contains(1, []int{12, 3, 44, 1}))
	})
	t.Run("NotExists", func(t *testing.T) {
		assert.True(t, !Contains(1, []int{12, 3, 44}))
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

func TestPtr(t *testing.T) {
	out := Ptr(123)
	assert.True(t, out != nil)
	check.Equal(t, *out, 123)

	// this is gross, but we have a pointer (non-nil) to an object
	// that is a pointer, which is nil.

	var dptr *string
	st := Ptr(dptr)
	assert.True(t, st != nil)
	assert.True(t, *st == nil)
	assert.Type[**string](t, st)
}

func TestDefault(t *testing.T) {
	assert.Equal(t, Default(0, 42), 42)
	assert.Equal(t, Default(77, 42), 77)

	assert.Equal(t, Default("", "kip"), "kip")
	assert.Equal(t, Default("merlin", "kip"), "merlin")
}
