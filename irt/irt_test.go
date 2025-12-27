package irt

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/rand/v2"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

func TestCollect(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		args     []int
		expected []int
	}{
		{
			name:     "EmptySequence",
			seq:      func(yield func(int) bool) {},
			expected: []int{},
		},
		{
			name: "SimpleSequence",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			expected: []int{1, 2, 3},
		},
		{
			name: "WithInitialSize",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:     []int{5},
			expected: []int{0, 0, 0, 0, 0, 1, 2},
		},
		{
			name: "WithInitialCapacity",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:     []int{0, 5},
			expected: []int{1, 2},
		},
		{
			name: "WithInitialCapacityAndLength",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:     []int{0, 5},
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(tt.seq, tt.args...)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Collect() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCollect2(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq2[string, int]
		args     []int
		expected map[string]int
	}{
		{
			name:     "EmptySequence",
			seq:      func(yield func(string, int) bool) {},
			expected: map[string]int{},
		},
		{
			name: "SimpleSequence",
			seq: func(yield func(string, int) bool) {
				if !yield("a", 1) {
					return
				}
				yield("b", 2)
			},
			expected: map[string]int{"a": 1, "b": 2},
		},
		{
			name: "WithInitialCapacity",
			seq: func(yield func(string, int) bool) {
				yield("x", 10)
			},
			args:     []int{5},
			expected: map[string]int{"x": 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect2(tt.seq, tt.args...)
			if len(result) != len(tt.expected) {
				t.Errorf("Collect2() length = %v, want %v", len(result), len(tt.expected))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Collect2()[%v] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestCollectFirstN(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		n        int
		expected []int
		maxCalls int
	}{
		{
			name:     "EmptySequence",
			seq:      func(yield func(int) bool) {},
			n:        3,
			expected: []int{},
			maxCalls: 0,
		},
		{
			name: "ExactNElements",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			n:        3,
			expected: []int{1, 2, 3},
			maxCalls: 3,
		},
		{
			name: "MoreThanNElementsShouldStopEarly",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(3) {
					return
				}
				if !yield(4) {
					return
				}
				yield(5)
			},
			n:        2,
			expected: []int{1, 2},
			maxCalls: 2,
		},
		{
			name:     "LessThanNElements",
			seq:      func(yield func(int) bool) { yield(1) },
			n:        3,
			expected: []int{1},
			maxCalls: 1,
		},
		{
			name: "NIsZero",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			n:        0,
			expected: []int{},
			maxCalls: 0,
		},
		{
			name: "NegativeN",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			n:        -1,
			expected: []int{},
			maxCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			wrappedSeq := func(yield func(int) bool) {
				tt.seq(func(v int) bool {
					callCount.Add(1)
					return yield(v)
				})
			}

			result := CollectFirstN(wrappedSeq, tt.n)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("CollectFirstN() = %v, want %v", result, tt.expected)
			}

			if int(callCount.Load()) != tt.maxCalls {
				t.Errorf("CollectFirstN() called sequence %d times, want %d", callCount.Load(), tt.maxCalls)
			}
		})
	}
}

func TestOne(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected []any
	}{
		{"Int", 42, []any{42}},
		{"String", "hello", []any{"hello"}},
		{"Nil", nil, []any{nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := One(tt.value)
			result := Collect(seq)
			if len(result) != 1 || result[0] != tt.value {
				t.Errorf("One() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTwo(t *testing.T) {
	tests := []struct {
		name string
		a, b any
	}{
		{"IntString", 42, "hello"},
		{"StringInt", "world", 123},
		{"NilNil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Two(tt.a, tt.b)
			count := 0
			for a, b := range seq {
				count++
				if a != tt.a || b != tt.b {
					t.Errorf("Two() yielded (%v, %v), want (%v, %v)", a, b, tt.a, tt.b)
				}
			}
			if count != 1 {
				t.Errorf("Two() yielded %d times, want 1", count)
			}
		})
	}
}

func TestMap(t *testing.T) {
	tests := []struct {
		name string
		mp   map[string]int
	}{
		{"Empty", map[string]int{}},
		{"Single", map[string]int{"a": 1}},
		{"Multiple", map[string]int{"a": 1, "b": 2, "c": 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Map(tt.mp)
			result := Collect2(seq)
			if len(result) != len(tt.mp) {
				t.Errorf("Map() length = %v, want %v", len(result), len(tt.mp))
			}
			for k, v := range tt.mp {
				if result[k] != v {
					t.Errorf("Map()[%v] = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestSlice(t *testing.T) {
	tests := []struct {
		name string
		sl   []int
	}{
		{"Empty", []int{}},
		{"Single", []int{1}},
		{"Multiple", []int{1, 2, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Slice(tt.sl)
			result := Collect(seq)
			if !slices.Equal(result, tt.sl) {
				t.Errorf("Slice() = %v, want %v", result, tt.sl)
			}
		})
	}
}

func TestArgs(t *testing.T) {
	tests := []struct {
		name  string
		items []int
	}{
		{"Empty", []int{}},
		{"Single", []int{1}},
		{"Multiple", []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Args(tt.items...)
			result := Collect(seq)
			if !slices.Equal(result, tt.items) {
				t.Errorf("Args() = %v, want %v", result, tt.items)
			}
		})
	}
}

func TestIndex(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[string]
		expected []struct {
			idx int
			val string
		}
	}{
		{
			name: "Empty",
			seq:  func(yield func(string) bool) {},
			expected: []struct {
				idx int
				val string
			}{},
		},
		{
			name: "Single",
			seq:  func(yield func(string) bool) { yield("a") },
			expected: []struct {
				idx int
				val string
			}{{0, "a"}},
		},
		{
			name: "Multiple",
			seq: func(yield func(string) bool) {
				if !yield("a") {
					return
				}
				if !yield("b") {
					return
				}
				yield("c")
			},
			expected: []struct {
				idx int
				val string
			}{{0, "a"}, {1, "b"}, {2, "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Index(tt.seq)
			i := 0
			for idx, val := range seq {
				if i >= len(tt.expected) {
					t.Errorf("Index() yielded more items than expected")
					break
				}
				if idx != tt.expected[i].idx || val != tt.expected[i].val {
					t.Errorf("Index() yielded (%v, %v), want (%v, %v)",
						idx, val, tt.expected[i].idx, tt.expected[i].val)
				}
				i++
			}
			if i != len(tt.expected) {
				t.Errorf("Index() yielded %d items, want %d", i, len(tt.expected))
			}
		})
	}
}

func TestJoinErrors(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[error]
		expected string
	}{
		{
			name:     "empty",
			seq:      func(yield func(error) bool) {},
			expected: "",
		},
		{
			name:     "SingleError",
			seq:      func(yield func(error) bool) { yield(errors.New("error1")) },
			expected: "error1",
		},
		{
			name: "MultipleErrors",
			seq: func(yield func(error) bool) {
				if !yield(errors.New("error1")) {
					return
				}
				yield(errors.New("error2"))
			},
			expected: "error1\nerror2",
		},
		{
			name: "WithNilError",
			seq: func(yield func(error) bool) {
				if !yield(errors.New("error1")) {
					return
				}
				if !yield(nil) {
					return
				}
				yield(errors.New("error2"))
			},
			expected: "error1\nerror2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinErrors(tt.seq)
			if tt.expected == "" {
				if result != nil {
					t.Errorf("JoinErrors() = %v, want nil", result)
				}
			} else {
				if result == nil || result.Error() != tt.expected {
					t.Errorf("JoinErrors() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestFlip(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq2[string, int]
		expected []struct {
			a int
			b string
		}
	}{
		{
			name: "Empty",
			seq:  func(yield func(string, int) bool) {},
			expected: []struct {
				a int
				b string
			}{},
		},
		{
			name: "Single",
			seq:  func(yield func(string, int) bool) { yield("a", 1) },
			expected: []struct {
				a int
				b string
			}{{1, "a"}},
		},
		{
			name: "Multiple",
			seq: func(yield func(string, int) bool) {
				if !yield("a", 1) {
					return
				}
				yield("b", 2)
			},
			expected: []struct {
				a int
				b string
			}{{1, "a"}, {2, "b"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Flip(tt.seq)
			i := 0
			for a, b := range seq {
				if i >= len(tt.expected) {
					t.Errorf("Flip() yielded more items than expected")
					break
				}
				if a != tt.expected[i].a || b != tt.expected[i].b {
					t.Errorf("Flip() yielded (%v, %v), want (%v, %v)",
						a, b, tt.expected[i].a, tt.expected[i].b)
				}
				i++
			}
		})
	}
}

func TestFirst(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq2[string, int]
		expected []string
	}{
		{
			name:     "empty",
			seq:      func(yield func(string, int) bool) {},
			expected: []string{},
		},
		{
			name:     "single",
			seq:      func(yield func(string, int) bool) { yield("a", 1) },
			expected: []string{"a"},
		},
		{
			name: "Multiple",
			seq: func(yield func(string, int) bool) {
				if !yield("a", 1) {
					return
				}
				yield("b", 2)
			},
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := First(tt.seq)
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("First() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSecond(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq2[string, int]
		expected []int
	}{
		{
			name:     "empty",
			seq:      func(yield func(string, int) bool) {},
			expected: []int{},
		},
		{
			name:     "single",
			seq:      func(yield func(string, int) bool) { yield("a", 1) },
			expected: []int{1},
		},
		{
			name: "Multiple",
			seq: func(yield func(string, int) bool) {
				if !yield("a", 1) {
					return
				}
				yield("b", 2)
			},
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Second(tt.seq)
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Second() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPtrs(t *testing.T) {
	tests := []struct {
		name   string
		values []int
	}{
		{"Empty", []int{}},
		{"Single", []int{1}},
		{"Multiple", []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Ptrs(Slice(tt.values))
			result := Collect(seq)
			if len(result) != len(tt.values) {
				t.Errorf("Ptrs() length = %v, want %v", len(result), len(tt.values))
			}
			for i, ptr := range result {
				if ptr == nil || *ptr != tt.values[i] {
					t.Errorf("Ptrs()[%d] = %v, want pointer to %v", i, ptr, tt.values[i])
				}
			}
		})
	}
}

func TestPtrsWithNils(t *testing.T) {
	tests := []struct {
		name     string
		values   []int
		expected []*int
	}{
		{"empty", []int{}, []*int{}},
		{"NoZeros", []int{1, 2, 3}, []*int{ptr(1), ptr(2), ptr(3)}},
		{"WithZeros", []int{1, 0, 3}, []*int{ptr(1), nil, ptr(3)}},
		{"AllZeros", []int{0, 0}, []*int{nil, nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := PtrsWithNils(Slice(tt.values))
			result := Collect(seq)
			if len(result) != len(tt.expected) {
				t.Errorf("PtrsWithNils() length = %v, want %v", len(result), len(tt.expected))
			}
			for i, ptr := range result {
				if tt.expected[i] == nil {
					if ptr != nil {
						t.Errorf("PtrsWithNils()[%d] = %v, want nil", i, ptr)
					}
				} else {
					if ptr == nil || *ptr != *tt.expected[i] {
						t.Errorf("PtrsWithNils()[%d] = %v, want %v", i, ptr, *tt.expected[i])
					}
				}
			}
		})
	}
}

func TestDeref(t *testing.T) {
	tests := []struct {
		name     string
		ptrs     []*int
		expected []int
	}{
		{"Empty", []*int{}, []int{}},
		{"NoNils", []*int{ptr(1), ptr(2)}, []int{1, 2}},
		{"WithNils", []*int{ptr(1), nil, ptr(3)}, []int{1, 3}},
		{"AllNils", []*int{nil, nil}, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Deref(Slice(tt.ptrs))
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Deref() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDerefWithZeros(t *testing.T) {
	tests := []struct {
		name     string
		ptrs     []*int
		expected []int
	}{
		{"Empty", []*int{}, []int{}},
		{"NoNils", []*int{ptr(1), ptr(2)}, []int{1, 2}},
		{"with nils", []*int{ptr(1), nil, ptr(3)}, []int{1, 0, 3}},
		{"all nils", []*int{nil, nil}, []int{0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := DerefWithZeros(Slice(tt.ptrs))
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("DerefWithZeros() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name     string
		gen      func() (int, bool)
		expected []int
	}{
		{
			name:     "EmptyGenerator",
			gen:      func() (int, bool) { return 0, false },
			expected: []int{},
		},
		{
			name: "SingleValue",
			gen: func() func() (int, bool) {
				called := false
				return func() (int, bool) {
					if called {
						return 0, false
					}
					called = true
					return 42, true
				}
			}(),
			expected: []int{42},
		},
		{
			name: "MultipleValues",
			gen: func() func() (int, bool) {
				count := 0
				return func() (int, bool) {
					if count >= 3 {
						return 0, false
					}
					count++
					return count, true
				}
			}(),
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := GenerateOk(tt.gen)
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Generate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerate2(t *testing.T) {
	tests := []struct {
		name     string
		gen      func() (string, int, bool)
		expected []struct {
			a string
			b int
		}
	}{
		{
			name: "empty generator",
			gen:  func() (string, int, bool) { return "", 0, false },
			expected: []struct {
				a string
				b int
			}{},
		},
		{
			name: "SingleValue",
			gen: func() func() (string, int, bool) {
				called := false
				return func() (string, int, bool) {
					if called {
						return "", 0, false
					}
					called = true
					return "a", 1, true
				}
			}(),
			expected: []struct {
				a string
				b int
			}{{"a", 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := GenerateOk2(tt.gen)
			i := 0
			for a, b := range seq {
				if i >= len(tt.expected) {
					t.Errorf("Generate2() yielded more items than expected")
					break
				}
				if a != tt.expected[i].a || b != tt.expected[i].b {
					t.Errorf("Generate2() yielded (%v, %v), want (%v, %v)",
						a, b, tt.expected[i].a, tt.expected[i].b)
				}
				i++
			}
		})
	}
}

func TestGenerateWhile(t *testing.T) {
	tests := []struct {
		name     string
		op       func() int
		while    func(int) bool
		expected []int
		maxCalls int
	}{
		{
			name:     "NeverTrue",
			op:       func() int { return 1 },
			while:    func(int) bool { return false },
			expected: []int{},
			maxCalls: 1,
		},
		{
			name: "LimitedCalls",
			op: func() func() int {
				count := 0
				return func() int {
					count++
					return count
				}
			}(),
			while:    func(v int) bool { return v <= 3 },
			expected: []int{1, 2, 3},
			maxCalls: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			wrappedOp := func() int {
				callCount.Add(1)
				return tt.op()
			}

			seq := GenerateWhile(wrappedOp, tt.while)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("GenerateWhile() = %v, want %v", result, tt.expected)
			}

			if int(callCount.Load()) > tt.maxCalls {
				t.Errorf("GenerateWhile() called op %d times, max expected %d", callCount.Load(), tt.maxCalls)
			}
		})
	}
}

func TestPerpetual(t *testing.T) {
	t.Run("LimitedIterations", func(t *testing.T) {
		var callCount atomic.Int32
		op := func() int {
			return int(callCount.Add(1))
		}

		seq := Generate(op)
		result := CollectFirstN(seq, 3)

		expected := []int{1, 2, 3}
		if !slices.Equal(result, expected) {
			t.Errorf("Perpetual() = %v, want %v", result, expected)
		}

		if callCount.Load() != 3 {
			t.Errorf("Perpetual() called op %d times, want 3", callCount.Load())
		}
	})
	t.Run("Arbitrary", func(t *testing.T) {
		var callCount atomic.Int32
		previous := callCount.Add(1)
		op := func() int32 {
			return callCount.Add(1)
		}

		for val := range Generate(op) {
			if val <= previous {
				t.Errorf("value %d shouldn't be less than previous %d", val, previous)
			}
			if callCount.Load() != val {
				t.Errorf("value %d should be the same as the current call count %d (no buffering)", val, callCount.Load())
			}
			if val > rand.Int32N(100)+1 {
				break
			}
		}
		if callCount.Load() <= 1 {
			t.Error("iteration was not observed")
		}
	})
}

func TestWith(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		op       func(int) string
		expected []struct {
			a int
			b string
		}
	}{
		{
			name: "Empty",
			seq:  func(yield func(int) bool) {},
			op:   func(i int) string { return string(rune('a' + i)) },
			expected: []struct {
				a int
				b string
			}{},
		},
		{
			name: "Single",
			seq:  func(yield func(int) bool) { yield(1) },
			op:   func(i int) string { return string(rune('a' + i)) },
			expected: []struct {
				a int
				b string
			}{{1, "b"}},
		},
		{
			name: "Multiple",
			seq: func(yield func(int) bool) {
				if !yield(0) {
					return
				}
				if !yield(1) {
					return
				}
				yield(2)
			},
			op: func(i int) string { return string(rune('a' + i)) },
			expected: []struct {
				a int
				b string
			}{{0, "a"}, {1, "b"}, {2, "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			wrappedOp := func(i int) string {
				callCount.Add(1)
				return tt.op(i)
			}

			seq := With(tt.seq, wrappedOp)
			i := 0
			for a, b := range seq {
				if i >= len(tt.expected) {
					t.Errorf("With() yielded more items than expected")
					break
				}
				if a != tt.expected[i].a || b != tt.expected[i].b {
					t.Errorf("With() yielded (%v, %v), want (%v, %v)",
						a, b, tt.expected[i].a, tt.expected[i].b)
				}
				i++
			}

			if int(callCount.Load()) != len(tt.expected) {
				t.Errorf("With() called op %d times, want %d", callCount.Load(), len(tt.expected))
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		op       func(int) string
		expected []string
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			op:       func(i int) string { return string(rune('a' + i)) },
			expected: []string{},
		},
		{
			name:     "single",
			seq:      func(yield func(int) bool) { yield(1) },
			op:       func(i int) string { return string(rune('a' + i)) },
			expected: []string{"b"},
		},
		{
			name: "Multiple",
			seq: func(yield func(int) bool) {
				if !yield(0) {
					return
				}
				if !yield(1) {
					return
				}
				yield(2)
			},
			op:       func(i int) string { return string(rune('a' + i)) },
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			wrappedOp := func(i int) string {
				callCount.Add(1)
				return tt.op(i)
			}

			seq := Convert(tt.seq, wrappedOp)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("Convert() = %v, want %v", result, tt.expected)
			}

			if int(callCount.Load()) != len(tt.expected) {
				t.Errorf("Convert() called op %d times, want %d", callCount.Load(), len(tt.expected))
			}
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name          string
		seq           iter.Seq[int]
		expectedCount int
		expectedSum   int
	}{
		{
			name:          "empty",
			seq:           func(yield func(int) bool) {},
			expectedCount: 0,
			expectedSum:   0,
		},
		{
			name:          "single",
			seq:           func(yield func(int) bool) { yield(5) },
			expectedCount: 1,
			expectedSum:   5,
		},
		{
			name: "Multiple",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			expectedCount: 3,
			expectedSum:   6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := 0
			var callCount atomic.Int32
			op := func(i int) {
				callCount.Add(1)
				sum += i
			}

			count := Apply(tt.seq, op)

			if count != tt.expectedCount {
				t.Errorf("Apply() count = %v, want %v", count, tt.expectedCount)
			}
			if sum != tt.expectedSum {
				t.Errorf("Apply() sum = %v, want %v", sum, tt.expectedSum)
			}
			if int(callCount.Load()) != tt.expectedCount {
				t.Errorf("Apply() called op %d times, want %d", callCount.Load(), tt.expectedCount)
			}
		})
	}
}

func TestChannel(t *testing.T) {
	tests := []struct {
		name     string
		values   []int
		expected []int
	}{
		{"Empty", []int{}, []int{}},
		{"Single", []int{1}, []int{1}},
		{"Multiple", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			ch := make(chan int, len(tt.values))
			for _, v := range tt.values {
				ch <- v
			}
			close(ch)

			seq := Channel(ctx, ch)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("Channel() = %v, want %v", result, tt.expected)
			}
		})
	}

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		pctx, pcancel := context.WithCancel(t.Context())
		defer pcancel()

		ch := make(chan int)
		go func() {
			defer close(ch)

			for idx := range 50 {
				select {
				case ch <- idx:
				case <-pctx.Done():
					return
				}
			}
		}()

		seq := Channel(ctx, ch)
		count := 0
		for range seq {
			count++
			if count == 10 {
				cancel() // Cancel after first item
			}
			if count > 11 {
				t.Error("Should have stopped after cancellation")
				break
			}
		}
		pcancel()
		if count > 12 {
			t.Errorf("Expected early termination, got %d items", count)
		}
	})
}

func TestPipe(t *testing.T) {
	tests := []struct {
		name     string
		values   []int
		expected []int
	}{
		{"Empty", []int{}, []int{}},
		{"Single", []int{1}, []int{1}},
		{"Multiple", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()

			seq := Slice(tt.values)
			ch := Pipe(ctx, seq)

			var result []int
			for v := range ch {
				result = append(result, v)
			}

			if !slices.Equal(result, tt.expected) {
				t.Errorf("Pipe() = %v, want %v", result, tt.expected)
			}
		})
	}

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()

		// Create a slow sequence
		seq := func(yield func(int) bool) {
			for i := 1; i <= 100; i++ {
				time.Sleep(5 * time.Millisecond)
				if !yield(i) {
					return
				}
			}
		}

		ch := Pipe(ctx, seq)
		count := 0
		for range ch {
			count++
		}

		// Should terminate early due to context timeout
		if count >= 100 {
			t.Error("Expected early termination due to context cancellation")
		}
	})
}

func TestChunk(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		num      int
		expected [][]int
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			num:      2,
			expected: [][]int{},
		},
		{
			name: "ExactChunks",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(3) {
					return
				}
				yield(4)
			},
			num:      2,
			expected: [][]int{{1, 2}, {3, 4}},
		},
		{
			name: "PartialLastChunk",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			num:      2,
			expected: [][]int{{1, 2}, {3}},
		},
		{
			name: "SingleElementChunks",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			num:      1,
			expected: [][]int{{1}, {2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Chunk(tt.seq, tt.num)
			var result [][]int
			for chunk := range seq {
				if cv := slices.Collect(chunk); len(cv) == 0 {
					continue
				} else {
					result = append(result, cv)
				}
			}
			testt.Log(t, "result", result)
			testt.Log(t, "expected", tt.expected)

			if len(result) != len(tt.expected) {
				t.Errorf("Chunk() length = %v, want %v", len(result), len(tt.expected))
			}
			for i, chunk := range result {
				if i < len(tt.expected) && !slices.Equal(chunk, tt.expected[i]) {
					t.Errorf("Chunk()[%d] = %v, want %v", i, chunk, tt.expected[i])
				}
			}
		})
	}

	t.Run("ZeroChunkSize", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for zero chunk size")
			}
		}()
		seq := Chunk(func(yield func(int) bool) { yield(1) }, 0)
		for range seq {
			break
		}
	})

	t.Run("NegativeChunkSize", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative chunk size")
			}
		}()
		seq := Chunk(func(yield func(int) bool) { yield(1) }, -1)
		for range seq {
			break
		}
	})
}

func TestChain(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[iter.Seq[int]]
		expected []int
	}{
		{
			name:     "empty",
			seq:      func(yield func(iter.Seq[int]) bool) {},
			expected: []int{},
		},
		{
			name: "SingleSequence",
			seq: func(yield func(iter.Seq[int]) bool) {
				yield(func(yield func(int) bool) {
					if !yield(1) {
						return
					}
					yield(2)
				})
			},
			expected: []int{1, 2},
		},
		{
			name: "MultipleSequences",
			seq: func(yield func(iter.Seq[int]) bool) {
				if !yield(func(yield func(int) bool) {
					if !yield(1) {
						return
					}
					yield(2)
				}) {
					return
				}
				yield(func(yield func(int) bool) {
					if !yield(3) {
						return
					}
					yield(4)
				})
			},
			expected: []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Chain(tt.seq)
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Chain() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestChainSlices(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[[]int]
		expected []int
	}{
		{
			name:     "Empty",
			seq:      func(yield func([]int) bool) {},
			expected: []int{},
		},
		{
			name: "EmptySlices",
			seq: func(yield func([]int) bool) {
				if !yield([]int{}) {
					return
				}
				yield([]int{})
			},
			expected: []int{},
		},
		{
			name: "NilSlices",
			seq: func(yield func([]int) bool) {
				if !yield(nil) {
					return
				}
				yield(nil)
			},
			expected: []int{},
		},
		{
			name: "MixedSlices",
			seq: func(yield func([]int) bool) {
				if !yield([]int{1, 2}) {
					return
				}
				if !yield([]int{}) {
					return
				}
				yield([]int{3})
			},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := ChainSlices(tt.seq)
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("ChainSlices() = %v, want %v", result, tt.expected)
			}
		})
	}

	t.Run("EarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		seq := func(yield func([]int) bool) {
			callCount.Add(1)
			if !yield([]int{1, 2}) {
				return
			}
			callCount.Add(1)
			yield([]int{3, 4})
		}

		chained := ChainSlices(seq)
		count := 0
		for range chained {
			count++
			if count == 2 {
				break
			}
		}

		if callCount.Load() != 1 {
			t.Errorf("Should stop after first slice, callCount = %d, want 1", callCount.Load())
		}
	})
}

func TestKeep(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		pred     func(int) bool
		expected []int
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			pred:     func(i int) bool { return i%2 == 0 },
			expected: []int{},
		},
		{
			name: "KeepEvens",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(3) {
					return
				}
				yield(4)
			},
			pred:     func(i int) bool { return i%2 == 0 },
			expected: []int{2, 4},
		},
		{
			name: "KeepAll",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			pred:     func(i int) bool { return true },
			expected: []int{1, 2},
		},
		{
			name: "KeepNone",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			pred:     func(i int) bool { return false },
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			wrappedPred := func(i int) bool {
				callCount.Add(1)
				return tt.pred(i)
			}

			seq := Keep(tt.seq, wrappedPred)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("Keep() = %v, want %v", result, tt.expected)
			}

			// Predicate should be called for each input element
			expectedCalls := len(Collect(tt.seq))
			if int(callCount.Load()) != expectedCalls {
				t.Errorf("Keep() called predicate %d times, want %d", callCount.Load(), expectedCalls)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		pred     func(int) bool
		expected []int
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			pred:     func(i int) bool { return i%2 == 0 },
			expected: []int{},
		},
		{
			name: "RemoveEvens",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(3) {
					return
				}
				yield(4)
			},
			pred:     func(i int) bool { return i%2 == 0 },
			expected: []int{1, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Remove(tt.seq, tt.pred)
			result := Collect(seq)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Remove() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		expected []int
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			expected: []int{},
		},
		{
			name: "NoDuplicates",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			expected: []int{1, 2, 3},
		},
		{
			name: "WithDuplicates",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(1) {
					return
				}
				if !yield(3) {
					return
				}
				yield(2)
			},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Unique(tt.seq)
			result := Collect(seq)
			// Since order might not be preserved, we check length and contents
			if len(result) != len(tt.expected) {
				t.Errorf("Unique() length = %v, want %v", len(result), len(tt.expected))
			}
			for _, expected := range tt.expected {
				if !slices.Contains(result, expected) {
					t.Errorf("Unique() missing %v in result %v", expected, result)
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		target   int
		expected bool
		maxCalls int
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			target:   1,
			expected: false,
			maxCalls: 0,
		},
		{
			name: "FoundEarlyShouldStop",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				if !yield(3) {
					return
				}
				yield(4)
			},
			target:   2,
			expected: true,
			maxCalls: 2,
		},
		{
			name: "NotFoundChecksAll",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			target:   4,
			expected: false,
			maxCalls: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			wrappedSeq := func(yield func(int) bool) {
				tt.seq(func(v int) bool {
					callCount.Add(1)
					return yield(v)
				})
			}

			result := Contains(wrappedSeq, tt.target)
			if result != tt.expected {
				t.Errorf("Contains() = %v, want %v", result, tt.expected)
			}

			if int(callCount.Load()) > tt.maxCalls {
				t.Errorf("Contains() called sequence %d times, max expected %d", callCount.Load(), tt.maxCalls)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		seq1     iter.Seq[int]
		seq2     iter.Seq[int]
		expected bool
	}{
		{
			name:     "both empty",
			seq1:     func(yield func(int) bool) {},
			seq2:     func(yield func(int) bool) {},
			expected: true,
		},
		{
			name: "EqualSequences",
			seq1: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			seq2: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			expected: true,
		},
		{
			name: "DifferentLengths",
			seq1: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			seq2: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			expected: false,
		},
		{
			name: "DifferentValues",
			seq1: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			seq2: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(4)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal(tt.seq1, tt.seq2)
			if result != tt.expected {
				t.Errorf("Equal() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestZip(t *testing.T) {
	tests := []struct {
		name     string
		seq1     iter.Seq[int]
		seq2     iter.Seq[string]
		expected []struct {
			a int
			b string
		}
	}{
		{
			name: "BothEmpty",
			seq1: func(yield func(int) bool) {},
			seq2: func(yield func(string) bool) {},
			expected: []struct {
				a int
				b string
			}{},
		},
		{
			name: "EqualLength",
			seq1: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			seq2: func(yield func(string) bool) {
				if !yield("a") {
					return
				}
				yield("b")
			},
			expected: []struct {
				a int
				b string
			}{{1, "a"}, {2, "b"}},
		},
		{
			name: "FirstShorter",
			seq1: func(yield func(int) bool) { yield(1) },
			seq2: func(yield func(string) bool) {
				if !yield("a") {
					return
				}
				yield("b")
			},
			expected: []struct {
				a int
				b string
			}{{1, "a"}},
		},
		{
			name: "SecondShorter",
			seq1: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			seq2: func(yield func(string) bool) { yield("a") },
			expected: []struct {
				a int
				b string
			}{{1, "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Zip(tt.seq1, tt.seq2)
			i := 0
			for a, b := range seq {
				if i >= len(tt.expected) {
					t.Errorf("Zip() yielded more items than expected")
					break
				}
				if a != tt.expected[i].a || b != tt.expected[i].b {
					t.Errorf("Zip() yielded (%v, %v), want (%v, %v)",
						a, b, tt.expected[i].a, tt.expected[i].b)
				}
				i++
			}
			if i != len(tt.expected) {
				t.Errorf("Zip() yielded %d items, want %d", i, len(tt.expected))
			}
		})
	}
}

func TestGenerateWhile2(t *testing.T) {
	var callCount atomic.Int32
	op := func() (string, int) {
		count := int(callCount.Add(1))
		return "item" + string(rune('0'+count)), count
	}

	seq := GenerateWhile2(op, func(s string, i int) bool { return i <= 3 })
	result := Collect2(seq)

	expected := map[string]int{"item1": 1, "item2": 2, "item3": 3}
	if len(result) != len(expected) {
		t.Errorf("GenerateWhile2() length = %v, want %v", len(result), len(expected))
	}

	if callCount.Load() != 4 { // Called 4 times: 3 successful + 1 that fails condition
		t.Errorf("GenerateWhile2() called op %d times, want 4", callCount.Load())
	}
}

func TestPerpetual2(t *testing.T) {
	var callCount atomic.Int32
	op := func() (string, int) {
		count := int(callCount.Add(1))
		return "item" + string(rune('0'+count)), count
	}

	seq := Generate2(op)
	count := 0
	for a, b := range seq {
		count++
		expectedA := "item" + string(rune('0'+count))
		if a != expectedA || b != count {
			t.Errorf("Perpetual2() yielded (%v, %v), want (%v, %v)", a, b, expectedA, count)
		}
		if count >= 3 {
			break
		}
	}

	if callCount.Load() != 3 {
		t.Errorf("Perpetual2() called op %d times, want 3", callCount.Load())
	}
}

func TestWith2(t *testing.T) {
	t.Run("Claude", func(t *testing.T) {
		var callCount atomic.Int32
		op := func(i int) (string, bool) {
			callCount.Add(1)
			return "item" + string(rune('0'+i)), i%2 == 0
		}

		seq := With2(func(yield func(int) bool) {
			if !yield(1) {
				return
			}
			if !yield(2) {
				return
			}
			yield(3)
		}, op)

		expected := []struct {
			a string
			b bool
		}{
			{"item1", false}, {"item2", true}, {"item3", false},
		}

		i := 0
		for a, b := range seq {
			if i >= len(expected) {
				t.Errorf("With2() yielded more items than expected")
				break
			}
			if a != expected[i].a || b != expected[i].b {
				t.Errorf("With2() yielded (%v, %v), want (%v, %v)", a, b, expected[i].a, expected[i].b)
			}
			i++
		}

		if callCount.Load() != 3 {
			t.Errorf("With2() called op %d times, want 3", callCount.Load())
		}
	})
	t.Run("GPT", func(t *testing.T) {
		t.Run("NormalOperation", func(t *testing.T) {
			input := Slice([]int{1, 2, 3, 4})
			gen := With2(input, func(x int) (int, int) {
				return x * x, x * x * x
			})

			output := Collect2(gen)
			expected := map[int]int{
				1:  1,
				4:  8,
				9:  27,
				16: 64,
			}
			if !maps.Equal(output, expected) {
				t.Errorf("With2() = %v, want %v", output, expected)
			}
		})

		t.Run("EmptyInput", func(t *testing.T) {
			input := Slice([]int{})
			gen := With2(input, func(x int) (int, int) {
				return x * x, x * x * x
			})

			output := Collect2(gen)
			expected := map[int]int{}
			if !maps.Equal(output, expected) {
				t.Errorf("With2() with empty input = %v, want %v", output, expected)
			}
		})

		t.Run("EarlyReturn", func(t *testing.T) {
			input := Slice([]int{1, 2, 3, 4})
			gen := With2(input, func(x int) (int, int) {
				return x * x, x * x * x
			})

			output := Collect2(ElemSplit(Limit(Elems(gen), 2)))

			expected := map[int]int{
				1: 1,
				4: 8,
			}
			if !maps.Equal(output, expected) {
				t.Errorf("With2() with early return = %v, want %v", output, expected)
			}
		})
	})
}

func TestApplyWhile(t *testing.T) {
	sum := 0
	var callCount atomic.Int32

	count := ApplyWhile(
		func(yield func(int) bool) {
			if !yield(1) {
				return
			}
			if !yield(2) {
				return
			}
			if !yield(3) {
				return
			}
			yield(4)
		},
		func(i int) bool {
			callCount.Add(1)
			sum += i
			return i < 3 // Stop at 3
		},
	)

	if count != 3 {
		t.Errorf("ApplyWhile() count = %v, want 3", count)
	}
	if sum != 6 { // 1+2+3
		t.Errorf("ApplyWhile() sum = %v, want 6", sum)
	}
	if callCount.Load() != 3 {
		t.Errorf("ApplyWhile() called op %d times, want 3", callCount.Load())
	}
}

func TestApplyUntil(t *testing.T) {
	t.Run("Claude", func(t *testing.T) {
		var callCount atomic.Int32

		err := ApplyUntil(
			func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				if !yield(2) {
					return
				}
				yield(3)
			},
			func(i int) error {
				callCount.Add(1)
				if i == 2 {
					return errors.New("stop at 2")
				}
				return nil
			},
		)

		if err == nil || err.Error() != "stop at 2" {
			t.Errorf("ApplyUntil() error = %v, want 'stop at 2'", err)
		}
		if callCount.Load() != 2 {
			t.Errorf("ApplyUntil() called op %d times, want 2", callCount.Load())
		}
	})
	t.Run("GPT", func(t *testing.T) {
		t.Run("HappyPath", func(t *testing.T) {
			input := Slice([]int{1, 2, 3, 4, 5})
			sum := 0
			err := ApplyUntil(input, func(v int) error {
				sum += v
				return nil
			})
			if err != nil {
				t.Errorf("ApplyUntil() returned error %v, want nil", err)
			}
			if sum != 15 {
				t.Errorf("ApplyUntil() sum = %d, want 15", sum)
			}
		})

		t.Run("EarlyError", func(t *testing.T) {
			input := Slice([]int{1, 2, 3, 4, 5})
			sum := 0
			err := ApplyUntil(input, func(v int) error {
				sum += v
				if v == 3 {
					return errors.New("early stop")
				}
				return nil
			})
			if err == nil || err.Error() != "early stop" {
				t.Errorf("ApplyUntil() error = %v, want 'early stop'", err)
			}
			if sum != 6 {
				t.Errorf("ApplyUntil() sum = %d, want 6", sum)
			}
		})
	})
}

func TestApplyAll(t *testing.T) {
	var callCount atomic.Int32

	err := ApplyAll(
		func(yield func(int) bool) {
			if !yield(1) {
				return
			}
			if !yield(2) {
				return
			}
			yield(3)
		},
		func(i int) error {
			callCount.Add(1)
			if i == 2 {
				return errors.New("error at 2")
			}
			return nil
		},
	)

	if err == nil {
		t.Error("ApplyAll() should return joined errors")
	}
	if callCount.Load() != 3 {
		t.Errorf("ApplyAll() called op %d times, want 3", callCount.Load())
	}
}

func TestReduce(t *testing.T) {
	result := Reduce(
		func(yield func(int) bool) {
			if !yield(1) {
				return
			}
			if !yield(2) {
				return
			}
			if !yield(3) {
				return
			}
			yield(4)
		},
		func(acc, val int) int { return acc + val },
	)

	if result != 10 {
		t.Errorf("Reduce() = %v, want 10", result)
	}
}

func TestGroupBy(t *testing.T) {
	seq := func(yield func(string) bool) {
		if !yield("apple") {
			return
		}
		if !yield("banana") {
			return
		}
		if !yield("apricot") {
			return
		}
		yield("blueberry")
	}

	result := Collect2(GroupBy(seq, func(s string) rune { return rune(s[0]) }))

	if len(result) != 2 {
		t.Errorf("GroupBy() groups = %v, want 2 groups", len(result))
	}

	aGroup := result['a']
	bGroup := result['b']

	if len(aGroup) != 2 || !slices.Contains(aGroup, "apple") || !slices.Contains(aGroup, "apricot") {
		t.Errorf("GroupBy() 'a' group = %v, want [apple, apricot]", aGroup)
	}

	if len(bGroup) != 2 || !slices.Contains(bGroup, "banana") || !slices.Contains(bGroup, "blueberry") {
		t.Errorf("GroupBy() 'b' group = %v, want [banana, blueberry]", bGroup)
	}
}

func TestGroup(t *testing.T) {
	seq := func(yield func(rune, string) bool) {
		if !yield('a', "apple") {
			return
		}
		if !yield('b', "banana") {
			return
		}
		yield('a', "apricot")
	}

	result := Collect2(Group(seq))

	if len(result) != 2 {
		t.Errorf("Group() groups = %v, want 2 groups", len(result))
	}

	aGroup := result['a']
	bGroup := result['b']

	if len(aGroup) != 2 || !slices.Contains(aGroup, "apple") || !slices.Contains(aGroup, "apricot") {
		t.Errorf("Group() 'a' group = %v, want [apple, apricot]", aGroup)
	}

	if len(bGroup) != 1 || bGroup[0] != "banana" {
		t.Errorf("Group() 'b' group = %v, want [banana]", bGroup)
	}
}

func TestUniqueBy(t *testing.T) {
	seq := func(yield func(string) bool) {
		if !yield("apple") {
			return
		}
		if !yield("banana") {
			return
		}
		if !yield("apricot") {
			return
		}
		yield("blueberry")
	}

	result := Collect(UniqueBy(seq, func(s string) rune { return rune(s[0]) }))

	if len(result) != 2 {
		t.Errorf("UniqueBy() length = %v, want 2", len(result))
	}

	// Should keep first occurrence of each key
	hasA := false
	hasB := false
	for _, s := range result {
		if s[0] == 'a' {
			hasA = true
		}
		if s[0] == 'b' {
			hasB = true
		}
	}

	if !hasA || !hasB {
		t.Errorf("UniqueBy() = %v, should have one item starting with 'a' and one with 'b'", result)
	}
}

func TestEarlyReturnBehavior(t *testing.T) {
	t.Run("CollectEarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		seq := func(yield func(int) bool) {
			for i := 1; i <= 5; i++ {
				callCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}

		// Simulate early return by breaking after 2 items
		count := 0
		for range seq {
			count++
			if count == 2 {
				break
			}
		}

		if callCount.Load() != 2 {
			t.Errorf("Sequence should stop after 2 calls, got %d", callCount.Load())
		}
	})

	t.Run("ConvertEarlyReturn", func(t *testing.T) {
		var sourceCallCount atomic.Int32
		var convertCallCount atomic.Int32

		source := func(yield func(int) bool) {
			for i := 1; i <= 5; i++ {
				sourceCallCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}

		converted := Convert(source, func(i int) string {
			convertCallCount.Add(1)
			return "item" + string(rune('0'+i))
		})

		count := 0
		for range converted {
			count++
			if count == 2 {
				break
			}
		}

		if sourceCallCount.Load() != 2 {
			t.Errorf("Source should be called 2 times, got %d", sourceCallCount.Load())
		}
		if convertCallCount.Load() != 2 {
			t.Errorf("Convert should be called 2 times, got %d", convertCallCount.Load())
		}
	})

	t.Run("KeepEarlyReturn", func(t *testing.T) {
		var sourceCallCount atomic.Int32
		var predicateCallCount atomic.Int32

		source := func(yield func(int) bool) {
			for i := 1; i <= 10; i++ {
				sourceCallCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}

		filtered := Keep(source, func(i int) bool {
			predicateCallCount.Add(1)
			return i%2 == 0 // Keep even numbers
		})

		count := 0
		for range filtered {
			count++
			if count == 2 { // Stop after getting 2 even numbers (2, 4)
				break
			}
		}

		if sourceCallCount.Load() != 4 { // Should process 1,2,3,4
			t.Errorf("Source should be called 4 times, got %d", sourceCallCount.Load())
		}
		if predicateCallCount.Load() != 4 {
			t.Errorf("Predicate should be called 4 times, got %d", predicateCallCount.Load())
		}
	})

	t.Run("ChainEarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32

		seq := func(yield func(iter.Seq[int]) bool) {
			inner1 := func(yield func(int) bool) {
				callCount.Add(1)
				if !yield(1) {
					return
				}
				callCount.Add(1)
				if !yield(2) {
					return
				}
			}
			if !yield(inner1) {
				return
			}

			inner2 := func(yield func(int) bool) {
				callCount.Add(1)
				if !yield(3) {
					return
				}
				callCount.Add(1)
				if !yield(4) {
					return
				}
			}
			yield(inner2)
		}

		chained := Chain(seq)
		count := 0
		for range chained {
			count++
			if count == 2 { // Stop after 2 items
				break
			}
		}

		if callCount.Load() != 2 {
			t.Errorf("Should stop early, callCount = %d, want 2", callCount.Load())
		}
	})

	t.Run("ZipEarlyReturn", func(t *testing.T) {
		var callCount1 atomic.Int32
		var callCount2 atomic.Int32

		seq1 := func(yield func(int) bool) {
			for i := 1; i <= 5; i++ {
				callCount1.Add(1)
				if !yield(i) {
					return
				}
			}
		}

		seq2 := func(yield func(string) bool) {
			for i := 1; i <= 5; i++ {
				callCount2.Add(1)
				if !yield("item" + string(rune('0'+i))) {
					return
				}
			}
		}

		zipped := Zip(seq1, seq2)
		count := 0
		for range zipped {
			count++
			if count == 2 {
				break
			}
		}

		if callCount1.Load() != 2 {
			t.Errorf("Seq1 should be called 2 times, got %d", callCount1.Load())
		}
		if callCount2.Load() != 2 {
			t.Errorf("Seq2 should be called 2 times, got %d", callCount2.Load())
		}
	})

	t.Run("RemoveZerosEarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		seq := func(yield func(int) bool) {
			for i := 0; i < 10; i++ {
				callCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}
		// 0 is removed. 1, 2 are kept.
		// We stop after 2 kept items.
		count := 0
		for range RemoveZeros(seq) {
			count++
			if count == 2 {
				break
			}
		}
		// items processed: 0 (removed), 1 (kept), 2 (kept).
		if callCount.Load() != 3 {
			t.Errorf("Source should be called 3 times, got %d", callCount.Load())
		}
	})

	t.Run("RemoveErrorsEarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		err := errors.New("error")
		seq := func(yield func(int, error) bool) {
			for i := 1; i <= 10; i++ {
				callCount.Add(1)
				e := error(nil)
				if i%2 == 0 {
					e = err
				}
				if !yield(i, e) {
					return
				}
			}
		}
		// 1 (nil err), 2 (err), 3 (nil err), 4 (err), 5 (nil err)
		// RemoveErrors keeps: 1, 3, 5...
		// We stop after 2 kept items (1 and 3).
		count := 0
		for range RemoveErrors(seq) {
			count++
			if count == 2 {
				break
			}
		}
		// items processed: 1 (kept), 2 (removed), 3 (kept).
		if callCount.Load() != 3 {
			t.Errorf("Source should be called 3 times, got %d", callCount.Load())
		}
	})

	t.Run("RemoveNilsEarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		seq := func(yield func(*int) bool) {
			for i := 1; i <= 10; i++ {
				callCount.Add(1)
				var p *int
				if i%2 != 0 {
					p = ptr(i)
				}
				if !yield(p) {
					return
				}
			}
		}
		// 1 (ptr), 2 (nil), 3 (ptr), 4 (nil), 5 (ptr)
		// RemoveNils keeps: 1, 3, 5...
		// We stop after 2 kept items (1 and 3).
		count := 0
		for range RemoveNils(seq) {
			count++
			if count == 2 {
				break
			}
		}
		// items processed: 1 (kept), 2 (removed), 3 (kept).
		if callCount.Load() != 3 {
			t.Errorf("Source should be called 3 times, got %d", callCount.Load())
		}
	})
}

// Additional tests for Shard and ShardByHash

func TestShard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workload := Slice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	numShards := 3

	shards := Shard(ctx, numShards, workload)

	// Ensure each shard contains a portion of the workload
	totalItems := 0
	for shard := range shards {
		shardItems := Collect(shard)
		totalItems += len(shardItems)
		t.Logf("Shard: %v", shardItems)
	}
	if totalItems != 10 {
		t.Errorf("Total items across shards = %d, want 10", totalItems)
	}

	t.Run("SupportEarlyReturn", func(t *testing.T) {
		// Ensure early return does not block other shards
		shards = Shard(ctx, numShards, workload)
		seen := 0
		for shard := range shards {
			seen++
			if seen == 2 {
				break // Simulate early return
			}

			count := 0
			for value := range shard {
				count++
				if value == 0 {
					t.Error("saw zero value")
				}
			}
			if count == 0 {
				t.Error("should have seen more than one", seen, count)
			}
		}
		if seen == 0 {
			t.Error("should have seen more than one", seen)
		}
	})
}

func TestFirstValue(t *testing.T) {
	t.Run("Smoke", func(t *testing.T) {
		// Ensure the iterator is only iterated once
		iterated := 0
		input := func(yield func(int) bool) {
			for _, v := range []int{10, 20, 30} {
				iterated++
				if !yield(v) {
					return
				}
			}
		}

		// Call FirstValue
		first, ok := FirstValue(input)

		// Assertions
		if !ok {
			t.Fatalf("FirstValue() returned ok = false, want true")
		}
		if first != 10 {
			t.Errorf("FirstValue() = %v, want %v", first, 10)
		}
		if iterated != 1 {
			t.Errorf("Iterator was iterated %d times, want 1", iterated)
		}
	})
	t.Run("Empty", func(t *testing.T) {
		val, ok := FirstValue(func(yield func(int) bool) {})
		if ok {
			t.Error("unexpected true ok")
		}
		if val != 0 {
			t.Error("unexpected value", val)
		}
	})
}

func TestFirstValue2(t *testing.T) {
	t.Run("Smoke", func(t *testing.T) {
		// Ensure the iterator is only iterated once and returns the first pair
		iterated := 0
		input := func(yield func(int, string) bool) {
			pairs := []struct {
				k int
				v string
			}{{10, "a"}, {20, "b"}, {30, "c"}}
			for _, p := range pairs {
				iterated++
				if !yield(p.k, p.v) {
					return
				}
			}
		}

		k, v, ok := FirstValue2(input)

		if !ok {
			t.Fatalf("FirstValue2() returned ok = false, want true")
		}
		if k != 10 || v != "a" {
			t.Errorf("FirstValue2() = (%v, %v), want (10, a)", k, v)
		}
		if iterated != 1 {
			t.Errorf("Iterator was iterated %d times, want 1", iterated)
		}
	})
	t.Run("Empty", func(t *testing.T) {
		k, v, ok := FirstValue2(func(yield func(string, float64) bool) {})
		if ok {
			t.Error("unexpected true ok for empty sequence")
		}
		if k != "" || v != 0.0 {
			t.Errorf("unexpected values for empty sequence: (%v, %v)", k, v)
		}
	})
	t.Run("Single", func(t *testing.T) {
		k, v, ok := FirstValue2(Two("hello", 42))
		if !ok {
			t.Fatal("expected ok")
		}
		if k != "hello" || v != 42 {
			t.Errorf("got (%v, %v), want (hello, 42)", k, v)
		}
	})
	t.Run("Types", func(t *testing.T) {
		type testStruct struct{ val int }
		s := testStruct{val: 100}
		k, v, ok := FirstValue2(Two(s, true))
		if !ok || k.val != 100 || v != true {
			t.Errorf("got (%v, %v, %v), want ({100}, true, true)", k, v, ok)
		}
	})
}

func TestHead(t *testing.T) {
	input := Slice([]int{1, 2, 3, 4, 5})
	output := Collect(Limit(input, 3))

	expected := []int{1, 2, 3}

	if len(output) != len(expected) {
		t.Log("output", output)
		t.Log("expected", expected)
		t.Fatal()
	}
	for idx := range output {
		if expected[idx] != output[idx] {
			t.Errorf("at index %d, output %d is not equal to expected %d", idx, expected[idx], output[idx])
		}
	}
}

func TestRange(t *testing.T) {
	t.Run("Smoke", func(t *testing.T) {
		output := Collect(Range(5, 10))

		expected := []int{5, 6, 7, 8, 9, 10}
		if len(output) != len(expected) {
			t.Log("output", output)
			t.Log("expected", expected)
			t.Fatal()
		}
		for idx := range output {
			if expected[idx] != output[idx] {
				t.Errorf("at index %d, output %d is not equal to expected %d", idx, expected[idx], output[idx])
			}
		}
	})

	t.Run("Graceful", func(t *testing.T) {
		cases := [][]int{
			{100, 0, 0},
			{-100, -100, 1},
			{100, -100, 0},
			{-100, 100, 201},
			{-3, 0, 4},
			{-3, -8, 0},
		}

		for tc := range Slice(cases) {
			first, second, size := tc[0], tc[1], tc[2]
			t.Run(fmt.Sprintf("From_%d_To_%d", first, second), func(t *testing.T) {
				if output := Collect(Range(first, second)); len(output) != size {
					t.Errorf("Range(%d, %d) -> %d (not %d)", first, second, len(output), size)
				}
			})
		}
	})
}

func TestGenerateN(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		count := 0
		gen := GenerateN(5, func() int {
			count++
			return count
		})

		output := Collect(gen)
		expected := []int{1, 2, 3, 4, 5}
		if !slices.Equal(output, expected) {
			t.Errorf("GenerateN() = %v, want %v", output, expected)
		}

		if count != 5 {
			t.Errorf("GenerateN() called generator %d times, want 5", count)
		}
	})

	t.Run("ZeroIterations", func(t *testing.T) {
		count := 0
		gen := GenerateN(0, func() int {
			count++
			return count
		})

		output := Collect(gen)
		expected := []int{}
		if !slices.Equal(output, expected) {
			t.Errorf("GenerateN() = %v, want %v", output, expected)
		}

		if count != 0 {
			t.Errorf("GenerateN() called generator %d times, want 0", count)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		count := 0
		gen := GenerateN(5, func() int {
			count++
			return count
		})

		output := Collect(Limit(gen, 2))
		expected := []int{1, 2}
		if !slices.Equal(output, expected) {
			t.Errorf("GenerateN() with early return = %v, want %v", output, expected)
		}

		if count != 2 {
			t.Errorf("GenerateN() called generator %d times, want 2", count)
		}
	})
}

func TestMonotonicFrom(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		gen := Limit(MonotonicFrom(1), 5)
		output := Collect(gen)
		expected := []int{1, 2, 3, 4, 5}
		if !slices.Equal(output, expected) {
			t.Errorf("Monotonic() = %v, want %v", output, expected)
		}
	})

	t.Run("ZeroIterations", func(t *testing.T) {
		gen := Limit(MonotonicFrom(1), 0)
		output := Collect(gen)
		expected := []int{}
		if !slices.Equal(output, expected) {
			t.Errorf("Monotonic() with no iteration = %v, want %v", output, expected)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		gen := Limit(MonotonicFrom(10), 3)
		output := Collect(gen)
		expected := []int{10, 11, 12}
		if !slices.Equal(output, expected) {
			t.Errorf("Monotonic() with early return = %v, want %v", output, expected)
		}
	})
}

func TestMonotonic(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		gen := Limit(Monotonic(), 5)
		output := Collect(gen)
		expected := []int{1, 2, 3, 4, 5}
		if !slices.Equal(output, expected) {
			t.Errorf("Monotonic() = %v, want %v", output, expected)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		gen := Limit(Monotonic(), 3)
		output := Collect(gen)
		expected := []int{1, 2, 3}
		if !slices.Equal(output, expected) {
			t.Errorf("Monotonic() with early return = %v, want %v", output, expected)
		}
	})
}

func TestApplyWhile2(t *testing.T) {
	t.Run("HappyPath", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		count := 0
		ApplyWhile2(input, func(k string, v int) bool {
			count++
			return true
		})
		if count != 3 {
			t.Errorf("ApplyWhile2() count = %d, want 3", count)
		}
	})

	t.Run("EarlyStop", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		count := 0
		ApplyWhile2(input, func(k string, v int) bool {
			count++
			return v != 2
		})
		if count < 1 {
			t.Errorf("ApplyWhile2() count = %d, want 2", count)
		}
	})
}

