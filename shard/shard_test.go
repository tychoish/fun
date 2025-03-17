package shard

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
)

func TestShardedMap(t *testing.T) {
	t.Run("LazyInitialization", func(t *testing.T) {
		m := &Map[string, int]{}
		assert.True(t, ft.Not(m.sh.Called()))
		m.Store("key", 42)
		assert.True(t, m.sh.Called())

		assert.Equal(t, defaultSize, m.shards().Len())
	})

	t.Run("Versions", func(t *testing.T) {
		m := &Map[string, int]{}
		check.Equal(t, m.Version(), 0)

		m.Store("a", 42)
		check.Equal(t, m.Version(), 1)
		check.Equal(t, ft.MustBeOk(m.Load("a")), 42)
		_, ok := m.Load("b")
		check.True(t, !ok)
		check.Equal(t, m.Versioned("a").Version(), 1)

		m.Store("a", 84)
		check.Equal(t, m.Version(), 2)
		check.Equal(t, ft.MustBeOk(m.Load("a")), 84)
		check.Equal(t, m.Versioned("a").Version(), 2)

		m.Store("b", 42)
		check.Equal(t, m.Version(), 3)
		check.Equal(t, ft.MustBeOk(m.Load("b")), 42)
		check.Equal(t, m.Versioned("a").Version(), 2)
		check.Equal(t, m.Versioned("b").Version(), 1)

		assert.Equal(t, m.Version(), m.Clocks()[0])
	})
}
