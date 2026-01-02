package shard

import (
	"fmt"
	"math"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
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

		check.True(t, ft.Not(m.Set("a", 12)))
		check.Equal(t, m.Version(), 1)
		check.True(t, m.Set("a", 42))
		check.Equal(t, m.Version(), 2)
		check.Equal(t, ft.MustOk(m.Load("a")), 42)
		_, ok := m.Load("b")
		check.True(t, !ok)
		check.Equal(t, ft.IgnoreSecond(m.Versioned("a")).Version(), 2)

		check.Zero(t, m.Get("c"))
		m.Store("a", 84)
		check.Equal(t, m.Version(), 3)
		check.Equal(t, m.Get("a"), 84)
		check.Equal(t, ft.IgnoreSecond(m.Versioned("a")).Version(), 3)

		m.Store("b", 42)
		check.Equal(t, m.Version(), 4)
		check.Equal(t, m.Get("b"), 42)
		check.Equal(t, ft.IgnoreSecond(m.Versioned("a")).Version(), 3)
		check.Equal(t, ft.IgnoreSecond(m.Versioned("b")).Version(), 1)

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
		m.Extend(irt.Map(map[string]int{"one": 1, "two": 2, "three": 3}))

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

	t.Run("IteratorSharded", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			m := &Map[string, int]{}
			m.Store("one", 1)
			m.Store("two", 2)
			m.Store("three", 3)

			seen := make(map[string]int)
			shardCount := 0

			for shardIter := range m.IteratorSharded() {
				shardCount++
				for k, v := range shardIter {
					seen[k] = v
				}
			}

			// Verify all items were seen
			assert.Equal(t, len(seen), 3)
			assert.Equal(t, seen["one"], 1)
			assert.Equal(t, seen["two"], 2)
			assert.Equal(t, seen["three"], 3)
			// Should have at least one shard with items
			assert.True(t, shardCount > 0 && shardCount <= 32)
		})

		t.Run("EmptyMap", func(t *testing.T) {
			m := &Map[string, int]{}

			count := 0
			shardCount := 0
			for shardIter := range m.IteratorSharded() {
				shardCount++
				for range shardIter {
					count++
				}
			}

			assert.Equal(t, count, 0)
			// All shards are still returned, just empty
			assert.Equal(t, shardCount, 32) // default number of shards
		})

		t.Run("LargeDataset", func(t *testing.T) {
			m := &Map[int, int]{}

			// Populate with 1000 items
			for i := 0; i < 1000; i++ {
				m.Store(i, i*10)
			}

			seen := make(map[int]int)
			shardCount := 0
			itemsPerShard := make(map[int]int)

			for shardIter := range m.IteratorSharded() {
				currentShard := shardCount
				shardCount++
				itemCount := 0

				for k, v := range shardIter {
					itemCount++
					seen[k] = v
					check.Equal(t, v, k*10) // Verify value matches expected
				}

				itemsPerShard[currentShard] = itemCount
			}

			// Verify all 1000 items were seen
			assert.Equal(t, len(seen), 1000)
			for i := 0; i < 1000; i++ {
				val, ok := seen[i]
				check.True(t, ok)
				check.Equal(t, val, i*10)
			}

			// Verify items are distributed across shards
			shardsWithItems := 0
			for _, count := range itemsPerShard {
				if count > 0 {
					shardsWithItems++
				}
			}
			// With 1000 items and good hashing, we should have multiple shards with items
			assert.True(t, shardsWithItems > 1)
		})

		t.Run("EarlyTermination", func(t *testing.T) {
			m := &Map[int, int]{}

			for i := 0; i < 100; i++ {
				m.Store(i, i*10)
			}

			// Break after first shard with items
			foundItems := false
			for shardIter := range m.IteratorSharded() {
				hasItems := false
				for range shardIter {
					hasItems = true
					break // Break from shard iterator
				}
				if hasItems {
					foundItems = true
					break // Break from sharded iterator
				}
			}

			assert.True(t, foundItems)
		})

		t.Run("ConcurrentAccess", func(t *testing.T) {
			m := &Map[int, int]{}

			// Populate the map with initial values
			for i := 0; i < 100; i++ {
				m.Store(i, i*10)
			}

			// Use channels to coordinate goroutines
			done := make(chan bool, 3)

			// Goroutine 1: Read items concurrently using IteratorSharded
			go func() {
				defer func() { done <- true }()
				for i := 0; i < 10; i++ {
					count := 0
					for shardIter := range m.IteratorSharded() {
						for k, v := range shardIter {
							count++
							// Basic sanity check
							check.True(t, v == k*10 || v == k*20)
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

			// Verify final state using IteratorSharded
			finalCount := 0
			finalSeen := make(map[int]int)
			for shardIter := range m.IteratorSharded() {
				for k, v := range shardIter {
					finalCount++
					finalSeen[k] = v
				}
			}
			assert.Equal(t, finalCount, 200)
			assert.Equal(t, len(finalSeen), 200)
		})

		t.Run("CompareWithNonSharded", func(t *testing.T) {
			m := &Map[string, int]{}
			m.Store("alpha", 1)
			m.Store("beta", 2)
			m.Store("gamma", 3)
			m.Store("delta", 4)
			m.Store("epsilon", 5)

			// Collect results from regular Iterator
			regularSeen := make(map[string]int)
			for k, v := range m.Iterator() {
				regularSeen[k] = v
			}

			// Collect results from IteratorSharded
			shardedSeen := make(map[string]int)
			for shardIter := range m.IteratorSharded() {
				for k, v := range shardIter {
					shardedSeen[k] = v
				}
			}

			// Both should see the same items
			assert.Equal(t, len(regularSeen), len(shardedSeen))
			for k, v := range regularSeen {
				val, ok := shardedSeen[k]
				check.True(t, ok)
				check.Equal(t, val, v)
			}
		})

		t.Run("NoItemDuplication", func(t *testing.T) {
			m := &Map[int, string]{}

			// Store 500 items
			for i := 0; i < 500; i++ {
				m.Store(i, fmt.Sprintf("value-%d", i))
			}

			// Track all keys seen across all shards
			seenKeys := make(map[int]int) // maps key to count

			for shardIter := range m.IteratorSharded() {
				for k := range shardIter {
					seenKeys[k]++
				}
			}

			// Verify we saw all 500 items
			assert.Equal(t, len(seenKeys), 500)

			// Verify no key appeared more than once
			for k, count := range seenKeys {
				if count != 1 {
					t.Errorf("key %d appeared %d times (expected 1)", k, count)
				}
			}
		})

		t.Run("CustomShardCount", func(t *testing.T) {
			m := &Map[int, int]{}
			m.Setup(8, MapTypeDefault) // Use 8 shards instead of default 32

			for i := 0; i < 100; i++ {
				m.Store(i, i*2)
			}

			shardCount := 0
			totalItems := 0

			for shardIter := range m.IteratorSharded() {
				shardCount++
				for range shardIter {
					totalItems++
				}
			}

			// Should see all 8 shards
			assert.Equal(t, shardCount, 8)
			// Should see all 100 items
			assert.Equal(t, totalItems, 100)
		})
	})
}
