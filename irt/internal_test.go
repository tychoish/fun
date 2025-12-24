package irt

import (
	"slices"
	"sync/atomic"
	"testing"
)

// Tests for internal/non-exported functions

func TestCounter(t *testing.T) {
	c := counter()

	tests := []int{1, 2, 3, 4, 5}
	for _, expected := range tests {
		result := c()
		if result != expected {
			t.Errorf("counter() = %v, want %v", result, expected)
		}
	}
}

func TestSeen(t *testing.T) {
	s := seen[int]()

	// First time should return false (not seen)
	if s(1) {
		t.Errorf("seen(1) first time = true, want false")
	}

	// Second time should return true (already seen)
	if !s(1) {
		t.Errorf("seen(1) second time = false, want true")
	}

	// Different value should return false
	if s(2) {
		t.Errorf("seen(2) first time = true, want false")
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected bool
	}{
		{"equal", 5, 5, true},
		{"not equal", 5, 3, false},
		{"zero values", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equal(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("equal(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestFirst(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected interface{}
	}{
		{"int-string", 42, "hello", 42},
		{"string-int", "world", 123, "world"},
		{"nil-value", nil, 42, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := first(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("first(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestSecond(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected interface{}
	}{
		{"int-string", 42, "hello", "hello"},
		{"string-int", "world", 123, 123},
		{"value-nil", 42, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := second(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("second(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestFlipFunc(t *testing.T) {
	tests := []struct {
		name string
		a, b interface{}
	}{
		{"int-string", 42, "hello"},
		{"string-int", "world", 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, a := flip(tt.a, tt.b)
			if a != tt.a || b != tt.b {
				t.Errorf("flip(%v, %v) = (%v, %v), want (%v, %v)", tt.a, tt.b, a, b, tt.a, tt.b)
			}
		})
	}
}

func TestNoop(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"int", 42},
		{"string", "hello"},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := noop(tt.input)
			if result != tt.input {
				t.Errorf("noop(%v) = %v, want %v", tt.input, result, tt.input)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	callCount := int32(0)
	op := func() string {
		atomic.AddInt32(&callCount, 1)
		return "result"
	}

	wrapped := wrap[int](op)

	result := wrapped(42)
	if result != "result" {
		t.Errorf("wrap(op)(42) = %v, want 'result'", result)
	}

	if callCount != 1 {
		t.Errorf("wrapped function called %d times, want 1", callCount)
	}
}

func TestArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
		{"multiple", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := args(tt.input...)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("args(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNotf(t *testing.T) {
	isEven := func(i int) bool { return i%2 == 0 }
	isOdd := notf(isEven)

	tests := []struct {
		input    int
		expected bool
	}{
		{2, true},  // 2 is even, so isOdd(2) should be true
		{3, false}, // 3 is odd, so isOdd(3) should be false
		{4, true},  // 4 is even, so isOdd(4) should be true
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := isOdd(tt.input)
			if result != tt.expected {
				t.Errorf("notf(isEven)(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNil(t *testing.T) {
	var nilPtr *int
	nonNilPtr := &[]int{1}[0]

	tests := []struct {
		name     string
		input    *int
		expected bool
	}{
		{"nil pointer", nilPtr, true},
		{"non-nil pointer", nonNilPtr, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNil(tt.input)
			if result != tt.expected {
				t.Errorf("isNil(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected bool
	}{
		{"zero", 0, true},
		{"non-zero", 42, false},
		{"negative", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isZero(tt.input)
			if result != tt.expected {
				t.Errorf("isZero(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPtr(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ptr(tt.input)
			if result == nil {
				t.Errorf("ptr(%v) returned nil", tt.input)
			}
			if *result != tt.input {
				t.Errorf("*ptr(%v) = %v, want %v", tt.input, *result, tt.input)
			}
		})
	}
}

func TestDeref(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := &tt.input
			result := deref(ptr)
			if result != tt.input {
				t.Errorf("deref(&%v) = %v, want %v", tt.input, result, tt.input)
			}
		})
	}
}

func TestDerefz(t *testing.T) {
	tests := []struct {
		name     string
		input    *int
		expected int
	}{
		{"nil pointer", nil, 0},
		{"non-nil pointer", &[]int{42}[0], 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := derefz(tt.input)
			if result != tt.expected {
				t.Errorf("derefz(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIdxorz(t *testing.T) {
	tests := []struct {
		name     string
		idx      int
		slice    []int
		expected int
	}{
		{"valid index", 1, []int{10, 20, 30}, 20},
		{"index 0", 0, []int{10, 20, 30}, 10},
		{"out of bounds", 5, []int{10, 20, 30}, 0},
		{"empty slice", 0, []int{}, 0},
		{"negative index", -1, []int{10, 20, 30}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idxorz(tt.idx, tt.slice)
			if result != tt.expected {
				t.Errorf("idxorz(%v, %v) = %v, want %v", tt.idx, tt.slice, result, tt.expected)
			}
		})
	}
}

func TestIfelse(t *testing.T) {
	tests := []struct {
		name      string
		condition bool
		then      string
		elsewise  string
		expected  string
	}{
		{"true condition", true, "yes", "no", "yes"},
		{"false condition", false, "yes", "no", "no"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ifelse(tt.condition, tt.then, tt.elsewise)
			if result != tt.expected {
				t.Errorf("ifelse(%v, %v, %v) = %v, want %v",
					tt.condition, tt.then, tt.elsewise, result, tt.expected)
			}
		})
	}
}

func TestIfelsedo(t *testing.T) {
	tests := []struct {
		name      string
		condition bool
		then      string
		expected  string
	}{
		{"true condition", true, "yes", "yes"},
		{"false condition", false, "yes", "no"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			elsewise := func() string {
				atomic.AddInt32(&callCount, 1)
				return "no"
			}

			result := ifelsedo(tt.condition, tt.then, elsewise)
			if result != tt.expected {
				t.Errorf("ifelsedo(%v, %v, elsewise) = %v, want %v",
					tt.condition, tt.then, result, tt.expected)
			}

			expectedCalls := int32(0)
			if !tt.condition {
				expectedCalls = 1
			}
			if callCount != expectedCalls {
				t.Errorf("elsewise called %d times, want %d", callCount, expectedCalls)
			}
		})
	}
}

func TestNtimes(t *testing.T) {
	tests := []struct {
		name     string
		times    int
		expected []int
	}{
		{"zero times", 0, []int{}},
		{"once", 1, []int{42}},
		{"multiple times", 3, []int{42, 42, 42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			op := func() int {
				atomic.AddInt32(&callCount, 1)
				return 42
			}

			fn := ntimes(op, tt.times)
			var result []int

			for {
				val, ok := fn()
				if !ok {
					break
				}
				result = append(result, val)
			}

			if !slices.Equal(result, tt.expected) {
				t.Errorf("ntimes(op, %d) = %v, want %v", tt.times, result, tt.expected)
			}

			if int(callCount) != tt.times {
				t.Errorf("op called %d times, want %d", callCount, tt.times)
			}
		})
	}
}

func TestWithlimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		input    []int
		expected []int
	}{
		{"limit 0 panics", 0, []int{1, 2, 3}, nil}, // Special case
		{"limit 1", 1, []int{1, 2, 3}, []int{1}},
		{"limit equals input", 3, []int{1, 2, 3}, []int{1, 2, 3}},
		{"limit exceeds input", 5, []int{1, 2, 3}, []int{1, 2, 3}},
		{"empty input", 3, []int{}, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.limit == 0 {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("withlimit(op, 0) should panic")
					}
				}()
				withlimit(func() (int, bool) { return 0, false }, 0)
				return
			}

			idx := 0
			op := func() (int, bool) {
				if idx >= len(tt.input) {
					return 0, false
				}
				val := tt.input[idx]
				idx++
				return val, true
			}

			fn := withlimit(op, tt.limit)
			var result []int

			for {
				val, okPtr := fn()
				if okPtr == nil {
					break
				}
				if !*okPtr {
					break
				}
				result = append(result, val)
			}

			if !slices.Equal(result, tt.expected) {
				t.Errorf("withlimit(op, %d) = %v, want %v", tt.limit, result, tt.expected)
			}
		})
	}
}

func TestSet(t *testing.T) {
	s := make(set[int])

	// Test add
	existed := s.add(1)
	if existed {
		t.Errorf("s.add(1) first time = true, want false")
	}

	existed = s.add(1)
	if !existed {
		t.Errorf("s.add(1) second time = false, want true")
	}

	// Test check
	if !s.check(1) {
		t.Errorf("s.check(1) = false, want true")
	}

	if s.check(2) {
		t.Errorf("s.check(2) = true, want false")
	}

	// Test iter
	values := Collect(s.iter())
	if len(values) != 1 || values[0] != 1 {
		t.Errorf("s.iter() = %v, want [1]", values)
	}

	// Test pop
	existed = s.pop(1)
	if !existed {
		t.Errorf("s.pop(1) = false, want true")
	}

	if s.check(1) {
		t.Errorf("s.check(1) after pop = true, want false")
	}
}

func TestGroups(t *testing.T) {
	g := make(groups[string, int])

	// Test add
	g.add("a", 1)
	g.add("a", 2)
	g.add("b", 3)

	// Test check
	if !g.check("a") {
		t.Errorf("g.check('a') = false, want true")
	}

	if g.check("c") {
		t.Errorf("g.check('c') = true, want false")
	}

	// Test pop
	values := g.pop("a")
	expected := []int{1, 2}
	if !slices.Equal(values, expected) {
		t.Errorf("g.pop('a') = %v, want %v", values, expected)
	}

	if g.check("a") {
		t.Errorf("g.check('a') after pop = true, want false")
	}
}

func TestOrderedGrouping(t *testing.T) {
	og := grouping(make(groups[string, int]))

	// Test add
	og.add("b", 2)
	og.add("a", 1)
	og.add("b", 3)
	og.add("c", 4)

	// Test iter preserves order
	var keys []string
	var values [][]int
	for k, v := range og.iter() {
		keys = append(keys, k)
		values = append(values, v)
	}

	expectedKeys := []string{"b", "a", "c"}
	if !slices.Equal(keys, expectedKeys) {
		t.Errorf("ordered grouping keys = %v, want %v", keys, expectedKeys)
	}

	expectedValues := [][]int{{2, 3}, {1}, {4}}
	if len(values) != len(expectedValues) {
		t.Errorf("ordered grouping values length = %v, want %v", len(values), len(expectedValues))
	}
	for i, v := range values {
		if i < len(expectedValues) && !slices.Equal(v, expectedValues[i]) {
			t.Errorf("ordered grouping values[%d] = %v, want %v", i, v, expectedValues[i])
		}
	}
}

func TestElem(t *testing.T) {
	// Test NewElem
	elem := NewElem(42, "hello")
	if elem.First != 42 || elem.Second != "hello" {
		t.Errorf("NewElem(42, 'hello') = %v, want {42, 'hello'}", elem)
	}

	// Test WithElem
	elem2 := WithElem(5, func(i int) string { return string(rune('a' + i)) })
	if elem2.First != 5 || elem2.Second != "f" {
		t.Errorf("WithElem(5, func) = %v, want {5, 'f'}", elem2)
	}

	// Test Split
	a, b := elem.Split()
	if a != 42 || b != "hello" {
		t.Errorf("elem.Split() = (%v, %v), want (42, 'hello')", a, b)
	}

	// Test Apply
	var appliedA int
	var appliedB string
	elem.Apply(func(a int, b string) {
		appliedA = a
		appliedB = b
	})
	if appliedA != 42 || appliedB != "hello" {
		t.Errorf("elem.Apply() set (%v, %v), want (42, 'hello')", appliedA, appliedB)
	}
}

func TestElems(t *testing.T) {
	seq := func(yield func(int, string) bool) {
		yield(1, "a")
		yield(2, "b")
	}

	elemSeq := Elems(seq)
	result := Collect(elemSeq)

	expected := []Elem[int, string]{
		{First: 1, Second: "a"},
		{First: 2, Second: "b"},
	}

	if len(result) != len(expected) {
		t.Errorf("Elems() length = %v, want %v", len(result), len(expected))
	}
	for i, elem := range result {
		if i < len(expected) && (elem.First != expected[i].First || elem.Second != expected[i].Second) {
			t.Errorf("Elems()[%d] = %v, want %v", i, elem, expected[i])
		}
	}
}

func TestSplits(t *testing.T) {
	elems := []Elem[int, string]{
		{First: 1, Second: "a"},
		{First: 2, Second: "b"},
	}

	seq := Splits(Slice(elems))

	var result []struct {
		a int
		b string
	}
	for a, b := range seq {
		result = append(result, struct {
			a int
			b string
		}{a, b})
	}

	expected := []struct {
		a int
		b string
	}{
		{1, "a"},
		{2, "b"},
	}

	if len(result) != len(expected) {
		t.Errorf("Splits() length = %v, want %v", len(result), len(expected))
	}
	for i, item := range result {
		if i < len(expected) && (item.a != expected[i].a || item.b != expected[i].b) {
			t.Errorf("Splits()[%d] = %v, want %v", i, item, expected[i])
		}
	}
}
package irt

import (
	"iter"
	"slices"
	"sync/atomic"
	"testing"
)

// Tests for internal/non-exported functions

func TestCounter(t *testing.T) {
	c := counter()
	
	tests := []int{1, 2, 3, 4, 5}
	for _, expected := range tests {
		result := c()
		if result != expected {
			t.Errorf("counter() = %v, want %v", result, expected)
		}
	}
}

func TestSeen(t *testing.T) {
	s := seen[int]()
	
	// First time should return false (not seen)
	if s(1) {
		t.Errorf("seen(1) first time = true, want false")
	}
	
	// Second time should return true (already seen)
	if !s(1) {
		t.Errorf("seen(1) second time = false, want true")
	}
	
	// Different value should return false
	if s(2) {
		t.Errorf("seen(2) first time = true, want false")
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected bool
	}{
		{"equal", 5, 5, true},
		{"not equal", 5, 3, false},
		{"zero values", 0, 0, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equal(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("equal(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestFirst(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected interface{}
	}{
		{"int-string", 42, "hello", 42},
		{"string-int", "world", 123, "world"},
		{"nil-value", nil, 42, nil},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := first(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("first(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestSecond(t *testing.T) {
	tests := []struct {
		name     string
		a, b     interface{}
		expected interface{}
	}{
		{"int-string", 42, "hello", "hello"},
		{"string-int", "world", 123, 123},
		{"value-nil", 42, nil, nil},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := second(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("second(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestFlipFunc(t *testing.T) {
	tests := []struct {
		name string
		a, b interface{}
	}{
		{"int-string", 42, "hello"},
		{"string-int", "world", 123},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, a := flip(tt.a, tt.b)
			if a != tt.a || b != tt.b {
				t.Errorf("flip(%v, %v) = (%v, %v), want (%v, %v)", tt.a, tt.b, a, b, tt.a, tt.b)
			}
		})
	}
}

func TestNoop(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"int", 42},
		{"string", "hello"},
		{"nil", nil},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := noop(tt.input)
			if result != tt.input {
				t.Errorf("noop(%v) = %v, want %v", tt.input, result, tt.input)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	callCount := int32(0)
	op := func() string {
		atomic.AddInt32(&callCount, 1)
		return "result"
	}
	
	wrapped := wrap[int](op)
	
	result := wrapped(42)
	if result != "result" {
		t.Errorf("wrap(op)(42) = %v, want 'result'", result)
	}
	
	if callCount != 1 {
		t.Errorf("wrapped function called %d times, want 1", callCount)
	}
}

func TestArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
		{"multiple", []int{1, 2, 3}, []int{1, 2, 3}},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := args(tt.input...)
			if !slices.Equal(result, tt.expected) {
				t.Errorf("args(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNotf(t *testing.T) {
	isEven := func(i int) bool { return i%2 == 0 }
	isOdd := notf(isEven)
	
	tests := []struct {
		input    int
		expected bool
	}{
		{2, true},  // 2 is even, so isOdd(2) should be true
		{3, false}, // 3 is odd, so isOdd(3) should be false
		{4, true},  // 4 is even, so isOdd(4) should be true
	}
	
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := isOdd(tt.input)
			if result != tt.expected {
				t.Errorf("notf(isEven)(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNil(t *testing.T) {
	var nilPtr *int
	nonNilPtr := &[]int{1}[0]
	
	tests := []struct {
		name     string
		input    *int
		expected bool
	}{
		{"nil pointer", nilPtr, true},
		{"non-nil pointer", nonNilPtr, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNil(tt.input)
			if result != tt.expected {
				t.Errorf("isNil(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected bool
	}{
		{"zero", 0, true},
		{"non-zero", 42, false},
		{"negative", -1, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isZero(tt.input)
			if result != tt.expected {
				t.Errorf("isZero(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPtr(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative", -1},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ptr(tt.input)
			if result == nil {
				t.Errorf("ptr(%v) returned nil", tt.input)
			}
			if *result != tt.input {
				t.Errorf("*ptr(%v) = %v, want %v", tt.input, *result, tt.input)
			}
		})
	}
}

func TestDeref(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative", -1},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := &tt.input
			result := deref(ptr)
			if result != tt.input {
				t.Errorf("deref(&%v) = %v, want %v", tt.input, result, tt.input)
			}
		})
	}
}

func TestDerefz(t *testing.T) {
	tests := []struct {
		name     string
		input    *int
		expected int
	}{
		{"nil pointer", nil, 0},
		{"non-nil pointer", &[]int{42}[0], 42},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := derefz(tt.input)
			if result != tt.expected {
				t.Errorf("derefz(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIdxorz(t *testing.T) {
	tests := []struct {
		name     string
		idx      int
		slice    []int
		expected int
	}{
		{"valid index", 1, []int{10, 20, 30}, 20},
		{"index 0", 0, []int{10, 20, 30}, 10},
		{"out of bounds", 5, []int{10, 20, 30}, 0},
		{"empty slice", 0, []int{}, 0},
		{"negative index", -1, []int{10, 20, 30}, 0},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := idxorz(tt.idx, tt.slice)
			if result != tt.expected {
				t.Errorf("idxorz(%v, %v) = %v, want %v", tt.idx, tt.slice, result, tt.expected)
			}
		})
	}
}

func TestIfelse(t *testing.T) {
	tests := []struct {
		name      string
		condition bool
		then      string
		elsewise  string
		expected  string
	}{
		{"true condition", true, "yes", "no", "yes"},
		{"false condition", false, "yes", "no", "no"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ifelse(tt.condition, tt.then, tt.elsewise)
			if result != tt.expected {
				t.Errorf("ifelse(%v, %v, %v) = %v, want %v", 
					tt.condition, tt.then, tt.elsewise, result, tt.expected)
			}
		})
	}
}

func TestIfelsedo(t *testing.T) {
	tests := []struct {
		name      string
		condition bool
		then      string
		expected  string
	}{
		{"true condition", true, "yes", "yes"},
		{"false condition", false, "yes", "no"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			elsewise := func() string {
				atomic.AddInt32(&callCount, 1)
				return "no"
			}
			
			result := ifelsedo(tt.condition, tt.then, elsewise)
			if result != tt.expected {
				t.Errorf("ifelsedo(%v, %v, elsewise) = %v, want %v", 
					tt.condition, tt.then, result, tt.expected)
			}
			
			expectedCalls := int32(0)
			if !tt.condition {
				expectedCalls = 1
			}
			if callCount != expectedCalls {
				t.Errorf("elsewise called %d times, want %d", callCount, expectedCalls)
			}
		})
	}
}

func TestNtimes(t *testing.T) {
	tests := []struct {
		name     string
		times    int
		expected []int
	}{
		{"zero times", 0, []int{}},
		{"once", 1, []int{42}},
		{"multiple times", 3, []int{42, 42, 42}},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := int32(0)
			op := func() int {
				atomic.AddInt32(&callCount, 1)
				return 42
			}
			
			fn := ntimes(op, tt.times)
			var result []int
			
			for {
				val, ok := fn()
				if !ok {
					break
				}
				result = append(result, val)
			}
			
			if !slices.Equal(result, tt.expected) {
				t.Errorf("ntimes(op, %d) = %v, want %v", tt.times, result, tt.expected)
			}
			
			if int(callCount) != tt.times {
				t.Errorf("op called %d times, want %d", callCount, tt.times)
			}
		})
	}
}

func TestWithlimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		input    []int
		expected []int
	}{
		{"limit 0 panics", 0, []int{1, 2, 3}, nil}, // Special case
		{"limit 1", 1, []int{1, 2, 3}, []int{1}},
		{"limit equals input", 3, []int{1, 2, 3}, []int{1, 2, 3}},
		{"limit exceeds input", 5, []int{1, 2, 3}, []int{1, 2, 3}},
		{"empty input", 3, []int{}, []int{}},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.limit == 0 {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("withlimit(op, 0) should panic")
					}
				}()
				withlimit(func() (int, bool) { return 0, false }, 0)
				return
			}
			
			idx := 0
			op := func() (int, bool) {
				if idx >= len(tt.input) {
					return 0, false
				}
				val := tt.input[idx]
				idx++
				return val, true
			}
			
			fn := withlimit(op, tt.limit)
			var result []int
			
			for {
				val, okPtr := fn()
				if okPtr == nil {
					break
				}
				if !*okPtr {
					break
				}
				result = append(result, val)
			}
			
			if !slices.Equal(result, tt.expected) {
				t.Errorf("withlimit(op, %d) = %v, want %v", tt.limit, result, tt.expected)
			}
		})
	}
}

func TestSet(t *testing.T) {
	s := make(set[int])
	
	// Test add
	existed := s.add(1)
	if existed {
		t.Errorf("s.add(1) first time = true, want false")
	}
	
	existed = s.add(1)
	if !existed {
		t.Errorf("s.add(1) second time = false, want true")
	}
	
	// Test check
	if !s.check(1) {
		t.Errorf("s.check(1) = false, want true")
	}
	
	if s.check(2) {
		t.Errorf("s.check(2) = true, want false")
	}
	
	// Test iter
	values := Collect(s.iter())
	if len(values) != 1 || values[0] != 1 {
		t.Errorf("s.iter() = %v, want [1]", values)
	}
	
	// Test pop
	existed = s.pop(1)
	if !existed {
		t.Errorf("s.pop(1) = false, want true")
	}
	
	if s.check(1) {
		t.Errorf("s.check(1) after pop = true, want false")
	}
}

func TestGroups(t *testing.T) {
	g := make(groups[string, int])
	
	// Test add
	g.add("a", 1)
	g.add("a", 2)
	g.add("b", 3)
	
	// Test check
	if !g.check("a") {
		t.Errorf("g.check('a') = false, want true")
	}
	
	if g.check("c") {
		t.Errorf("g.check('c') = true, want false")
	}
	
	// Test pop
	values := g.pop("a")
	expected := []int{1, 2}
	if !slices.Equal(values, expected) {
		t.Errorf("g.pop('a') = %v, want %v", values, expected)
	}
	
	if g.check("a") {
		t.Errorf("g.check('a') after pop = true, want false")
	}
}

func TestOrderedGrouping(t *testing.T) {
	og := grouping(make(groups[string, int]))
	
	// Test add
	og.add("b", 2)
	og.add("a", 1)
	og.add("b", 3)
	og.add("c", 4)
	
	// Test iter preserves order
	var keys []string
	var values [][]int
	for k, v := range og.iter() {
		keys = append(keys, k)
		values = append(values, v)
	}
	
	expectedKeys := []string{"b", "a", "c"}
	if !slices.Equal(keys, expectedKeys) {
		t.Errorf("ordered grouping keys = %v, want %v", keys, expectedKeys)
	}
	
	expectedValues := [][]int{{2, 3}, {1}, {4}}
	if len(values) != len(expectedValues) {
		t.Errorf("ordered grouping values length = %v, want %v", len(values), len(expectedValues))
	}
	for i, v := range values {
		if i < len(expectedValues) && !slices.Equal(v, expectedValues[i]) {
			t.Errorf("ordered grouping values[%d] = %v, want %v", i, v, expectedValues[i])
		}
	}
}

func TestElem(t *testing.T) {
	// Test NewElem
	elem := NewElem(42, "hello")
	if elem.First != 42 || elem.Second != "hello" {
		t.Errorf("NewElem(42, 'hello') = %v, want {42, 'hello'}", elem)
	}
	
	// Test WithElem
	elem2 := WithElem(5, func(i int) string { return string(rune('a' + i)) })
	if elem2.First != 5 || elem2.Second != "f" {
		t.Errorf("WithElem(5, func) = %v, want {5, 'f'}", elem2)
	}
	
	// Test Split
	a, b := elem.Split()
	if a != 42 || b != "hello" {
		t.Errorf("elem.Split() = (%v, %v), want (42, 'hello')", a, b)
	}
	
	// Test Apply
	var appliedA int
	var appliedB string
	elem.Apply(func(a int, b string) {
		appliedA = a
		appliedB = b
	})
	if appliedA != 42 || appliedB != "hello" {
		t.Errorf("elem.Apply() set (%v, %v), want (42, 'hello')", appliedA, appliedB)
	}
}

func TestElems(t *testing.T) {
	seq := func(yield func(int, string) bool) {
		yield(1, "a")
		yield(2, "b")
	}
	
	elemSeq := Elems(seq)
	result := Collect(elemSeq)
	
	expected := []Elem[int, string]{
		{First: 1, Second: "a"},
		{First: 2, Second: "b"},
	}
	
	if len(result) != len(expected) {
		t.Errorf("Elems() length = %v, want %v", len(result), len(expected))
	}
	for i, elem := range result {
		if i < len(expected) && (elem.First != expected[i].First || elem.Second != expected[i].Second) {
			t.Errorf("Elems()[%d] = %v, want %v", i, elem, expected[i])
		}
	}
}

func TestSplits(t *testing.T) {
	elems := []Elem[int, string]{
		{First: 1, Second: "a"},
		{First: 2, Second: "b"},
	}
	
	seq := Splits(Slice(elems))
	
	var result []struct{ a int; b string }
	for a, b := range seq {
		result = append(result, struct{ a int; b string }{a, b})
	}
	
	expected := []struct{ a int; b string }{
		{1, "a"},
		{2, "b"},
	}
	
	if len(result) != len(expected) {
		t.Errorf("Splits() length = %v, want %v", len(result), len(expected))
	}
	for i, item := range result {
		if i < len(expected) && (item.a != expected[i].a || item.b != expected[i].b) {
			t.Errorf("Splits()[%d] = %v, want %v", i, item, expected[i])
		}
	}
}
