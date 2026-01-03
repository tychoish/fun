package irt

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
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

func TestIfcallelsecallHelper(t *testing.T) {
	t.Run("TrueCondition", func(t *testing.T) {
		thenCalled := false
		elseCalled := false
		then := func() { thenCalled = true }
		elsewise := func() { elseCalled = true }

		ifcallelsecall[any](true, then, elsewise)

		if !thenCalled {
			t.Error("then was not called")
		}
		if elseCalled {
			t.Error("elsewise was called")
		}
	})

	t.Run("FalseCondition", func(t *testing.T) {
		thenCalled := false
		elseCalled := false
		then := func() { thenCalled = true }
		elsewise := func() { elseCalled = true }

		ifcallelsecall[any](false, then, elsewise)

		if thenCalled {
			t.Error("then was called")
		}
		if !elseCalled {
			t.Error("elsewise was not called")
		}
	})
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
	t.Run("RepatOK", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			count := 0
			op := repeatok(4, func() (int, bool) { count++; return count, true })
			for seen, tri := op(); tri != nil && deref(tri); seen, tri = op() {
				if count != seen {
					t.Error("unexpected value, count=", count, "seen=", seen)
				}
			}
			thenNum, thenVal := op()
			if thenNum != 0 {
				t.Error("got", thenNum)
			}
			if thenVal != nil {
				t.Error("did got over limit signal", deref(thenVal))
			}
			if count != 4 {
				t.Error("unexpected:", count)
			}
		})
		t.Run("EarlyReturn", func(t *testing.T) {
			count := 0
			op := repeatok(4, func() (int, bool) { count++; return count, true })
			for seen, tri := op(); tri != nil && deref(tri); seen, tri = op() {
				if count == 2 {
					break
				}
				if count != seen {
					t.Error("unexpected value, count=", count, "seen=", seen)
				}
			}
			thenNum, thenVal := op()
			if thenNum != 3 {
				t.Error("got", thenNum)
			}
			if thenVal == nil {
				t.Fatal("got over limit signal")
			}
			if !deref(thenVal) {
				t.Error("there should be one more iteration ")
			}
			if count != 3 {
				t.Error("unexpected:", count)
			}
			thenNum, thenVal = op()
			if thenNum != 4 {
				t.Error(thenNum)
			}
			if thenVal == nil || !deref(thenVal) {
				t.Error(thenVal, deref(thenVal))
			}

			thenNum, thenVal = op()
			if thenNum != 0 {
				t.Error("got", thenNum)
			}
			if thenVal != nil {
				t.Error("did not get over limit signal")
			}
			if count != 4 {
				t.Error("unexpected:", count)
			}
		})
	})
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

func TestYieldHelpers(t *testing.T) {
	t.Run("yieldTo", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch := make(chan int, 10)
		y := yieldTo(ctx, ch)

		if !y(1) {
			t.Error("yieldTo(1) returned false")
		}
		if !y(2) {
			t.Error("yieldTo(2) returned false")
		}

		cancel()
		// After cancel, yieldTo should return false
		if y(3) {
			t.Error("yieldTo(3) returned true after cancel")
		}

		close(ch)
		var results []int
		for v := range ch {
			results = append(results, v)
		}
		if !slices.Equal(results, []int{1, 2}) {
			t.Errorf("got %v, want [1, 2]", results)
		}
	})

	t.Run("yieldFrom", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			ctx := context.Background()
			ch := make(chan int, 2)
			ch <- 1
			ch <- 2
			close(ch)

			var results []int
			yield := func(v int) bool {
				results = append(results, v)
				return true
			}

			if !yieldFrom(ctx, ch, yield) {
				t.Error("yieldFrom returned false on first item")
			}
			if !yieldFrom(ctx, ch, yield) {
				t.Error("yieldFrom returned false on second item")
			}
			if yieldFrom(ctx, ch, yield) {
				t.Error("yieldFrom returned true on closed channel")
			}

			if !slices.Equal(results, []int{1, 2}) {
				t.Errorf("got %v, want [1, 2]", results)
			}
		})

		t.Run("ContextCancel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			ch := make(chan int, 1)
			ch <- 1
			cancel()

			yield := func(v int) bool { return true }
			if yieldFrom(ctx, ch, yield) {
				t.Error("yieldFrom returned true after cancel")
			}
		})

		t.Run("YieldFalse", func(t *testing.T) {
			ctx := context.Background()
			ch := make(chan int, 1)
			ch <- 1

			yield := func(v int) bool { return false }
			if yieldFrom(ctx, ch, yield) {
				t.Error("yieldFrom returned true when yield returned false")
			}
		})
	})
}

