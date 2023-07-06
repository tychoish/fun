package ensure_test

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ensure"
	"github.com/tychoish/fun/ensure/is"
)

func TestEnsure(t *testing.T) {
	t.Run("Basic", func(t *testing.T) { ensure.That(is.EqualTo(1, 1)).Verbose().Log("one").Run(t) })
	t.Run("Failing", func(t *testing.T) { check.Failing(t, func(t *testing.T) { ensure.That(is.EqualTo(1, 2)).Run(t) }) })
	t.Run("Error", func(t *testing.T) {
		count := 0
		check.Failing(t, func(t *testing.T) {
			ensure.That(is.EqualTo(1, 2)).Error().Run(t)
			count++
		})
		assert.Equal(t, 1, count)
	})
	t.Run("Fatal", func(t *testing.T) {
		count := 0
		check.Failing(t, func(t *testing.T) {
			ensure.That(is.EqualTo(1, 2)).Logf("log<%s>", t.Name()).Fatal().Run(t)
			count++
		})
		assert.Equal(t, 0, count)
	})
	t.Run("AbortDefautl", func(t *testing.T) {
		count := 0
		check.Failing(t, func(t *testing.T) {
			ensure.That(is.EqualTo(1, 2)).Run(t)
			count++
		})
		assert.Equal(t, 0, count)
	})
	t.Run("Metadata", func(t *testing.T) {
		m := &mock{}
		t.Run("Subset", func(t *testing.T) {
			m.T = t
			tt := ensure.That(is.True(true)).
				Verbose().
				Metadata(is.Plist().Add("hello", "world").Add("name", t.Name())).
				Logf("the-end: %d", 42)
			tt.Verbose().Run(m)
		})
		assert.Equal(t, len(m.messages), 3)
		assert.Equal(t, m.messages[0], `hello: "world"`)
		assert.Equal(t, m.messages[1], fmt.Sprintf(`name: "%s/Subset"`, t.Name()))
		assert.Substring(t, m.messages[2], "the-end: 42")
	})
	t.Run("Subtest", func(t *testing.T) {
		t.Run("Testing", ensure.That(is.EqualTo(1, 1)).Queit().Log("one").Test())
		t.Run("Benchmark", func(t *testing.T) {
			assert.NotPanic(t, func() {
				b := &testing.B{}
				ensure.That(is.EqualTo(1, 1)).Queit().Log("one").Benchmark()(b)
			})
		})
	})
}

type mock struct {
	*testing.T
	messages []string
}

func (t *mock) Log(args ...any) { t.messages = append(t.messages, fmt.Sprint(args...)) }
