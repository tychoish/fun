package adt

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

func TestSet(t *testing.T) {
	for name, builder := range map[string]func() *Set[string]{
		"Unordered/Basic": func() *Set[string] { s := &Set[string]{}; return s },
		"Ordered/Basic":   func() *Set[string] { s := &Set[string]{}; s.Order(); return s },
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("EmptyIteraton", func(t *testing.T) {
				set := builder()
				ct := 0
				assert.NotPanic(t, func() {
					for range set.Iterator() {
						ct++
					}
				})
				assert.Zero(t, ct)
			})

			t.Run("Initialization", func(t *testing.T) {
				set := builder()
				if set.Len() != 0 {
					t.Fatal("initialized non-empty set")
				}

				// safe to set order more than once
				check.NotPanic(t, set.Order)
				check.NotPanic(t, set.Order)
			})
			t.Run("JSON", func(t *testing.T) {
				set := builder()
				set.Add("hello")
				set.Add("buddy")
				set.Add("hello")
				set.Add("kip")
				check.Equal(t, set.Len(), 3) // hello is a dupe

				data, err := json.Marshal(set)
				check.NotError(t, err)
				count := 0
				rjson := string(data)
				for item := range set.Iterator() {
					count++
					switch {
					case strings.Contains(rjson, "hello"):
						continue
					case strings.Contains(rjson, "buddy"):
						continue
					case strings.Contains(rjson, "kip"):
						continue
					default:
						t.Errorf("unexpeced item %q<%d>", item, count)
					}
				}
				check.Equal(t, count, set.Len())
				nset := builder()
				assert.NotError(t, nset.UnmarshalJSON(data))
			})
			t.Run("Recall", func(t *testing.T) {
				set := builder()
				set.Add("abc")
				if set.Len() != 1 {
					t.Error("should have added one")
				}
				if !set.Check("abc") {
					t.Error("should recall item")
				}
				set.Add("abc")
				if set.Len() != 1 {
					t.Error("should have only added key once")
				}
			})
			t.Run("Delete", func(t *testing.T) {
				set := builder()
				set.Add("abc")

				if !set.Check("abc") {
					t.Error("should recall item")
				}

				set.Delete("abc")

				if set.Check("abc") {
					t.Error("should have deleted item")
				}
			})
			t.Run("DeleteCheck", func(t *testing.T) {
				set := builder()
				set.Add("abc")

				if !set.DeleteCheck("abc") {
					t.Error("set item should have been present")
				}

				if set.DeleteCheck("abc") {
					t.Error("set item should not have been present")
				}
			})
			t.Run("AddCheck", func(t *testing.T) {
				set := builder()

				if set.AddCheck("abc") {
					t.Error("set item should not have been present")
				}
				if !set.AddCheck("abc") {
					t.Error("set item should have been present")
				}
			})
			t.Run("Sort", func(t *testing.T) {
				t.Run("Quick", func(t *testing.T) {
					t.Parallel()
					t.Run("UnorderedToStart", func(t *testing.T) {
						set := builder()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortQuick(cmp.LessThanNative[string])

						sl = irt.Collect(set.Iterator())

						check.Equal(t, "abc", sl[0])
						check.Equal(t, "def", sl[1])
						check.Equal(t, "ghi", sl[2])
						check.Equal(t, "lmn", sl[3])
						check.Equal(t, "opq", sl[4])
						check.Equal(t, "xyz", sl[5])
					})
					t.Run("OrderedStart", func(t *testing.T) {
						set := builder()
						set.Order()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")
						testt.Log(t, set.Len())

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortQuick(cmp.LessThanNative[string])

						sll := irt.Collect(set.Iterator())
						testt.Log(t, set.Len(), len(sl), sll)

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
				})
				t.Run("Merge", func(t *testing.T) {
					t.Parallel()
					t.Run("UnorderedToStart", func(t *testing.T) {
						set := builder()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")

						testt.Log(t, set.Len())

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortMerge(cmp.LessThanNative[string])

						sll := irt.Collect(set.Iterator())

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
					t.Run("OrderedStart", func(t *testing.T) {
						set := builder()
						set.Order()
						set.Add("xyz")
						set.Add("lmn")
						set.Add("abc")
						set.Add("opq")
						set.Add("def")
						set.Add("ghi")

						sl := irt.Collect(set.Iterator())

						if sl[0] == "abc" && sl[1] == "def" && sl[2] == "lmn" && sl[3] == "opq" && sl[4] == "xyz" {
							t.Fatal("should not be ordred by default")
						}
						set.SortMerge(cmp.LessThanNative[string])

						sll := irt.Collect(set.Iterator())

						check.Equal(t, "abc", sll[0])
						check.Equal(t, "def", sll[1])
						check.Equal(t, "ghi", sll[2])
						check.Equal(t, "lmn", sll[3])
						check.Equal(t, "opq", sll[4])
						check.Equal(t, "xyz", sll[5])
					})
				})
			})
		})
	}
	t.Run("JSONCheck", func(t *testing.T) {
		// want to make sure this works normally, without the
		// extra cast, as above.
		set := &Set[int]{}
		set.Order()
		err := json.Unmarshal([]byte("[1,2,3]"), &set)
		check.NotError(t, err)
		check.Equal(t, 3, set.Len())
	})
	t.Run("Constructors", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			var passed bool
			for attempt := 0; attempt < 10; attempt++ {
				func() {
					ls := make([]uint64, 0, 100)
					for i := 0; i < 100; i++ {
						ls = append(ls, rand.Uint64())
					}
					set := MakeSet(irt.Slice(ls))
					if set.Len() != 100 {
						return
					}
					set2 := MakeSet(irt.Slice(ls))
					if set2.Len() != 100 {
						return
					}
					if !irt.Equal(set.Iterator(), set2.Iterator()) {
						return
					}

					passed = true
				}()
				if passed {
					return
				}
			}
			t.Fatal("test could not pass")
		})
	})
}

