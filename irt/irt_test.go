package irt

import (
	"context"
	"errors"
	"iter"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollect(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		args     []int
		expected []int
	}{
		{
			name:     "empty sequence",
			seq:      func(yield func(int) bool) {},
			expected: []int{},
		},
		{
			name:     "simple sequence",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			expected: []int{1, 2, 3},
		},
		{
			name:     "with initial capacity",
			seq:      func(yield func(int) bool) { yield(1); yield(2) },
			args:     []int{5},
			expected: []int{1, 2},
		},
		{
			name:     "with initial capacity and length",
			seq:      func(yield func(int) bool) { yield(1); yield(2) },
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
			name:     "empty sequence",
			seq:      func(yield func(string, int) bool) {},
			expected: map[string]int{},
		},
		{
			name: "simple sequence",
			seq: func(yield func(string, int) bool) {
				yield("a", 1)
				yield("b", 2)
			},
			expected: map[string]int{"a": 1, "b": 2},
		},
		{
			name: "with initial capacity",
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
	}{
		{
			name:     "empty sequence",
			seq:      func(yield func(int) bool) {},
			n:        3,
			expected: []int{},
		},
		{
			name:     "exact n elements",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			n:        3,
			expected: []int{1, 2, 3},
		},
		{
			name:     "more than n elements",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3); yield(4) },
			n:        2,
			expected: []int{1, 2},
		},
		{
			name:     "less than n elements",
			seq:      func(yield func(int) bool) { yield(1) },
			n:        3,
			expected: []int{1},
		},
		{
			name:     "n is zero",
			seq:      func(yield func(int) bool) { yield(1); yield(2) },
			n:        0,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CollectFirstN(tt.seq, tt.n)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("CollectFirstN() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestOne(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected []interface{}
	}{
		{"int", 42, []interface{}{42}},
		{"string", "hello", []interface{}{"hello"}},
		{"nil", nil, []interface{}{nil}},
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
		a, b interface{}
	}{
		{"int-string", 42, "hello"},
		{"string-int", "world", 123},
		{"nil-nil", nil, nil},
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
		{"empty", map[string]int{}},
		{"single", map[string]int{"a": 1}},
		{"multiple", map[string]int{"a": 1, "b": 2, "c": 3}},
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
		{"empty", []int{}},
		{"single", []int{1}},
		{"multiple", []int{1, 2, 3, 4}},
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
		{"empty", []int{}},
		{"single", []int{1}},
		{"multiple", []int{1, 2, 3}},
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
			name: "empty",
			seq:  func(yield func(string) bool) {},
			expected: []struct {
				idx int
				val string
			}{},
		},
		{
			name: "single",
			seq:  func(yield func(string) bool) { yield("a") },
			expected: []struct {
				idx int
				val string
			}{{1, "a"}},
		},
		{
			name: "multiple",
			seq:  func(yield func(string) bool) { yield("a"); yield("b"); yield("c") },
			expected: []struct {
				idx int
				val string
			}{{1, "a"}, {2, "b"}, {3, "c"}},
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
			name:     "single error",
			seq:      func(yield func(error) bool) { yield(errors.New("error1")) },
			expected: "error1",
		},
		{
			name: "multiple errors",
			seq: func(yield func(error) bool) {
				yield(errors.New("error1"))
				yield(errors.New("error2"))
			},
			expected: "error1\nerror2",
		},
		{
			name: "with nil error",
			seq: func(yield func(error) bool) {
				yield(errors.New("error1"))
				yield(nil)
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
			name: "empty",
			seq:  func(yield func(string, int) bool) {},
			expected: []struct {
				a int
				b string
			}{},
		},
		{
			name: "single",
			seq:  func(yield func(string, int) bool) { yield("a", 1) },
			expected: []struct {
				a int
				b string
			}{{1, "a"}},
		},
		{
			name: "multiple",
			seq: func(yield func(string, int) bool) {
				yield("a", 1)
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
			name: "multiple",
			seq: func(yield func(string, int) bool) {
				yield("a", 1)
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
			name: "multiple",
			seq: func(yield func(string, int) bool) {
				yield("a", 1)
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
		{"empty", []int{}},
		{"single", []int{1}},
		{"multiple", []int{1, 2, 3}},
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
		{"no zeros", []int{1, 2, 3}, []*int{ptr(1), ptr(2), ptr(3)}},
		{"with zeros", []int{1, 0, 3}, []*int{ptr(1), nil, ptr(3)}},
		{"all zeros", []int{0, 0}, []*int{nil, nil}},
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
		{"empty", []*int{}, []int{}},
		{"no nils", []*int{ptr(1), ptr(2)}, []int{1, 2}},
		{"with nils", []*int{ptr(1), nil, ptr(3)}, []int{1, 3}},
		{"all nils", []*int{nil, nil}, []int{}},
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
		{"empty", []*int{}, []int{}},
		{"no nils", []*int{ptr(1), ptr(2)}, []int{1, 2}},
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
			name:     "empty generator",
			gen:      func() (int, bool) { return 0, false },
			expected: []int{},
		},
		{
			name: "single value",
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
			name: "multiple values",
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
			seq := Generate(tt.gen)
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
			name: "single value",
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
			seq := Generate2(tt.gen)
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
			name:     "never true",
			op:       func() int { return 1 },
			while:    func(int) bool { return false },
			expected: []int{},
			maxCalls: 1,
		},
		{
			name: "limited calls",
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
			callCount := int32(0)
			wrappedOp := func() int {
				atomic.AddInt32(&callCount, 1)
				return tt.op()
			}

			seq := GenerateWhile(wrappedOp, tt.while)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("GenerateWhile() = %v, want %v", result, tt.expected)
			}

			if int(callCount) > tt.maxCalls {
				t.Errorf("GenerateWhile() called op %d times, max expected %d", callCount, tt.maxCalls)
			}
		})
	}
}

func TestPerpetual(t *testing.T) {
	t.Run("limited iterations", func(t *testing.T) {
		callCount := int32(0)
		op := func() int {
			return int(atomic.AddInt32(&callCount, 1))
		}

		seq := Perpetual(op)
		result := CollectFirstN(seq, 3)

		expected := []int{1, 2, 3}
		if !slices.Equal(result, expected) {
			t.Errorf("Perpetual() = %v, want %v", result, expected)
		}

		if callCount != 3 {
			t.Errorf("Perpetual() called op %d times, want 3", callCount)
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
			name: "empty",
			seq:  func(yield func(int) bool) {},
			op:   func(i int) string { return string(rune('a' + i)) },
			expected: []struct {
				a int
				b string
			}{},
		},
		{
			name: "single",
			seq:  func(yield func(int) bool) { yield(1) },
			op:   func(i int) string { return string(rune('a' + i)) },
			expected: []struct {
				a int
				b string
			}{{1, "b"}},
		},
		{
			name: "multiple",
			seq:  func(yield func(int) bool) { yield(0); yield(1); yield(2) },
			op:   func(i int) string { return string(rune('a' + i)) },
			expected: []struct {
				a int
				b string
			}{{0, "a"}, {1, "b"}, {2, "c"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			wrappedOp := func(i int) string {
				atomic.AddInt32(&callCount, 1)
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

			if int(callCount) != len(tt.expected) {
				t.Errorf("With() called op %d times, want %d", callCount, len(tt.expected))
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
			name:     "multiple",
			seq:      func(yield func(int) bool) { yield(0); yield(1); yield(2) },
			op:       func(i int) string { return string(rune('a' + i)) },
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			wrappedOp := func(i int) string {
				atomic.AddInt32(&callCount, 1)
				return tt.op(i)
			}

			seq := Convert(tt.seq, wrappedOp)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("Convert() = %v, want %v", result, tt.expected)
			}

			if int(callCount) != len(tt.expected) {
				t.Errorf("Convert() called op %d times, want %d", callCount, len(tt.expected))
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
			name:          "multiple",
			seq:           func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			expectedCount: 3,
			expectedSum:   6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := 0
			callCount := int32(0)
			op := func(i int) {
				atomic.AddInt32(&callCount, 1)
				sum += i
			}

			count := Apply(tt.seq, op)

			if count != tt.expectedCount {
				t.Errorf("Apply() count = %v, want %v", count, tt.expectedCount)
			}
			if sum != tt.expectedSum {
				t.Errorf("Apply() sum = %v, want %v", sum, tt.expectedSum)
			}
			if int(callCount) != tt.expectedCount {
				t.Errorf("Apply() called op %d times, want %d", callCount, tt.expectedCount)
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
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
		{"multiple", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
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
}

func TestPipe(t *testing.T) {
	tests := []struct {
		name     string
		values   []int
		expected []int
	}{
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
		{"multiple", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
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
			name:     "exact chunks",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3); yield(4) },
			num:      2,
			expected: [][]int{{1, 2}, {3, 4}},
		},
		{
			name:     "partial last chunk",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			num:      2,
			expected: [][]int{{1, 2}, {3}},
		},
		{
			name:     "single element chunks",
			seq:      func(yield func(int) bool) { yield(1); yield(2) },
			num:      1,
			expected: [][]int{{1}, {2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := Chunk(tt.seq, tt.num)
			var result [][]int
			for chunk := range seq {
				result = append(result, Collect(chunk))
			}

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
			name: "single sequence",
			seq: func(yield func(iter.Seq[int]) bool) {
				yield(func(yield func(int) bool) { yield(1); yield(2) })
			},
			expected: []int{1, 2},
		},
		{
			name: "multiple sequences",
			seq: func(yield func(iter.Seq[int]) bool) {
				yield(func(yield func(int) bool) { yield(1); yield(2) })
				yield(func(yield func(int) bool) { yield(3); yield(4) })
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
			name:     "keep evens",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3); yield(4) },
			pred:     func(i int) bool { return i%2 == 0 },
			expected: []int{2, 4},
		},
		{
			name:     "keep all",
			seq:      func(yield func(int) bool) { yield(1); yield(2) },
			pred:     func(i int) bool { return true },
			expected: []int{1, 2},
		},
		{
			name:     "keep none",
			seq:      func(yield func(int) bool) { yield(1); yield(2) },
			pred:     func(i int) bool { return false },
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			wrappedPred := func(i int) bool {
				atomic.AddInt32(&callCount, 1)
				return tt.pred(i)
			}

			seq := Keep(tt.seq, wrappedPred)
			result := Collect(seq)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("Keep() = %v, want %v", result, tt.expected)
			}

			// Predicate should be called for each input element
			expectedCalls := len(Collect(tt.seq))
			if int(callCount) != expectedCalls {
				t.Errorf("Keep() called predicate %d times, want %d", callCount, expectedCalls)
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
			name:     "remove evens",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3); yield(4) },
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
			name:     "no duplicates",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			expected: []int{1, 2, 3},
		},
		{
			name:     "with duplicates",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(1); yield(3); yield(2) },
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
	}{
		{
			name:     "empty",
			seq:      func(yield func(int) bool) {},
			target:   1,
			expected: false,
		},
		{
			name:     "found",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			target:   2,
			expected: true,
		},
		{
			name:     "not found",
			seq:      func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			target:   4,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.seq, tt.target)
			if result != tt.expected {
				t.Errorf("Contains() = %v, want %v", result, tt.expected)
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
			name:     "equal sequences",
			seq1:     func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			seq2:     func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			expected: true,
		},
		{
			name:     "different lengths",
			seq1:     func(yield func(int) bool) { yield(1); yield(2) },
			seq2:     func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			expected: false,
		},
		{
			name:     "different values",
			seq1:     func(yield func(int) bool) { yield(1); yield(2); yield(3) },
			seq2:     func(yield func(int) bool) { yield(1); yield(2); yield(4) },
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
			name: "both empty",
			seq1: func(yield func(int) bool) {},
			seq2: func(yield func(string) bool) {},
			expected: []struct {
				a int
				b string
			}{},
		},
		{
			name: "equal length",
			seq1: func(yield func(int) bool) { yield(1); yield(2) },
			seq2: func(yield func(string) bool) { yield("a"); yield("b") },
			expected: []struct {
				a int
				b string
			}{{1, "a"}, {2, "b"}},
		},
		{
			name: "first shorter",
			seq1: func(yield func(int) bool) { yield(1) },
			seq2: func(yield func(string) bool) { yield("a"); yield("b") },
			expected: []struct {
				a int
				b string
			}{{1, "a"}},
		},
		{
			name: "second shorter",
			seq1: func(yield func(int) bool) { yield(1); yield(2) },
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
