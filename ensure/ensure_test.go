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
	t.Parallel()
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
			count := 0
			op := func() []string { count++; return nil }

			assert.NotPanic(t, func() {
				ensure.That(op).Queit().Log("one").Benchmark()(&testing.B{})
			})
			check.Equal(t, 1, count)
			assert.NotPanic(t, func() {
				_ = testing.Benchmark(func(b *testing.B) {
					ensure.That(op).Add(ensure.That(op)).Add(ensure.That(op)).
						Queit().Log("one").Run(b)
				})
			})
			check.Equal(t, 4, count)
		})
		t.Run("UnknownTestingTB", func(t *testing.T) {
			count := 0
			op := func() []string { count++; return nil }
			assert.Failing(t, func(t *testing.T) {
				ensure.That(op).Add(ensure.That(op)).Add(ensure.That(op)).
					Queit().Log("one").Run(struct{ *testing.T }{T: t})
			})
			check.Equal(t, 1, count)
		})
		t.Run("Add", func(t *testing.T) {
			count := 0
			op := func() []string { count++; return nil }
			ensure.That(op).Add(ensure.That(op)).Run(t)
			assert.Equal(t, count, 2)
		})
		t.Run("With", func(t *testing.T) {
			count := 0
			op := func() []string { count++; return nil }
			ensure.That(op).With("check", func(ensure *ensure.Assertion) {
				ensure.That(op)
			}).Run(t)
			assert.Equal(t, count, 2)
		})
	})
	t.Run("NoTests", func(t *testing.T) {
		assert.Failing(t, func(t *testing.T) {
			ensured := &ensure.Assertion{}
			ensured.Run(t)
		})
	})
}

type mock struct {
	*testing.T
	messages []string
}

func (t *mock) Log(args ...any) { t.messages = append(t.messages, fmt.Sprint(args...)) }