func TestApplyUnless2(t *testing.T) {
	t.Run("HappyPath", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		count := 0
		ApplyUnless2(input, func(k string, v int) bool {
			count++
			return false
		})
		if count != 3 {
			t.Errorf("ApplyUnless2() count = %d, want 3", count)
		}
	})

	t.Run("EarlyStop", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		count := 0
		ApplyUnless2(input, func(k string, v int) bool {
			count++
			return v == 2
		})
		if count < 1 {
			t.Errorf("ApplyUnless2() count = %d, want 2", count)
		}
	})
}

func TestApplyUntil2(t *testing.T) {
	t.Run("HappyPath", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		count := 0
		err := ApplyUntil2(input, func(k string, v int) error {
			count++
			return nil
		})
		if err != nil {
			t.Errorf("ApplyUntil2() returned error %v, want nil", err)
		}
		if count != 3 {
			t.Errorf("ApplyUntil2() count = %d, want 3", count)
		}
	})

	t.Run("EarlyError", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		count := 0
		err := ApplyUntil2(input, func(k string, v int) error {
			count++
			if v == 2 {
				return errors.New("early stop")
			}
			return nil
		})
		if err == nil || err.Error() != "early stop" {
			t.Errorf("ApplyUntil2() error = %v, want 'early stop'", err)
		}
		if count < 1 {
			t.Errorf("ApplyUntil2() count = %d", count)
		}
	})
}

