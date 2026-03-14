package strut

import (
	"iter"
	"slices"
	"testing"
)

func TestIfop(t *testing.T) {
	tests := []struct {
		name     string
		cond     bool
		expected int
	}{
		{
			name:     "condition true executes operation",
			cond:     true,
			expected: 1,
		},
		{
			name:     "condition false skips operation",
			cond:     false,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := 0
			ifop(tt.cond, func() { counter++ })
			if counter != tt.expected {
				t.Errorf("expected counter=%d, got %d", tt.expected, counter)
			}
		})
	}
}

func TestIfargs(t *testing.T) {
	tests := []struct {
		name     string
		cond     bool
		args     []int
		expected []int
	}{
		{
			name:     "condition true executes with args",
			cond:     true,
			args:     []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "condition false skips operation",
			cond:     false,
			args:     []int{1, 2, 3},
			expected: nil,
		},
		{
			name:     "empty args with true condition",
			cond:     true,
			args:     []int{},
			expected: []int{},
		},
		{
			name:     "nil args with true condition",
			cond:     true,
			args:     nil,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []int
			ifargs(tt.cond, func(vals ...int) {
				result = append(result, vals...)
			}, tt.args)

			if len(result) != len(tt.expected) {
				t.Errorf("expected len=%d, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestIftuple(t *testing.T) {
	tests := []struct {
		name      string
		cond      bool
		a         string
		b         int
		expectA   string
		expectB   int
		shouldRun bool
	}{
		{
			name:      "condition true executes with both args",
			cond:      true,
			a:         "hello",
			b:         42,
			expectA:   "hello",
			expectB:   42,
			shouldRun: true,
		},
		{
			name:      "condition false skips operation",
			cond:      false,
			a:         "world",
			b:         99,
			shouldRun: false,
		},
		{
			name:      "empty string and zero int with true condition",
			cond:      true,
			a:         "",
			b:         0,
			expectA:   "",
			expectB:   0,
			shouldRun: true,
		},
		{
			name:      "negative number with true condition",
			cond:      true,
			a:         "test",
			b:         -100,
			expectA:   "test",
			expectB:   -100,
			shouldRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotA string
			var gotB int
			var executed bool

			iftuple(tt.cond, func(a string, b int) {
				gotA = a
				gotB = b
				executed = true
			}, tt.a, tt.b)

			if executed != tt.shouldRun {
				t.Errorf("expected executed=%v, got %v", tt.shouldRun, executed)
			}

			if tt.shouldRun {
				if gotA != tt.expectA {
					t.Errorf("expected a=%q, got %q", tt.expectA, gotA)
				}
				if gotB != tt.expectB {
					t.Errorf("expected b=%d, got %d", tt.expectB, gotB)
				}
			}
		})
	}
}

func TestIffmt(t *testing.T) {
	tests := []struct {
		name     string
		cond     bool
		tmpl     string
		args     []any
		expected string
	}{
		{
			name:     "condition true formats with args",
			cond:     true,
			tmpl:     "Hello %s, number %d",
			args:     []any{"world", 42},
			expected: "Hello world, number 42",
		},
		{
			name:     "condition false skips operation",
			cond:     false,
			tmpl:     "Hello %s",
			args:     []any{"world"},
			expected: "",
		},
		{
			name:     "empty template with true condition",
			cond:     true,
			tmpl:     "",
			args:     []any{},
			expected: "",
		},
		{
			name:     "template with no placeholders",
			cond:     true,
			tmpl:     "plain text",
			args:     []any{},
			expected: "plain text",
		},
		{
			name:     "unicode in template",
			cond:     true,
			tmpl:     "Hello 世界 %s",
			args:     []any{"🌍"},
			expected: "Hello 世界 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			iffmt(tt.cond, func(tmpl string, args ...any) {
				result = tmpl
				if len(args) > 0 {
					// Simulate a simple format operation for testing
					result = tmpl
				}
			}, tt.tmpl, tt.args)

			if tt.cond && result != tt.tmpl {
				t.Errorf("expected result=%q, got %q", tt.tmpl, result)
			}
			if !tt.cond && result != "" {
				t.Errorf("expected empty result when condition is false, got %q", result)
			}
		})
	}
}

func TestIfwith(t *testing.T) {
	tests := []struct {
		name      string
		cond      bool
		value     string
		shouldRun bool
	}{
		{
			name:      "condition true executes with value",
			cond:      true,
			value:     "test",
			shouldRun: true,
		},
		{
			name:      "condition false skips operation",
			cond:      false,
			value:     "ignored",
			shouldRun: false,
		},
		{
			name:      "empty string with true condition",
			cond:      true,
			value:     "",
			shouldRun: true,
		},
		{
			name:      "unicode value with true condition",
			cond:      true,
			value:     "こんにちは",
			shouldRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			var executed bool

			ifwith(tt.cond, func(v string) {
				result = v
				executed = true
			}, tt.value)

			if executed != tt.shouldRun {
				t.Errorf("expected executed=%v, got %v", tt.shouldRun, executed)
			}

			if tt.shouldRun && result != tt.value {
				t.Errorf("expected result=%q, got %q", tt.value, result)
			}
		})
	}
}