func TestYieldHookHelpers(t *testing.T) {
	t.Run("yieldPre", func(t *testing.T) {
		var order []string
		h := func() { order = append(order, "hook") }
		yield := func(in int) bool {
			order = append(order, "yield")
			return in > 0
		}

		wrapped := yieldPre(h, yield)

		if !wrapped(1) {
			t.Error("expected true")
		}
		if !slices.Equal(order, []string{"hook", "yield"}) {
			t.Errorf("unexpected order: %v", order)
		}

		order = nil
		if wrapped(0) {
			t.Error("expected false")
		}
		if !slices.Equal(order, []string{"hook", "yield"}) {
			t.Errorf("unexpected order: %v", order)
		}
	})

	t.Run("yieldPost", func(t *testing.T) {
		var order []string
		h := func() { order = append(order, "hook") }
		yield := func(in int) bool {
			order = append(order, "yield")
			return in > 0
		}

		wrapped := yieldPost(h, yield)

		if !wrapped(1) {
			t.Error("expected true")
		}
		if !slices.Equal(order, []string{"yield", "hook"}) {
			t.Errorf("unexpected order: %v", order)
		}

		order = nil
		if wrapped(0) {
			t.Error("expected false")
		}
		if !slices.Equal(order, []string{"yield", "hook"}) {
			t.Errorf("unexpected order: %v", order)
		}
	})

	t.Run("yieldPreHook", func(t *testing.T) {
		var observed int
		var order []string
		h := func(in int) {
			observed = in
			order = append(order, "hook")
		}
		yield := func(in int) bool {
			order = append(order, "yield")
			return in > 0
		}

		wrapped := yieldPreHook(h, yield)

		if !wrapped(42) {
			t.Error("expected true")
		}
		if observed != 42 {
			t.Errorf("expected 42, got %d", observed)
		}
		if !slices.Equal(order, []string{"hook", "yield"}) {
			t.Errorf("unexpected order: %v", order)
		}
	})

	t.Run("yieldPostHook", func(t *testing.T) {
		var observed int
		var order []string
		h := func(in int) {
			observed = in
			order = append(order, "hook")
		}
		yield := func(in int) bool {
			order = append(order, "yield")
			return in > 0
		}

		wrapped := yieldPostHook(h, yield)

		if !wrapped(42) {
			t.Error("expected true")
		}
		if observed != 42 {
			t.Errorf("expected 42, got %d", observed)
		}
		if !slices.Equal(order, []string{"yield", "hook"}) {
			t.Errorf("unexpected order: %v", order)
		}
	})
}