// TestSetConcurrentIterationAndModification tests that iterating over a Set
// while concurrently modifying it does not cause data races or panics.
// Run with: go test -race
func TestSetConcurrentIterationAndModification(t *testing.T) {
	t.Run("OrderedSet", func(t *testing.T) {
		s := &Set[int]{}
		s.Order()

		// Pre-populate the set
		for i := 0; i < 100; i++ {
			s.Add(i)
		}

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Track that both goroutines ran
		var iteratorRan atomic.Bool
		var modifierRan atomic.Bool

		// Start iterator goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			iteratorRan.Store(true)

			// Iterate multiple times to increase chance of catching races
			for i := 0; i < 5; i++ {
				var count int
				for range s.Iterator() {
					count++
					// Small sleep to allow modifications to interleave
					time.Sleep(1 * time.Millisecond)
				}
				// We should see some items, but the exact count is undefined
				// due to concurrent modifications
				if count == 0 {
					t.Error("Iterator returned no items")
				}
			}
		}()

		// Start modifier goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			modifierRan.Store(true)

			for i := 0; i < 50; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					// Add new items
					s.Add(1000 + i)
					// Delete some items
					s.Delete(i)
					// Check some items
					_ = s.Check(50 + i)
					time.Sleep(2 * time.Millisecond)
				}
			}
		}()

		wg.Wait()

		if !iteratorRan.Load() {
			t.Error("Iterator goroutine did not run")
		}
		if !modifierRan.Load() {
			t.Error("Modifier goroutine did not run")
		}
	})

	t.Run("UnorderedSet", func(t *testing.T) {
		s := &Set[string]{}

		// Pre-populate
		for i := 0; i < 50; i++ {
			s.Add(string(rune('a' + i)))
		}

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Iterator goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				var items []string
				for item := range s.Iterator() {
					items = append(items, item)
					time.Sleep(500 * time.Microsecond)
				}
			}
		}()

		// Multiple modifier goroutines
		for g := 0; g < 3; g++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					select {
					case <-ctx.Done():
						return
					default:
						key := string(rune('A' + (id*100 + i)))
						if i%3 == 0 {
							s.Add(key)
						} else if i%3 == 1 {
							s.Delete(key)
						} else {
							_ = s.Check(key)
						}
						time.Sleep(1 * time.Millisecond)
					}
				}
			}(g)
		}

		wg.Wait()
	})
}

// TestMapConcurrentIterationAndModification tests that iterating over a Map
// while concurrently modifying it does not cause data races or panics.
// Run with: go test -race
func TestMapConcurrentIterationAndModification(t *testing.T) {
	t.Run("BasicConcurrentAccess", func(t *testing.T) {
		m := &Map[int, string]{}

		// Pre-populate
		for i := 0; i < 100; i++ {
			m.Store(i, string(rune('a'+(i%26))))
		}

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var iteratorRan atomic.Bool
		var modifierRan atomic.Bool

		// Iterator goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			iteratorRan.Store(true)

			for i := 0; i < 5; i++ {
				var count int
				for k, v := range m.Iterator() {
					count++
					// Use the values to prevent optimization
					_ = k
					_ = v
					time.Sleep(1 * time.Millisecond)
				}
				if count == 0 {
					t.Error("Iterator returned no items")
				}
			}
		}()

		// Modifier goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			modifierRan.Store(true)

			for i := 0; i < 50; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					// Store new items
					m.Store(1000+i, "new")
					// Delete items
					m.Delete(i)
					// Load items
					_, _ = m.Load(50 + i)
					time.Sleep(2 * time.Millisecond)
				}
			}
		}()

		wg.Wait()

		if !iteratorRan.Load() {
			t.Error("Iterator goroutine did not run")
		}
		if !modifierRan.Load() {
			t.Error("Modifier goroutine did not run")
		}
	})

	t.Run("MultipleReaders", func(t *testing.T) {
		m := &Map[string, int]{}

		// Pre-populate
		for i := 0; i < 50; i++ {
			m.Store(string(rune('a'+i)), i)
		}

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Multiple iterator goroutines
		for r := 0; r < 5; r++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < 3; i++ {
					for k, v := range m.Iterator() {
						_ = k
						_ = v
						time.Sleep(500 * time.Microsecond)
					}
				}
			}(r)
		}

		// Writer goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					m.Store(string(rune('A'+i)), i*10)
					time.Sleep(2 * time.Millisecond)
				}
			}
		}()

		wg.Wait()
	})
}

