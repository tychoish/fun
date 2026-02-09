package irt

import (
	"bytes"
	"encoding/json"
	"iter"
	"strings"
	"testing"
)

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[any]
		expected string
	}{
		{
			name:     "EmptySequence",
			seq:      func(yield func(any) bool) {},
			expected: "[]",
		},
		{
			name: "SingleInt",
			seq: func(yield func(any) bool) {
				yield(42)
			},
			expected: "[42]",
		},
		{
			name: "MultipleInts",
			seq: func(yield func(any) bool) {
				yield(1)
				yield(2)
				yield(3)
			},
			expected: "[1,2,3]",
		},
		{
			name: "MultipleStrings",
			seq: func(yield func(any) bool) {
				yield("hello")
				yield("world")
			},
			expected: `["hello","world"]`,
		},
		{
			name: "MixedTypes",
			seq: func(yield func(any) bool) {
				yield(42)
				yield("hello")
				yield(true)
				yield(nil)
			},
			expected: `[42,"hello",true,null]`,
		},
		{
			name: "NestedStructs",
			seq: func(yield func(any) bool) {
				yield(map[string]any{"name": "Alice", "age": 30})
				yield(map[string]any{"name": "Bob", "age": 25})
			},
			expected: `[{"age":30,"name":"Alice"},{"age":25,"name":"Bob"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalJSON(tt.seq)
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}

			// Compare by unmarshaling both to ensure semantic equality
			var resultData, expectedData any
			if err := json.Unmarshal(result, &resultData); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedData); err != nil {
				t.Fatalf("Failed to unmarshal expected: %v", err)
			}

			resultJSON, _ := json.Marshal(resultData)
			expectedJSON, _ := json.Marshal(expectedData)

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("MarshalJSON() = %s, want %s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestMarshalJSONTyped(t *testing.T) {
	t.Run("IntSlice", func(t *testing.T) {
		seq := func(yield func(int) bool) {
			yield(1)
			yield(2)
			yield(3)
		}

		result, err := MarshalJSON(seq)
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		expected := "[1,2,3]"
		if string(result) != expected {
			t.Errorf("MarshalJSON() = %s, want %s", result, expected)
		}
	})

	t.Run("StringSlice", func(t *testing.T) {
		seq := func(yield func(string) bool) {
			yield("apple")
			yield("banana")
			yield("cherry")
		}

		result, err := MarshalJSON(seq)
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		expected := `["apple","banana","cherry"]`
		if string(result) != expected {
			t.Errorf("MarshalJSON() = %s, want %s", result, expected)
		}
	})
}

func TestMarshalJSON2(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq2[string, any]
		expected string
	}{
		{
			name:     "EmptySequence",
			seq:      func(yield func(string, any) bool) {},
			expected: "{}",
		},
		{
			name: "SinglePair",
			seq: func(yield func(string, any) bool) {
				yield("name", "Alice")
			},
			expected: `{"name":"Alice"}`,
		},
		{
			name: "MultiplePairs",
			seq: func(yield func(string, any) bool) {
				yield("name", "Alice")
				yield("age", 30)
				yield("active", true)
			},
			expected: `{"name":"Alice","age":30,"active":true}`,
		},
		{
			name: "NestedObject",
			seq: func(yield func(string, any) bool) {
				yield("user", map[string]any{"name": "Alice", "age": 30})
				yield("admin", false)
			},
			expected: `{"user":{"age":30,"name":"Alice"},"admin":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalJSON2(tt.seq)
			if err != nil {
				t.Fatalf("MarshalJSONObject() error = %v", err)
			}

			// Compare by unmarshaling both to ensure semantic equality
			var resultData, expectedData any
			if err := json.Unmarshal(result, &resultData); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedData); err != nil {
				t.Fatalf("Failed to unmarshal expected: %v", err)
			}

			resultJSON, _ := json.Marshal(resultData)
			expectedJSON, _ := json.Marshal(expectedData)

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("MarshalJSONObject() = %s, want %s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestMarshalJSON2Typed(t *testing.T) {
	t.Run("StringIntObject", func(t *testing.T) {
		seq := func(yield func(string, int) bool) {
			yield("one", 1)
			yield("two", 2)
			yield("three", 3)
		}

		result, err := MarshalJSON2(seq)
		if err != nil {
			t.Fatalf("MarshalJSONObject() error = %v", err)
		}

		// Unmarshal to compare semantically since map order isn't guaranteed
		var resultData map[string]int
		if err := json.Unmarshal(result, &resultData); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		expected := map[string]int{"one": 1, "two": 2, "three": 3}
		if len(resultData) != len(expected) {
			t.Fatalf("Length mismatch: got %d, want %d", len(resultData), len(expected))
		}

		for k, v := range expected {
			if resultData[k] != v {
				t.Errorf("MarshalJSONObject()[%q] = %d, want %d", k, resultData[k], v)
			}
		}
	})
}

func TestMarshalJSONWithRemoveZeros(t *testing.T) {
	seq := func(yield func(int) bool) {
		yield(1)
		yield(0)
		yield(2)
		yield(0)
		yield(3)
	}

	result, err := MarshalJSON(RemoveZeros(seq))
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	expected := "[1,2,3]"
	if string(result) != expected {
		t.Errorf("MarshalJSON(RemoveZeros()) = %s, want %s", result, expected)
	}
}

func TestMarshalJSONError(t *testing.T) {
	// Create a type that can't be marshaled
	type Unmarshallable struct {
		Ch chan int
	}

	seq := func(yield func(Unmarshallable) bool) {
		yield(Unmarshallable{Ch: make(chan int)})
	}

	_, err := MarshalJSON(seq)
	if err == nil {
		t.Error("MarshalJSON() expected error for unmarshallable type, got nil")
	}
}

func TestMarshalJSONWithHelpers(t *testing.T) {
	t.Run("WithSlice", func(t *testing.T) {
		data := []int{1, 2, 3, 4, 5}
		result, err := MarshalJSON(Slice(data))
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		expected := "[1,2,3,4,5]"
		if string(result) != expected {
			t.Errorf("MarshalJSON(Slice()) = %s, want %s", result, expected)
		}
	})

	t.Run("WithMap", func(t *testing.T) {
		data := map[string]int{"a": 1, "b": 2, "c": 3}
		result, err := MarshalJSON2(Map(data))
		if err != nil {
			t.Fatalf("MarshalJSONObject() error = %v", err)
		}

		// Unmarshal to compare semantically
		var resultData map[string]int
		if err := json.Unmarshal(result, &resultData); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if len(resultData) != len(data) {
			t.Fatalf("Length mismatch: got %d, want %d", len(resultData), len(data))
		}

		for k, v := range data {
			if resultData[k] != v {
				t.Errorf("MarshalJSONObject()[%q] = %d, want %d", k, resultData[k], v)
			}
		}
	})
}

func TestMarshalJSONErrorCases(t *testing.T) {
	t.Run("ChannelType", func(t *testing.T) {
		type WithChannel struct {
			Ch chan int
		}

		seq := func(yield func(WithChannel) bool) {
			yield(WithChannel{Ch: make(chan int)})
		}

		_, err := MarshalJSON(seq)
		if err == nil {
			t.Error("MarshalJSON() expected error for channel type, got nil")
		}
	})

	t.Run("FunctionType", func(t *testing.T) {
		seq := func(yield func(func()) bool) {
			yield(func() {})
		}

		_, err := MarshalJSON(seq)
		if err == nil {
			t.Error("MarshalJSON() expected error for function type, got nil")
		}
	})

	t.Run("ComplexType", func(t *testing.T) {
		seq := func(yield func(complex128) bool) {
			yield(complex(1, 2))
		}

		_, err := MarshalJSON(seq)
		if err == nil {
			t.Error("MarshalJSON() expected error for complex type, got nil")
		}
	})

	t.Run("UnsafePointer", func(t *testing.T) {
		type WithUnsafePointer struct {
			Ch chan int
		}

		seq := func(yield func(*WithUnsafePointer) bool) {
			yield(&WithUnsafePointer{Ch: make(chan int)})
		}

		_, err := MarshalJSON(seq)
		if err == nil {
			t.Error("MarshalJSON() expected error for type with channel field, got nil")
		}
	})
}

func TestMarshalJSON2ErrorCases(t *testing.T) {
	t.Run("UnmarshallableValue", func(t *testing.T) {
		type BadValue struct {
			Ch chan int
		}

		seq := func(yield func(string, BadValue) bool) {
			yield("key", BadValue{Ch: make(chan int)})
		}

		_, err := MarshalJSON2(seq)
		if err == nil {
			t.Error("MarshalJSON2() expected error for unmarshallable value, got nil")
		}
	})

	t.Run("FunctionValue", func(t *testing.T) {
		seq := func(yield func(string, func()) bool) {
			yield("key", func() {})
		}

		_, err := MarshalJSON2(seq)
		if err == nil {
			t.Error("MarshalJSON2() expected error for function value, got nil")
		}
	})

	t.Run("ChannelValue", func(t *testing.T) {
		seq := func(yield func(string, chan int) bool) {
			yield("key", make(chan int))
		}

		_, err := MarshalJSON2(seq)
		if err == nil {
			t.Error("MarshalJSON2() expected error for channel value, got nil")
		}
	})

	t.Run("PartialSequenceError", func(t *testing.T) {
		// Test that error occurs after marshalling some valid items
		type BadValue struct {
			Ch chan int
		}

		seq := func(yield func(string, any) bool) {
			if !yield("valid1", 1) {
				return
			}
			if !yield("valid2", "hello") {
				return
			}
			yield("invalid", BadValue{Ch: make(chan int)}) // This should cause error
		}

		_, err := MarshalJSON2(seq)
		if err == nil {
			t.Error("MarshalJSON2() expected error when encountering unmarshallable value in sequence, got nil")
		}
	})

	t.Run("ComplexValue", func(t *testing.T) {
		seq := func(yield func(string, complex128) bool) {
			yield("key", complex(1, 2))
		}

		_, err := MarshalJSON2(seq)
		if err == nil {
			t.Error("MarshalJSON2() expected error for complex value, got nil")
		}
	})

	t.Run("StructWithChannel", func(t *testing.T) {
		type Container struct {
			Name string
			Ch   chan int
		}

		seq := func(yield func(string, Container) bool) {
			yield("item", Container{Name: "test", Ch: make(chan int)})
		}

		_, err := MarshalJSON2(seq)
		if err == nil {
			t.Error("MarshalJSON2() expected error for struct with channel field, got nil")
		}
	})
}

func TestMarshalJSONPartialSequenceError(t *testing.T) {
	// Test that MarshalJSON errors correctly in the middle of a sequence
	type BadType struct {
		Ch chan int
	}

	seq := func(yield func(any) bool) {
		if !yield(1) {
			return
		}
		if !yield("valid") {
			return
		}
		yield(BadType{Ch: make(chan int)}) // Error here
	}

	_, err := MarshalJSON(seq)
	if err == nil {
		t.Error("MarshalJSON() expected error when encountering unmarshallable value in sequence, got nil")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
		wantErr  bool
	}{
		{
			name:     "EmptyArray",
			input:    "[]",
			expected: []int{},
			wantErr:  false,
		},
		{
			name:     "SingleInt",
			input:    "[42]",
			expected: []int{42},
			wantErr:  false,
		},
		{
			name:     "MultipleInts",
			input:    "[1,2,3,4,5]",
			expected: []int{1, 2, 3, 4, 5},
			wantErr:  false,
		},
		{
			name:     "ArrayWithWhitespace",
			input:    "[ 1 , 2 , 3 ]",
			expected: []int{1, 2, 3},
			wantErr:  false,
		},
		{
			name:     "NotAnArray",
			input:    "{}",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "InvalidJSON",
			input:    "[1,2,",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "TypeMismatch",
			input:    `["string"]`,
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			seq := UnmarshalJSON[int](reader)

			var results []int
			var gotErr bool

			for value, err := range seq {
				if err != nil {
					gotErr = true
					break
				}
				results = append(results, value)
			}

			if gotErr != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(results) != len(tt.expected) {
					t.Errorf("UnmarshalJSON() got %d items, want %d", len(results), len(tt.expected))
					return
				}

				for i, v := range results {
					if v != tt.expected[i] {
						t.Errorf("UnmarshalJSON()[%d] = %v, want %v", i, v, tt.expected[i])
					}
				}
			}
		})
	}
}