func TestApplyAll2(t *testing.T) {
	t.Run("HappyPath", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		err := ApplyAll2(input, func(k string, v int) error {
			return nil
		})
		if err != nil {
			t.Errorf("ApplyAll2() returned error %v, want nil", err)
		}
	})

	t.Run("ErrorInAll", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		err := ApplyAll2(input, func(k string, v int) error {
			if v == 2 {
				return errors.New("error at 2")
			}
			return nil
		})
		if err == nil || err.Error() != "error at 2" {
			t.Errorf("ApplyAll2() error = %v, want 'error at 2'", err)
		}
	})
}

func TestUntilNil(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		input := Slice([]*int{ptr(1), ptr(2), ptr(3), nil, ptr(4)})
		output := Collect(UntilNil(input))
		expected := []int{1, 2, 3}
		if !slices.Equal(output, expected) {
			t.Errorf("UntilNil() = %v, want %v", output, expected)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		input := Slice([]*int{ptr(1), ptr(2), nil, ptr(3)})
		output := Collect(UntilNil(input))
		expected := []int{1, 2}
		if !slices.Equal(output, expected) {
			t.Errorf("UntilNil() with early return = %v, want %v", output, expected)
		}
	})

	t.Run("ZeroValueHandling", func(t *testing.T) {
		input := Slice([]*int{})
		output := Collect(UntilNil(input))
		expected := []int{}
		if !slices.Equal(output, expected) {
			t.Errorf("UntilNil() with zero value = %v, want %v", output, expected)
		}
	})
}