func TestPredicateSemantics(t *testing.T) {
	t.Run("Comparison", func(t *testing.T) {
		if !predLT(10)(5) {
			t.Error("predLT")
		}
		if predLT(10)(10) {
			t.Error("predLT eq")
		}
		if !predGT(5)(10) {
			t.Error("predGT")
		}
		if predGT(5)(5) {
			t.Error("predGT eq")
		}
		if !predEQ(5)(5) {
			t.Error("predEQ")
		}
		if predEQ(5)(6) {
			t.Error("predEQ neq")
		}
		if !predLTE(10)(5) || !predLTE(10)(10) {
			t.Error("predLTE")
		}
		if predLTE(10)(11) {
			t.Error("predLTE gt")
		}
		if !predGTE(5)(10) || !predGTE(5)(5) {
			t.Error("predGTE")
		}
		if predGTE(5)(4) {
			t.Error("predGTE lt")
		}
	})

	t.Run("Not", func(t *testing.T) {
		if not(true) {
			t.Error("not true")
		}
		if !not(false) {
			t.Error("not false")
		}

		if notf2(func(a, b int) bool { return a == b })(1, 1) {
			t.Error("notf2")
		}
	})

	t.Run("NilAndZero", func(t *testing.T) {
		var c chan int
		if !isNilChan(c) {
			t.Error("isNilChan nil")
		}
		c = make(chan int)
		if isNilChan(c) {
			t.Error("isNilChan non-nil")
		}

		if isNotZero(0) {
			t.Error("isNotZero zero")
		}
		if !isNotZero(1) {
			t.Error("isNotZero non-zero")
		}
	})

	t.Run("OkAndError", func(t *testing.T) {
		if !isOk(0, true) {
			t.Error("isOk true")
		}
		if isOk(0, false) {
			t.Error("isOk false")
		}

		err := errors.New("error")
		if !isError(err) {
			t.Error("isError")
		}
		if isError(nil) {
			t.Error("isError nil")
		}
		if !isError2(0, err) {
			t.Error("isError2")
		}
		if isError2(0, nil) {
			t.Error("isError2 nil")
		}

		if isSuccess(err) {
			t.Error("isSuccess")
		}
		if !isSuccess(nil) {
			t.Error("isSuccess nil")
		}
		if isSuccess2(0, err) {
			t.Error("isSuccess2")
		}
		if !isSuccess2(0, nil) {
			t.Error("isSuccess2 nil")
		}
	})

	t.Run("isWithin", func(t *testing.T) {
		if !isWithin(0, 1) {
			t.Error("isWithin start")
		}
		if !isWithin(1, 2) {
			t.Error("isWithin end")
		}
		if isWithin(-1, 1) {
			t.Error("isWithin neg")
		}
		if isWithin(1, 1) {
			t.Error("isWithin out")
		}
	})

	t.Run("withcheck", func(t *testing.T) {
		err := errors.New("error")
		v, ok := withcheck(42, err)
		if !ok || v != 42 {
			t.Errorf("withcheck err: %v, %v", v, ok)
		}
		v, ok = withcheck(42, nil)
		if ok || v != 0 {
			t.Errorf("withcheck nil: %v, %v", v, ok)
		}
	})
}

func TestResultManipulators(t *testing.T) {
	t.Run("flipfn", func(t *testing.T) {
		op := func(a int, b string) (int, string) { return a + 1, b + "!" }
		flipped := flipfn(op)
		s, i := flipped("hi", 10)
		if s != "hi!" || i != 11 {
			t.Errorf("got %v, %v", s, i)
		}
	})
	t.Run("ignoreSecond", func(t *testing.T) {
		op := func(a int) int { return a * 2 }
		ignored := ignoreSecond[int, string](op)
		if ignored(21, "ignored") != 42 {
			t.Error("wrong result")
		}
	})
	t.Run("ignoreFirst", func(t *testing.T) {
		op := func(b string) string { return b + "!" }
		ignored := ignoreFirst[int, string](op)
		if ignored(123, "hi") != "hi!" {
			t.Error("wrong result")
		}
	})
}

func TestDefaultConstructors(t *testing.T) {
	t.Run("orDefault", func(t *testing.T) {
		if orDefault(0, 42) != 42 {
			t.Error("zero value not replaced")
		}
		if orDefault(10, 42) != 10 {
			t.Error("non-zero value replaced")
		}
	})
	t.Run("orDefaultNew", func(t *testing.T) {
		called := false
		op := func() int { called = true; return 42 }
		if orDefaultNew(0, op) != 42 || !called {
			t.Error("zero value not replaced or op not called")
		}
		called = false
		if orDefaultNew(10, op) != 10 || called {
			t.Error("non-zero value replaced or op called")
		}
	})
}