func TestUnmarshalJSONStrings(t *testing.T) {
	input := `["apple","banana","cherry"]`
	reader := strings.NewReader(input)
	seq := UnmarshalJSON[string](reader)

	expected := []string{"apple", "banana", "cherry"}
	results := make([]string, 0, 3)

	for value, err := range seq {
		if err != nil {
			t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
		}
		results = append(results, value)
	}

	if len(results) != len(expected) {
		t.Fatalf("UnmarshalJSON() got %d items, want %d", len(results), len(expected))
	}

	for i, v := range results {
		if v != expected[i] {
			t.Errorf("UnmarshalJSON()[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestUnmarshalJSONStructs(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	input := `[{"name":"Alice","age":30},{"name":"Bob","age":25}]`
	reader := strings.NewReader(input)
	seq := UnmarshalJSON[Person](reader)

	expected := []Person{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}
	results := make([]Person, 0, 2)

	for value, err := range seq {
		if err != nil {
			t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
		}
		results = append(results, value)
	}

	if len(results) != len(expected) {
		t.Fatalf("UnmarshalJSON() got %d items, want %d", len(results), len(expected))
	}

	for i, v := range results {
		if v != expected[i] {
			t.Errorf("UnmarshalJSON()[%d] = %+v, want %+v", i, v, expected[i])
		}
	}
}

func TestUnmarshalJSONEarlyTermination(t *testing.T) {
	input := "[1,2,3,4,5]"
	reader := strings.NewReader(input)
	seq := UnmarshalJSON[int](reader)

	count := 0
	for value, err := range seq {
		if err != nil {
			t.Fatalf("UnmarshalJSON() unexpected error: %v", err)
		}
		count++
		if value == 3 {
			break
		}
	}

	if count != 3 {
		t.Errorf("UnmarshalJSON() processed %d items, want 3", count)
	}
}

func TestUnmarshalJSON2(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]int
		wantErr  bool
	}{
		{
			name:     "EmptyObject",
			input:    "{}",
			expected: map[string]int{},
			wantErr:  false,
		},
		{
			name:     "SinglePair",
			input:    `{"one":1}`,
			expected: map[string]int{"one": 1},
			wantErr:  false,
		},
		{
			name:     "MultiplePairs",
			input:    `{"one":1,"two":2,"three":3}`,
			expected: map[string]int{"one": 1, "two": 2, "three": 3},
			wantErr:  false,
		},
		{
			name:     "ObjectWithWhitespace",
			input:    `{ "one" : 1 , "two" : 2 }`,
			expected: map[string]int{"one": 1, "two": 2},
			wantErr:  false,
		},
		{
			name:     "NotAnObject",
			input:    "[]",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "InvalidJSON",
			input:    `{"one":1,`,
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "TypeMismatch",
			input:    `{"one":"string"}`,
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			seq := UnmarshalJSON2[string, int](reader)

			results := make(map[string]int)
			var gotErr bool

			for kv, err := range seq {
				if err != nil {
					gotErr = true
					break
				}
				results[kv.Key] = kv.Value
			}

			if gotErr != tt.wantErr {
				t.Errorf("UnmarshalJSON2() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(results) != len(tt.expected) {
					t.Errorf("UnmarshalJSON2() got %d items, want %d", len(results), len(tt.expected))
					return
				}

				for k, v := range tt.expected {
					if results[k] != v {
						t.Errorf("UnmarshalJSON2()[%q] = %v, want %v", k, results[k], v)
					}
				}
			}
		})
	}
}