// TestSetIteratorWithIRTFunctions tests that using irt functions with
// concurrent modifications doesn't cause races.
func TestSetIteratorWithIRTFunctions(t *testing.T) {
	s := &Set[int]{}
	s.Order()

	// Pre-populate
	for i := 0; i < 100; i++ {
		s.Add(i)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Goroutine using irt.Collect
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			result := irt.Collect(s.Iterator())
			// Should have collected something
			if len(result) == 0 {
				t.Error("irt.Collect returned empty result")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Goroutine using irt.Apply
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			var sum int
			irt.Apply(s.Iterator(), func(n int) { sum += n })
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Goroutine using irt.Count
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			count := irt.Count(s.Iterator())
			if count == 0 {
				t.Error("irt.Count returned 0")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Modifier goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				s.Add(200 + i)
				s.Delete(i)
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	wg.Wait()
}

// TestMapIteratorWithIRTFunctions tests that using irt functions with
// concurrent modifications doesn't cause races.
func TestMapIteratorWithIRTFunctions(t *testing.T) {
	m := &Map[string, int]{}

	// Pre-populate
	for i := 0; i < 100; i++ {
		m.Store(string(rune('a'+(i%26))), i)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Goroutine using irt.Collect2
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			result := irt.Collect2(m.Iterator())
			if len(result) == 0 {
				t.Error("irt.Collect2 returned empty result")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Goroutine using irt.First
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			keys := irt.Collect(irt.First(m.Iterator()))
			if len(keys) == 0 {
				t.Error("irt.First returned empty result")
			}
			time.Sleep(3 * time.Millisecond)
		}
	}()

	// Goroutine using irt.Count2
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			count := irt.Count2(m.Iterator())
			if count == 0 {
				t.Error("irt.Count2 returned 0")
			}
			time.Sleep(3 * time.Millisecond)
		}
	}()

	// Modifier goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				m.Store(string(rune('A'+i)), i*100)
				m.Delete(string(rune('a' + (i % 26))))
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	wg.Wait()
}

// TestEarlyTerminationDuringConcurrentModification tests that breaking
// out of iteration early while modifications are happening is safe.
func TestEarlyTerminationDuringConcurrentModification(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		s := &Set[int]{}

		for i := 0; i < 1000; i++ {
			s.Add(i)
		}

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Iterator that terminates early
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 20; round++ {
				count := 0
				for range s.Iterator() {
					count++
					if count >= 5 {
						break // Early termination
					}
					time.Sleep(100 * time.Microsecond)
				}
			}
		}()

		// Heavy modifier
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					s.Add(2000 + i)
					s.Delete(i)
					time.Sleep(500 * time.Microsecond)
				}
			}
		}()

		wg.Wait()
	})

	t.Run("Map", func(t *testing.T) {
		m := &Map[int, string]{}

		for i := 0; i < 1000; i++ {
			m.Store(i, "value")
		}

		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Iterator that terminates early
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 0; round < 20; round++ {
				count := 0
				for range m.Iterator() {
					count++
					if count >= 5 {
						break // Early termination
					}
					time.Sleep(100 * time.Microsecond)
				}
			}
		}()

		// Heavy modifier
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					m.Store(2000+i, "new")
					m.Delete(i)
					time.Sleep(500 * time.Microsecond)
				}
			}
		}()

		wg.Wait()
	})
}

func BenchmarkSet(b *testing.B) {
	const size = 10000
	b.Run("Append", func(b *testing.B) {
		operation := func(set *Set[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * i)
			}
		}
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})
	b.Run("Mixed", func(b *testing.B) {
		operation := func(set *Set[int]) {
			var last int
			for i := 0; i < size; i++ {
				val := i * i * size
				set.Add(val)
				if i%3 == 0 {
					set.Delete(last)
				}
				last = val
			}
		}
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

	b.Run("Deletion", func(b *testing.B) {
		operation := func(set *Set[int]) {
			for i := 0; i < size; i++ {
				set.Add(i)
				set.Add(i * size)
			}
			for i := 0; i < size; i++ {
				set.Delete(i)
				set.Delete(i + 1)
				if i%3 == 0 {
					set.Delete(i * size)
				}
			}
		}
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})

	b.Run("Iteration", func(b *testing.B) {
		operation := func(set *Set[int]) {
			for i := 0; i < size; i++ {
				set.Add(i * size)
			}
			count := 0
			for range set.Iterator() {
				count++
			}
			check.Equal(b, size, count)
		}
		b.Run("Map", func(b *testing.B) {
			set := &Set[int]{}
			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
		b.Run("Ordered", func(b *testing.B) {
			set := &Set[int]{}
			set.Order()

			for j := 0; j < b.N; j++ {
				operation(set)
			}
		})
	})
}