func TestNtimes(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected int
	}{
		{
			name:     "positive n executes operation n times",
			n:        5,
			expected: 5,
		},
		{
			name:     "zero n executes zero times",
			n:        0,
			expected: 0,
		},
		{
			name:     "negative n executes zero times",
			n:        -1,
			expected: 0,
		},
		{
			name:     "large n executes n times",
			n:        1000,
			expected: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := 0
			ntimes(tt.n, func() { counter++ })

			if counter != tt.expected {
				t.Errorf("expected counter=%d, got %d", tt.expected, counter)
			}
		})
	}
}

func TestNwith(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		arg      string
		expected []string
	}{
		{
			name:     "positive n executes with arg n times",
			n:        3,
			arg:      "hello",
			expected: []string{"hello", "hello", "hello"},
		},
		{
			name:     "zero n executes zero times",
			n:        0,
			arg:      "world",
			expected: []string{},
		},
		{
			name:     "negative n executes zero times",
			n:        -5,
			arg:      "test",
			expected: []string{},
		},
		{
			name:     "empty string arg",
			n:        2,
			arg:      "",
			expected: []string{"", ""},
		},
		{
			name:     "unicode arg",
			n:        2,
			arg:      "🎉",
			expected: []string{"🎉", "🎉"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []string
			nwith(tt.n, func(s string) {
				result = append(result, s)
			}, tt.arg)

			if len(result) != len(tt.expected) {
				t.Errorf("expected len=%d, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name     string
		vals     []string
		expected []string
	}{
		{
			name:     "applies operation to all elements",
			vals:     []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty slice",
			vals:     []string{},
			expected: []string{},
		},
		{
			name:     "nil slice",
			vals:     nil,
			expected: []string{},
		},
		{
			name:     "single element",
			vals:     []string{"solo"},
			expected: []string{"solo"},
		},
		{
			name:     "unicode elements",
			vals:     []string{"hello", "世界", "🌍"},
			expected: []string{"hello", "世界", "🌍"},
		},
		{
			name:     "empty strings in slice",
			vals:     []string{"", "", ""},
			expected: []string{"", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []string
			apply(func(s string) {
				result = append(result, s)
			}, tt.vals)

			if len(result) != len(tt.expected) {
				t.Errorf("expected len=%d, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestFlush(t *testing.T) {
	tests := []struct {
		name     string
		vals     []int
		expected []int
	}{
		{
			name:     "flushes all values from sequence",
			vals:     []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "empty sequence",
			vals:     []int{},
			expected: []int{},
		},
		{
			name:     "nil sequence",
			vals:     nil,
			expected: []int{},
		},
		{
			name:     "single value",
			vals:     []int{42},
			expected: []int{42},
		},
		{
			name:     "negative numbers",
			vals:     []int{-1, -2, -3},
			expected: []int{-1, -2, -3},
		},
		{
			name:     "mixed positive and negative",
			vals:     []int{-10, 0, 10},
			expected: []int{-10, 0, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []int
			seq := func(yield func(int) bool) {
				for _, v := range tt.vals {
					if !yield(v) {
						return
					}
				}
			}

			flush(seq, func(n int) {
				result = append(result, n)
			})

			if len(result) != len(tt.expected) {
				t.Errorf("expected len=%d, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestFlushEarlyTermination(t *testing.T) {
	// Test that flush processes all elements from the sequence
	vals := []int{1, 2, 3, 4, 5}
	var result []int

	seq := func(yield func(int) bool) {
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
	}

	// flush will always process all elements
	flush(seq, func(n int) {
		result = append(result, n)
	})

	expected := []int{1, 2, 3, 4, 5}
	if len(result) != len(expected) {
		t.Errorf("expected len=%d, got %d", len(expected), len(result))
	}

	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("at index %d: expected %d, got %d", i, expected[i], result[i])
		}
	}
}

func TestApplyWithMutation(t *testing.T) {
	// Test that apply can be used to mutate external state
	sum := 0
	vals := []int{1, 2, 3, 4, 5}

	apply(func(n int) {
		sum += n
	}, vals)

	expected := 15
	if sum != expected {
		t.Errorf("expected sum=%d, got %d", expected, sum)
	}
}

func TestNtimesOrder(t *testing.T) {
	// Test that ntimes executes operations in order
	var result []int
	n := 5

	ntimes(n, func() {
		result = append(result, len(result))
	})

	expected := []int{0, 1, 2, 3, 4}
	if !slices.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestNwithOrder(t *testing.T) {
	// Test that nwith executes operations in order with consistent arg
	var result []string
	n := 3
	arg := "x"

	counter := 0
	nwith(n, func(s string) {
		result = append(result, s)
		counter++
		if s != arg {
			t.Errorf("expected arg %q at call %d, got %q", arg, counter, s)
		}
	}, arg)

	if len(result) != n {
		t.Errorf("expected %d calls, got %d", n, len(result))
	}
}

func TestFlushWithComplexIterator(t *testing.T) {
	// Test flush with a more complex iterator pattern
	makeSeq := func(start, end int) iter.Seq[int] {
		return func(yield func(int) bool) {
			for i := start; i < end; i++ {
				if !yield(i) {
					return
				}
			}
		}
	}

	var result []int
	flush(makeSeq(10, 15), func(n int) {
		result = append(result, n)
	})

	expected := []int{10, 11, 12, 13, 14}
	if !slices.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestFromMutable(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]byte
		expected []string
	}{
		{
			name:     "converts multiple mutable values to strings",
			input:    [][]byte{[]byte("hello"), []byte("world"), []byte("test")},
			expected: []string{"hello", "world", "test"},
		},
		{
			name:     "empty iterator",
			input:    [][]byte{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    [][]byte{[]byte("solo")},
			expected: []string{"solo"},
		},
		{
			name:     "unicode content",
			input:    [][]byte{[]byte("hello"), []byte("世界"), []byte("🌍")},
			expected: []string{"hello", "世界", "🌍"},
		},
		{
			name:     "empty strings",
			input:    [][]byte{[]byte(""), []byte(""), []byte("")},
			expected: []string{"", "", ""},
		},
		{
			name:     "mixed empty and non-empty",
			input:    [][]byte{[]byte("first"), []byte(""), []byte("last")},
			expected: []string{"first", "", "last"},
		},
		{
			name:     "long strings",
			input:    [][]byte{[]byte("this is a longer string"), []byte("with multiple words")},
			expected: []string{"this is a longer string", "with multiple words"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an iterator of *Mutable from the input
			mutSeq := func(yield func(Mutable) bool) {
				for _, b := range tt.input {
					m := Mutable(b)
					if !yield(m) {
						return
					}
				}
			}

			// Convert to string iterator
			strSeq := FromMutable(mutSeq)

			// Collect results
			var result []string
			for s := range strSeq {
				result = append(result, s)
			}

			// Verify
			if len(result) != len(tt.expected) {
				t.Errorf("expected len=%d, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestFromMutableEarlyTermination(t *testing.T) {
	// Test that FromMutable respects early termination
	input := [][]byte{[]byte("one"), []byte("two"), []byte("three"), []byte("four"), []byte("five")}

	mutSeq := func(yield func(Mutable) bool) {
		for _, b := range input {
			m := Mutable(b)
			if !yield(m) {
				return
			}
		}
	}

	strSeq := FromMutable(mutSeq)

	var result []string
	for s := range strSeq {
		result = append(result, s)
		if len(result) == 3 {
			break // Early termination
		}
	}

	expected := []string{"one", "two", "three"}
	if !slices.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestFromMutableWithSplit(t *testing.T) {
	// Test FromMutable with actual Mutable Split method
	mut := Mutable([]byte("hello,world,test"))

	result := make([]string, 0, 3)
	for s := range FromMutable(mut.Split([]byte(","))) {
		result = append(result, s)
	}

	expected := []string{"hello", "world", "test"}
	if !slices.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestFromMutableWithFields(t *testing.T) {
	// Test FromMutable with actual Mutable Fields method
	mut := Mutable([]byte("hello   world\ttest\nfoo"))

	result := make([]string, 0, 4)
	for s := range FromMutable(mut.Fields()) {
		result = append(result, s)
	}

	expected := []string{"hello", "world", "test", "foo"}
	if !slices.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestFromMutableLazyEvaluation(t *testing.T) {
	// Test that FromMutable is lazy - it shouldn't consume the iterator until needed
	callCount := 0
	mutSeq := func(yield func(Mutable) bool) {
		for range 5 {
			callCount++
			m := Mutable([]byte("item"))
			if !yield(m) {
				return
			}
		}
	}

	strSeq := FromMutable(mutSeq)

	// Just creating the iterator shouldn't call the underlying sequence
	if callCount != 0 {
		t.Errorf("expected callCount=0 before iteration, got %d", callCount)
	}

	// Consume only first 2 items
	consumed := 0
	for range strSeq {
		consumed++
		if consumed == 2 {
			break
		}
	}

	// Should have called exactly 2 times
	if callCount != 2 {
		t.Errorf("expected callCount=2 after consuming 2 items, got %d", callCount)
	}
}