func TestUntilError(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		input := Args(1, 2, 3, 4)
		seq := With2(input, func(v int) (int, error) {
			if v == 4 {
				return 0, errors.New("stop")
			}
			return v, nil
		})
		output := Collect(UntilError(seq))
		expected := []int{1, 2, 3}
		if !slices.Equal(output, expected) {
			t.Errorf("UntilError() = %v, want %v", output, expected)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		input := Args(1, 2, 3)
		seq := With2(input, func(v int) (int, error) {
			if v == 3 {
				return 0, errors.New("stop")
			}
			return v, nil
		})
		output := Collect(UntilError(seq))
		expected := []int{1, 2}
		if !slices.Equal(output, expected) {
			t.Errorf("UntilError() with early return = %v, want %v", output, expected)
		}
	})

	t.Run("ZeroValueHandling", func(t *testing.T) {
		input := Args[int]()
		seq := With2(input, func(v int) (int, error) {
			return v, nil
		})
		output := Collect(UntilError(seq))
		expected := []int{}
		if !slices.Equal(output, expected) {
			t.Errorf("UntilError() with zero value = %v, want %v", output, expected)
		}
	})
}

func TestUntil(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		input := Slice([]int{1, 2, 3, 4, 5})
		output := Collect(Until(input, func(v int) bool {
			return v > 3
		}))
		expected := []int{1, 2, 3}
		if !slices.Equal(output, expected) {
			t.Errorf("Until() = %v, want %v", output, expected)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		input := Slice([]int{1, 2, 3, 4, 5})
		output := Collect(Until(input, func(v int) bool {
			return v == 3
		}))
		expected := []int{1, 2}
		if !slices.Equal(output, expected) {
			t.Errorf("Until() with early return = %v, want %v", output, expected)
		}
	})

	t.Run("ZeroValueHandling", func(t *testing.T) {
		input := Slice([]int{})
		output := Collect(Until(input, func(v int) bool {
			return v > 0
		}))
		expected := []int{}
		if !slices.Equal(output, expected) {
			t.Errorf("Until() with zero value = %v, want %v", output, expected)
		}
	})
}

