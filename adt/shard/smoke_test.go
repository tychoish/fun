package shard_test

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun/adt/shard"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
)

func sumUint64(in []uint64) (out uint64) {
	for _, val := range in {
		out += val
	}
	return
}

func TestShardedMap(t *testing.T) {
	for _, impl := range []shard.MapType{
		shard.MapTypeMutex,
		shard.MapTypeRWMutex,
		shard.MapTypeStdlib,
		shard.MapTypeSync,
	} {
		t.Run(fmt.Sprintf("MapImpl_%s", impl), func(t *testing.T) {
			t.Run("Smoke", func(t *testing.T) {
				m := &shard.Map[string, int]{}
				m.Setup(32, impl)

				check.Equal(t, m.Version(), 0)

				m.Store("a", 42)
				check.Equal(t, m.Version(), 1)
				check.Equal(t, erc.MustOk(m.Load("a")), 42)
				_, ok := m.Load("b")
				check.True(t, !ok)

				m.Store("a", 84)
				check.Equal(t, m.Version(), 2)

				m.Store("b", 42)
				check.Equal(t, m.Version(), 3)
				check.Equal(t, erc.MustOk(m.Load("b")), 42)
				assert.Equal(t, sumUint64(m.Clocks())/2, m.Version())
			})
			t.Run("Stream", func(t *testing.T) {
				m := &shard.Map[string, int]{}
				m.Setup(32, impl)

				m.Store("one", 1)
				assert.Equal(t, sumUint64(m.Clocks())/2, m.Version())
				m.Store("two", 2)
				assert.Equal(t, sumUint64(m.Clocks())/2, m.Version())
				m.Store("three", 3)
				assert.Equal(t, sumUint64(m.Clocks())/2, m.Version())

				keys := map[string]struct{}{"one": {}, "two": {}, "three": {}}

				check := func(t *testing.T, item shard.MapItem[string, int]) {
					t.Helper()

					assert.True(t, item.Exists)
					_, ok := keys[item.Key]
					assert.True(t, ok)
					assert.Equal(t, 3, item.GlobalVersion)
					assert.Equal(t, 32, item.NumShards)
				}

				t.Run("Standard", func(t *testing.T) {
					ct := 0
					for item := range m.Items() {
						ct++
						check(t, item)
					}
					assert.Equal(t, ct, 3)
				})
			})
			t.Run("Fetch", func(t *testing.T) {
				var item shard.MapItem[string, int]

				m := &shard.Map[string, int]{}
				m.Setup(32, impl)

				m.Store("one", 37)
				item = m.Fetch("one")
				assert.True(t, item.Exists)
				assert.Equal(t, item.Value, 37)
				assert.Equal(t, item.Key, "one")
				assert.Equal(t, item.NumShards, 32)
			})
			t.Run("SetupCustomShards", func(t *testing.T) {
				var item shard.MapItem[string, int]

				m := &shard.Map[string, int]{}
				m.Setup(42, impl)
				assert.Equal(t, len(m.Clocks()), 42+1)

				t.Run("BeforePopulated", func(t *testing.T) {
					item = m.Fetch("one")
					assert.True(t, ft.Not(m.Check("one")))
					assert.True(t, ft.Not(item.Exists))
					assert.Equal(t, item.NumShards, 42)
					assert.Equal(t, 42+1, len(m.Clocks()))
				})
				t.Run("AfterPopulated", func(t *testing.T) {
					m.Store("one", 37)
					assert.True(t, m.Check("one"))
					item = m.Fetch("one")
					assert.True(t, item.Exists)
					assert.Equal(t, item.Value, 37)
					assert.Equal(t, item.Key, "one")
					assert.Equal(t, item.NumShards, 42)
				})
				t.Run("PostDelete", func(t *testing.T) {
					assert.True(t, m.Check("one"))
					m.Delete("one")
					assert.True(t, ft.Not(m.Check("one")))

					item = m.Fetch("one")
					assert.True(t, ft.Not(item.Exists))
					assert.Equal(t, item.NumShards, 42)
				})
			})
			t.Run("Keys", func(t *testing.T) {
				m := &shard.Map[string, int]{}
				m.Setup(32, impl)

				m.Store("one", 1)
				m.Store("two", 2)
				m.Store("three", 3)
				keys := map[string]struct{}{"one": {}, "two": {}, "three": {}}
				t.Run("Standard", func(t *testing.T) {
					ct := 0
					for item := range m.Keys() {
						ct++
						_, ok := keys[item]
						assert.True(t, ok)
					}
					assert.Equal(t, ct, 3)
				})
			})
			t.Run("Values", func(t *testing.T) {
				m := &shard.Map[string, int]{}
				m.Setup(32, impl)
				sum := 0
				for idx := range 100 * 32 {
					sum += idx
					m.Store(fmt.Sprint(idx), idx)
				}

				obsum := 0
				ct := 0
				for val := range m.Values() {
					obsum += val
					ct++
				}
				assert.Equal(t, obsum, sum)
				assert.Equal(t, ct, 100*32)
			})
		})
	}
	t.Run("DefaultSetup", func(t *testing.T) {
		var item shard.MapItem[string, int]

		m := &shard.Map[string, int]{}
		t.Run("BeforePopulated", func(t *testing.T) {
			item = m.Fetch("one")
			assert.True(t, ft.Not(m.Check("one")))
			assert.True(t, ft.Not(item.Exists))
			assert.Equal(t, item.NumShards, 32)
		})
		t.Run("AfterPopulated", func(t *testing.T) {
			m.Store("one", 37)
			assert.True(t, m.Check("one"))
			item = m.Fetch("one")
			assert.True(t, item.Exists)
			assert.Equal(t, item.Value, 37)
			assert.Equal(t, item.Key, "one")
			assert.Equal(t, item.NumShards, 32)
		})
		t.Run("PostDelete", func(t *testing.T) {
			assert.True(t, m.Check("one"))
			m.Delete("one")
			assert.True(t, ft.Not(m.Check("one")))
			item = m.Fetch("one")
			assert.True(t, ft.Not(item.Exists))
			assert.Equal(t, item.NumShards, 32)
		})
	})
	t.Run("VersionedObject", func(t *testing.T) {
		var item *shard.Versioned[int]
		t.Run("NilSafe", func(t *testing.T) {
			assert.True(t, ft.Not(item.Ok()))
			assert.Zero(t, item.Load())
			assert.Zero(t, item.Version())
		})
	})
}
