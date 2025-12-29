package irt

import (
	"context"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
)

// Tests for internal/non-exported functions

func TestCounterHelper(t *testing.T) {
	c := counter()

	tests := []int{1, 2, 3, 4, 5}
	for _, expected := range tests {
		result := c()
		if result != expected {
			t.Errorf("counter() = %v, want %v", result, expected)
		}
	}
}

func TestSeenHelper(t *testing.T) {
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

func TestEqualHelper(t *testing.T) {
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

func TestFirstHelper(t *testing.T) {
	tests := []struct {
		name     string
		a, b     any
		expected any
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

func TestSecondHelper(t *testing.T) {
	tests := []struct {
		name     string
		a, b     any
		expected any
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

func TestFlipFuncHelper(t *testing.T) {
	tests := []struct {
		name string
		a, b any
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

func TestNoopHelper(t *testing.T) {
	tests := []struct {
		name  string
		input any
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

func TestWrapHelper(t *testing.T) {
	var callCount atomic.Int32
	op := func() string {
		callCount.Add(1)
		return "result"
	}

	wrapped := wrap(op, func(in string) int {
		if in != "result" {
			t.Fatalf("got %s", in)
		}
		return 52
	})

	result, num := wrapped()
	if result != "result" {
		t.Errorf("wrap(op)(42) = %v, want 'result'", result)
	}
	if num != 52 {
		t.Errorf("wrap(op)(42) = %v, want '52'", result)
	}
	if callCount.Load() != 1 {
		t.Errorf("wrapped function called %d times, want 1", callCount.Load())
	}
}

func TestArgsHelper(t *testing.T) {
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

func TestNotfHelper(t *testing.T) {
	isEven := func(i int) bool { return i%2 == 0 }
	isOdd := notf(isEven)

	tests := []struct {
		input    int
		expected bool
	}{
		{2, false}, // 2 is even, so isOdd(2) should be false
		{3, true},  // 3 is odd, so isOdd(3) should be true
		{4, false}, // 4 is even, so isOdd(4) should be false
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

func TestIsNilHelper(t *testing.T) {
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

func TestIsZeroHelper(t *testing.T) {
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

func TestPtrHelper(t *testing.T) {
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

func TestDerefHelper(t *testing.T) {
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

func TestDerefzHelper(t *testing.T) {
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

func TestIdxorzHelper(t *testing.T) {
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
			result := idxorz(tt.slice, tt.idx)
			if result != tt.expected {
				t.Errorf("idxorz(%v, %v) = %v, want %v", tt.idx, tt.slice, result, tt.expected)
			}
		})
	}
}

func TestIfelseHelper(t *testing.T) {
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

func TestIfelsedoHelper(t *testing.T) {
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

func TestNtimesHelper(t *testing.T) {
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

			fn := repeat(tt.times, op)
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

func TestWithlimitHelper(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		input    []int
		expected []int
	}{
		{"limit 0 oops", 0, []int{1, 2, 3}, nil}, // Special case
		{"limit 1", 1, []int{1, 2, 3}, []int{1}},
		{"limit equals input", 3, []int{1, 2, 3}, []int{1, 2, 3}},
		{"limit exceeds input", 5, []int{1, 2, 3}, []int{1, 2, 3}},
		{"empty input", 3, []int{}, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := 0
			op := func() (int, *bool) {
				if idx >= len(tt.input) {
					return 0, ptr(false)
				}
				val := tt.input[idx]
				idx++
				return val, ptr(true)
			}

			fn := repeat2(tt.limit, op)
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

func TestSetHelper(t *testing.T) {
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

func TestGroupsHelper(t *testing.T) {
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

func TestOrderedGroupingHelper(t *testing.T) {
	og := grouping(make(groups[string, int]))

	// Test add
	og.add("b", 2)
	og.add("a", 1)
	og.add("b", 3)
	og.add("c", 4)

	// Test iter preserves order
	keys := make([]string, 0, 3)
	values := make([][]int, 0, 3)
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

func TestElemHelper(t *testing.T) {
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

func TestElemsHelper(t *testing.T) {
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

func TestSplitsHelper(t *testing.T) {
	elems := []Elem[int, string]{
		{First: 1, Second: "a"},
		{First: 2, Second: "b"},
	}

	seq := ElemsSplit(Slice(elems))

	result := make([]struct {
		a int
		b string
	}, 0, 3)
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

func TestMapPop(t *testing.T) {
	// Initialize a map with some key-value pairs
	input := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	// Test popping an existing key
	keyToPop := "b"
	value, ok := mapPop(input, keyToPop)

	// Verify the value and existence
	if !ok {
		t.Fatalf("mapPop() failed to find key %q, expected it to exist", keyToPop)
	}
	if value != 2 {
		t.Errorf("mapPop() returned value %v for key %q, want %v", value, keyToPop, 2)
	}

	// Verify the key is removed from the map
	if _, exists := input[keyToPop]; exists {
		t.Errorf("mapPop() did not remove key %q from the map", keyToPop)
	}

	// Test popping a non-existing key
	nonExistentKey := "z"
	value, ok = mapPop(input, nonExistentKey)

	// Verify the value and existence
	if ok {
		t.Errorf("mapPop() found key %q, expected it to not exist", nonExistentKey)
	}
	if value != 0 {
		t.Errorf("mapPop() returned value %v for non-existent key %q, want %v", value, nonExistentKey, 0)
	}
}

func TestConjunctionHelper(t *testing.T) {
	t.Run("Or", func(t *testing.T) {
		t.Run("True", func(t *testing.T) {
			t.Run("Late", func(t *testing.T) {
				count := 0
				result := orf(
					func() bool { count++; return false },
					func() bool { count++; return false },
					func() bool { count++; return false },
					func() bool { count++; return false },
					func() bool { count++; return false },
					func() bool { count++; return false },
					func() bool { count++; return true },
					func() bool { count++; return false },
				)
				if !result {
					t.Error("unexpected result")
				}
				if count != 7 {
					t.Error("unexpected early return", count)
				}
			})
			t.Run("Early", func(t *testing.T) {
				count := 0
				result := orf(
					func() bool { count++; return true },
					func() bool { count++; return false },
					func() bool { count++; return false },
					func() bool { count++; return false },
				)
				if !result {
					t.Error("unexpected result")
				}
				if count != 1 {
					t.Error("unexpected early return")
				}
			})
		})
	})
	t.Run("And", func(t *testing.T) {
		t.Run("False", func(t *testing.T) {
			count := 0
			result := andf(
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { return false },
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { count++; return true },
			)
			if result {
				t.Error("unexpected result")
			}
			if count != 5 {
				t.Error("unexpected early return", count)
			}
		})
		t.Run("True", func(t *testing.T) {
			count := 0
			result := andf(
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { count++; return true },
				func() bool { count++; return true },
			)
			if !result {
				t.Error("unexpected result")
			}
			if count != 4 {
				t.Error("unexpected early return")
			}
		})
	})

	t.Run("Value", func(t *testing.T) {
		t.Run("And", func(t *testing.T) {
			t.Run("False", func(t *testing.T) {
				result := andv(
					true,
					false,
				)
				if result {
					t.Error("unexpected result value")
				}
			})
			t.Run("True", func(t *testing.T) {
				result := andv(
					true,
					true,
				)
				if !result {
					t.Error("unexpected result value")
				}
			})
		})
		t.Run("Or", func(t *testing.T) {
			t.Run("True", func(t *testing.T) {
				result := orv(
					true,
					false,
				)
				if !result {
					t.Error("unexpected result value")
				}
			})
			t.Run("False", func(t *testing.T) {
				result := orv(
					false,
					false,
				)
				if result {
					t.Error("unexpected result value")
				}
			})
		})
	})
}

func TestWhenopHelpers(t *testing.T) {
	t.Run("whenop", func(t *testing.T) {
		var called bool
		op := func() { called = true }
		whenop(op)
		if !called {
			t.Error("whenop did not call the operation")
		}
	})

	t.Run("whenopwith", func(t *testing.T) {
		var result int
		op := func(v int) { result = v }
		whenopwith(op, 42)
		if result != 42 {
			t.Errorf("whenopwith did not pass the correct argument, got %v, want 42", result)
		}
	})

	t.Run("whenopdo", func(t *testing.T) {
		op := func() int { return 42 }
		result := whenopdo(op)
		if result != 42 {
			t.Errorf("whenopdo did not return the correct value, got %v, want 42", result)
		}
	})

	t.Run("whenopdowith", func(t *testing.T) {
		op := func(v int) string { return string(rune('a' + v)) }
		result := whenopdowith(op, 5)
		if result != "f" {
			t.Errorf("whenopdowith did not return the correct value, got %v, want 'f'", result)
		}
	})
}

func TestFuncallHelpers(t *testing.T) {
	t.Run("funcall", func(t *testing.T) {
		var result int
		op := func(v int) { result = v }
		funcall(op, 42)
		if result != 42 {
			t.Errorf("funcall did not pass the correct argument, got %v, want 42", result)
		}
	})

	t.Run("funcallok", func(t *testing.T) {
		var result int
		op := func(v int) { result = v }
		ok := funcallok(op, 42)
		if !ok {
			t.Error("funcallok did not return true")
		}
		if result != 42 {
			t.Errorf("funcallok did not pass the correct argument, got %v, want 42", result)
		}
	})
	t.Run("funcallv", func(t *testing.T) {
		var result []int
		op := func(args ...int) {
			result = args
		}
		funcallv(op, 1, 2, 3)
		expected := []int{1, 2, 3}
		if !slices.Equal(result, expected) {
			t.Errorf("funcallv did not pass the correct arguments, got %v, want %v", result, expected)
		}
	})
	t.Run("funcalls", func(t *testing.T) {
		var result []int
		op := func(args []int) {
			result = args
		}
		funcalls(op, []int{4, 5, 6})
		expected := []int{4, 5, 6}
		if !slices.Equal(result, expected) {
			t.Errorf("funcalls did not pass the correct arguments, got %v, want %v", result, expected)
		}
	})
	t.Run("funcallr", func(t *testing.T) {
		op := func(v int) string {
			return string(rune('a' + v))
		}
		result := funcallr(op, 5)
		expected := "f"
		if result != expected {
			t.Errorf("funcallr did not return the correct value, got %v, want %v", result, expected)
		}
	})
}

func TestMethodizeHelpers(t *testing.T) {
	t.Run("methodize", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		method := func(r receiver, v int) int { return r.value + v }
		m := methodize(r, method)
		result := m(8)
		if result != 50 {
			t.Errorf("methodize did not return the correct value, got %v, want 50", result)
		}
	})

	t.Run("methodize1", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		method := func(r receiver, v int) int { return r.value + v }
		m := methodize1(r, method)
		result := m(8)
		if result != 50 {
			t.Errorf("methodize1 did not return the correct value, got %v, want 50", result)
		}
	})

	t.Run("methodize2", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		method := func(r receiver, v int) (int, string) { return r.value + v, "ok" }
		m := methodize2(r, method)
		result1, result2 := m(8)
		if result1 != 50 || result2 != "ok" {
			t.Errorf("methodize2 did not return the correct values, got (%v, %v), want (50, 'ok')", result1, result2)
		}
	})

	t.Run("methodize3", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		method := func(r receiver, v int, s string) string { return s + string(rune('a'+v)) }
		m := methodize3(r, method)
		result := m(5, "prefix-")
		if result != "prefix-f" {
			t.Errorf("methodize3 did not return the correct value, got %v, want 'prefix-f'", result)
		}
	})
	t.Run("methodize4", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		method := func(r receiver, v int, s string) (int, string) {
			return r.value + v, s + string(rune('a'+v))
		}
		m := methodize4(r, method)
		result1, result2 := m(5, "prefix-")
		if result1 != 47 {
			t.Errorf("methodize4 did not return the correct first value, got %v, want 47", result1)
		}
		if result2 != "prefix-f" {
			t.Errorf("methodize4 did not return the correct second value, got %v, want 'prefix-f'", result2)
		}
	})

	t.Run("withctx", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		op := func(ctx context.Context, v int) int {
			return r.value + v
		}
		ctx := context.Background()
		m := withctx(ctx, op)
		result := m(8)
		if result != 50 {
			t.Errorf("withctx did not return the correct value, got %v, want 50", result)
		}
	})

	t.Run("withctx1", func(t *testing.T) {
		ctx := context.Background()
		op := func(ictx context.Context, v int, s string) string {
			if ictx == nil || ictx != ctx {
				t.Error("unexpected internal value")
			}
			return s + string(rune('a'+v))
		}
		m := withctx1(ctx, op)
		result := m(5, "prefix-")
		if result != "prefix-f" {
			t.Errorf("withctx1 did not return the correct value, got %v, want 'prefix-f'", result)
		}
	})

	t.Run("withctx2", func(t *testing.T) {
		type receiver struct {
			value int
		}
		ctx := context.Background()
		r := receiver{value: 42}
		op := func(ictx context.Context, v int, s string) (int, string) {
			if ictx == nil || ictx != ctx {
				t.Error("unexpected internal value")
			}
			return r.value + v, s + string(rune('a'+v))
		}
		m := withctx2(ctx, op)
		result1, result2 := m(5, "prefix-")
		if result1 != 47 {
			t.Errorf("withctx2 did not return the correct first value, got %v, want 47", result1)
		}
		if result2 != "prefix-f" {
			t.Errorf("withctx2 did not return the correct second value, got %v, want 'prefix-f'", result2)
		}
	})

	t.Run("methodizethread", func(t *testing.T) {
		type receiver struct {
			value int
		}
		r := receiver{value: 42}
		op := func(r receiver, v int) (int, string) {
			return r.value + 100, "elephant" + strconv.Itoa(v)
		}
		wrap := func(i int, s string) string {
			return s + "pig"
		}
		m := methodizethread(r, op, wrap)
		result := m(8)
		if result != "elephant8pig" {
			t.Errorf("methodizethread did not return the correct value, got %v, want 'f5'", result)
		}
	})

	t.Run("threadzip", func(t *testing.T) {
		op := func(v int) (int, string) {
			return v + 1, string(rune('a' + v))
		}
		wrap := func(i int, s string) string {
			return s + string(rune('0'+i))
		}
		m := threadzip(op, wrap)
		result := m(5)
		if result != "f6" {
			t.Errorf("threadzip did not return the correct value, got %v, want 'f6'", result)
		}
	})
}

func TestWraping(t *testing.T) {
	t.Run("wrap", func(t *testing.T) {
		op := func() int {
			return 42
		}
		wrapped := wrap(op, func(v int) string {
			if v != 42 {
				t.Error("got unexpected input", 42)
			}
			return "elephant" + strconv.Itoa(v)
		})
		result1, result2 := wrapped()
		if result1 != 42 {
			t.Errorf("wrap did not return the correct first value, got %v, want 42", result1)
		}
		if result2 != "elephant42" {
			t.Errorf("wrap did not return the correct second value, got %v, want 'k'", result2)
		}
	})
	t.Run("curry", func(t *testing.T) {
		op := func(v int) string {
			return string(rune('a' + v))
		}
		curried := curry(op, 5)
		result := curried()
		expected := "f"
		if result != expected {
			t.Errorf("curry did not return the correct value, got %v, want %v", result, expected)
		}
	})
}

func TestWhenCallHelpers(t *testing.T) {
	t.Run("whencallok", func(t *testing.T) {
		var called bool
		op := func() { called = true }
		ok := whencallok(true, op)
		if !ok {
			t.Error("whencallok did not return true")
		}
		if !called {
			t.Error("whencallok did not call the operation")
		}
	})

	t.Run("whencallwithok", func(t *testing.T) {
		var result int
		op := func(v int) { result = v }
		ok := whencallwithok(true, op, 42)
		if !ok {
			t.Error("whencallwithok did not return true")
		}
		if result != 42 {
			t.Errorf("whencallwithok did not pass the correct argument, got %v, want 42", result)
		}
	})

	t.Run("whencallfn", func(t *testing.T) {
		var called bool
		op := func() { called = true }
		fn := whencallfn(true, op)
		fn()
		if !called {
			t.Error("whencallfn did not call the operation")
		}
	})

	t.Run("whencallwithfn", func(t *testing.T) {
		var result int
		op := func(v int) { result = v }
		fn := whencallwithfn(true, op, 42)
		fn()
		if result != 42 {
			t.Errorf("whencallwithfn did not pass the correct argument, got %v, want 42", result)
		}
	})
}

func TestWhenDoHelpers(t *testing.T) {
	t.Run("whendook", func(t *testing.T) {
		op := func() int { return 42 }
		result, ok := whendook(true, op)
		if !ok {
			t.Error("whendook did not return true")
		}
		if result != 42 {
			t.Errorf("whendook did not return the correct value, got %v, want 42", result)
		}
	})

	t.Run("whendofn", func(t *testing.T) {
		op := func() int { return 42 }
		fn := whendofn(true, op)
		result := fn()
		if result != 42 {
			t.Errorf("whendofn did not return the correct value, got %v, want 42", result)
		}
	})

	t.Run("whendowithok", func(t *testing.T) {
		op := func(v int) string { return string(rune('a' + v)) }
		result, ok := whendowithok(true, op, 5)
		if !ok {
			t.Error("whendowithok did not return true")
		}
		if result != "f" {
			t.Errorf("whendowithok did not return the correct value, got %v, want 'f'", result)
		}
	})

	t.Run("whendowithfn", func(t *testing.T) {
		op := func(v int) string { return string(rune('a' + v)) }
		fn := whendowithfn(true, op, 5)
		result := fn()
		if result != "f" {
			t.Errorf("whendowithfn did not return the correct value, got %v, want 'f'", result)
		}
	})
}

func TestElemOrderingSemantics(t *testing.T) {
	t.Run("ElemCmp", func(t *testing.T) {
		elem1 := Elem[int, string]{First: 1, Second: "a"}
		elem2 := Elem[int, string]{First: 2, Second: "b"}
		elem3 := Elem[int, string]{First: 1, Second: "c"}

		t.Run("Compare by First", func(t *testing.T) {
			if ElemCmpFirst(elem1, elem2) >= 0 {
				t.Errorf("Expected elem1 < elem2, got %v", ElemCmpFirst(elem1, elem2))
			}
			if ElemCmpFirst(elem2, elem1) <= 0 {
				t.Errorf("Expected elem2 > elem1, got %v", ElemCmpFirst(elem2, elem1))
			}
			if ElemCmpFirst(elem1, elem3) != 0 {
				t.Errorf("Expected elem1 == elem3, got %v", ElemCmpFirst(elem1, elem3))
			}
		})

		t.Run("Compare by Second", func(t *testing.T) {
			if ElemCmpSecond(elem1, elem3) >= 0 {
				t.Errorf("Expected elem1 < elem3, got %v", ElemCmpSecond(elem1, elem3))
			}
			if ElemCmpSecond(elem3, elem1) <= 0 {
				t.Errorf("Expected elem3 > elem1, got %v", ElemCmpSecond(elem3, elem1))
			}
			if ElemCmpSecond(elem1, elem1) != 0 {
				t.Errorf("Expected elem1 == elem1, got %v", ElemCmpSecond(elem1, elem1))
			}
		})
	})

	t.Run("Compare", func(t *testing.T) {
		elem1 := Elem[int, string]{First: 1, Second: "a"}
		elem2 := Elem[int, string]{First: 2, Second: "b"}
		if elem1.Compare(cmpf, cmpf).With(elem2) >= 0 {
			t.Errorf("Expected elem1 < elem2, got %v", elem1.Compare(cmpf, cmpf).With(elem2))
		}
		if elem2.Compare(cmpf, cmpf).With(elem1) <= 0 {
			t.Errorf("Expected elem2 > elem1, got %v", elem2.Compare(cmpf, cmpf).With(elem1))
		}
		if elem1.Compare(cmpf, cmpf).With(elem1) != 0 {
			t.Errorf("Expected elem1 == elem1, got %v", elem1.Compare(cmpf, cmpf).With(elem1))
		}
	})

	t.Run("ElemCmp", func(t *testing.T) {
		elem1 := Elem[int, string]{First: 1, Second: "a"}
		elem2 := Elem[int, string]{First: 2, Second: "b"}
		if ElemCmp(elem1, elem2) >= 0 {
			t.Errorf("Expected elem1 < elem2, got %v", ElemCmp(elem1, elem2))
		}
		if ElemCmp(elem2, elem1) <= 0 {
			t.Errorf("Expected elem2 > elem1, got %v", ElemCmp(elem2, elem1))
		}
		if ElemCmp(elem1, elem1) != 0 {
			t.Errorf("Expected elem1 == elem1, got %v", ElemCmp(elem1, elem1))
		}
	})

	t.Run("WithPanicsWithoutComparators", func(t *testing.T) {
		elem := elemcmp[int, int]{lh: NewElem(100, 200)}
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		if c := elem.With(NewElem(1, 2)); c != -1 {
			t.Error("should never run, but this is even weirder if it does", c)
		}
	})

	t.Run("CompareFirst", func(t *testing.T) {
		elem1 := Elem[int, string]{First: 1, Second: "a"}
		elem2 := Elem[int, string]{First: 2, Second: "b"}
		if elem1.CompareFirst(cmpf).With(elem2) >= 0 {
			t.Errorf("Expected elem1 < elem2 by First, got %v", elem1.CompareFirst(cmpf).With(elem2))
		}
		if elem2.CompareFirst(cmpf).With(elem1) <= 0 {
			t.Errorf("Expected elem2 > elem1 by First, got %v", elem2.CompareFirst(cmpf).With(elem1))
		}
		if elem1.CompareFirst(cmpf).With(elem1) != 0 {
			t.Errorf("Expected elem1 == elem1 by First, got %v", elem1.CompareFirst(cmpf).With(elem1))
		}
	})

	t.Run("CompareSecond", func(t *testing.T) {
		elem1 := Elem[int, string]{First: 1, Second: "a"}
		elem2 := Elem[int, string]{First: 1, Second: "b"}
		if elem1.CompareSecond(cmpf).With(elem2) >= 0 {
			t.Errorf("Expected elem1 < elem2 by Second, got %v", elem1.CompareSecond(cmpf).With(elem2))
		}
		if elem2.CompareSecond(cmpf).With(elem1) <= 0 {
			t.Errorf("Expected elem2 > elem1 by Second, got %v", elem2.CompareSecond(cmpf).With(elem1))
		}
		if elem1.CompareSecond(cmpf).With(elem1) != 0 {
			t.Errorf("Expected elem1 == elem1 by Second, got %v", elem1.CompareSecond(cmpf).With(elem1))
		}
	})
}