func TestUnmarshalJSON2Strings(t *testing.T) {
	input := `{"name":"Alice","city":"NYC","country":"USA"}`
	reader := strings.NewReader(input)
	seq := UnmarshalJSON2[string, string](reader)

	expected := map[string]string{
		"name":    "Alice",
		"city":    "NYC",
		"country": "USA",
	}
	results := make(map[string]string)

	for kv, err := range seq {
		if err != nil {
			t.Fatalf("UnmarshalJSON2() unexpected error: %v", err)
		}
		results[kv.Key] = kv.Value
	}

	if len(results) != len(expected) {
		t.Fatalf("UnmarshalJSON2() got %d items, want %d", len(results), len(expected))
	}

	for k, v := range expected {
		if results[k] != v {
			t.Errorf("UnmarshalJSON2()[%q] = %q, want %q", k, results[k], v)
		}
	}
}

func TestUnmarshalJSON2ComplexValues(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	input := `{"alice":{"name":"Alice","age":30},"bob":{"name":"Bob","age":25}}`
	reader := strings.NewReader(input)
	seq := UnmarshalJSON2[string, Person](reader)

	expected := map[string]Person{
		"alice": {Name: "Alice", Age: 30},
		"bob":   {Name: "Bob", Age: 25},
	}
	results := make(map[string]Person)

	for kv, err := range seq {
		if err != nil {
			t.Fatalf("UnmarshalJSON2() unexpected error: %v", err)
		}
		results[kv.Key] = kv.Value
	}

	if len(results) != len(expected) {
		t.Fatalf("UnmarshalJSON2() got %d items, want %d", len(results), len(expected))
	}

	for k, v := range expected {
		if results[k] != v {
			t.Errorf("UnmarshalJSON2()[%q] = %+v, want %+v", k, results[k], v)
		}
	}
}

