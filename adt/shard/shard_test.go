package shard

import (
	"fmt"
	"math"
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
	t.Run("Maps", func(t *testing.T) {
		m := &Map[string, string]{}

		var invalid MapType = math.MaxUint8
		assert.Substring(t, invalid.String(), "invalid")
		assert.Substring(t, invalid.String(), fmt.Sprint(math.MaxUint8))

		// because it's invalid
		assert.Panic(t, func() { m.Setup(100, invalid) })

		// because it only runs once
		assert.NotPanic(t, func() { m.Setup(100, invalid) })
	})

	t.Run("Maps", func(t *testing.T) {
		m := &Map[string, string]{}
		assert.Substring(t, m.String(), "[string, string]")
		assert.Substring(t, m.String(), "adt.Map")
		assert.Substring(t, m.String(), "Shards(32)")
		assert.Substring(t, m.String(), "Version(0)")
	})

}
