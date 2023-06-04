package fun

import (
	"context"
	"math/rand"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func randomIntSlice(size int) Slice[int] {
	out := make([]int, size)
	for idx := range out {
		out[idx] = rand.Intn(size + 1*2)
	}
	return Sliceify(out)
}

func TestSlice(t *testing.T) {
	t.Run("Len", func(t *testing.T) {
		s := randomIntSlice(100)
		assert.Equal(t, s.Len(), 100)
		assert.True(t, !s.IsEmpty())
	})
	t.Run("ItemByIndex", func(t *testing.T) {
		s := randomIntSlice(100)
		assert.Equal(t, s[25], s.Item(25))
	})
	t.Run("AddItems", func(t *testing.T) {
		t.Run("Add", func(t *testing.T) {
			s := Slice[int]{}
			assert.True(t, s.IsEmpty())

			s.Add(42)
			assert.Equal(t, 42, s.Item(0))
			assert.Equal(t, 1, s.Len())
		})
		t.Run("Append", func(t *testing.T) {
			s := Slice[int]{}
			s.Append(42, 300, 64)
			assert.Equal(t, 42, s.Item(0))
			assert.Equal(t, 300, s.Item(1))
			assert.Equal(t, 64, s.Item(2))
			assert.Equal(t, 3, s.Len())
		})
		t.Run("Extend", func(t *testing.T) {
			s := Slice[int]{}
			s.Extend([]int{42, 300, 64})
			assert.Equal(t, 42, s.Item(0))
			assert.Equal(t, 300, s.Item(1))
			assert.Equal(t, 64, s.Item(2))
			assert.Equal(t, 3, s.Len())
		})
	})
	t.Run("Last", func(t *testing.T) {
		t.Run("Populated", func(t *testing.T) {
			s := randomIntSlice(100)
			assert.Equal(t, s.Last(), 99)
			assert.Equal(t, s.Len(), 100)
		})
		t.Run("Empty", func(t *testing.T) {
			s := Slice[int]{}
			assert.Equal(t, s.Last(), -1)
			assert.Equal(t, s.Len(), 0)
		})
		t.Run("Two", func(t *testing.T) {
			s := randomIntSlice(2)
			assert.Equal(t, s.Last(), 1)
			assert.Equal(t, s.Len(), 2)
		})
		t.Run("One", func(t *testing.T) {
			s := randomIntSlice(1)
			assert.Equal(t, s.Last(), 0)
			assert.Equal(t, s.Len(), 1)
		})
	})
	t.Run("Panics", func(t *testing.T) {
		t.Run("TruncateEmpty", func(t *testing.T) {
			s := Slice[int]{}
			assert.Panic(t, func() {
				s.Truncate(50)
			})

			s.Append(1, 2, 3)
			assert.Panic(t, func() {
				s.Truncate(4)
			})
		})
		t.Run("Reslice", func(t *testing.T) {
			s := Slice[int]{}
			assert.Panic(t, func() { s.Reslice(1, 2) })
			s = randomIntSlice(100)
			assert.Panic(t, func() { s.Reslice(4, 2) })
			assert.Panic(t, func() { s.Reslice(-1, 4) })
			assert.Panic(t, func() { s.ResliceBeginning(-1) })
			assert.Panic(t, func() { s.ResliceEnd(-1) })
		})
	})
	t.Run("Reslice", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			s := Slice[int]{}
			s.Reslice(0, 0)
			assert.Equal(t, s.Len(), 0)
		})
		t.Run("Shrink", func(t *testing.T) {
			s := randomIntSlice(100)
			s.Reslice(10, 90)
			assert.Equal(t, s.Len(), 80)
		})
		t.Run("Front", func(t *testing.T) {
			s := randomIntSlice(100)
			ts := s.Copy()
			ts.ResliceBeginning(50)
			assert.Equal(t, ts.Len(), 50)
			assert.NotEqual(t, ts.Item(1), s[1])
		})
		t.Run("Rear", func(t *testing.T) {
			s := randomIntSlice(100)
			ts := s.Copy()
			ts.ResliceEnd(50)
			assert.Equal(t, ts.Len(), 50)
			assert.NotEqual(t, ts.Item(ts.Last()), s[len(s)-1])
		})
	})
	t.Run("Copy", func(t *testing.T) {
		one := randomIntSlice(100)
		two := one.Copy()
		assert.EqualItems(t, one, two)
		two[77] = 42
		assert.NotEqualItems(t, one, two)
		assert.NotEqual(t, one[77], two[77])
	})
	t.Run("Sort", func(t *testing.T) {
		one := randomIntSlice(100)
		one.Sort(func(a, b int) bool { return a < b })
		var prev int
		for index, item := range one {
			if index == 0 {
				prev = item
				continue
			}
			if prev > item {
				t.Errorf("at index %d, item %d is greater than %d at %d", index-1, prev, item, index)
			}
		}
	})
	t.Run("Truncate", func(t *testing.T) {
		s := randomIntSlice(100)
		s.Truncate(50)
		assert.Equal(t, s.Last(), 49)
		assert.Equal(t, s.Len(), 50)
	})
	t.Run("Iterator", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		s := randomIntSlice(100)
		iter := s.Iterator()
		count := 0
		for iter.Next(ctx) {
			func(val int) {
				defer func() { count++ }()
				check.Equal(t, val, s[count])
				check.Equal(t, val, s.Item(count))
				if t.Failed() {
					t.Log("index", count)
					t.Fail()
				}
			}(iter.Value())
		}
		assert.NotError(t, iter.Close())
	})
	t.Run("Empty", func(t *testing.T) {
		s := randomIntSlice(100)
		check.Equal(t, s.Cap(), 100)
		check.Equal(t, s.Len(), 100)
		s.Empty()
		check.Equal(t, s.Cap(), 100)
		check.Equal(t, s.Len(), 0)
		check.True(t, s.IsEmpty())
	})
	t.Run("Reset", func(t *testing.T) {
		s := randomIntSlice(100)
		check.Equal(t, s.Cap(), 100)
		check.Equal(t, s.Len(), 100)
		s.Reset()
		check.Equal(t, s.Cap(), 0)
		check.Equal(t, s.Len(), 0)
		check.True(t, s.IsEmpty())
	})
}