func TestCounterHelpersExtended(t *testing.T) {
	t.Run("counterLT", func(t *testing.T) {
		lt3 := counterLT(3)
		if !lt3() {
			t.Error("1 < 3")
		}
		if !lt3() {
			t.Error("2 < 3")
		}
		if lt3() {
			t.Error("3 < 3")
		}
	})
	t.Run("counterLTE", func(t *testing.T) {
		lte3 := counterLTE(3)
		if !lte3() {
			t.Error("1 <= 3")
		}
		if !lte3() {
			t.Error("2 <= 3")
		}
		if !lte3() {
			t.Error("3 <= 3")
		}
		if lte3() {
			t.Error("4 <= 3")
		}
	})
}

func TestLazyPointerHelpers(t *testing.T) {
	t.Run("ptrznillazy", func(t *testing.T) {
		fn := ptrznillazy(0)
		if fn() != nil {
			t.Error("expected nil for zero value")
		}
		fn = ptrznillazy(42)
		p := fn()
		if p == nil || *p != 42 {
			t.Errorf("expected 42, got %v", p)
		}
	})
	t.Run("derefzlazy", func(t *testing.T) {
		var p *int
		fn := derefzlazy(p)
		if fn() != 0 {
			t.Error("expected zero value for nil pointer")
		}
		val := 42
		p = &val
		fn = derefzlazy(p)
		if fn() != 42 {
			t.Errorf("expected 42, got %v", fn())
		}
	})
}

func TestSliceAccessors(t *testing.T) {
	sl := []int{10, 20, 30}
	t.Run("idx", func(t *testing.T) {
		if idx(sl, 1) != 20 {
			t.Error("wrong value")
		}
	})
	t.Run("idxfn", func(t *testing.T) {
		fn := idxfn(sl, 2)
		if fn() != 30 {
			t.Error("wrong value")
		}
	})
	t.Run("idxcheck", func(t *testing.T) {
		if !idxcheck(sl, 0) {
			t.Error("0 should be valid")
		}
		if idxcheck(sl, 3) {
			t.Error("3 should be invalid")
		}
		if idxcheck(sl, -1) {
			t.Error("-1 should be invalid")
		}
	})
	t.Run("idxorzfn", func(t *testing.T) {
		fn := idxorzfn(sl)
		if fn(1) != 20 {
			t.Error("wrong value for valid index")
		}
		if fn(5) != 0 {
			t.Error("wrong value for invalid index")
		}
	})
}

func TestSendTo(t *testing.T) {
	t.Run("BlockedAndCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan int) // unbuffered
		cancel()
		if sendTo(ctx, 1, ch) {
			t.Error("expected false")
		}
	})
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		ch := make(chan int, 1)
		if !sendTo(ctx, 1, ch) {
			t.Error("expected true")
		}
		if <-ch != 1 {
			t.Error("wrong value")
		}
	})
}

func TestRecieveFrom(t *testing.T) {
	t.Run("Canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan int)
		cancel()
		_, ok := recieveFrom(ctx, ch)
		if ok {
			t.Error("expected !ok")
		}
	})
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		ch := make(chan int, 1)
		ch <- 42
		val, ok := recieveFrom(ctx, ch)
		if !ok || val != 42 {
			t.Errorf("got %v, %v", val, ok)
		}
	})
}

func TestMapIterators(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	t.Run("keys", func(t *testing.T) {
		k := Collect(keys(m))
		slices.Sort(k)
		if !slices.Equal(k, []string{"a", "b"}) {
			t.Errorf("got %v", k)
		}
	})
	t.Run("values", func(t *testing.T) {
		v := Collect(values(m))
		slices.Sort(v)
		if !slices.Equal(v, []int{1, 2}) {
			t.Errorf("got %v", v)
		}
	})
}

func TestSeenKey(t *testing.T) {
	s := seenkey[string, int]()
	if s("a", 1) {
		t.Error("first time should be false")
	}
	if !s("a", 2) {
		t.Error("second time with same key should be true")
	}
	if s("b", 1) {
		t.Error("new key should be false")
	}
}

