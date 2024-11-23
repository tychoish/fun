package dt

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
)

func randomIntSlice(size int) Slice[int] {
	out := make([]int, size)
	for idx := range out {
		out[idx] = intish.Abs(rand.Intn(size) + 1)
	}
	return NewSlice(out)
}

func intSlice(size int) Slice[int] {
	out := make([]int, size)
	for idx := range out {
		out[idx] = idx
	}
	return NewSlice(out)
}

func TestSlice(t *testing.T) {
	t.Run("Len", func(t *testing.T) {
		s := randomIntSlice(100)
		assert.Equal(t, s.Len(), 100)
		assert.True(t, !s.IsEmpty())
	})
	t.Run("ItemByIndex", func(t *testing.T) {
		s := randomIntSlice(100)
		assert.Equal(t, s[25], s.Index(25))
	})
	t.Run("Variadic", func(t *testing.T) {
		s := Variadic(randomIntSlice(100)...)
		assert.Equal(t, s[25], s.Index(25))
	})
	t.Run("Grow", func(t *testing.T) {
		t.Run("Expanded", func(t *testing.T) {
			sl := Slice[int]{1, 2, 3}
			check.Equal(t, len(sl), 3)
			sl.Grow(5)
			assert.Equal(t, len(sl), 5)
			check.Equal(t, sl[3], 0)
			check.Equal(t, sl[4], 0)
		})
		t.Run("Invalid", func(t *testing.T) {
			check.Panic(t, func() {
				sl := Slice[int]{1, 2, 3}
				sl.Grow(-1)
			})
			check.Panic(t, func() {
				sl := Slice[int]{1, 2, 3}
				sl.Grow(2)
			})
		})
	})
	t.Run("AddItems", func(t *testing.T) {
		t.Run("Add", func(t *testing.T) {
			s := Slice[int]{}
			assert.True(t, s.IsEmpty())

			s.Add(42)
			assert.Equal(t, 42, s.Index(0))
			assert.Equal(t, 1, s.Len())
		})
		t.Run("Append", func(t *testing.T) {
			s := Slice[int]{}
			s.Append(42, 300, 64)
			assert.Equal(t, 42, s.Index(0))
			assert.Equal(t, 300, s.Index(1))
			assert.Equal(t, 64, s.Index(2))
			assert.Equal(t, 3, s.Len())
		})
		t.Run("Extend", func(t *testing.T) {
			s := Slice[int]{}
			s.Extend([]int{42, 300, 64})
			assert.Equal(t, 42, s.Index(0))
			assert.Equal(t, 300, s.Index(1))
			assert.Equal(t, 64, s.Index(2))
			assert.Equal(t, 3, s.Len())
		})
	})
	t.Run("GrowCapacity", func(t *testing.T) {
		t.Run("Zero", func(t *testing.T) {
			s := Slice[int]{}
			check.Equal(t, cap(s), 0)
			check.Equal(t, len(s), 0)
			s.GrowCapacity(32)
			check.Equal(t, cap(s), 32)
			check.Equal(t, len(s), 0)
		})
		t.Run("Big", func(t *testing.T) {
			s := Slice[int]{1, 2, 3}
			check.Equal(t, cap(s), 3)
			check.Equal(t, len(s), 3)
			s.GrowCapacity(32)
			check.Equal(t, cap(s), 32)
			check.Equal(t, len(s), 3)

		})

	})
	t.Run("Sparse", func(t *testing.T) {
		s := Slice[*int]{ft.Ptr(1), ft.Ptr(42), nil, nil}
		check.Equal(t, len(s), 4)
		sp := s.Sparse()
		check.Equal(t, len(sp), 2)
		check.NotNil(t, sp[0])
		check.NotNil(t, sp[1])
		check.Equal(t, ft.Ref(sp[1]), 42)
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
			s := intSlice(100)
			s.Reslice(10, 90)
			assert.Equal(t, s.Len(), 80)
		})
		t.Run("Front", func(t *testing.T) {
			s := intSlice(100)
			ts := s.Copy()
			ts.ResliceBeginning(50)
			assert.Equal(t, ts.Len(), 50)
			assert.NotEqual(t, ts.Index(1), s[1])
		})
		t.Run("Rear", func(t *testing.T) {
			s := intSlice(100)
			ts := s.Copy()
			ts.ResliceEnd(50)
			assert.Equal(t, ts.Len(), 50)

			assert.NotEqual(t, ts.Index(ts.Last()), s[len(s)-1])
		})
	})
	t.Run("Copy", func(t *testing.T) {
		one := randomIntSlice(100)
		two := one.Copy()
		check.EqualItems(t, one, two)
		one[77] = 33
		two[77] = 42
		check.NotEqualItems(t, one, two)
		check.NotEqual(t, one[77], two[77])
		if t.Failed() {
			t.Log("one", one)
			t.Log("two", two)
		}
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
				check.Equal(t, val, s.Index(count))
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
	t.Run("Process", func(t *testing.T) {
		const batchSize = 100

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run("Full", func(t *testing.T) {
			s := randomIntSlice(batchSize)
			count := 0
			worker := s.Process(func(_ context.Context, in int) error {
				count++
				check.NotZero(t, in)
				return nil
			})
			err := worker(ctx)
			assert.Equal(t, count, batchSize)
			assert.NotError(t, err)
		})
		t.Run("SafeAbort", func(t *testing.T) {
			s := randomIntSlice(batchSize)
			count := 0
			worker := s.Process(func(_ context.Context, in int) error {
				count++
				if count > batchSize/2 {
					return io.EOF
				}
				check.NotZero(t, in)
				return nil
			})
			err := worker(ctx)
			check.Equal(t, count, batchSize/2+1)
			check.NotError(t, err)
		})
		t.Run("PropogateError", func(t *testing.T) {
			s := randomIntSlice(batchSize)
			count := 0
			worker := s.Process(func(_ context.Context, in int) error {
				count++
				if count == 64 {
					return ers.ErrLimitExceeded
				}
				check.NotZero(t, in)
				return nil
			})
			err := worker(ctx)
			check.Equal(t, count, 64)
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrLimitExceeded)
		})
	})
	t.Run("When", func(t *testing.T) {
		sl := Slice[int]{}
		t.Run("Add", func(t *testing.T) {
			sl.AddWhen(true, 1)
			check.Equal(t, len(sl), 1)
			sl.AddWhen(false, 100)
			check.Equal(t, len(sl), 1)
		})
		sl.Reset()

		t.Run("Append", func(t *testing.T) {
			sl.AppendWhen(true, 1, 100, 1000)
			check.Equal(t, len(sl), 3)
			sl.AppendWhen(false, 42, 420, 4200)
			check.Equal(t, len(sl), 3)
		})
		sl.Reset()

		t.Run("Extend", func(t *testing.T) {
			sl.ExtendWhen(true, Slice[int]{1, 100, 1000})
			check.Equal(t, len(sl), 3)
			sl.ExtendWhen(false, Slice[int]{42, 420, 4200})
			check.Equal(t, len(sl), 3)
		})
	})
	t.Run("Filter", func(t *testing.T) {
		sl := Slice[int]{100, 100, 40, 42}
		check.Equal(t, sl.Len(), 4)
		next := sl.Filter(func(in int) bool { return in != 100 })
		check.Equal(t, next.Len(), 2)
		check.Equal(t, next[0], 40)
		check.Equal(t, next[1], 42)
	})
	t.Run("FilterFuture", func(t *testing.T) {
		sl := Slice[int]{100, 100, 40, 42}
		check.Equal(t, sl.Len(), 4)
		future := sl.FilterFuture(func(in int) bool { return in != 100 })
		next := future()
		check.Equal(t, next.Len(), 2)
		check.Equal(t, next[0], 40)
		check.Equal(t, next[1], 42)
	})
	t.Run("Transform", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("ContinueEarlEnd", func(t *testing.T) {
			count := 0
			out, err := Transform(randomIntSlice(100),
				fun.ConverterErr(func(in int) (string, error) {
					count++
					if count >= 50 {
						return "", io.EOF
					}
					if count%2 == 0 {
						return "", fun.ErrIteratorSkip
					}
					return fmt.Sprint(in), nil
				})).Resolve(ctx)
			check.NotError(t, err)
			check.Equal(t, count, 50)
			check.Equal(t, len(out), 25)
			check.Equal(t, cap(out), 100)
		})
		t.Run("Basic", func(t *testing.T) {
			count := 0
			out, err := Transform(randomIntSlice(100),
				fun.ConverterErr(func(in int) (string, error) {
					count++
					return fmt.Sprint(in), nil
				})).Resolve(ctx)
			check.NotError(t, err)
			check.Equal(t, count, 100)
			check.Equal(t, len(out), 100)
			check.Equal(t, cap(out), 100)
		})
		t.Run("EarlyError", func(t *testing.T) {
			count := 0
			out, err := Transform(randomIntSlice(100),
				fun.ConverterErr(func(in int) (string, error) {
					if count < 10 {
						count++
						return fmt.Sprint(in), nil
					}
					return "", ers.ErrInvalidInput
				})).Resolve(ctx)
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrInvalidInput)
			check.Equal(t, count, 10)
			check.True(t, out == nil)
		})
	})
	t.Run("Ptr", func(t *testing.T) {
		strs := Slice[int]{100}
		assert.Equal(t, *strs.Ptr(0), 100)
	})
	t.Run("Ptrs", func(t *testing.T) {
		strs := Slice[int]{100, 400, 100}
		ptrs := strs.Ptrs()

		assert.Equal(t, len(strs), len(ptrs))
		for idx := range strs {
			assert.Equal(t, strs[idx], *ptrs[idx])
		}
		assert.NotEqual(t, *ptrs[1], 100)
		strs[1] = 100
		assert.Equal(t, *ptrs[1], 100)
	})
	t.Run("List", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sl := randomIntSlice(128)
		ls, err := NewListFromIterator(ctx, sl.Iterator())

		assert.NotError(t, err)

		exp := ls.Slice()

		check.Equal(t, sl.Len(), ls.Len())
		check.EqualItems(t, sl, exp)
	})
	t.Run("Default", func(t *testing.T) {
		t.Run("EndToEnd", func(t *testing.T) {
			sl := make([]int, 0)
			check.True(t, sl != nil)
			slCp := sl
			check.EqualItems(t, sl, slCp)
			sl = DefaultSlice[int](sl, 12)
			check.True(t, sl != nil)
			check.Equal(t, cap(sl), 0)
			check.EqualItems(t, sl, slCp)
			sl = DefaultSlice[int](nil, 12)

			check.NotEqual(t, cap(sl), cap(slCp))
			check.Equal(t, cap(sl), 12)
		})
		t.Run("Passthrough", func(t *testing.T) {
			sl := DefaultSlice[string]([]string{"hello"}, 11)
			assert.Equal(t, len(sl), 1)
			check.Equal(t, sl[0], "hello")
		})

		t.Run("ZeroLength", func(t *testing.T) {
			sl := DefaultSlice[string](nil)
			check.Equal(t, len(sl), 0)
			check.Equal(t, cap(sl), 0)
		})
		t.Run("LengthOnly", func(t *testing.T) {
			sl := DefaultSlice[string](nil, 32)
			check.Equal(t, len(sl), 32)
			check.Equal(t, cap(sl), 32)
		})
		t.Run("CapOnly", func(t *testing.T) {
			sl := DefaultSlice[string](nil, 0, 32)
			check.Equal(t, len(sl), 0)
			check.Equal(t, cap(sl), 32)
		})
		t.Run("CapAndLen", func(t *testing.T) {
			sl := DefaultSlice[string](nil, 16, 32)
			check.Equal(t, len(sl), 16)
			check.Equal(t, cap(sl), 32)
		})
		t.Run("WeirdIgnored", func(t *testing.T) {
			check.Panic(t, func() {
				_ = DefaultSlice[string](nil, 16, 32, -1)
			})
		})
	})
	t.Run("Constructors", func(t *testing.T) {
		sl := []int8{0, 1, 2, 3, 4, 5, 6, 7}
		t.Run("Ptrs", func(t *testing.T) {
			psl := SlicePtrs(sl)
			for idx := range psl {
				assert.True(t, psl[idx] != nil)
				check.Equal(t, *psl[idx], int8(idx))
				check.Equal(t, sl[idx], *psl[idx])
			}
		})
		t.Run("Ref", func(t *testing.T) {
			t.Run("RoundTrip", func(t *testing.T) {
				rsl := SliceRefs(NewSlice(sl).Ptrs())
				for idx := range rsl {
					check.Equal(t, rsl[idx], sl[idx])
					check.Equal(t, rsl[idx], int8(idx))
				}
			})
			t.Run("NilHandling", func(t *testing.T) {
				pstrs := []*string{nil, nil, nil, ft.Ptr("one"), ft.Ptr("two"), ft.Ptr("three")}
				sl := SliceRefs(pstrs)
				check.Equal(t, len(sl), 6)
				check.Equal(t, sl[0], "")
				check.Equal(t, sl[1], "")
				check.Equal(t, sl[2], "")
				check.Equal(t, sl[3], "one")
				check.Equal(t, sl[4], "two")
				check.Equal(t, sl[5], "three")
			})
		})
		t.Run("Sparse", func(t *testing.T) {
			pstrs := []*string{nil, nil, nil, ft.Ptr("one"), ft.Ptr("two"), ft.Ptr("three")}
			sl := SliceSparseRefs(pstrs)
			check.Equal(t, len(sl), 3)
			check.Equal(t, sl[0], "one")
			check.Equal(t, sl[1], "two")
			check.Equal(t, sl[2], "three")
		})
	})
	t.Run("Prepend", func(t *testing.T) {
		powers := Variadic(100, 1000, 10000)
		powers.Prepend(10)
		check.Equal(t, powers.Len(), 4)
		check.Equal(t, powers.Index(0), 10)
		check.Equal(t, powers.Index(1), 100)
	})
	t.Run("Zero", func(t *testing.T) {
		s := NewSlice([]int{100, 100, 100, 100, 100, 100})
		s.Zero()
		for i := range s {
			check.Zero(t, s[i])
		}
	})
	t.Run("ZeroRange", func(t *testing.T) {
		s := NewSlice([]int{100, 100, 100, 100, 100, 100})
		s.ZeroRange(0, 2)
		check.Equal(t, s[0]+s[1]+s[2], 0)
		check.Equal(t, s[3]+s[4]+s[5], 300)
	})
	t.Run("Merge", func(t *testing.T) {
		sl := MergeSlices(
			Variadic(100, 1000, 10000),
			NewSlice([]int{400, 4000, 40000}),
			Variadic(800, 8000, 80000),
		)
		check.Equal(t, sl.Len(), 9)
		check.Equal(t, sl.Index(0), 100)
		check.Equal(t, sl.Index(3), 400)
		check.Equal(t, sl.Index(6), 800)
	})

}