func TestUnmarshalJSON2EarlyTermination(t *testing.T) {
	input := `{"one":1,"two":2,"three":3,"four":4,"five":5}`
	reader := strings.NewReader(input)
	seq := UnmarshalJSON2[string, int](reader)

	count := 0
	for _, err := range seq {
		if err != nil {
			t.Fatalf("UnmarshalJSON2() unexpected error: %v", err)
		}
		count++
		if count == 3 {
			break
		}
	}

	if count != 3 {
		t.Errorf("UnmarshalJSON2() processed %d items, want 3", count)
	}
}

func TestUnmarshalJSONRoundTrip(t *testing.T) {
	// Test that marshaling and then unmarshaling produces the same result
	original := []int{1, 2, 3, 4, 5}
	seq := Slice(original)

	// Marshal
	data, err := MarshalJSON(seq)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	// Unmarshal
	reader := bytes.NewReader(data)
	unmarshalSeq := UnmarshalJSON[int](reader)

	results := make([]int, 0, 4)
	for value, err := range unmarshalSeq {
		if err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		results = append(results, value)
	}

	if len(results) != len(original) {
		t.Fatalf("Round trip got %d items, want %d", len(results), len(original))
	}

	for i, v := range results {
		if v != original[i] {
			t.Errorf("Round trip[%d] = %v, want %v", i, v, original[i])
		}
	}
}

