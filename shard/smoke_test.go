package shard_test

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/shard"
)

func TestSmoke(t *testing.T) {
	m := &shard.Map[string, int]{}
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
}