func TestOnceHelpers(t *testing.T) {
	t.Run("oncev", func(t *testing.T) {
		count := 0
		op := func() int { count++; return 42 }
		fn := oncev(op)
		if fn() != 42 || fn() != 42 || count != 1 {
			t.Errorf("count=%d", count)
		}
	})
	t.Run("oncevs", func(t *testing.T) {
		count := 0
		op := func() (int, string) { count++; return 42, "hi" }
		fn := oncevs(op)
		a, b := fn()
		c, d := fn()
		if a != 42 || b != "hi" || c != 42 || d != "hi" || count != 1 {
			t.Errorf("count=%d", count)
		}
	})
}

func TestGoOnce(t *testing.T) {
	t.Run("RunsOnce", func(t *testing.T) {
		var count atomic.Int32
		op := func() {
			count.Add(1)
		}

		fn := oncego(op)

		// Call the function multiple times
		fn()
		fn()
		fn()

		// Give goroutine time to execute
		ctx, cancel := context.WithTimeout(context.Background(), 100*1000000) // 100ms
		defer cancel()
		<-ctx.Done()

		// Should only have run once
		if count.Load() != 1 {
			t.Errorf("goOnce executed %d times, want 1", count.Load())
		}
	})

	t.Run("RunsInGoroutine", func(t *testing.T) {
		done := make(chan struct{})
		op := func() {
			close(done)
		}

		fn := oncego(op)
		fn()

		// Should complete asynchronously
		select {
		case <-done:
			// Success - goroutine completed
		case <-context.Background().Done():
			t.Error("goOnce did not execute in goroutine")
		}
	})

	t.Run("MultipleCallsOnlyStartOneGoroutine", func(t *testing.T) {
		var count atomic.Int32
		started := make(chan struct{})
		op := func() {
			count.Add(1)
			close(started)
		}

		fn := oncego(op)

		// Call multiple times rapidly
		for i := 0; i < 10; i++ {
			fn()
		}

		<-started

		// Give a bit more time to ensure no other goroutines start
		ctx, cancel := context.WithTimeout(context.Background(), 50*1000000) // 50ms
		defer cancel()
		<-ctx.Done()

		if count.Load() != 1 {
			t.Errorf("goOnce started %d goroutines, want 1", count.Load())
		}
	})
}