func TestUnmarshalJSON2RoundTrip(t *testing.T) {
	// Test that marshaling and then unmarshaling produces the same result
	original := map[string]int{"one": 1, "two": 2, "three": 3}
	seq := Map(original)

	// Marshal
	data, err := MarshalJSON2(seq)
	if err != nil {
		t.Fatalf("MarshalJSON2() error = %v", err)
	}

	// Unmarshal
	reader := bytes.NewReader(data)
	unmarshalSeq := UnmarshalJSON2[string, int](reader)

	results := make(map[string]int)
	for kv, err := range unmarshalSeq {
		if err != nil {
			t.Fatalf("UnmarshalJSON2() error = %v", err)
		}
		results[kv.Key] = kv.Value
	}

	if len(results) != len(original) {
		t.Fatalf("Round trip got %d items, want %d", len(results), len(original))
	}

	for k, v := range original {
		if results[k] != v {
			t.Errorf("Round trip[%q] = %v, want %v", k, results[k], v)
		}
	}
}

func TestUnmarshalErroneousJSON(t *testing.T) {
	for value := range Args(
		"42",
		"[}",
		"{1,2,3}",
		`{1:"hi",2:"what"}`,
		`{"hi":what}`,
		"{hi:what}",
		"null",
		"false",
		`"foo"`,
		`{"hi":`,
		`{"hi":1`,
		`{"hi":"2erer`,
		`"hello":"world"}`,
		`1,2]`,
		`{null:true}`,
		`{":true}`,
		`{,,:true}`,
		`{[]:true}`,
		`{{}:true}`,
		``,
		`\\x00`,
	) {
		t.Run(value, func(t *testing.T) {
			var buf bytes.Buffer
			must2(buf.WriteString(value))

			t.Run("Array", func(t *testing.T) {
				count := 0
				for elem, err := range UnmarshalJSON[any](&buf) {
					count++
					if err == nil {
						t.Error(elem)
					}
				}
				if count != 1 {
					t.Error(count)
				}
			})

			t.Run("Object", func(t *testing.T) {
				count := 0
				for kv, err := range UnmarshalJSON2[string, any](&buf) {
					count++
					if err == nil {
						t.Error(kv)
					}
				}
				if count != 1 {
					t.Error(count)
				}
			})
		})
	}
	t.Run("Edges", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("{")
		buf.WriteByte('\x00')
		count := 0
		for kv, err := range UnmarshalJSON2[string, any](&buf) {
			count++
			if err == nil {
				t.Error(kv)
			}
		}
		if count != 1 {
			t.Error(count)
		}
	})
}