func TestUntil2(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		input := Collect(Elems(Map(map[string]int{"a": 1, "b": 2, "c": 3, "d": 4})))
		slices.SortFunc(input, ElemCmp)

		output := maps.Collect(Until2(ElemSplit(Slice(input)), func(k string, v int) bool {
			return v > 2
		}))

		_, notC := output["c"]
		_, notD := output["d"]
		if notC || notD {
			t.Errorf("Until2() = %v, want all values should have been greater than 2", output)
		}
		_, hasA := output["a"]
		_, hasB := output["b"]
		if !hasA && !hasB {
			t.Error("Util2 should have seen one valid value, instead:", output)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		output := Collect2(Until2(input, func(k string, v int) bool {
			return v == 2
		}))
		if _, ok := output["b"]; ok {
			t.Errorf("Until2() with early return = %v, unexpected 'b' value", output)
		}
	})

	t.Run("ZeroValueHandling", func(t *testing.T) {
		input := Map(map[string]int{})
		output := Collect2(Until2(input, func(k string, v int) bool {
			return v > 0
		}))
		expected := map[string]int{}
		if !maps.Equal(output, expected) {
			t.Errorf("Until2() with zero value = %v, want %v", output, expected)
		}
	})
}

func TestLimit(t *testing.T) {
	t.Run("Positive", func(t *testing.T) {
		input := Slice([]int{1, 2, 3, 4, 5})
		output := Collect(Limit(input, 3))
		expected := []int{1, 2, 3}
		if !slices.Equal(output, expected) {
			t.Errorf("Limit() = %v, want %v", output, expected)
		}
	})
	t.Run("Zero", func(t *testing.T) {
		input := Slice([]int{1, 2, 3})
		output := Collect(Limit(input, 0))
		if len(output) != 0 {
			t.Errorf("Limit(0) produced %d items", len(output))
		}
	})
	t.Run("Negative", func(t *testing.T) {
		input := Slice([]int{1, 2, 3})
		output := Collect(Limit(input, -1))
		if len(output) != 0 {
			t.Errorf("Limit(-1) produced %d items", len(output))
		}
	})
	t.Run("MoreThanAvailable", func(t *testing.T) {
		input := Slice([]int{1, 2})
		output := Collect(Limit(input, 5))
		expected := []int{1, 2}
		if !slices.Equal(output, expected) {
			t.Errorf("Limit() = %v, want %v", output, expected)
		}
	})
}