func TestFlushTo(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctx := context.Background()
		seq := Slice([]int{1, 2, 3, 4, 5})
		ch := make(chan int, 10)

		result := flushTo(ctx, seq, ch)

		if !result {
			t.Error("flushTo returned false, want true")
		}

		close(ch)
		var collected []int
		for v := range ch {
			collected = append(collected, v)
		}

		if !slices.Equal(collected, []int{1, 2, 3, 4, 5}) {
			t.Errorf("flushTo sent %v, want [1, 2, 3, 4, 5]", collected)
		}
	})

	t.Run("EmptySequence", func(t *testing.T) {
		ctx := context.Background()
		seq := Slice([]int{})
		ch := make(chan int, 1)

		result := flushTo(ctx, seq, ch)

		if !result {
			t.Error("flushTo returned false for empty sequence, want true")
		}

		close(ch)
		var collected []int
		for v := range ch {
			collected = append(collected, v)
		}

		if len(collected) != 0 {
			t.Errorf("flushTo sent %v, want empty slice", collected)
		}
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		seq := Slice([]int{1, 2, 3, 4, 5})
		ch := make(chan int, 10)

		result := flushTo(ctx, seq, ch)

		if result {
			t.Error("flushTo returned true with canceled context, want false")
		}
	})

	t.Run("ContextCanceledMidStream", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Create a sequence that will trigger cancellation after first element
		var yielded atomic.Int32
		seq := func(yield func(int) bool) {
			for i := 1; i <= 5; i++ {
				yielded.Add(1)
				if i == 2 {
					cancel()
				}
				if !yield(i) {
					return
				}
			}
		}

		ch := make(chan int, 10)
		result := flushTo(ctx, seq, ch)

		if result {
			t.Error("flushTo returned true after context cancellation, want false")
		}

		close(ch)
		var collected []int
		for v := range ch {
			collected = append(collected, v)
		}

		// Should have sent at least one value before cancellation
		if len(collected) == 0 {
			t.Error("flushTo sent no values before cancellation")
		}
		if len(collected) >= 5 {
			t.Error("flushTo sent all values despite cancellation")
		}
	})

	t.Run("BufferedChannel", func(t *testing.T) {
		ctx := context.Background()
		seq := Slice([]int{1, 2, 3})
		ch := make(chan int, 5) // Larger buffer

		result := flushTo(ctx, seq, ch)

		if !result {
			t.Error("flushTo returned false, want true")
		}

		if len(ch) != 3 {
			t.Errorf("channel has %d items, want 3", len(ch))
		}
	})

	t.Run("UnbufferedChannel", func(t *testing.T) {
		ctx := context.Background()
		seq := Slice([]int{1, 2, 3})
		ch := make(chan int) // Unbuffered

		done := make(chan bool)
		go func() {
			result := flushTo(ctx, seq, ch)
			done <- result
		}()

		// Receive all values
		var collected []int
		for i := 0; i < 3; i++ {
			v := <-ch
			collected = append(collected, v)
		}

		result := <-done

		if !result {
			t.Error("flushTo returned false, want true")
		}

		if !slices.Equal(collected, []int{1, 2, 3}) {
			t.Errorf("flushTo sent %v, want [1, 2, 3]", collected)
		}
	})
}

