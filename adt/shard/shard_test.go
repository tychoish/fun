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
	t.Run("Iterator", func(t *testing.T) {
		m := &Map[string, int]{}
		m.Store("one", 1)
		m.Store("two", 2)
		m.Store("three", 3)
		count := 0
		for k, v := range m.Iterator() {
			count++
			switch v {
			case 1:
				check.Equal(t, k, "one")
			case 2:
				check.Equal(t, k, "two")
			case 3:
				check.Equal(t, k, "three")
			default:
				t.Fatalf("unexpected key [%s] value [%d] in iteration", k, v)
			}
		}
		assert.Equal(t, count, 3)
	})

	t.Run("ItemsSharded", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			m := &Map[string, int]{}
			m.Store("one", 1)
			m.Store("two", 2)
			m.Store("three", 3)

			seen := make(map[string]int)
			shardCount := 0

			for shardItems := range m.ItemsSharded() {
				shardCount++
				for item := range shardItems {
					check.True(t, item.Exists)
					check.True(t, item.GlobalVersion > 0)
					check.True(t, item.ShardVersion > 0)
					check.True(t, item.Version > 0)
					check.True(t, item.ShardID < item.NumShards)
					check.Equal(t, item.NumShards, uint64(32)) // default size
					seen[item.Key] = item.Value
				}
			}

			assert.Equal(t, len(seen), 3)
			assert.Equal(t, seen["one"], 1)
			assert.Equal(t, seen["two"], 2)
			assert.Equal(t, seen["three"], 3)
			assert.True(t, shardCount > 0 && shardCount <= 32)
		})

		t.Run("ConcurrentAccess", func(t *testing.T) {
			m := &Map[int, int]{}

			// Populate the map with initial values
			for i := 0; i < 100; i++ {
				m.Store(i, i*10)
			}

			// Use channels to coordinate goroutines
			done := make(chan bool, 3)

			// Goroutine 1: Read items concurrently
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 10; i++ {
					count := 0
					for shardItems := range m.ItemsSharded() {
						for item := range shardItems {
							count++
							check.True(t, item.Exists)
							check.True(t, item.GlobalVersion > 0)
							check.True(t, item.ShardID < item.NumShards)
						}
					}
					// We should see at least the initial items
					check.True(t, count >= 100)
				}
			}()

			// Goroutine 2: Write new items concurrently
			go func() {
				defer func() { done <- true }()
				for i := 100; i < 200; i++ {
					m.Store(i, i*10)
				}
			}()

			// Goroutine 3: Update existing items concurrently
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 50; i++ {
					m.Store(i, i*20)
				}
			}()

			// Wait for all goroutines to complete
			<-done
			<-done
			<-done

			// Verify final state
			finalCount := 0
			for shardItems := range m.ItemsSharded() {
				for item := range shardItems {
					finalCount++
					check.True(t, item.Exists)
				}
			}
			assert.Equal(t, finalCount, 200)
		})

		t.Run("EmptyMap", func(t *testing.T) {
			m := &Map[string, int]{}
			count := 0
			for shardItems := range m.ItemsSharded() {
				for range shardItems {
					count++
				}
			}
			assert.Equal(t, count, 0)
		})

		t.Run("VersionTracking", func(t *testing.T) {
			m := &Map[string, int]{}
			m.Store("key1", 100)
			initialVersion := m.Version()

			for shardItems := range m.ItemsSharded() {
				for item := range shardItems {
					if item.Key == "key1" {
						check.Equal(t, item.GlobalVersion, initialVersion)
						check.True(t, item.Version > 0)
					}
				}
			}

			// Update the key
			m.Store("key1", 200)
			updatedVersion := m.Version()
			check.True(t, updatedVersion > initialVersion)

			for shardItems := range m.ItemsSharded() {
				for item := range shardItems {
					if item.Key == "key1" {
						// Global version should reflect the update
						check.True(t, item.GlobalVersion >= updatedVersion)
						check.Equal(t, item.Value, 200)
					}
				}
			}
		})
	})
}