func TestLimit2(t *testing.T) {
	t.Run("Positive", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2, "c": 3})
		output := Collect2(Limit2(input, 2))
		if len(output) != 2 {
			t.Errorf("Limit2(2) produced %d items, want 2", len(output))
		}
	})
	t.Run("Zero", func(t *testing.T) {
		input := Map(map[string]int{"a": 1})
		output := Collect2(Limit2(input, 0))
		if len(output) != 0 {
			t.Errorf("Limit2(0) produced %d items", len(output))
		}
	})
}

func TestConvert2(t *testing.T) {
	input := Map(map[string]int{"a": 1, "b": 2})
	output := Collect2(Convert2(input, func(k string, v int) (string, int) {
		return k + "!", v * 10
	}))
	expected := map[string]int{"a!": 10, "b!": 20}
	if !maps.Equal(output, expected) {
		t.Errorf("Convert2() = %v, want %v", output, expected)
	}
}

func TestMerge(t *testing.T) {
	input := Map(map[string]int{"a": 1, "b": 2})
	output := Collect(Merge(input, func(k string, v int) string {
		return fmt.Sprintf("%s:%d", k, v)
	}))
	slices.Sort(output)
	expected := []string{"a:1", "b:2"}
	if !slices.Equal(output, expected) {
		t.Errorf("Merge() = %v, want %v", output, expected)
	}
}