func TestMtxHelpers(t *testing.T) {
	t.Run("mtxcall", func(t *testing.T) {
		mu := &sync.Mutex{}
		var executed bool
		var lockHeld bool

		op := func() {
			executed = true
			// Try to lock - should fail if mutex is held
			if mu.TryLock() {
				mu.Unlock()
				lockHeld = false
			} else {
				lockHeld = true
			}
		}

		wrapped := mtxcall(mu, op)
		wrapped()

		if !executed {
			t.Error("mtxcall: operation not executed")
		}

		if !lockHeld {
			t.Error("mtxcall: mutex was not held during operation")
		}

		// Mutex should be unlocked after wrapped() completes
		if !mu.TryLock() {
			t.Error("mtxcall: mutex still locked after wrapped() returned")
		}
		mu.Unlock()
	})

	t.Run("mtxdo", func(t *testing.T) {
		mu := &sync.Mutex{}
		var lockHeld bool

		op := func() int {
			// Try to lock - should fail if mutex is held
			if mu.TryLock() {
				mu.Unlock()
				lockHeld = false
			} else {
				lockHeld = true
			}
			return 42
		}

		wrapped := mtxdo(mu, op)
		result := wrapped()

		if result != 42 {
			t.Errorf("mtxdo: got %d, want 42", result)
		}

		if !lockHeld {
			t.Error("mtxdo: mutex was not held during operation")
		}

		// Mutex should be unlocked after wrapped() completes
		if !mu.TryLock() {
			t.Error("mtxdo: mutex still locked after wrapped() returned")
		}
		mu.Unlock()
	})

	t.Run("mtxdoOK", func(t *testing.T) {
		mu := &sync.Mutex{}
		var lockHeld bool

		op := func() (int, bool) {
			// Try to lock - should fail if mutex is held
			if mu.TryLock() {
				mu.Unlock()
				lockHeld = false
			} else {
				lockHeld = true
			}
			return 42, true
		}

		wrapped := mtxdo2(mu, op)
		value, ok := wrapped()

		if value != 42 {
			t.Errorf("mtxdoOK: got value %d, want 42", value)
		}

		if !ok {
			t.Error("mtxdoOK: got ok=false, want true")
		}

		if !lockHeld {
			t.Error("mtxdoOK: mutex was not held during operation")
		}

		// Mutex should be unlocked after wrapped() completes
		if !mu.TryLock() {
			t.Error("mtxdoOK: mutex still locked after wrapped() returned")
		}
		mu.Unlock()
	})

	t.Run("mtxdoOK2", func(t *testing.T) {
		mu := &sync.Mutex{}
		var lockHeld bool

		op := func() (int, string, bool) {
			// Try to lock - should fail if mutex is held
			if mu.TryLock() {
				mu.Unlock()
				lockHeld = false
			} else {
				lockHeld = true
			}
			return 42, "hello", true
		}

		wrapped := mtxdo3(mu, op)
		v1, v2, ok := wrapped()

		if v1 != 42 {
			t.Errorf("mtxdoOK2: got v1=%d, want 42", v1)
		}

		if v2 != "hello" {
			t.Errorf("mtxdoOK2: got v2=%q, want \"hello\"", v2)
		}

		if !ok {
			t.Error("mtxdoOK2: got ok=false, want true")
		}

		if !lockHeld {
			t.Error("mtxdoOK2: mutex was not held during operation")
		}

		// Mutex should be unlocked after wrapped() completes
		if !mu.TryLock() {
			t.Error("mtxdoOK2: mutex still locked after wrapped() returned")
		}
		mu.Unlock()
	})

	t.Run("mtxcallwith", func(t *testing.T) {
		mu := &sync.Mutex{}
		var executed bool
		var lockHeld bool
		var receivedValue int

		op := func(arg int) {
			executed = true
			receivedValue = arg
			// Try to lock - should fail if mutex is held
			if mu.TryLock() {
				mu.Unlock()
				lockHeld = false
			} else {
				lockHeld = true
			}
		}

		wrapped := mtxcallwith(mu, op)
		wrapped(42)

		if !executed {
			t.Error("mtxcallwith: operation not executed")
		}

		if receivedValue != 42 {
			t.Errorf("mtxcallwith: got arg %d, want 42", receivedValue)
		}

		if !lockHeld {
			t.Error("mtxcallwith: mutex was not held during operation")
		}

		// Mutex should be unlocked after wrapped() completes
		if !mu.TryLock() {
			t.Error("mtxcallwith: mutex still locked after wrapped() returned")
		}
		mu.Unlock()
	})

	t.Run("mtxdowith", func(t *testing.T) {
		mu := &sync.Mutex{}
		var lockHeld bool
		var receivedValue int

		op := func(arg int) int {
			receivedValue = arg
			// Try to lock - should fail if mutex is held
			if mu.TryLock() {
				mu.Unlock()
				lockHeld = false
			} else {
				lockHeld = true
			}
			return arg * 2
		}

		wrapped := mtxdowith(mu, op)
		result := wrapped(21)

		if result != 42 {
			t.Errorf("mtxdowith: got result %d, want 42", result)
		}

		if receivedValue != 21 {
			t.Errorf("mtxdowith: got arg %d, want 21", receivedValue)
		}

		if !lockHeld {
			t.Error("mtxdowith: mutex was not held during operation")
		}

		// Mutex should be unlocked after wrapped() completes
		if !mu.TryLock() {
			t.Error("mtxdowith: mutex still locked after wrapped() returned")
		}
		mu.Unlock()
	})

	t.Run("ConcurrentSafety", func(t *testing.T) {
		mu := &sync.Mutex{}
		var counter atomic.Int32

		op := func() int {
			val := counter.Load()
			counter.Add(1)
			return int(val)
		}

		wrapped := mtxdo(mu, op)

		var wg sync.WaitGroup
		results := make([]int, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				results[idx] = wrapped()
			}(i)
		}

		wg.Wait()

		// All results should be unique (0-99 in some order)
		seen := make(map[int]bool)
		for _, v := range results {
			if seen[v] {
				t.Errorf("ConcurrentSafety: duplicate value %d", v)
			}
			seen[v] = true
		}

		if len(seen) != 100 {
			t.Errorf("ConcurrentSafety: got %d unique values, want 100", len(seen))
		}
	})
}
