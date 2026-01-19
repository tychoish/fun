package irt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// PullWithMutex wraps iter.Pull and returns thread-safe next and stop functions.
// The mutex protects all calls to next(), allowing multiple goroutines to safely
// pull values from the same iterator.
func PullWithMutex[T any](seq iter.Seq[T], mu *sync.Mutex) (next func() (T, bool), stop func()) {
	next, stop = iter.Pull(seq)
	next = mtxdo2(mu, next)
	stop = mtxcall(mu, stop)

	return next, stop
}

func TestCollect(t *testing.T) {
	tests := []struct {
		name             string
		seq              iter.Seq[int]
		args             []int
		expected         []int
		expectedCapacity int
		panics           bool
	}{
		{
			name:             "EmptySequence",
			seq:              func(yield func(int) bool) {},
			expected:         []int{},
			expectedCapacity: 0,
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
			expected:         []int{1, 2, 3},
			expectedCapacity: 4,
		},
		{
			name: "WithInitialCapacity/First",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:             []int{5, 0},
			expected:         []int{1, 2},
			expectedCapacity: 5,
		},
		{
			name: "WithInitialCapacity/Second",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:             []int{0, 5},
			expected:         []int{1, 2},
			expectedCapacity: 5,
		},
		{
			name: "WithInitialCapacity/many",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:             []int{0, 5, 0, 0, 0},
			expected:         []int{1, 2},
			expectedCapacity: 2,
		},
		{
			name: "WithInitialCapacity/Negative",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:             []int{0, -55, 0, 0, 0},
			expected:         []int{1, 2},
			expectedCapacity: 2,
		},
		{
			name: "WithInitialCapacityAndNoData/Negative",
			seq: func(yield func(int) bool) {
				return
			},
			args:             []int{0, -55, 0, 0, 0},
			expected:         []int{},
			expectedCapacity: 0,
		},
		{
			name: "WithInitialCapacityAndLength",
			seq: func(yield func(int) bool) {
				if !yield(1) {
					return
				}
				yield(2)
			},
			args:             []int{0, 5},
			expected:         []int{1, 2},
			expectedCapacity: 5,
		},
		{
			name:   "Panics/NilSeq",
			seq:    nil,
			args:   []int{0, 5},
			panics: true,
		},
		{
			name:   "Panics/Two",
			seq:    func(func(int) bool) {},
			args:   []int{1, 1},
			panics: true,
		},
		{
			name:   "Panics/Three",
			seq:    func(func(int) bool) {},
			args:   []int{1, 1, 0},
			panics: true,
		},
		{
			name:             "NotPanics",
			seq:              func(func(int) bool) {},
			args:             []int{1},
			expectedCapacity: 1,
			panics:           false,
		},
	}

	for _, tt := range tests {
		if tt.panics {
			t.Run(tt.name, func(t *testing.T) {
				defer func() {
					p := recover()
					if p == nil {
						t.Error("expected panic")
					}
				}()
				_ = Collect(tt.seq, tt.args...)
			})

			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(tt.seq, tt.args...)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Collect() = %v, want %v", result, tt.expected)
			}
			if cap(result) != tt.expectedCapacity {
				t.Errorf("saw capcity of %d not %d", cap(result), tt.expectedCapacity)
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
	t.Run("LazyExecution", func(t *testing.T) {
		var sourceCount atomic.Int32
		var opCount atomic.Int32
		source := func(yield func(int) bool) {
			for i := 1; i <= 10; i++ {
				sourceCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}
		seq := With(source, func(i int) int {
			opCount.Add(1)
			return i * 2
		})
		next, stop := iter.Pull2(seq)
		defer stop()

		if sourceCount.Load() != 0 || opCount.Load() != 0 {
			t.Errorf("With should be lazy: sourceCount=%d, opCount=%d", sourceCount.Load(), opCount.Load())
		}
		next()
		if sourceCount.Load() != 1 || opCount.Load() != 1 {
			t.Errorf("With should execute on demand: sourceCount=%d, opCount=%d", sourceCount.Load(), opCount.Load())
		}
	})
}

func TestWithEach(t *testing.T) {
	t.Run("NormalOperation", func(t *testing.T) {
		var callCount atomic.Int32
		source := Slice([]int{1, 2, 3})
		op := func() string {
			callCount.Add(1)
			return "fixed"
		}
		seq := WithEach(source, op)
		result := Collect2(seq)
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
		if callCount.Load() != 3 {
			t.Errorf("expected 3 calls, got %d", callCount.Load())
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		source := Slice([]int{1, 2, 3, 4, 5})
		op := func() string {
			callCount.Add(1)
			return "fixed"
		}
		seq := WithEach(source, op)
		count := 0
		for range seq {
			count++
			if count == 2 {
				break
			}
		}
		if callCount.Load() != 2 {
			t.Errorf("expected 2 calls, got %d", callCount.Load())
		}
	})
	t.Run("LazyExecution", func(t *testing.T) {
		var sourceCount atomic.Int32
		var opCount atomic.Int32
		source := func(yield func(int) bool) {
			for i := 1; i <= 10; i++ {
				sourceCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}
		seq := WithEach(source, func() int {
			opCount.Add(1)
			return 42
		})
		next, stop := iter.Pull2(seq)
		defer stop()

		if sourceCount.Load() != 0 || opCount.Load() != 0 {
			t.Errorf("WithEach should be lazy: sourceCount=%d, opCount=%d", sourceCount.Load(), opCount.Load())
		}
		next()
		if sourceCount.Load() != 1 || opCount.Load() != 1 {
			t.Errorf("WithEach should execute on demand: sourceCount=%d, opCount=%d", sourceCount.Load(), opCount.Load())
		}
	})
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
	t.Run("RunAll", func(t *testing.T) {
		t.Run("Nil", func(t *testing.T) {
			defer func() {
				if p := recover(); p == nil {
					t.Error("expectedPanic")
				}
			}()
			if ct := RunAll(Args[func()](nil, nil)); ct != 0 {
				t.Errorf("saw %d", ct)
			}
		})

		t.Run("Empty", func(t *testing.T) {
			if ct := RunAll(Args(func() {}, func() {})); ct != 2 {
				t.Errorf("saw %d", ct)
			}
		})

		t.Run("Observe", func(t *testing.T) {
			called := 0
			if ct := RunAll(Args(func() { called++ }, func() { called++ })); ct != 2 || called != 2 {
				t.Errorf("saw %d, with effect of %d", ct, called)
			}
		})
	})
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
			t.Cleanup(func() {
				if t.Failed() {
					return
				}
				t.Log(t, "result", result)
				t.Log(t, "expected", tt.expected)
			})

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
		seq := Chunk(func(yield func(int) bool) { yield(1) }, 0)
		count := 0
		for range seq {
			count++
			break
		}
		if count != 0 {
			t.Error("should not have iterated", count)
		}
	})

	t.Run("NegativeChunkSize", func(t *testing.T) {
		count := 0
		seq := Chunk(func(yield func(int) bool) { yield(1) }, -1)
		for range seq {
			count++
			break
		}
		if count != 0 {
			t.Error("should not have iterated", count)
		}
	})
	t.Run("EarlyReaturn", func(t *testing.T) {
		count := 0
		chunks := 0
		for chunk := range Chunk(GenerateN(30, func() int { count++; return count }), 3) {
			chunks++
			innerct := 0
			for item := range chunk {
				if item == 0 {
					t.Fatal("impossible")
				}
				innerct++
			}
			if chunks == 2 {
				break
			}

			if innerct == 0 {
				t.Error("should iterate", innerct, chunks, count)
			}
			if count%3 != 0 {
				t.Error("impossible chunk size", innerct, chunks, count)
			}
		}
		if count != 6 {
			t.Error("unexpected iteration", count)
		}
		if chunks != 2 {
			t.Error("unexpected chunk iteration", count)
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

func TestAppend(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		with     []int
		expected []int
	}{
		{
			name:     "EmptySequenceEmptyAppend",
			seq:      func(yield func(int) bool) {},
			with:     []int{},
			expected: []int{},
		},
		{
			name:     "EmptySequenceWithValues",
			seq:      func(yield func(int) bool) {},
			with:     []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name: "SequenceWithEmptyAppend",
			seq: func(yield func(int) bool) {
				_ = yield(1) && yield(2) && yield(3)
			},
			with:     []int{},
			expected: []int{1, 2, 3},
		},
		{
			name: "SequenceWithSingleValue",
			seq: func(yield func(int) bool) {
				_ = yield(1) && yield(2)
			},
			with:     []int{3},
			expected: []int{1, 2, 3},
		},
		{
			name: "SequenceWithMultipleValues",
			seq: func(yield func(int) bool) {
				_ = yield(1) && yield(2)
			},
			with:     []int{3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name: "LargeAppend",
			seq: func(yield func(int) bool) {
				_ = yield(1)
			},
			with:     []int{2, 3, 4, 5, 6, 7, 8, 9, 10},
			expected: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(Append(tt.seq, tt.with...))
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Append() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name     string
		seqs     []iter.Seq[int]
		expected []int
	}{
		{
			name:     "NoSequences",
			seqs:     []iter.Seq[int]{},
			expected: []int{},
		},
		{
			name: "SingleEmptySequence",
			seqs: []iter.Seq[int]{
				func(yield func(int) bool) {},
			},
			expected: []int{},
		},
		{
			name: "SingleSequence",
			seqs: []iter.Seq[int]{
				func(yield func(int) bool) {
					_ = yield(1) && yield(2) && yield(3)
				},
			},
			expected: []int{1, 2, 3},
		},
		{
			name: "TwoSequences",
			seqs: []iter.Seq[int]{
				func(yield func(int) bool) {
					_ = yield(1) && yield(2)
				},
				func(yield func(int) bool) {
					_ = yield(3) && yield(4)
				},
			},
			expected: []int{1, 2, 3, 4},
		},
		{
			name: "ThreeSequences",
			seqs: []iter.Seq[int]{
				func(yield func(int) bool) {
					yield(1)
				},
				func(yield func(int) bool) {
					_ = yield(2) && yield(3)
				},
				func(yield func(int) bool) {
					_ = yield(4) && yield(5) && yield(6)
				},
			},
			expected: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name: "FiveSequences",
			seqs: []iter.Seq[int]{
				func(yield func(int) bool) { yield(1) },
				func(yield func(int) bool) { yield(2) },
				func(yield func(int) bool) { yield(3) },
				func(yield func(int) bool) { yield(4) },
				func(yield func(int) bool) { yield(5) },
			},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name: "MixedWithEmpty",
			seqs: []iter.Seq[int]{
				func(yield func(int) bool) {
					_ = yield(1) && yield(2)
				},
				func(yield func(int) bool) {},
				func(yield func(int) bool) {
					yield(3)
				},
				func(yield func(int) bool) {},
				func(yield func(int) bool) {
					_ = yield(4) && yield(5)
				},
			},
			expected: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Collect(Join(tt.seqs...))
			if !slices.Equal(result, tt.expected) {
				t.Errorf("Join() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestJoin2(t *testing.T) {
	tests := []struct {
		name         string
		seqs         []iter.Seq2[string, int]
		expectedKeys []string
		expectedVals []int
	}{
		{
			name:         "NoSequences",
			seqs:         []iter.Seq2[string, int]{},
			expectedKeys: []string{},
			expectedVals: []int{},
		},
		{
			name: "SingleEmptySequence",
			seqs: []iter.Seq2[string, int]{
				func(yield func(string, int) bool) {},
			},
			expectedKeys: []string{},
			expectedVals: []int{},
		},
		{
			name: "SingleSequence",
			seqs: []iter.Seq2[string, int]{
				func(yield func(string, int) bool) {
					_ = yield("a", 1) && yield("b", 2)
				},
			},
			expectedKeys: []string{"a", "b"},
			expectedVals: []int{1, 2},
		},
		{
			name: "TwoSequences",
			seqs: []iter.Seq2[string, int]{
				func(yield func(string, int) bool) {
					_ = yield("a", 1) && yield("b", 2)
				},
				func(yield func(string, int) bool) {
					_ = yield("c", 3) && yield("d", 4)
				},
			},
			expectedKeys: []string{"a", "b", "c", "d"},
			expectedVals: []int{1, 2, 3, 4},
		},
		{
			name: "ThreeSequences",
			seqs: []iter.Seq2[string, int]{
				func(yield func(string, int) bool) {
					yield("x", 10)
				},
				func(yield func(string, int) bool) {
					_ = yield("y", 20) && yield("z", 30)
				},
				func(yield func(string, int) bool) {
					yield("w", 40)
				},
			},
			expectedKeys: []string{"x", "y", "z", "w"},
			expectedVals: []int{10, 20, 30, 40},
		},
		{
			name: "MixedWithEmpty",
			seqs: []iter.Seq2[string, int]{
				func(yield func(string, int) bool) {
					yield("a", 1)
				},
				func(yield func(string, int) bool) {},
				func(yield func(string, int) bool) {
					yield("b", 2)
				},
				func(yield func(string, int) bool) {},
				func(yield func(string, int) bool) {
					yield("c", 3)
				},
			},
			expectedKeys: []string{"a", "b", "c"},
			expectedVals: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string
			var vals []int
			for k, v := range Join2(tt.seqs...) {
				keys = append(keys, k)
				vals = append(vals, v)
			}
			if !slices.Equal(keys, tt.expectedKeys) {
				t.Errorf("Join2() keys = %v, want %v", keys, tt.expectedKeys)
			}
			if !slices.Equal(vals, tt.expectedVals) {
				t.Errorf("Join2() values = %v, want %v", vals, tt.expectedVals)
			}
		})
	}
}

func TestChain2(t *testing.T) {
	tests := []struct {
		name         string
		seq          iter.Seq[iter.Seq2[string, int]]
		expectedKeys []string
		expectedVals []int
	}{
		{
			name:         "Empty",
			seq:          func(yield func(iter.Seq2[string, int]) bool) {},
			expectedKeys: []string{},
			expectedVals: []int{},
		},
		{
			name: "SingleEmptySequence",
			seq: func(yield func(iter.Seq2[string, int]) bool) {
				yield(func(yield func(string, int) bool) {})
			},
			expectedKeys: []string{},
			expectedVals: []int{},
		},
		{
			name: "SingleSequence",
			seq: func(yield func(iter.Seq2[string, int]) bool) {
				yield(func(yield func(string, int) bool) {
					_ = yield("a", 1) && yield("b", 2)
				})
			},
			expectedKeys: []string{"a", "b"},
			expectedVals: []int{1, 2},
		},
		{
			name: "MultipleSequences",
			seq: func(yield func(iter.Seq2[string, int]) bool) {
				if !yield(func(yield func(string, int) bool) {
					_ = yield("a", 1) && yield("b", 2)
				}) {
					return
				}
				yield(func(yield func(string, int) bool) {
					_ = yield("c", 3) && yield("d", 4)
				})
			},
			expectedKeys: []string{"a", "b", "c", "d"},
			expectedVals: []int{1, 2, 3, 4},
		},
		{
			name: "MixedWithEmpty",
			seq: func(yield func(iter.Seq2[string, int]) bool) {
				if !yield(func(yield func(string, int) bool) {
					yield("a", 1)
				}) {
					return
				}
				if !yield(func(yield func(string, int) bool) {}) {
					return
				}
				if !yield(func(yield func(string, int) bool) {
					yield("b", 2)
				}) {
					return
				}
				yield(func(yield func(string, int) bool) {
					_ = yield("c", 3) && yield("d", 4)
				})
			},
			expectedKeys: []string{"a", "b", "c", "d"},
			expectedVals: []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var keys []string
			var vals []int
			for k, v := range Chain2(tt.seq) {
				keys = append(keys, k)
				vals = append(vals, v)
			}
			if !slices.Equal(keys, tt.expectedKeys) {
				t.Errorf("Chain2() keys = %v, want %v", keys, tt.expectedKeys)
			}
			if !slices.Equal(vals, tt.expectedVals) {
				t.Errorf("Chain2() values = %v, want %v", vals, tt.expectedVals)
			}
		})
	}

	t.Run("EarlyReturn", func(t *testing.T) {
		var outerCount atomic.Int32
		var innerCounts [3]atomic.Int32

		seq := func(yield func(iter.Seq2[string, int]) bool) {
			// First inner sequence
			outerCount.Add(1)
			if !yield(func(yield func(string, int) bool) {
				innerCounts[0].Add(1)
				if !yield("a", 1) {
					return
				}
				innerCounts[0].Add(1)
				yield("b", 2)
			}) {
				return
			}

			// Second inner sequence
			outerCount.Add(1)
			if !yield(func(yield func(string, int) bool) {
				innerCounts[1].Add(1)
				if !yield("c", 3) {
					return
				}
				innerCounts[1].Add(1)
				yield("d", 4)
			}) {
				return
			}

			// Third inner sequence (should not be reached)
			outerCount.Add(1)
			yield(func(yield func(string, int) bool) {
				innerCounts[2].Add(1)
				if !yield("e", 5) {
					return
				}
				innerCounts[2].Add(1)
				yield("f", 6)
			})
		}

		var keys []string
		var vals []int
		count := 0
		for k, v := range Chain2(seq) {
			keys = append(keys, k)
			vals = append(vals, v)
			count++
			// Break after 3 items: "a", "b", "c"
			if count == 3 {
				break
			}
		}

		// Verify we got exactly 3 items
		expectedKeys := []string{"a", "b", "c"}
		expectedVals := []int{1, 2, 3}
		if !slices.Equal(keys, expectedKeys) {
			t.Errorf("keys = %v, want %v", keys, expectedKeys)
		}
		if !slices.Equal(vals, expectedVals) {
			t.Errorf("values = %v, want %v", vals, expectedVals)
		}

		// Verify outer sequence was only iterated twice (for first two inner sequences)
		if outerCount.Load() != 2 {
			t.Errorf("outer sequence iterated %d times, expected 2", outerCount.Load())
		}

		// Verify first inner sequence was fully consumed (both items)
		if innerCounts[0].Load() != 2 {
			t.Errorf("first inner sequence iterated %d times, expected 2", innerCounts[0].Load())
		}

		// Verify second inner sequence was only partially consumed (only first item)
		if innerCounts[1].Load() != 1 {
			t.Errorf("second inner sequence iterated %d times, expected 1", innerCounts[1].Load())
		}

		// Verify third inner sequence was never touched
		if innerCounts[2].Load() != 0 {
			t.Errorf("third inner sequence iterated %d times, expected 0", innerCounts[2].Load())
		}
	})
}

func TestChainStandardTypes(t *testing.T) {
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
	t.Run("SmokeMap", func(t *testing.T) {
		elems := ChainMaps(Args(map[string]string{"a": "b"}, map[string]string{"c": "d"}))
		seen := 0
		for k, v := range elems {
			seen++
			switch k {
			case "a":
				if v != "b" {
					t.Errorf("a saw %s", v)
				}
			case "c":
				if v != "d" {
					t.Errorf("c saw %s", v)
				}
			default:
				t.Errorf("unexpected pair %s, %s", k, v)
			}
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

			output := Collect2(KVsplit(Limit(KVjoin(gen), 2)))

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
	t.Run("Single", func(t *testing.T) {
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
	})
	t.Run("Double", func(t *testing.T) {
		result := Reduce2(
			func(yield func(int, int) bool) {
				if !yield(1, 1) {
					return
				}
				if !yield(2, 2) {
					return
				}
				if !yield(3, 3) {
					return
				}
				yield(4, 4)
			},
			func(acc, a, b int) int { return acc + a + b },
		)

		if result != 20 {
			t.Errorf("Reduce() = %v, want 20", result)
		}
	})
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

	t.Run("Convert2EarlyReturn", func(t *testing.T) {
		var sourceCallCount atomic.Int32
		var convertCallCount atomic.Int32

		source := func(yield func(string, int) bool) {
			for i := 1; i <= 5; i++ {
				sourceCallCount.Add(1)
				if !yield(fmt.Sprint(i), i) {
					return
				}
			}
		}

		converted := Convert2(source, func(k string, v int) (string, int) {
			convertCallCount.Add(1)
			return k + "!", v * 10
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
			t.Errorf("Convert2 should be called 2 times, got %d", convertCallCount.Load())
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
		input := slices.SortedFunc(KVjoin(Map(map[string]int{"a": 1, "b": 2, "c": 3, "d": 4})), KVcmpSecond)
		t.Log(input)

		if len(input) != 4 {
			t.Error(input)
		}

		intermediate := Collect(KVjoin(Until2(KVsplit(Slice(input)), func(k string, v int) bool {
			return v > 2
		})))

		if len(intermediate) != 2 {
			t.Error(intermediate)
		}

		output := Collect2(KVsplit(Slice(intermediate)))

		if len(output) != 2 {
			t.Error(output)
		}

		_, hasC := output["c"]
		_, hasD := output["d"]
		if hasC || hasD {
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
	t.Run("NormalOperation", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2})
		output := Collect2(Convert2(input, func(k string, v int) (string, int) {
			return k + "!", v * 10
		}))
		expected := map[string]int{"a!": 10, "b!": 20}
		if !maps.Equal(output, expected) {
			t.Errorf("Convert2() = %v, want %v", output, expected)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var sourceCallCount atomic.Int32
		source := func(yield func(string, int) bool) {
			for i := 1; i <= 5; i++ {
				sourceCallCount.Add(1)
				if !yield(fmt.Sprint(i), i) {
					return
				}
			}
		}

		converted := Convert2(source, func(k string, v int) (string, int) {
			return k + "!", v * 10
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
	})
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

func TestModify(t *testing.T) {
	t.Run("SingleTransformation", func(t *testing.T) {
		input := Slice([]int{1, 2, 3, 4, 5})
		result := Collect(Modify(input, func(x int) int {
			return x * 2
		}))
		expected := []int{2, 4, 6, 8, 10}
		if !slices.Equal(result, expected) {
			t.Errorf("Modify() = %v, want %v", result, expected)
		}
	})

	t.Run("StringTransformation", func(t *testing.T) {
		input := Slice([]string{"a", "b", "c"})
		result := Collect(Modify(input, func(s string) string {
			return s + "!"
		}))
		expected := []string{"a!", "b!", "c!"}
		if !slices.Equal(result, expected) {
			t.Errorf("Modify() = %v, want %v", result, expected)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		input := func(yield func(int) bool) {
			for i := 1; i <= 10; i++ {
				callCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}

		modified := Modify(input, func(x int) int {
			return x * 2
		})

		count := 0
		for range modified {
			count++
			if count == 3 {
				break
			}
		}

		if callCount.Load() != 3 {
			t.Errorf("expected 3 calls to source, got %d", callCount.Load())
		}
	})
}

func TestModifyAll(t *testing.T) {
	t.Run("AllNilOperations", func(t *testing.T) {
		input := Slice([]int{1, 2, 3})
		result := Collect(ModifyAll(input, nil, nil, func(in int) int { return in }))
		expected := []int{1, 2, 3}
		if !slices.Equal(result, expected) {
			t.Errorf("ModifyAll() with all nil = %v, want %v", result, expected)
		}
	})

	t.Run("SingleOperation", func(t *testing.T) {
		input := Slice([]int{1, 2, 3})
		result := Collect(ModifyAll(input, func(x int) int {
			return x * 2
		}))
		expected := []int{2, 4, 6}
		if !slices.Equal(result, expected) {
			t.Errorf("ModifyAll() = %v, want %v", result, expected)
		}
	})

	t.Run("MultipleOperations", func(t *testing.T) {
		input := Slice([]int{1, 2, 3})
		result := Collect(ModifyAll(input,
			func(x int) int { return x * 2 },  // multiply by 2
			func(x int) int { return x + 10 }, // add 10
			func(x int) int { return x * 3 },  // multiply by 3
		))
		// 1: 1*2=2, 2+10=12, 12*3=36
		// 2: 2*2=4, 4+10=14, 14*3=42
		// 3: 3*2=6, 6+10=16, 16*3=48
		expected := []int{36, 42, 48}
		if !slices.Equal(result, expected) {
			t.Errorf("ModifyAll() = %v, want %v", result, expected)
		}
	})

	t.Run("MixedNilAndNonNil", func(t *testing.T) {
		input := Slice([]int{5, 10})
		result := Collect(ModifyAll(input,
			nil,
			func(x int) int { return x * 2 },
			nil,
			func(x int) int { return x + 1 },
			nil,
		))
		// 5: 5*2=10, 10+1=11
		// 10: 10*2=20, 20+1=21
		expected := []int{11, 21}
		if !slices.Equal(result, expected) {
			t.Errorf("ModifyAll() = %v, want %v", result, expected)
		}
	})

	t.Run("StringOperations", func(t *testing.T) {
		input := Slice([]string{"a", "b"})
		result := Collect(ModifyAll(input,
			func(s string) string { return s + "1" },
			func(s string) string { return s + "2" },
			func(s string) string { return s + "3" },
		))
		expected := []string{"a123", "b123"}
		if !slices.Equal(result, expected) {
			t.Errorf("ModifyAll() = %v, want %v", result, expected)
		}
	})
}

func TestModify2(t *testing.T) {
	t.Run("SingleTransformation", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2})
		result := Collect2(Modify2(input, func(k string, v int) (string, int) {
			return k + "!", v * 10
		}))
		expected := map[string]int{"a!": 10, "b!": 20}
		if !maps.Equal(result, expected) {
			t.Errorf("Modify2() = %v, want %v", result, expected)
		}
	})

	t.Run("SwapKeyValue", func(t *testing.T) {
		input := func(yield func(string, int) bool) {
			_ = yield("a", 1) && yield("b", 2) && yield("c", 3)
		}
		var keys []string
		var vals []int
		for k, v := range Modify2(input, func(k string, v int) (string, int) {
			return fmt.Sprint(v), len(k)
		}) {
			keys = append(keys, k)
			vals = append(vals, v)
		}
		expectedKeys := []string{"1", "2", "3"}
		expectedVals := []int{1, 1, 1}
		if !slices.Equal(keys, expectedKeys) {
			t.Errorf("Modify2() keys = %v, want %v", keys, expectedKeys)
		}
		if !slices.Equal(vals, expectedVals) {
			t.Errorf("Modify2() vals = %v, want %v", vals, expectedVals)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		input := func(yield func(string, int) bool) {
			for i := 1; i <= 10; i++ {
				callCount.Add(1)
				if !yield(fmt.Sprint(i), i) {
					return
				}
			}
		}

		modified := Modify2(input, func(k string, v int) (string, int) {
			return k + "!", v * 2
		})

		count := 0
		for range modified {
			count++
			if count == 3 {
				break
			}
		}

		if callCount.Load() != 3 {
			t.Errorf("expected 3 calls to source, got %d", callCount.Load())
		}
	})
}

func TestModifyAll2(t *testing.T) {
	t.Run("NoOperations", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2})
		result := Collect2(ModifyAll2(input))
		expected := map[string]int{"a": 1, "b": 2}
		if !maps.Equal(result, expected) {
			t.Errorf("ModifyAll2() with no ops = %v, want %v", result, expected)
		}
	})

	t.Run("AllNilOperations", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2})
		result := Collect2(ModifyAll2(input, nil, nil, nil))
		expected := map[string]int{"a": 1, "b": 2}
		if !maps.Equal(result, expected) {
			t.Errorf("ModifyAll2() with all nil = %v, want %v", result, expected)
		}
	})

	t.Run("SingleOperation", func(t *testing.T) {
		input := Map(map[string]int{"a": 1, "b": 2})
		result := Collect2(ModifyAll2(input, func(k string, v int) (string, int) {
			return k + "!", v * 2
		}))
		expected := map[string]int{"a!": 2, "b!": 4}
		if !maps.Equal(result, expected) {
			t.Errorf("ModifyAll2() = %v, want %v", result, expected)
		}
	})

	t.Run("MultipleOperations", func(t *testing.T) {
		input := func(yield func(string, int) bool) {
			_ = yield("x", 1) && yield("y", 2)
		}
		var keys []string
		var vals []int
		for k, v := range ModifyAll2(input,
			func(k string, v int) (string, int) { return k + "1", v * 2 },
			func(k string, v int) (string, int) { return k + "2", v + 10 },
			func(k string, v int) (string, int) { return k + "3", v * 3 },
		) {
			keys = append(keys, k)
			vals = append(vals, v)
		}
		// x,1: x1,2 -> x12,12 -> x123,36
		// y,2: y1,4 -> y12,14 -> y123,42
		expectedKeys := []string{"x123", "y123"}
		expectedVals := []int{36, 42}
		if !slices.Equal(keys, expectedKeys) {
			t.Errorf("ModifyAll2() keys = %v, want %v", keys, expectedKeys)
		}
		if !slices.Equal(vals, expectedVals) {
			t.Errorf("ModifyAll2() vals = %v, want %v", vals, expectedVals)
		}
	})

	t.Run("MixedNilAndNonNil", func(t *testing.T) {
		input := func(yield func(string, int) bool) {
			_ = yield("a", 5) && yield("b", 10)
		}
		var keys []string
		var vals []int
		for k, v := range ModifyAll2(input,
			nil,
			func(k string, v int) (string, int) { return k + "x", v * 2 },
			nil,
			func(k string, v int) (string, int) { return k + "y", v + 1 },
			nil,
		) {
			keys = append(keys, k)
			vals = append(vals, v)
		}
		// a,5: ax,10 -> axy,11
		// b,10: bx,20 -> bxy,21
		expectedKeys := []string{"axy", "bxy"}
		expectedVals := []int{11, 21}
		if !slices.Equal(keys, expectedKeys) {
			t.Errorf("ModifyAll2() keys = %v, want %v", keys, expectedKeys)
		}
		if !slices.Equal(vals, expectedVals) {
			t.Errorf("ModifyAll2() vals = %v, want %v", vals, expectedVals)
		}
	})
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

func TestElems(t *testing.T) {
	t.Run("Apply", func(t *testing.T) {
		count := 0
		KVapply(KVjoin(With(GenerateN(30, func() int { return 7 }), strconv.Itoa)), func(n int, s string) {
			count++
			if n != 7 {
				t.Error("unexpected value")
			}
			if s != "7" {
				t.Errorf("unexpected")
			}
		})
		if count == 0 {
			t.Error("too many")
		}
	})
}

func TestKeepOk(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		input := func(yield func(int, bool) bool) {
			_ = yield(1, true) && yield(2, false) && yield(3, true)
		}
		got := Collect(KeepOk(input))
		want := []int{1, 3}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var count atomic.Int32
		input := func(yield func(int, bool) bool) {
			for i := 0; i < 10; i++ {
				count.Add(1)
				if !yield(i, true) {
					return
				}
			}
		}
		// Stop after 2 items
		i := 0
		for range KeepOk(input) {
			i++
			if i == 2 {
				break
			}
		}
		if count.Load() != 2 {
			t.Errorf("expected 2 iterations, got %d", count.Load())
		}
	})
}

func TestKeepErrors(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		input := func(yield func(error) bool) {}
		got := Collect(KeepErrors(input))
		if len(got) != 0 {
			t.Errorf("expected empty result, got %v", got)
		}
	})

	t.Run("AllNilErrors", func(t *testing.T) {
		input := func(yield func(error) bool) {
			_ = yield(nil) && yield(nil) && yield(nil)
		}
		got := Collect(KeepErrors(input))
		if len(got) != 0 {
			t.Errorf("expected empty result, got %v", got)
		}
	})

	t.Run("AllNonNilErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		err3 := errors.New("error3")

		input := func(yield func(error) bool) {
			_ = yield(err1) && yield(err2) && yield(err3)
		}
		got := Collect(KeepErrors(input))
		want := []error{err1, err2, err3}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("MixedNilAndNonNil", func(t *testing.T) {
		err1 := errors.New("first")
		err2 := errors.New("second")
		err3 := errors.New("third")

		input := func(yield func(error) bool) {
			_ = yield(nil) &&
				yield(err1) &&
				yield(nil) &&
				yield(nil) &&
				yield(err2) &&
				yield(nil) &&
				yield(err3) &&
				yield(nil)
		}
		got := Collect(KeepErrors(input))
		want := []error{err1, err2, err3}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("WithContextErrors", func(t *testing.T) {
		input := func(yield func(error) bool) {
			_ = yield(context.Canceled) &&
				yield(nil) &&
				yield(context.DeadlineExceeded) &&
				yield(nil)
		}
		got := Collect(KeepErrors(input))
		want := []error{context.Canceled, context.DeadlineExceeded}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		var count atomic.Int32
		err1 := errors.New("err1")
		err2 := errors.New("err2")

		input := func(yield func(error) bool) {
			for _, err := range []error{nil, err1, nil, err2, nil} {
				count.Add(1)
				if !yield(err) {
					return
				}
			}
		}

		// Stop after collecting 1 non-nil error
		collected := 0
		for range KeepErrors(input) {
			collected++
			if collected == 1 {
				break
			}
		}

		// Should have iterated through: nil (skip), err1 (yield)
		// Count should be 2 because we stopped after first non-nil error
		if count.Load() != 2 {
			t.Errorf("expected 2 iterations, got %d", count.Load())
		}
	})

	t.Run("PreservesErrorTypes", func(t *testing.T) {
		input := func(yield func(error) bool) {
			custom := &customError{msg: "custom"}

			_ = yield(nil) &&
				yield(custom) &&
				yield(errors.New("regular")) &&
				yield(nil)
		}

		got := Collect(KeepErrors(input))
		if len(got) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(got))
		}

		// Verify the custom error is preserved
		if ce, ok := got[0].(*customError); !ok {
			t.Errorf("expected customError, got %T", got[0])
		} else if ce.msg != "custom" {
			t.Errorf("expected msg 'custom', got %q", ce.msg)
		}
	})

	t.Run("IteratorReset", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")

		input := func(yield func(error) bool) {
			_ = yield(nil) && yield(err1) && yield(nil) && yield(err2)
		}

		seq := KeepErrors(input)

		// First iteration
		first := Collect(seq)
		// Second iteration
		second := Collect(seq)

		want := []error{err1, err2}
		if !slices.Equal(first, want) {
			t.Errorf("first iteration: got %v, want %v", first, want)
		}
		if !slices.Equal(second, want) {
			t.Errorf("second iteration: got %v, want %v", second, want)
		}
	})
}

func TestWhileOk(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		input := func(yield func(int, bool) bool) {
			_ = yield(1, true) && yield(2, true) && yield(3, false) && yield(4, true)
		}
		got := Collect(WhileOk(input))
		want := []int{1, 2}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestWhileSuccess(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		err := errors.New("fail")
		input := func(yield func(int, error) bool) {
			_ = yield(1, nil) && yield(2, nil) && yield(3, err) && yield(4, nil)
		}
		got := Collect(WhileSuccess(input))
		want := []int{1, 2}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestWithHooks(t *testing.T) {
	t.Run("ExecutionOrder", func(t *testing.T) {
		var log []string
		seq := WithHooks(
			Slice([]int{1}),
			func() { log = append(log, "before") },
			func() { log = append(log, "after") },
		)
		for range seq {
			log = append(log, "item")
		}
		want := []string{"before", "item", "after"}
		if !slices.Equal(log, want) {
			t.Errorf("got %v, want %v", log, want)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var afterCalled bool
		seq := WithHooks(
			Slice([]int{1, 2, 3}),
			nil,
			func() { afterCalled = true },
		)
		for range seq {
			break
		}
		if !afterCalled {
			t.Error("after hook should be called even on early return")
		}
	})
}

func TestWithSetup(t *testing.T) {
	t.Run("Once", func(t *testing.T) {
		var count atomic.Int32
		seq := WithSetup(Slice([]int{1, 2}), func() { count.Add(1) })
		Collect(seq)
		Collect(seq)
		if count.Load() != 1 {
			t.Errorf("setup called %d times, want 1", count.Load())
		}
	})
}

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input iter.Seq[int]
		want  int
	}{
		{"Empty input", Slice([]int{}), 0},
		{"Single element", Slice([]int{1}), 1},
		{"Two elements", Slice([]int{1, 2}), 2},
		{"Normal input", Slice([]int{1, 2, 3, 4, 5, 6, 7, 8}), 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Count(tt.input)
			if got != tt.want {
				t.Errorf("Count() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCount2(t *testing.T) {
	tests := []struct {
		name  string
		input iter.Seq2[int, string]
		want  int
	}{
		{"Empty input", KVsplit(Slice([]KV[int, string]{})), 0},
		{"Single element", KVsplit(Slice([]KV[int, string]{{1, "a"}})), 1},
		{"Two elements", KVsplit(Slice([]KV[int, string]{{1, "a"}, {2, "b"}})), 2},
		{"Normal input", KVsplit(Slice([]KV[int, string]{
			{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}, {5, "e"}, {6, "f"}, {7, "g"}, {8, "h"},
		})), 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Count2(tt.input)
			if got != tt.want {
				t.Errorf("Count2() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortBy(t *testing.T) {
	tests := []struct {
		name  string
		input iter.Seq[int]
		want  []int
	}{
		{"Empty input", Slice([]int{}), []int{}},
		{"Single element", Slice([]int{1}), []int{1}},
		{"Two elements", Slice([]int{2, 1}), []int{1, 2}},
		{"Normal input", Slice([]int{5, 3, 8, 1, 2, 7, 4, 6}), []int{1, 2, 3, 4, 5, 6, 7, 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Collect(SortBy(tt.input, func(v int) int { return v }))
			if !slices.Equal(got, tt.want) {
				t.Errorf("SortBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortBy2(t *testing.T) {
	tests := []struct {
		name  string
		input iter.Seq2[int, string]
		want  []KV[int, string]
	}{
		{"Empty input", KVsplit(Slice([]KV[int, string]{})), []KV[int, string]{}},
		{"Single element", KVsplit(Slice([]KV[int, string]{{1, "a"}})), []KV[int, string]{{1, "a"}}},
		{"Two elements", KVsplit(Slice([]KV[int, string]{{2, "b"}, {1, "a"}})), []KV[int, string]{{1, "a"}, {2, "b"}}},
		{"Normal input", KVsplit(Slice([]KV[int, string]{
			{5, "e"}, {3, "c"}, {8, "h"}, {1, "a"}, {2, "b"}, {7, "g"}, {4, "d"}, {6, "f"},
		})), []KV[int, string]{
			{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}, {5, "e"}, {6, "f"}, {7, "g"}, {8, "h"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Collect(KVjoin(SortBy2(tt.input, func(k int, v string) int { return k })))
			if !slices.Equal(got, tt.want) {
				t.Errorf("SortBy2() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestForEach(t *testing.T) {
	cases := []struct {
		name      string
		seq       iter.Seq[int]
		op        func(int)
		earlyExit int
		expected  []int
	}{
		{
			name: "Empty",
			seq:  func(func(int) bool) {},
			op:   func(int) { t.Error("should not be called") },
		},
		{
			name:     "One",
			seq:      Args(1),
			expected: []int{1},
		},
		{
			name:     "Two",
			seq:      Args(1, 2),
			expected: []int{1, 2},
		},
		{
			name:      "EarlyExit",
			seq:       Args(1, 2, 3, 4, 5, 6, 7, 8),
			earlyExit: 4,
			expected:  []int{1, 2, 3, 4},
		},
		{
			name:     "Many",
			seq:      Args(1, 2, 3, 4, 5, 6, 7, 8),
			expected: []int{1, 2, 3, 4, 5, 6, 7, 8},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var opcount atomic.Int64
			var output []int
			op := func(in int) {
				opcount.Add(1)
				output = append(output, in)
			}
			if cs.op != nil {
				op = cs.op
			}

			seq := ForEach(cs.seq, op)
			if cs.earlyExit > 0 {
				seq = Limit(seq, cs.earlyExit)
			}

			res := Collect(seq)
			if !slices.Equal(cs.expected, res) {
				t.Error("unexpected result", "have=", res, "want=", cs.expected)
			}
			switch {
			case cs.earlyExit > 0 && opcount.Load() != int64(cs.earlyExit):
				t.Error("[first] op call count mismatch", "have=", opcount.Load(), "want=", cs.earlyExit)
			case cs.earlyExit > 0 && len(cs.expected) != cs.earlyExit:
				t.Error("[second] invalid test case")
			case len(cs.expected) > 0 && opcount.Load() != int64(len(cs.expected)):
				t.Error("[third] op call count mismatch", "have=", opcount.Load(), "want=", len(cs.expected))
			case len(cs.expected) == 0 && opcount.Load() != 0:
				t.Error("[fourth] op call count mismatch", "have=", opcount.Load(), "want=", len(cs.expected))
			}
		})
	}
}

func TestForEach2(t *testing.T) {
	cases := []struct {
		name      string
		seq       iter.Seq2[int, int]
		op        func(int, int)
		earlyExit int
		expected  map[int]int
	}{
		{
			name: "Empty",
			seq:  func(func(int, int) bool) {},
			op:   func(int, int) { t.Error("should not be called") },
		},
		{
			name:     "One",
			seq:      Map(map[int]int{1: 1}),
			expected: map[int]int{1: 1},
		},
		{
			name:     "Two",
			seq:      Map(map[int]int{1: 1, 2: 2}),
			expected: map[int]int{1: 1, 2: 2},
		},
		{
			name:      "EarlyExit",
			seq:       Map(map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8}),
			earlyExit: 4,
			expected:  map[int]int{1: 1, 2: 2, 3: 3, 4: 4},
		},
		{
			name:     "Many",
			seq:      Map(map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8}),
			expected: map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var opcount atomic.Int64
			output := make(map[int]int)
			op := func(k, v int) {
				opcount.Add(1)
				output[k] = v
			}
			if cs.op != nil {
				op = cs.op
			}

			seq := ForEach2(cs.seq, op)
			if cs.earlyExit > 0 {
				seq = Limit2(seq, cs.earlyExit)
			}

			res := Collect2(seq)
			if cs.earlyExit > 0 {
				if len(res) != cs.earlyExit {
					t.Error("unexpected result length", "have=", len(res), "want=", cs.earlyExit)
				}
			} else if !maps.Equal(cs.expected, res) {
				t.Error("unexpected result", "have=", res, "want=", cs.expected)
			}

			switch {
			case cs.earlyExit > 0 && opcount.Load() != int64(cs.earlyExit):
				t.Error("[first] op call count mismatch", "have=", opcount.Load(), "want=", cs.earlyExit)
			case cs.earlyExit > 0 && len(cs.expected) != cs.earlyExit:
				t.Error("[second] invalid test case")
			case len(cs.expected) > 0 && opcount.Load() != int64(len(cs.expected)):
				t.Error("[third] op call count mismatch", "have=", opcount.Load(), "want=", len(cs.expected))
			case len(cs.expected) == 0 && opcount.Load() != 0:
				t.Error("[fourth] op call count mismatch", "have=", opcount.Load(), "want=", len(cs.expected))
			}
		})
	}
}

func TestForEachWhile(t *testing.T) {
	cases := []struct {
		name            string
		seq             iter.Seq[int]
		op              func(int) bool
		earlyExit       int
		expected        []int
		expectedOpCount int
	}{
		{
			name: "Empty",
			seq:  func(func(int) bool) {},
			op:   func(int) bool { t.Error("should not be called"); return false },
		},
		{
			name:            "One",
			seq:             Args(1),
			expected:        []int{1},
			expectedOpCount: 1,
		},
		{
			name:            "Two",
			seq:             Args(1, 2),
			expected:        []int{1, 2},
			expectedOpCount: 2,
		},
		{
			name:            "EarlyExit",
			seq:             Args(1, 2, 3, 4, 5, 6, 7, 8),
			earlyExit:       4,
			expected:        []int{1, 2, 3, 4},
			expectedOpCount: 4,
		},
		{
			name:            "Many",
			seq:             Args(1, 2, 3, 4, 5, 6, 7, 8),
			expected:        []int{1, 2, 3, 4, 5, 6, 7, 8},
			expectedOpCount: 8,
		},
		{
			name:            "Predicate",
			seq:             Args(1, 2, 3, 4, 5, 6, 7, 8),
			op:              func(in int) bool { return in < 3 },
			expected:        []int{1, 2},
			expectedOpCount: 3,
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var opcount atomic.Int64
			op := func(in int) bool {
				opcount.Add(1)
				return true
			}
			if cs.op != nil {
				op = func(in int) bool {
					opcount.Add(1)
					return cs.op(in)
				}
			}

			seq := ForEachWhile(cs.seq, op)
			if cs.earlyExit > 0 {
				seq = Limit(seq, cs.earlyExit)
			}

			res := Collect(seq)
			if !slices.Equal(cs.expected, res) {
				t.Error("unexpected result", "have=", res, "want=", cs.expected)
			}
			if opcount.Load() != int64(cs.expectedOpCount) {
				t.Error("op call count mismatch", "have=", opcount.Load(), "want=", cs.expectedOpCount)
			}
		})
	}
}

func TestForEachWhile2(t *testing.T) {
	cases := []struct {
		name            string
		seq             iter.Seq2[int, int]
		op              func(int, int) bool
		earlyExit       int
		expected        map[int]int
		expectedOpCount int
	}{
		{
			name: "Empty",
			seq:  func(func(int, int) bool) {},
			op:   func(int, int) bool { t.Error("should not be called"); return false },
		},
		{
			name:            "One",
			seq:             Map(map[int]int{1: 1}),
			expected:        map[int]int{1: 1},
			expectedOpCount: 1,
		},
		{
			name:            "Two",
			seq:             Map(map[int]int{1: 1, 2: 2}),
			expected:        map[int]int{1: 1, 2: 2},
			expectedOpCount: 2,
		},
		{
			name:            "EarlyExit",
			seq:             Map(map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8}),
			earlyExit:       4,
			expected:        map[int]int{1: 1, 2: 2, 3: 3, 4: 4},
			expectedOpCount: 4,
		},
		{
			name:            "Many",
			seq:             Map(map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8}),
			expected:        map[int]int{1: 1, 2: 2, 3: 3, 4: 4, 5: 5, 6: 6, 7: 7, 8: 8},
			expectedOpCount: 8,
		},
		{
			name:            "Predicate",
			seq:             Index(Args(1, 2, 3, 4, 5, 6, 7, 8)),
			op:              func(k, v int) bool { return v < 3 },
			expected:        map[int]int{0: 1, 1: 2},
			expectedOpCount: 3,
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var opcount atomic.Int64
			op := func(k, v int) bool {
				opcount.Add(1)
				return true
			}
			if cs.op != nil {
				op = func(k, v int) bool {
					opcount.Add(1)
					return cs.op(k, v)
				}
			}

			seq := ForEachWhile2(cs.seq, op)
			if cs.earlyExit > 0 {
				seq = Limit2(seq, cs.earlyExit)
			}

			res := Collect2(seq)
			if cs.earlyExit > 0 {
				if len(res) != cs.earlyExit {
					t.Error("unexpected result length", "have=", len(res), "want=", cs.earlyExit)
				}
			} else if !maps.Equal(cs.expected, res) {
				t.Error("unexpected result", "have=", res, "want=", cs.expected)
			}
			if opcount.Load() != int64(cs.expectedOpCount) {
				t.Error("op call count mismatch", "have=", opcount.Load(), "want=", cs.expectedOpCount)
			}
		})
	}
}

func TestLegacyUtils(t *testing.T) {
	t.Run("Lines", func(t *testing.T) {
		buf := &bytes.Buffer{}
		last := sha256.Sum256([]byte(fmt.Sprint(time.Now().UTC().UnixMilli())))
		_, _ = fmt.Fprintf(buf, "%x", last)
		for i := 1; i < 128; i++ {
			next := sha256.Sum256(last[:])
			_, _ = fmt.Fprintf(buf, "\n%x", next)
		}

		count := 0
		var prev string
		for line := range RemoveZeros(Convert(ReadLines(buf), strings.TrimSpace)) {
			count++
			if len(line) != 64 {
				t.Error("unexpected line length", len(line), line)
			}
			if prev == line {
				t.Errorf("duplicate line %d %q vs %q ", count, line, prev)
			}
		}

		if count != 128 {
			t.Error(count)
		}
	})
	t.Run("EarlyReturn", func(t *testing.T) {
		var buf bytes.Buffer

		_, _ = buf.WriteString("123\n456\n\n789")
		count := 0
		for line := range ReadLines(&buf) {
			if line == "" {
				break
			}
			count++
			if len(line) != 3 {
				t.Error("unexpected length", len(line), line)
			}
			if _, err := strconv.Atoi(line); err != nil {
				t.Error("should have been able to parse as number:", line, err)
			}
		}
		if count != 2 {
			t.Error("unexpected count", count)
		}
	})
	t.Run("ReadLinesErrWithError", func(t *testing.T) {
		expectedErr := errors.New("read error")
		reader := &testReader{
			data: []byte("line1\nline2"),
			err:  expectedErr,
		}

		var lines []string
		var errs []error
		for line, err := range ReadLinesErr(reader) {
			if line != "" {
				lines = append(lines, line)
			}
			if err != nil {
				errs = append(errs, err)
			}
		}

		if !slices.Equal(lines, []string{"line1", "line2"}) {
			t.Errorf("got lines %v, want %v", lines, []string{"line1", "line2"})
		}
		if len(errs) != 1 || !errors.Is(errs[0], expectedErr) {
			t.Errorf("got errors %v, want [%v]", errs, expectedErr)
		}
	})
}

func TestAsGenerator(t *testing.T) {
	t.Run("LazyInitialization", func(t *testing.T) {
		started := &atomic.Bool{}
		values := []int{1, 2, 3, 4, 5}

		seq := func(yield func(int) bool) {
			started.Store(true)
			for _, v := range values {
				if !yield(v) {
					return
				}
			}
		}

		gen := AsGenerator(seq)

		// Verify iterator has not started before first generator call
		if started.Load() {
			t.Error("iterator should not have started before generator is called")
		}

		// First call should trigger initialization
		val, ok := gen(t.Context())
		if !ok {
			t.Error("expected first value to be available")
		}
		if val != 1 {
			t.Errorf("expected first value to be 1, got %d", val)
		}

		// Now iterator should have started
		if !started.Load() {
			t.Error("iterator should have started after first generator call")
		}
	})

	t.Run("ExhaustionReturnsZeroValue", func(t *testing.T) {
		values := []int{10, 20, 30}

		seq := func(yield func(int) bool) {
			for _, v := range values {
				if !yield(v) {
					return
				}
			}
		}

		gen := AsGenerator(seq)

		// Read all values
		for i, expected := range values {
			val, ok := gen(t.Context())
			if !ok {
				t.Errorf("iteration %d: expected value to be available", i)
			}
			if val != expected {
				t.Errorf("iteration %d: expected %d, got %d", i, expected, val)
			}
		}

		// After exhaustion, should return zero value and false
		for i := 0; i < 5; i++ {
			val, ok := gen(t.Context())
			if ok {
				t.Errorf("after exhaustion call %d: expected ok to be false", i)
			}
			if val != 0 {
				t.Errorf("after exhaustion call %d: expected zero value (0), got %d", i, val)
			}
		}
	})

	t.Run("EmptyIterator", func(t *testing.T) {
		seq := func(yield func(int) bool) {
			// Empty iterator
		}

		gen := AsGenerator(seq)

		// First call should return zero value and false
		val, ok := gen(t.Context())
		if ok {
			t.Error("expected empty iterator to return false immediately")
		}
		if val != 0 {
			t.Errorf("expected zero value (0), got %d", val)
		}

		// Subsequent calls should also return zero value and false
		val, ok = gen(t.Context())
		if ok {
			t.Error("expected second call to return false")
		}
		if val != 0 {
			t.Errorf("expected zero value (0), got %d", val)
		}
	})

	t.Run("StringType", func(t *testing.T) {
		values := []string{"hello", "world", "test"}

		seq := func(yield func(string) bool) {
			for _, v := range values {
				if !yield(v) {
					return
				}
			}
		}

		gen := AsGenerator(seq)

		// Read all values
		for i, expected := range values {
			val, ok := gen(t.Context())
			if !ok {
				t.Errorf("iteration %d: expected value to be available", i)
			}
			if val != expected {
				t.Errorf("iteration %d: expected %q, got %q", i, expected, val)
			}
		}

		// After exhaustion, should return empty string and false
		val, ok := gen(t.Context())
		if ok {
			t.Error("after exhaustion: expected ok to be false")
		}
		if val != "" {
			t.Errorf("after exhaustion: expected empty string, got %q", val)
		}
	})

	t.Run("AdvancementTracking", func(t *testing.T) {
		callCount := &atomic.Int64{}
		values := []int{100, 200, 300}

		seq := func(yield func(int) bool) {
			for _, v := range values {
				callCount.Add(1)
				if !yield(v) {
					return
				}
			}
		}

		gen := AsGenerator(seq)

		// Before any calls, iterator should not have advanced
		if callCount.Load() != 0 {
			t.Errorf("expected 0 iterator calls before generator is used, got %d", callCount.Load())
		}

		// Call generator once
		gen(t.Context())

		// Give goroutine time to start
		time.Sleep(10 * time.Millisecond)

		// Iterator should have advanced at least once (possibly all values due to channel buffering)
		if callCount.Load() == 0 {
			t.Error("expected iterator to have advanced after first generator call")
		}

		// Consume remaining values
		gen(t.Context())
		gen(t.Context())

		// Give goroutine time to finish
		time.Sleep(10 * time.Millisecond)

		// All values should have been generated
		if callCount.Load() != int64(len(values)) {
			t.Errorf("expected %d iterator calls, got %d", len(values), callCount.Load())
		}
	})
}

type testReader struct {
	data []byte
	err  error
}

func (r *testReader) Read(p []byte) (n int, err error) {
	if len(r.data) > 0 {
		n = copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}

type customError struct {
	msg string
}

func (ce *customError) Error() string { return ce.msg }

func TestWithBuffer(t *testing.T) {
	t.Run("SimpleTest", func(t *testing.T) {
		ctx := context.Background()

		seq := func(yield func(int) bool) {
			t.Log("Producer starting")
			for i := 1; i <= 3; i++ {
				t.Logf("Producer yielding %d", i)
				if !yield(i) {
					t.Log("Producer stopping early")
					return
				}
			}
			t.Log("Producer done")
		}

		buffered := WithBuffer(ctx, seq, 2)

		t.Log("Starting consumption")
		var result []int
		for val := range buffered {
			t.Logf("Consumer got %d", val)
			result = append(result, val)
		}
		t.Log("Consumption done")

		expected := []int{1, 2, 3}
		if !slices.Equal(result, expected) {
			t.Errorf("WithBuffer collected %v, want %v", result, expected)
		}
	})

	t.Run("BasicBuffering", func(t *testing.T) {
		ctx := context.Background()
		values := []int{1, 2, 3, 4, 5}
		seq := Slice(values)

		buffered := WithBuffer(ctx, seq, 3)
		result := Collect(buffered)

		if !slices.Equal(result, values) {
			t.Errorf("WithBuffer collected %v, want %v", result, values)
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		ctx := context.Background()
		seq := Slice([]int{})

		buffered := WithBuffer(ctx, seq, 5)
		result := Collect(buffered)

		if len(result) != 0 {
			t.Errorf("WithBuffer collected %v, want empty slice", result)
		}
	})

	t.Run("BufferSizeBehavior", func(t *testing.T) {
		ctx := context.Background()
		var producerCount atomic.Int32
		var consumerStarted atomic.Bool

		// Producer that tracks how many items it has produced
		seq := func(yield func(int) bool) {
			for i := 1; i <= 10; i++ {
				producerCount.Add(1)
				if !yield(i) {
					return
				}
				// Wait a bit for consumer to start if it hasn't
				if i == 1 {
					time.Sleep(50 * time.Millisecond)
				}
			}
		}

		buffered := WithBuffer(ctx, seq, 3)

		// Start consuming slowly
		var consumed []int
		for val := range buffered {
			if !consumerStarted.Load() {
				consumerStarted.Store(true)
				// At this point, the buffer should have filled up
				// Give producer time to fill buffer
				time.Sleep(10 * time.Millisecond)
			}
			consumed = append(consumed, val)
			if len(consumed) == 5 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Producer should have produced more items than consumed due to buffering
		produced := producerCount.Load()
		if produced < int32(len(consumed)) {
			t.Errorf("Producer should have produced at least %d items, got %d", len(consumed), produced)
		}

		if !slices.Equal(consumed, []int{1, 2, 3, 4, 5}) {
			t.Errorf("Consumed %v, want [1 2 3 4 5]", consumed)
		}
	})

	t.Run("SlowConsumer", func(t *testing.T) {
		ctx := context.Background()
		var producerDone atomic.Bool

		// Fast producer
		seq := func(yield func(int) bool) {
			for i := 1; i <= 5; i++ {
				if !yield(i) {
					return
				}
			}
			producerDone.Store(true)
		}

		buffered := WithBuffer(ctx, seq, 10)

		// Slow consumer
		var consumed []int
		for val := range buffered {
			consumed = append(consumed, val)
			time.Sleep(20 * time.Millisecond) // Slow consumption
		}

		// Producer should finish before consumer due to buffering
		if !producerDone.Load() {
			t.Error("Producer should have finished")
		}

		// With buffering, producer should finish quickly while consumer is still working
		if !slices.Equal(consumed, []int{1, 2, 3, 4, 5}) {
			t.Errorf("Consumed %v, want [1 2 3 4 5]", consumed)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Create a long sequence
		seq := func(yield func(int) bool) {
			for i := 1; i <= 100; i++ {
				if !yield(i) {
					return
				}
			}
		}

		buffered := WithBuffer(ctx, seq, 5)

		var consumed []int
		for val := range buffered {
			consumed = append(consumed, val)
			if len(consumed) == 3 {
				cancel() // Cancel after consuming 3 items
			}
			if len(consumed) >= 10 {
				// Safety break in case cancellation doesn't work
				break
			}
		}

		// Should have stopped after cancellation
		// Might get a few more due to buffering, but not all 100
		if len(consumed) >= 50 {
			t.Errorf("Consumed %d items after cancellation, expected much fewer", len(consumed))
		}
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		ctx := context.Background()
		var producerCount atomic.Int32

		seq := func(yield func(int) bool) {
			for i := 1; i <= 20; i++ {
				producerCount.Add(1)
				if !yield(i) {
					return
				}
			}
		}

		buffered := WithBuffer(ctx, seq, 5)

		// Consume only first 3 items then stop
		var consumed []int
		for val := range buffered {
			consumed = append(consumed, val)
			if len(consumed) == 3 {
				break
			}
		}

		if !slices.Equal(consumed, []int{1, 2, 3}) {
			t.Errorf("Consumed %v, want [1 2 3]", consumed)
		}

		// Producer might have produced more than 3 due to buffering
		produced := producerCount.Load()
		if produced < 3 {
			t.Errorf("Producer should have produced at least 3 items, got %d", produced)
		}
		if produced > 10 {
			t.Errorf("Producer should have stopped soon after consumer stopped, produced %d items", produced)
		}
	})

	t.Run("ZeroBufferSize", func(t *testing.T) {
		ctx := context.Background()
		values := []int{1, 2, 3}
		seq := Slice(values)

		// Buffer size 0 should still work (creates unbuffered channel)
		buffered := WithBuffer(ctx, seq, 0)
		result := Collect(buffered)

		if !slices.Equal(result, values) {
			t.Errorf("WithBuffer(0) collected %v, want %v", result, values)
		}
	})

	t.Run("LazyExecution", func(t *testing.T) {
		ctx := context.Background()
		var executed atomic.Bool

		seq := func(yield func(int) bool) {
			executed.Store(true)
			yield(1)
		}

		buffered := WithBuffer(ctx, seq, 5)

		// Just creating the buffered sequence shouldn't execute it
		time.Sleep(10 * time.Millisecond)
		if executed.Load() {
			t.Error("Sequence should not execute until iteration starts")
		}

		// Start iteration
		for range buffered {
			break
		}

		// Now it should have executed
		time.Sleep(10 * time.Millisecond)
		if !executed.Load() {
			t.Error("Sequence should execute when iteration starts")
		}
	})

	t.Run("MultipleIterations", func(t *testing.T) {
		ctx := context.Background()
		var setupCount atomic.Int32

		seq := func(yield func(int) bool) {
			setupCount.Add(1)
			for i := 1; i <= 3; i++ {
				if !yield(i) {
					return
				}
			}
		}

		buffered := WithBuffer(ctx, seq, 2)

		// First iteration
		result1 := Collect(buffered)
		if !slices.Equal(result1, []int{1, 2, 3}) {
			t.Errorf("First iteration got %v, want [1 2 3]", result1)
		}

		// Second iteration should reuse the same setup
		result2 := Collect(buffered)
		if !slices.Equal(result2, []int{1, 2, 3}) {
			t.Errorf("Second iteration got %v, want [1 2 3]", result2)
		}

		// Setup should happen only once per iteration
		time.Sleep(20 * time.Millisecond)
		if setupCount.Load() != 2 {
			t.Errorf("Setup called %d times, want 2 (once per iteration)", setupCount.Load())
		}
	})

	t.Run("BufferFillsUpBeforeConsumption", func(t *testing.T) {
		ctx := context.Background()
		bufferSize := 5
		var itemsProduced atomic.Int32
		var firstItemConsumed atomic.Bool
		var itemsProducedBeforeFirstConsume int32

		seq := func(yield func(int) bool) {
			for i := 1; i <= 20; i++ {
				itemsProduced.Add(1)
				if !yield(i) {
					return
				}
				// Check if we've consumed anything yet
				if !firstItemConsumed.Load() && i < 20 {
					time.Sleep(5 * time.Millisecond) // Give time for buffer to fill
				}
			}
		}

		buffered := WithBuffer(ctx, seq, bufferSize)

		// Wait a bit before consuming to let buffer fill
		time.Sleep(50 * time.Millisecond)

		var consumed []int
		for val := range buffered {
			if !firstItemConsumed.Load() {
				firstItemConsumed.Store(true)
				itemsProducedBeforeFirstConsume = itemsProduced.Load()
			}
			consumed = append(consumed, val)
		}

		// Buffer should have allowed producer to run ahead
		if itemsProducedBeforeFirstConsume <= int32(bufferSize) {
			t.Logf("Items produced before first consume: %d (buffer size: %d)",
				itemsProducedBeforeFirstConsume, bufferSize)
		}

		if !slices.Equal(consumed, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}) {
			t.Errorf("Consumed incorrect values: %v", consumed)
		}
	})
}

func TestWithMutex(t *testing.T) {
	t.Run("BasicIteration", func(t *testing.T) {
		seq := Range(1, 5)
		mu := &sync.Mutex{}

		wrapped := WithMutex(seq, mu)
		result := Collect(wrapped)

		if len(result) != 5 {
			t.Errorf("expected 5 elements, got %d", len(result))
		}

		for i, v := range result {
			if v != i+1 {
				t.Errorf("expected element %d to be %d, got %d", i, i+1, v)
			}
		}
	})

	t.Run("MultipleIterations", func(t *testing.T) {
		// WithMutex allows the sequence to be safely iterated multiple times
		// or from different goroutines sequentially
		seq := Range(1, 10)
		mu := &sync.Mutex{}

		wrapped := WithMutex(seq, mu)

		// First iteration
		result1 := Collect(wrapped)

		if len(result1) != 10 {
			t.Errorf("first iteration: expected 10 elements, got %d", len(result1))
		}

		for i, v := range result1 {
			if v != i+1 {
				t.Errorf("first iteration: expected element %d to be %d, got %d", i, i+1, v)
			}
		}

		// The sequence is exhausted after first iteration, so second would be empty
		// unless we create a new wrapped sequence
		seq2 := Range(1, 10)
		wrapped2 := WithMutex(seq2, mu)
		result2 := Collect(wrapped2)

		if len(result2) != 10 {
			t.Errorf("second iteration: expected 10 elements, got %d", len(result2))
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		seq := Limit(Range(1, 10), 0)
		mu := &sync.Mutex{}

		wrapped := WithMutex(seq, mu)
		result := Collect(wrapped)

		if len(result) != 0 {
			t.Errorf("expected 0 elements, got %d", len(result))
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		seq := Range(1, 100)
		mu := &sync.Mutex{}

		wrapped := WithMutex(seq, mu)

		// Only take first 5 elements
		result := CollectFirstN(wrapped, 5)

		if len(result) != 5 {
			t.Errorf("expected 5 elements, got %d", len(result))
		}

		for i, v := range result {
			if v != i+1 {
				t.Errorf("expected element %d to be %d, got %d", i, i+1, v)
			}
		}
	})
}

func TestWithMutex2(t *testing.T) {
	t.Run("BasicIteration", func(t *testing.T) {
		seq := Index(Range(1, 5))
		mu := &sync.Mutex{}

		wrapped := WithMutex2(seq, mu)
		result := Collect2(wrapped)

		if len(result) != 5 {
			t.Errorf("expected 5 pairs, got %d", len(result))
		}

		for i := 0; i < 5; i++ {
			if result[i] != i+1 {
				t.Errorf("expected result[%d] to be %d, got %d", i, i+1, result[i])
			}
		}
	})

	t.Run("MultipleIterations", func(t *testing.T) {
		seq := Index(Range(1, 10))
		mu := &sync.Mutex{}

		wrapped := WithMutex2(seq, mu)

		// First iteration
		result1 := Collect2(wrapped)

		if len(result1) != 10 {
			t.Errorf("first iteration: expected 10 pairs, got %d", len(result1))
		}

		for i := 0; i < 10; i++ {
			if result1[i] != i+1 {
				t.Errorf("first iteration: expected result1[%d] to be %d, got %d", i, i+1, result1[i])
			}
		}

		// The sequence is exhausted after first iteration
		seq2 := Index(Range(1, 10))
		wrapped2 := WithMutex2(seq2, mu)
		result2 := Collect2(wrapped2)

		if len(result2) != 10 {
			t.Errorf("second iteration: expected 10 pairs, got %d", len(result2))
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		seq := Index(Limit(Range(1, 10), 0))
		mu := &sync.Mutex{}

		wrapped := WithMutex2(seq, mu)
		result := Collect2(wrapped)

		if len(result) != 0 {
			t.Errorf("expected 0 pairs, got %d", len(result))
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		seq := Index(Range(1, 100))
		mu := &sync.Mutex{}

		wrapped := WithMutex2(seq, mu)

		// Only take first 5 pairs
		count := 0
		result := make(map[int]int)
		for k, v := range wrapped {
			result[k] = v
			count++
			if count >= 5 {
				break
			}
		}

		if len(result) != 5 {
			t.Errorf("expected 5 pairs, got %d", len(result))
		}

		for i := 0; i < 5; i++ {
			if result[i] != i+1 {
				t.Errorf("expected result[%d] to be %d, got %d", i, i+1, result[i])
			}
		}
	})
}

func TestWith3(t *testing.T) {
	t.Run("BasicOperation", func(t *testing.T) {
		var callCount atomic.Int32
		op := func(i int) (string, bool) {
			callCount.Add(1)
			return "item" + string(rune('0'+i)), i%2 == 0
		}

		seq := With3(func(yield func(int) bool) {
			if !yield(1) {
				return
			}
			if !yield(2) {
				return
			}
			yield(3)
		}, op)

		type result struct {
			elem  KV[int, string]
			check bool
		}

		expected := []result{
			{MakeKV(1, "item1"), false},
			{MakeKV(2, "item2"), true},
			{MakeKV(3, "item3"), false},
		}

		i := 0
		for elem, c := range seq {
			if i >= len(expected) {
				t.Errorf("With3() yielded more items than expected")
				break
			}
			if elem.Key != expected[i].elem.Key || elem.Value != expected[i].elem.Value {
				t.Errorf("With3() yielded elem (%v, %v), want (%v, %v)",
					elem.Key, elem.Value, expected[i].elem.Key, expected[i].elem.Value)
			}
			if c != expected[i].check {
				t.Errorf("With3() yielded check %v, want %v", c, expected[i].check)
			}
			i++
		}

		if i != len(expected) {
			t.Errorf("With3() yielded %d items, want %d", i, len(expected))
		}

		if callCount.Load() != 3 {
			t.Errorf("With3() called op %d times, want 3", callCount.Load())
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		var callCount atomic.Int32
		op := func(i int) (string, int) {
			callCount.Add(1)
			return "value", i * 2
		}

		seq := With3(Slice([]int{}), op)

		count := 0
		for range seq {
			count++
		}

		if count != 0 {
			t.Errorf("expected 0, got %v", count)
		}
		if callCount.Load() != int32(0) {
			t.Errorf("expected int32(0), got %v", callCount.Load())
		}
	})

	t.Run("SingleElement", func(t *testing.T) {
		op := func(i int) (int, int) {
			return i * i, i * i * i
		}

		seq := With3(Slice([]int{5}), op)

		var results []struct {
			elem  KV[int, int]
			third int
		}

		for elem, third := range seq {
			results = append(results, struct {
				elem  KV[int, int]
				third int
			}{elem, third})
		}

		if len(results) != 1 {
			t.Errorf("expected 1, got %v", len(results))
		}
		if results[0].elem.Key != 5 {
			t.Errorf("expected 5, got %v", results[0].elem.Key)
		} // original input
		if results[0].elem.Value != 25 {
			t.Errorf("expected 25, got %v", results[0].elem.Value)
		} // 5^2
		if results[0].third != 125 {
			t.Errorf("expected 125, got %v", results[0].third)
		} // 5^3
	})

	t.Run("EarlyReturn", func(t *testing.T) {
		var callCount atomic.Int32
		op := func(i int) (int, bool) {
			callCount.Add(1)
			return i * 10, i > 2
		}

		seq := With3(Slice([]int{1, 2, 3, 4, 5}), op)

		count := 0
		for _, shouldStop := range seq {
			count++
			if shouldStop {
				break
			}
		}

		if count != 3 {
			t.Errorf("expected 3, got %v", count)
		} // Should process 1, 2, 3 and stop at 3
		if callCount.Load() != int32(3) {
			t.Errorf("expected int32(3, got %v", callCount.Load())
		}
	})

	t.Run("MultipleTypes", func(t *testing.T) {
		op := func(name string) (int, bool) {
			return len(name), len(name) > 5
		}

		seq := With3(
			Slice([]string{"Alice", "Bob", "Charlotte", "Dan"}),
			op,
		)

		results := Collect(KVjoin(seq))

		if len(results) != 4 {
			t.Errorf("expected 4, got %v", len(results))
		}

		// First result
		if results[0].Key.Key != "Alice" {
			t.Errorf("expected %v, got %v", "Alice", results[0].Key.Key)
		}
		if results[0].Key.Value != 5 {
			t.Errorf("expected 5, got %v", results[0].Key.Value)
		}
		if results[0].Value != false {
			t.Errorf("expected false, got %v", results[0].Value)
		}

		// Second result
		if results[1].Key.Key != "Bob" {
			t.Errorf("expected %v, got %v", "Bob", results[1].Key.Key)
		}
		if results[1].Key.Value != 3 {
			t.Errorf("expected 3, got %v", results[1].Key.Value)
		}
		if results[1].Value != false {
			t.Errorf("expected false, got %v", results[1].Value)
		}

		// Third result
		if results[2].Key.Key != "Charlotte" {
			t.Errorf("expected %v, got %v", "Charlotte", results[2].Key.Key)
		}
		if results[2].Key.Value != 9 {
			t.Errorf("expected 9, got %v", results[2].Key.Value)
		}
		if results[2].Value != true {
			t.Errorf("expected true, got %v", results[2].Value)
		}

		// Fourth result
		if results[3].Key.Key != "Dan" {
			t.Errorf("expected %v, got %v", "Dan", results[3].Key.Key)
		}
		if results[3].Key.Value != 3 {
			t.Errorf("expected 3, got %v", results[3].Key.Value)
		}
		if results[3].Value != false {
			t.Errorf("expected false, got %v", results[3].Value)
		}
	})

	t.Run("OperationCalledForEachElement", func(t *testing.T) {
		callOrder := []int{}
		op := func(i int) (string, int) {
			callOrder = append(callOrder, i)
			return "x", i
		}

		seq := With3(Slice([]int{10, 20, 30}), op)

		count := 0
		for range seq {
			count++
		}

		if count != 3 {
			t.Errorf("expected 3, got %v", count)
		}
		if len(callOrder) != 3 {
			t.Errorf("expected 3, got %v", len(callOrder))
		}
		if callOrder[0] != 10 {
			t.Errorf("expected 10, got %v", callOrder[0])
		}
		if callOrder[1] != 20 {
			t.Errorf("expected 20, got %v", callOrder[1])
		}
		if callOrder[2] != 30 {
			t.Errorf("expected 30, got %v", callOrder[2])
		}
	})

	t.Run("PreservesOrder", func(t *testing.T) {
		op := func(i int) (int, int) {
			return i * 2, i * 3
		}

		seq := With3(Range(1, 6), op)

		results := Collect(KVjoin(seq))

		for i := 0; i < 5; i++ {
			expectedInput := i + 1
			if results[i].Key.Key != expectedInput {
				t.Errorf("expected expectedInput, got %v", results[i].Key.Key)
			}
			if results[i].Key.Value != expectedInput*2 {
				t.Errorf("expected expectedInput*2, got %v", results[i].Key.Value)
			}
			if results[i].Value != expectedInput*3 {
				t.Errorf("expected expectedInput*3, got %v", results[i].Value)
			}
		}
	})

	t.Run("WithPointers", func(t *testing.T) {
		op := func(i int) (*int, *string) {
			val := i * 2
			str := "test"
			return &val, &str
		}

		seq := With3(Slice([]int{1, 2}), op)

		results := Collect(KVjoin(seq))

		if len(results) != 2 {
			t.Errorf("expected 2, got %v", len(results))
		}
		if results[0].Key.Key != 1 {
			t.Errorf("expected 1, got %v", results[0].Key.Key)
		}
		if *results[0].Key.Value != 2 {
			t.Errorf("expected 2, got %v", *results[0].Key.Value)
		}
		if *results[0].Value != "test" {
			t.Errorf("expected %v, got %v", "test", *results[0].Value)
		}

		if results[1].Key.Key != 2 {
			t.Errorf("expected 2, got %v", results[1].Key.Key)
		}
		if *results[1].Key.Value != 4 {
			t.Errorf("expected 4, got %v", *results[1].Key.Value)
		}
		if *results[1].Value != "test" {
			t.Errorf("expected %v, got %v", "test", *results[1].Value)
		}
	})

	t.Run("WithNilReturns", func(t *testing.T) {
		op := func(i int) (*int, *string) {
			if i%2 == 0 {
				return nil, nil
			}
			val := i
			str := "odd"
			return &val, &str
		}

		seq := With3(Slice([]int{1, 2, 3, 4}), op)

		results := Collect(KVjoin(seq))

		if len(results) != 4 {
			t.Errorf("expected 4, got %v", len(results))
		}

		// Odd numbers
		if results[0].Key.Key != 1 {
			t.Errorf("expected 1, got %v", results[0].Key.Key)
		}
		if !(results[0].Key.Value != nil) {
			t.Error("expected condition to be true")
		}
		if !(results[0].Value != nil) {
			t.Error("expected condition to be true")
		}

		// Even numbers
		if results[1].Key.Key != 2 {
			t.Errorf("expected 2, got %v", results[1].Key.Key)
		}
		if !(results[1].Key.Value == nil) {
			t.Error("expected condition to be true")
		}
		if !(results[1].Value == nil) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("LargeSequence", func(t *testing.T) {
		op := func(i int) (int, bool) {
			return i * i, i > 500
		}

		seq := With3(Range(0, 999), op)

		count := 0
		for elem, isLarge := range seq {
			count++
			if elem.Key <= 500 {
				if !(!isLarge) {
					t.Error("expected condition to be true")
				}
			} else {
				if !(isLarge) {
					t.Error("expected condition to be true")
				}
			}
		}

		if count != 1000 {
			t.Errorf("expected 1000, got %v", count)
		}
	})
}

func TestKVmap(t *testing.T) {
	t.Run("EmptyMap", func(t *testing.T) {
		input := map[int]string{}
		result := Collect(KVmap(input))

		if len(result) != 0 {
			t.Errorf("expected 0, got %v", len(result))
		}
	})

	t.Run("SingleElement", func(t *testing.T) {
		input := map[string]int{"a": 1}
		result := Collect(KVmap(input))

		if len(result) != 1 {
			t.Errorf("expected 1, got %v", len(result))
		}
		if result[0].Key != "a" {
			t.Errorf("expected %v, got %v", "a", result[0].Key)
		}
		if result[0].Value != 1 {
			t.Errorf("expected 1, got %v", result[0].Value)
		}
	})

	t.Run("MultipleElements", func(t *testing.T) {
		input := map[string]int{
			"a": 1,
			"b": 2,
			"c": 3,
		}
		result := Collect(KVmap(input))

		if len(result) != 3 {
			t.Errorf("expected 3, got %v", len(result))
		}

		// Convert to map to verify all elements are present (map iteration order is not guaranteed)
		resultMap := make(map[string]int)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("IntToString", func(t *testing.T) {
		input := map[int]string{
			1: "one",
			2: "two",
			3: "three",
		}
		result := Collect(KVmap(input))

		if len(result) != 3 {
			t.Errorf("expected 3, got %v", len(result))
		}

		resultMap := make(map[int]string)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("StringToString", func(t *testing.T) {
		input := map[string]string{
			"hello":   "world",
			"foo":     "bar",
			"testing": "123",
		}
		result := Collect(KVmap(input))

		if len(result) != 3 {
			t.Errorf("expected 3, got %v", len(result))
		}

		resultMap := make(map[string]string)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("LargeMap", func(t *testing.T) {
		input := make(map[int]int)
		for i := 0; i < 100; i++ {
			input[i] = i * 2
		}

		result := Collect(KVmap(input))

		if len(result) != 100 {
			t.Errorf("expected 100, got %v", len(result))
		}

		resultMap := make(map[int]int)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		input := map[int]string{
			1: "a",
			2: "b",
			3: "c",
			4: "d",
			5: "e",
		}

		count := 0
		for range KVmap(input) {
			count++
			if count == 3 {
				break
			}
		}

		if count != 3 {
			t.Errorf("expected 3, got %v", count)
		}
	})

	t.Run("IteratorReuse", func(t *testing.T) {
		input := map[string]int{
			"x": 10,
			"y": 20,
			"z": 30,
		}

		seq := KVmap(input)

		// First iteration
		result1 := Collect(seq)
		if len(result1) != 3 {
			t.Errorf("expected 3, got %v", len(result1))
		}

		// Second iteration (iterator should be reusable)
		result2 := Collect(seq)
		if len(result2) != 3 {
			t.Errorf("expected 3, got %v", len(result2))
		}

		// Both iterations should produce the same elements
		map1 := make(map[string]int)
		for _, kv := range result1 {
			map1[kv.Key] = kv.Value
		}

		map2 := make(map[string]int)
		for _, kv := range result2 {
			map2[kv.Key] = kv.Value
		}

		if !(maps.Equal(map1, map2)) {
			t.Error("expected condition to be true")
		}
		if !(maps.Equal(input, map1)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("KVStructureCorrectness", func(t *testing.T) {
		input := map[int]string{42: "answer"}
		result := Collect(KVmap(input))

		if len(result) != 1 {
			t.Errorf("expected 1, got %v", len(result))
		}
		kv := result[0]
		if kv.Key != 42 {
			t.Errorf("expected 42, got %v", kv.Key)
		}
		if kv.Value != "answer" {
			t.Errorf("expected %v, got %v", "answer", kv.Value)
		}

		// Test that Split() works correctly
		key, value := kv.Split()
		if key != 42 {
			t.Errorf("expected 42, got %v", key)
		}
		if value != "answer" {
			t.Errorf("expected %v, got %v", "answer", value)
		}
	})

	t.Run("WithPointerValues", func(t *testing.T) {
		val1 := 100
		val2 := 200
		input := map[string]*int{
			"a": &val1,
			"b": &val2,
		}

		result := Collect(KVmap(input))

		if len(result) != 2 {
			t.Errorf("expected 2, got %v", len(result))
		}

		resultMap := make(map[string]*int)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if len(resultMap) != 2 {
			t.Errorf("expected 2, got %v", len(resultMap))
		}
		if *resultMap["a"] != 100 {
			t.Errorf("expected 100, got %v", *resultMap["a"])
		}
		if *resultMap["b"] != 200 {
			t.Errorf("expected 200, got %v", *resultMap["b"])
		}
	})

	t.Run("ConvertBackToMap", func(t *testing.T) {
		input := map[int]string{
			1: "one",
			2: "two",
			3: "three",
		}

		// KVmap -> Collect -> KVsplit -> Collect2
		result := Collect2(KVsplit(Slice(Collect(KVmap(input)))))

		if !(maps.Equal(input, result)) {
			t.Error("expected condition to be true")
		}
	})
}

func TestMapKV(t *testing.T) {
	t.Run("EmptyMap", func(t *testing.T) {
		input := map[int]string{}
		result := Collect(MapKV(input))

		if len(result) != 0 {
			t.Errorf("expected 0, got %v", len(result))
		}
	})

	t.Run("SingleElement", func(t *testing.T) {
		input := map[string]int{"a": 1}
		result := Collect(MapKV(input))

		if len(result) != 1 {
			t.Errorf("expected 1, got %v", len(result))
		}
		if result[0].Key != "a" {
			t.Errorf("expected %v, got %v", "a", result[0].Key)
		}
		if result[0].Value != 1 {
			t.Errorf("expected 1, got %v", result[0].Value)
		}
	})

	t.Run("MultipleElements", func(t *testing.T) {
		input := map[string]int{
			"a": 1,
			"b": 2,
			"c": 3,
		}
		result := Collect(MapKV(input))

		if len(result) != 3 {
			t.Errorf("expected 3, got %v", len(result))
		}

		// Convert to map to verify all elements are present
		resultMap := make(map[string]int)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("IntToString", func(t *testing.T) {
		input := map[int]string{
			1: "one",
			2: "two",
			3: "three",
		}
		result := Collect(MapKV(input))

		if len(result) != 3 {
			t.Errorf("expected 3, got %v", len(result))
		}

		resultMap := make(map[int]string)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("StringToString", func(t *testing.T) {
		input := map[string]string{
			"hello":   "world",
			"foo":     "bar",
			"testing": "123",
		}
		result := Collect(MapKV(input))

		if len(result) != 3 {
			t.Errorf("expected 3, got %v", len(result))
		}

		resultMap := make(map[string]string)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("LargeMap", func(t *testing.T) {
		input := make(map[int]int)
		for i := 0; i < 100; i++ {
			input[i] = i * 2
		}

		result := Collect(MapKV(input))

		if len(result) != 100 {
			t.Errorf("expected 100, got %v", len(result))
		}

		resultMap := make(map[int]int)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(input, resultMap)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		input := map[int]string{
			1: "a",
			2: "b",
			3: "c",
			4: "d",
			5: "e",
		}

		count := 0
		for range MapKV(input) {
			count++
			if count == 3 {
				break
			}
		}

		if count != 3 {
			t.Errorf("expected 3, got %v", count)
		}
	})

	t.Run("IteratorReuse", func(t *testing.T) {
		input := map[string]int{
			"x": 10,
			"y": 20,
			"z": 30,
		}

		seq := MapKV(input)

		// First iteration
		result1 := Collect(seq)
		if len(result1) != 3 {
			t.Errorf("expected 3, got %v", len(result1))
		}

		// Second iteration (iterator should be reusable)
		result2 := Collect(seq)
		if len(result2) != 3 {
			t.Errorf("expected 3, got %v", len(result2))
		}

		// Both iterations should produce the same elements
		map1 := make(map[string]int)
		for _, kv := range result1 {
			map1[kv.Key] = kv.Value
		}

		map2 := make(map[string]int)
		for _, kv := range result2 {
			map2[kv.Key] = kv.Value
		}

		if !(maps.Equal(map1, map2)) {
			t.Error("expected condition to be true")
		}
		if !(maps.Equal(input, map1)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("KVStructureCorrectness", func(t *testing.T) {
		input := map[int]string{42: "answer"}
		result := Collect(MapKV(input))

		if len(result) != 1 {
			t.Errorf("expected 1, got %v", len(result))
		}
		kv := result[0]
		if kv.Key != 42 {
			t.Errorf("expected 42, got %v", kv.Key)
		}
		if kv.Value != "answer" {
			t.Errorf("expected %v, got %v", "answer", kv.Value)
		}

		// Test that Split() works correctly
		key, value := kv.Split()
		if key != 42 {
			t.Errorf("expected 42, got %v", key)
		}
		if value != "answer" {
			t.Errorf("expected %v, got %v", "answer", value)
		}
	})

	t.Run("WithPointerValues", func(t *testing.T) {
		val1 := 100
		val2 := 200
		input := map[string]*int{
			"a": &val1,
			"b": &val2,
		}

		result := Collect(MapKV(input))

		if len(result) != 2 {
			t.Errorf("expected 2, got %v", len(result))
		}

		resultMap := make(map[string]*int)
		for _, kv := range result {
			resultMap[kv.Key] = kv.Value
		}

		if len(resultMap) != 2 {
			t.Errorf("expected 2, got %v", len(resultMap))
		}
		if *resultMap["a"] != 100 {
			t.Errorf("expected 100, got %v", *resultMap["a"])
		}
		if *resultMap["b"] != 200 {
			t.Errorf("expected 200, got %v", *resultMap["b"])
		}
	})

	t.Run("ConvertBackToMap", func(t *testing.T) {
		input := map[int]string{
			1: "one",
			2: "two",
			3: "three",
		}

		// MapKV -> Collect -> KVsplit -> Collect2
		result := Collect2(KVsplit(Slice(Collect(MapKV(input)))))

		if !(maps.Equal(input, result)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("KVmapAndMapKVEquivalence", func(t *testing.T) {
		input := map[string]int{
			"alpha": 1,
			"beta":  2,
			"gamma": 3,
		}

		// Both should produce the same results
		kvmapResult := Collect(KVmap(input))
		mapkvResult := Collect(MapKV(input))

		if len(kvmapResult) != len(mapkvResult) {
			t.Errorf("expected len(mapkvResult, got %v", len(kvmapResult))
		}

		// Convert both to maps for comparison
		kvmapMap := make(map[string]int)
		for _, kv := range kvmapResult {
			kvmapMap[kv.Key] = kv.Value
		}

		mapkvMap := make(map[string]int)
		for _, kv := range mapkvResult {
			mapkvMap[kv.Key] = kv.Value
		}

		if !(maps.Equal(kvmapMap, mapkvMap)) {
			t.Error("expected condition to be true")
		}
		if !(maps.Equal(input, kvmapMap)) {
			t.Error("expected condition to be true")
		}
		if !(maps.Equal(input, mapkvMap)) {
			t.Error("expected condition to be true")
		}
	})
}

func TestCall(t *testing.T) {
	t.Run("LazyExecution", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func() int { callCount.Add(1); return 1 },
			func() int { callCount.Add(1); return 2 },
			func() int { callCount.Add(1); return 3 },
		)

		seq := Resolve(funcs)

		// Before iteration, no functions should be called
		if int32(0) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", 0)
		}

		// Iterate
		result := Collect(seq)

		// After iteration, all functions should be called
		if int32(3) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(3))
		}
		if !(slices.Equal([]int{1, 2, 3}, result)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("AllFunctionsRun", func(t *testing.T) {
		called := make([]bool, 5)

		funcs := Args(
			func() int { called[0] = true; return 10 },
			func() int { called[1] = true; return 20 },
			func() int { called[2] = true; return 30 },
			func() int { called[3] = true; return 40 },
			func() int { called[4] = true; return 50 },
		)

		result := Collect(Resolve(funcs))

		for i, c := range called {
			if !(c) {
				t.Error("expected condition to be true")
			}
			if !c {
				t.Errorf("function %d was not called", i)
			}
		}
		if !(slices.Equal([]int{10, 20, 30, 40, 50}, result)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func() int { callCount.Add(1); return 1 },
			func() int { callCount.Add(1); return 2 },
			func() int { callCount.Add(1); return 3 },
			func() int { callCount.Add(1); return 4 },
			func() int { callCount.Add(1); return 5 },
		)

		// Only take first 2
		result := Collect(Limit(Resolve(funcs), 2))

		if int32(2) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(2))
		}
		if !(slices.Equal([]int{1, 2}, result)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		funcs := func(yield func(func() int) bool) {}
		result := Collect(Resolve(funcs))
		if 0 != len(result) {
			t.Errorf("expected len(result, got %v", 0)
		}
	})
}

func TestCall2(t *testing.T) {
	t.Run("LazyExecution", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func() (string, int) { callCount.Add(1); return "a", 1 },
			func() (string, int) { callCount.Add(1); return "b", 2 },
			func() (string, int) { callCount.Add(1); return "c", 3 },
		)

		seq := Resolve2(funcs)

		// Before iteration, no functions should be called
		if int32(0) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(0))
		}

		// Iterate
		result := Collect2(seq)

		// After iteration, all functions should be called
		if int32(3) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(3))
		}
		if 1 != result["a"] {
			t.Errorf("expected %v, got %v", result["a"], 1)
		}
		if 2 != result["b"] {
			t.Errorf("expected %v, got %v", result["b"], 2)
		}
		if 3 != result["c"] {
			t.Errorf("expected %v, got %v", result["c"], 3)
		}
	})

	t.Run("AllFunctionsRun", func(t *testing.T) {
		called := make([]bool, 4)

		funcs := Args(
			func() (int, string) { called[0] = true; return 0, "zero" },
			func() (int, string) { called[1] = true; return 1, "one" },
			func() (int, string) { called[2] = true; return 2, "two" },
			func() (int, string) { called[3] = true; return 3, "three" },
		)

		result := Collect2(Resolve2(funcs))

		for i, c := range called {
			if !(c) {
				t.Error("expected condition to be true")
			}
			if !c {
				t.Errorf("function %d was not called", i)
			}
		}
		if 4 != len(result) {
			t.Errorf("expected len(result, got %v", 4)
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func() (int, int) { callCount.Add(1); return 1, 10 },
			func() (int, int) { callCount.Add(1); return 2, 20 },
			func() (int, int) { callCount.Add(1); return 3, 30 },
			func() (int, int) { callCount.Add(1); return 4, 40 },
		)

		// Only take first 2
		result := Collect2(Limit2(Resolve2(funcs), 2))

		if int32(2) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(2))
		}
		if 2 != len(result) {
			t.Errorf("expected len(result, got %v", 2)
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		funcs := func(yield func(func() (string, int)) bool) {}
		result := Collect2(Resolve2(funcs))
		if 0 != len(result) {
			t.Errorf("expected len(result, got %v", 0)
		}
	})
}

func TestCallWrap(t *testing.T) {
	t.Run("LazyExecution", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func(n int) int { callCount.Add(1); return n * 1 },
			func(n int) int { callCount.Add(1); return n * 2 },
			func(n int) int { callCount.Add(1); return n * 3 },
		)

		seq := ResolveWrap(funcs, 10)

		// Before iteration, no functions should be called
		if int32(0) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(0))
		}

		// Iterate
		result := Collect(seq)

		// After iteration, all functions should be called
		if int32(3) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(3))
		}
		if !slices.Equal([]int{10, 20, 30}, result) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("AllFunctionsRun", func(t *testing.T) {
		called := make([]bool, 5)

		funcs := Args(
			func(s string) string { called[0] = true; return s + "0" },
			func(s string) string { called[1] = true; return s + "1" },
			func(s string) string { called[2] = true; return s + "2" },
			func(s string) string { called[3] = true; return s + "3" },
			func(s string) string { called[4] = true; return s + "4" },
		)

		result := Collect(ResolveWrap(funcs, "x"))

		for i, c := range called {
			if !(c) {
				t.Error("expected condition to be true")
			}
			if !c {
				t.Errorf("function %d was not called", i)
			}
		}
		if !slices.Equal([]string{"x0", "x1", "x2", "x3", "x4"}, result) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func(n int) int { callCount.Add(1); return n + 1 },
			func(n int) int { callCount.Add(1); return n + 2 },
			func(n int) int { callCount.Add(1); return n + 3 },
			func(n int) int { callCount.Add(1); return n + 4 },
			func(n int) int { callCount.Add(1); return n + 5 },
		)

		// Only take first 3
		result := Collect(Limit(ResolveWrap(funcs, 100), 3))

		if int32(3) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(3))
		}
		if !(slices.Equal([]int{101, 102, 103}, result)) {
			t.Error("expected condition to be true")
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		funcs := func(yield func(func(int) int) bool) {}
		result := Collect(ResolveWrap(funcs, 42))
		if 0 != len(result) {
			t.Errorf("expected len(result, got %v", 0)
		}
	})

	t.Run("WrappingValuePassedCorrectly", func(t *testing.T) {
		var receivedValues []int

		funcs := Args(
			func(n int) int { receivedValues = append(receivedValues, n); return n },
			func(n int) int { receivedValues = append(receivedValues, n); return n },
			func(n int) int { receivedValues = append(receivedValues, n); return n },
		)

		_ = Collect(ResolveWrap(funcs, 99))

		if !slices.Equal([]int{99, 99, 99}, receivedValues) {
			t.Error("expected condition to be true")
		}
	})
}

func TestCallWrap2(t *testing.T) {
	t.Run("LazyExecution", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func(n int) (string, int) { callCount.Add(1); return "a", n * 1 },
			func(n int) (string, int) { callCount.Add(1); return "b", n * 2 },
			func(n int) (string, int) { callCount.Add(1); return "c", n * 3 },
		)

		seq := ResolveWrap2(funcs, 10)

		// Before iteration, no functions should be called
		if int32(0) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(0))
		}

		// Iterate
		result := Collect2(seq)

		// After iteration, all functions should be called
		if int32(3) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(3))
		}
		if 10 != result["a"] {
			t.Errorf("expected %v, got %v", result["a"], 10)
		}
		if 20 != result["b"] {
			t.Errorf("expected %v, got %v", result["b"], 20)
		}
		if 30 != result["c"] {
			t.Errorf("expected %v, got %v", result["c"], 30)
		}
	})

	t.Run("AllFunctionsRun", func(t *testing.T) {
		called := make([]bool, 4)

		funcs := Args(
			func(s string) (int, string) { called[0] = true; return 0, s + "!" },
			func(s string) (int, string) { called[1] = true; return 1, s + "?" },
			func(s string) (int, string) { called[2] = true; return 2, s + "." },
			func(s string) (int, string) { called[3] = true; return 3, s + "," },
		)

		result := Collect2(ResolveWrap2(funcs, "test"))

		for i, c := range called {
			if !(c) {
				t.Error("expected condition to be true")
			}
			if !c {
				t.Errorf("function %d was not called", i)
			}
		}
		if 4 != len(result) {
			t.Errorf("expected len(result, got %v", 4)
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		var callCount atomic.Int32

		funcs := Args(
			func(n int) (int, int) { callCount.Add(1); return 1, n * 1 },
			func(n int) (int, int) { callCount.Add(1); return 2, n * 2 },
			func(n int) (int, int) { callCount.Add(1); return 3, n * 3 },
			func(n int) (int, int) { callCount.Add(1); return 4, n * 4 },
		)

		// Only take first 2
		result := Collect2(Limit2(ResolveWrap2(funcs, 5), 2))

		if int32(2) != callCount.Load() {
			t.Errorf("expected callCount.Load(, got %v", int32(2))
		}
		if 2 != len(result) {
			t.Errorf("expected len(result, got %v", 2)
		}
		if 5 != result[1] {
			t.Errorf("expected result[1], got %v", 5)
		}
		if 10 != result[2] {
			t.Errorf("expected result[2], got %v", 10)
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		funcs := func(yield func(func(int) (string, int)) bool) {}
		result := Collect2(ResolveWrap2(funcs, 42))
		if 0 != len(result) {
			t.Errorf("expected len(result, got %v", 0)
		}
	})

	t.Run("WrappingValuePassedCorrectly", func(t *testing.T) {
		var receivedValues []string

		funcs := Args(
			func(s string) (int, int) { receivedValues = append(receivedValues, s); return 0, 0 },
			func(s string) (int, int) { receivedValues = append(receivedValues, s); return 1, 1 },
			func(s string) (int, int) { receivedValues = append(receivedValues, s); return 2, 2 },
		)

		_ = Collect2(ResolveWrap2(funcs, "wrapped"))

		if !slices.Equal([]string{"wrapped", "wrapped", "wrapped"}, receivedValues) {
			t.Error("expected condition to be true")
		}
	})
}