func TestRemoveZeros(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		expected []int
	}{
		{
			name:     "Empty",
			seq:      Args[int](),
			expected: []int{},
		},
		{
			name:     "NoZeros",
			seq:      Args(1, 2, 3),
			expected: []int{1, 2, 3},
		},
		{
			name:     "WithZeros",
			seq:      Args(1, 0, 2, 0, 3),
			expected: []int{1, 2, 3},
		},
		{
			name:     "AllZeros",
			seq:      Args(0, 0, 0),
			expected: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(RemoveZeros(tt.seq))
			if !slices.Equal(result, tt.expected) {
				t.Errorf("RemoveZeros() = %v, want %v", result, tt.expected)
			}
		})
	}
	t.Run("Strings", func(t *testing.T) {
		seq := Args("a", "", "b", "")
		result := Collect(RemoveZeros(seq))
		expected := []string{"a", "b"}
		if !slices.Equal(result, expected) {
			t.Errorf("RemoveZeros() = %v, want %v", result, expected)
		}
	})
}

func TestRemoveErrors(t *testing.T) {
	err := errors.New("error")
	tests := []struct {
		name     string
		seq      iter.Seq2[int, error]
		expected []int
	}{
		{
			name:     "Empty",
			seq:      func(yield func(int, error) bool) {},
			expected: []int{},
		},
		{
			name: "NoErrors",
			seq: func(yield func(int, error) bool) {
				yield(1, nil)
				yield(2, nil)
			},
			expected: []int{1, 2},
		},
		{
			name: "WithErrors",
			seq: func(yield func(int, error) bool) {
				yield(1, nil)
				yield(2, err)
				yield(3, nil)
			},
			expected: []int{1, 3},
		},
		{
			name: "AllErrors",
			seq: func(yield func(int, error) bool) {
				yield(1, err)
				yield(2, err)
			},
			expected: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(RemoveErrors(tt.seq))
			if !slices.Equal(result, tt.expected) {
				t.Errorf("RemoveErrors() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRemoveNils(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[*int]
		expected []*int
	}{
		{
			name:     "Empty",
			seq:      Args[*int](),
			expected: []*int{},
		},
		{
			name:     "NoNils",
			seq:      Args(ptr(1), ptr(2)),
			expected: []*int{ptr(1), ptr(2)},
		},
		{
			name:     "WithNils",
			seq:      Args(ptr(1), nil, ptr(2)),
			expected: []*int{ptr(1), ptr(2)},
		},
		{
			name:     "AllNils",
			seq:      Args[*int](nil, nil),
			expected: []*int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(RemoveNils(tt.seq))
			if len(result) != len(tt.expected) {
				t.Errorf("RemoveNils() length = %v, want %v", len(result), len(tt.expected))
			}
			for i := range result {
				if *result[i] != *tt.expected[i] {
					t.Errorf("RemoveNils()[%d] = %v, want %v", i, *result[i], *tt.expected[i])
				}
			}
		})
	}
}
