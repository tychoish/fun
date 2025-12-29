package irt

import (
	"iter"
	"maps"
	"slices"
	"sync/atomic"
	"testing"
)

func TestForEach(t *testing.T) {
	cases := []struct {
		name      string
		seq       iter.Seq[int]
		op        func(int)
		earlyExit int
		expected  []int
	}{
		{
			name: "Nil",
			seq:  nil,
			op:   func(int) { t.Error("should not be called") },
		},
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
			if cs.earlyExit > 0 {
				if opcount.Load() != int64(cs.earlyExit) {
					t.Error("op call count mismatch", "have=", opcount.Load(), "want=", cs.earlyExit)
				}
				if len(cs.expected) != cs.earlyExit {
					t.Error("invalid test case")
				}
			} else if len(cs.expected) > 0 {
				if opcount.Load() != int64(len(cs.expected)) {
					t.Error("op call count mismatch", "have=", opcount.Load(), "want=", len(cs.expected))
				}
			} else {
				if opcount.Load() != 0 {
					t.Error("op call count mismatch", "have=", opcount.Load(), "want=", 0)

				}
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
			name: "Nil",
			seq:  nil,
			op:   func(int, int) { t.Error("should not be called") },
		},
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

			if cs.earlyExit > 0 {
				if opcount.Load() != int64(cs.earlyExit) {
					t.Error("op call count mismatch", "have=", opcount.Load(), "want=", cs.earlyExit)
				}
				if len(cs.expected) != cs.earlyExit {
					t.Error("invalid test case", len(cs.expected), cs.earlyExit)
				}
			} else if len(cs.expected) > 0 {
				if opcount.Load() != int64(len(cs.expected)) {
					t.Error("op call count mismatch", "have=", opcount.Load(), "want=", len(cs.expected))
				}
			} else {
				if opcount.Load() != 0 {
					t.Error("op call count mismatch", "have=", opcount.Load(), "want=", 0)
				}
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
			name: "Nil",
			seq:  nil,
			op:   func(int) bool { t.Error("should not be called"); return false },
		},
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
			name: "Nil",
			seq:  nil,
			op:   func(int, int) bool { t.Error("should not be called"); return false },
		},
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
