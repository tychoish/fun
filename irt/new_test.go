package irt

import (
	"errors"
	"slices"
	"testing"
)

func TestAny(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int) bool) {}
		result := Any(seq)

		count := 0
		for range result {
			count++
		}
		if count != 0 {
			t.Errorf("expected 0 elements, got %d", count)
		}
	})

	t.Run("IntSequence", func(t *testing.T) {
		seq := func(yield func(int) bool) {
			if !yield(1) {
				return
			}
			if !yield(2) {
				return
			}
			yield(3)
		}
		result := Any(seq)

		values := Collect(result)
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}

		// Verify values can be type-asserted back to int
		intVal, ok := values[0].(int)
		if !ok {
			t.Error("expected first value to be int")
		}
		if intVal != 1 {
			t.Errorf("expected first value to be 1, got %d", intVal)
		}

		intVal, ok = values[1].(int)
		if !ok {
			t.Error("expected second value to be int")
		}
		if intVal != 2 {
			t.Errorf("expected second value to be 2, got %d", intVal)
		}

		intVal, ok = values[2].(int)
		if !ok {
			t.Error("expected third value to be int")
		}
		if intVal != 3 {
			t.Errorf("expected third value to be 3, got %d", intVal)
		}
	})

	t.Run("StringSequence", func(t *testing.T) {
		seq := func(yield func(string) bool) {
			if !yield("hello") {
				return
			}
			if !yield("world") {
				return
			}
			yield("test")
		}
		result := Any(seq)

		values := Collect(result)
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}

		str, ok := values[0].(string)
		if !ok {
			t.Error("expected first value to be string")
		}
		if str != "hello" {
			t.Errorf("expected first value to be 'hello', got %q", str)
		}

		str, ok = values[1].(string)
		if !ok {
			t.Error("expected second value to be string")
		}
		if str != "world" {
			t.Errorf("expected second value to be 'world', got %q", str)
		}

		str, ok = values[2].(string)
		if !ok {
			t.Error("expected third value to be string")
		}
		if str != "test" {
			t.Errorf("expected third value to be 'test', got %q", str)
		}
	})

	t.Run("StructSequence", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		seq := func(yield func(Person) bool) {
			if !yield(Person{Name: "Alice", Age: 30}) {
				return
			}
			yield(Person{Name: "Bob", Age: 25})
		}
		result := Any(seq)

		values := Collect(result)
		if len(values) != 2 {
			t.Fatalf("expected 2 values, got %d", len(values))
		}

		person, ok := values[0].(Person)
		if !ok {
			t.Error("expected first value to be Person")
		}
		if person.Name != "Alice" {
			t.Errorf("expected first person name to be 'Alice', got %q", person.Name)
		}
		if person.Age != 30 {
			t.Errorf("expected first person age to be 30, got %d", person.Age)
		}

		person, ok = values[1].(Person)
		if !ok {
			t.Error("expected second value to be Person")
		}
		if person.Name != "Bob" {
			t.Errorf("expected second person name to be 'Bob', got %q", person.Name)
		}
		if person.Age != 25 {
			t.Errorf("expected second person age to be 25, got %d", person.Age)
		}
	})

	t.Run("PointerSequence", func(t *testing.T) {
		val1 := 42
		val2 := 100

		seq := func(yield func(*int) bool) {
			if !yield(&val1) {
				return
			}
			yield(&val2)
		}
		result := Any(seq)

		values := Collect(result)
		if len(values) != 2 {
			t.Fatalf("expected 2 values, got %d", len(values))
		}

		ptr, ok := values[0].(*int)
		if !ok {
			t.Error("expected first value to be *int")
		}
		if *ptr != 42 {
			t.Errorf("expected first pointer value to be 42, got %d", *ptr)
		}

		ptr, ok = values[1].(*int)
		if !ok {
			t.Error("expected second value to be *int")
		}
		if *ptr != 100 {
			t.Errorf("expected second pointer value to be 100, got %d", *ptr)
		}
	})

	t.Run("MixedTypeConversion", func(t *testing.T) {
		// Demonstrate that different typed sequences can all convert to any
		intSeq := func(yield func(int) bool) {
			yield(1)
		}
		stringSeq := func(yield func(string) bool) {
			yield("test")
		}

		intAsAny := Any(intSeq)
		stringAsAny := Any(stringSeq)

		intValues := Collect(intAsAny)
		stringValues := Collect(stringAsAny)

		if len(intValues) != 1 {
			t.Errorf("expected 1 int value, got %d", len(intValues))
		}
		if len(stringValues) != 1 {
			t.Errorf("expected 1 string value, got %d", len(stringValues))
		}

		_, intOk := intValues[0].(int)
		_, stringOk := stringValues[0].(string)

		if !intOk {
			t.Error("expected int value to be int type")
		}
		if !stringOk {
			t.Error("expected string value to be string type")
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		seq := func(yield func(int) bool) {
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
		}
		result := Any(seq)

		count := 0
		for v := range result {
			count++
			intVal, ok := v.(int)
			if !ok {
				t.Error("expected int value")
			}
			if intVal == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("expected to iterate 2 times, got %d", count)
		}
	})
}

func TestAny2(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(string, int) bool) {}
		result := Any2(seq)

		count := 0
		for range result {
			count++
		}
		if count != 0 {
			t.Errorf("expected 0 elements, got %d", count)
		}
	})

	t.Run("StringIntPairs", func(t *testing.T) {
		seq := func(yield func(string, int) bool) {
			if !yield("one", 1) {
				return
			}
			if !yield("two", 2) {
				return
			}
			yield("three", 3)
		}
		result := Any2(seq)

		keys := make([]string, 0)
		values := make([]any, 0)

		for k, v := range result {
			keys = append(keys, k)
			values = append(values, v)
		}

		if len(keys) != 3 {
			t.Errorf("expected 3 keys, got %d", len(keys))
		}
		if len(values) != 3 {
			t.Errorf("expected 3 values, got %d", len(values))
		}

		expectedKeys := []string{"one", "two", "three"}
		if !slices.Equal(keys, expectedKeys) {
			t.Errorf("expected keys %v, got %v", expectedKeys, keys)
		}

		// Verify values are converted to any but can be asserted back
		intVal, ok := values[0].(int)
		if !ok {
			t.Error("expected first value to be int")
		}
		if intVal != 1 {
			t.Errorf("expected first value to be 1, got %d", intVal)
		}

		intVal, ok = values[1].(int)
		if !ok {
			t.Error("expected second value to be int")
		}
		if intVal != 2 {
			t.Errorf("expected second value to be 2, got %d", intVal)
		}

		intVal, ok = values[2].(int)
		if !ok {
			t.Error("expected third value to be int")
		}
		if intVal != 3 {
			t.Errorf("expected third value to be 3, got %d", intVal)
		}
	})

	t.Run("IntStringPairs", func(t *testing.T) {
		seq := func(yield func(int, string) bool) {
			if !yield(1, "first") {
				return
			}
			if !yield(2, "second") {
				return
			}
			yield(3, "third")
		}
		result := Any2(seq)

		collected := Collect2(result)
		if len(collected) != 3 {
			t.Errorf("expected 3 pairs, got %d", len(collected))
		}

		// First key type is preserved
		str, ok := collected[1].(string)
		if !ok {
			t.Error("expected value at key 1 to be string")
		}
		if str != "first" {
			t.Errorf("expected 'first', got %q", str)
		}

		str, ok = collected[2].(string)
		if !ok {
			t.Error("expected value at key 2 to be string")
		}
		if str != "second" {
			t.Errorf("expected 'second', got %q", str)
		}

		str, ok = collected[3].(string)
		if !ok {
			t.Error("expected value at key 3 to be string")
		}
		if str != "third" {
			t.Errorf("expected 'third', got %q", str)
		}
	})

	t.Run("StructPairs", func(t *testing.T) {
		type Key struct {
			ID int
		}
		type Value struct {
			Name string
		}

		seq := func(yield func(Key, Value) bool) {
			if !yield(Key{ID: 1}, Value{Name: "Alice"}) {
				return
			}
			yield(Key{ID: 2}, Value{Name: "Bob"})
		}
		result := Any2(seq)

		keys := make([]Key, 0)
		values := make([]any, 0)

		for k, v := range result {
			keys = append(keys, k)
			values = append(values, v)
		}

		if len(keys) != 2 {
			t.Errorf("expected 2 keys, got %d", len(keys))
		}
		if len(values) != 2 {
			t.Errorf("expected 2 values, got %d", len(values))
		}

		// Keys are preserved
		if keys[0].ID != 1 {
			t.Errorf("expected first key ID to be 1, got %d", keys[0].ID)
		}
		if keys[1].ID != 2 {
			t.Errorf("expected second key ID to be 2, got %d", keys[1].ID)
		}

		// Values can be asserted back
		val, ok := values[0].(Value)
		if !ok {
			t.Error("expected first value to be Value type")
		}
		if val.Name != "Alice" {
			t.Errorf("expected first value name to be 'Alice', got %q", val.Name)
		}

		val, ok = values[1].(Value)
		if !ok {
			t.Error("expected second value to be Value type")
		}
		if val.Name != "Bob" {
			t.Errorf("expected second value name to be 'Bob', got %q", val.Name)
		}
	})

	t.Run("MapConversion", func(t *testing.T) {
		mp := map[string]int{
			"a": 1,
			"b": 2,
			"c": 3,
		}

		result := Any2(Map(mp))
		collected := Collect2(result)

		if len(collected) != 3 {
			t.Errorf("expected 3 pairs, got %d", len(collected))
		}

		// Verify all values are present and can be asserted
		for k, v := range collected {
			intVal, ok := v.(int)
			if !ok {
				t.Errorf("expected value for key %q to be int", k)
				continue
			}
			if mp[k] != intVal {
				t.Errorf("expected value for key %q to be %d, got %d", k, mp[k], intVal)
			}
		}
	})

	t.Run("EarlyTermination", func(t *testing.T) {
		seq := func(yield func(int, string) bool) {
			if !yield(1, "one") {
				return
			}
			if !yield(2, "two") {
				return
			}
			if !yield(3, "three") {
				return
			}
			yield(4, "four")
		}
		result := Any2(seq)

		count := 0
		for k := range result {
			count++
			if k == 2 {
				break
			}
		}
		if count != 2 {
			t.Errorf("expected to iterate 2 times, got %d", count)
		}
	})

	t.Run("ErrorPairs", func(t *testing.T) {
		testErr := errors.New("test error")
		seq := func(yield func(string, error) bool) {
			if !yield("success", nil) {
				return
			}
			yield("failure", testErr)
		}
		result := Any2(seq)

		keys := make([]string, 0)
		values := make([]any, 0)

		for k, v := range result {
			keys = append(keys, k)
			values = append(values, v)
		}

		if len(keys) != 2 {
			t.Errorf("expected 2 keys, got %d", len(keys))
		}
		if len(values) != 2 {
			t.Errorf("expected 2 values, got %d", len(values))
		}

		// First value is nil error
		if values[0] != nil {
			t.Errorf("expected first value to be nil, got %v", values[0])
		}

		// Second value is an error
		err, ok := values[1].(error)
		if !ok {
			t.Error("expected second value to be error")
		}
		if err == nil {
			t.Error("expected non-nil error")
		}
	})
}

func TestToAny(t *testing.T) {
	t.Run("IntToAny", func(t *testing.T) {
		result := toany(42)
		if result == nil {
			t.Fatal("expected non-nil result")
		}

		intVal, ok := result.(int)
		if !ok {
			t.Error("expected int type")
		}
		if intVal != 42 {
			t.Errorf("expected 42, got %d", intVal)
		}
	})

	t.Run("StringToAny", func(t *testing.T) {
		result := toany("hello")

		strVal, ok := result.(string)
		if !ok {
			t.Error("expected string type")
		}
		if strVal != "hello" {
			t.Errorf("expected 'hello', got %q", strVal)
		}
	})

	t.Run("StructToAny", func(t *testing.T) {
		type TestStruct struct {
			Field1 string
			Field2 int
		}

		input := TestStruct{Field1: "test", Field2: 100}
		result := toany(input)

		structVal, ok := result.(TestStruct)
		if !ok {
			t.Error("expected TestStruct type")
		}
		if structVal.Field1 != "test" {
			t.Errorf("expected Field1 to be 'test', got %q", structVal.Field1)
		}
		if structVal.Field2 != 100 {
			t.Errorf("expected Field2 to be 100, got %d", structVal.Field2)
		}
	})

	t.Run("PointerToAny", func(t *testing.T) {
		val := 42
		result := toany(&val)

		ptrVal, ok := result.(*int)
		if !ok {
			t.Error("expected *int type")
		}
		if *ptrVal != 42 {
			t.Errorf("expected pointer to 42, got %d", *ptrVal)
		}
	})

	t.Run("NilPointerToAny", func(t *testing.T) {
		var ptr *int
		result := toany(ptr)

		ptrVal, ok := result.(*int)
		if !ok {
			t.Error("expected *int type")
		}
		if ptrVal != nil {
			t.Errorf("expected nil pointer, got %v", ptrVal)
		}
	})

	t.Run("SliceToAny", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := toany(input)

		sliceVal, ok := result.([]int)
		if !ok {
			t.Error("expected []int type")
		}
		expected := []int{1, 2, 3}
		if !slices.Equal(sliceVal, expected) {
			t.Errorf("expected %v, got %v", expected, sliceVal)
		}
	})

	t.Run("MapToAny", func(t *testing.T) {
		input := map[string]int{"a": 1, "b": 2}
		result := toany(input)

		mapVal, ok := result.(map[string]int)
		if !ok {
			t.Error("expected map[string]int type")
		}
		if len(mapVal) != 2 {
			t.Errorf("expected map length 2, got %d", len(mapVal))
		}
		if mapVal["a"] != 1 {
			t.Errorf("expected mapVal['a'] to be 1, got %d", mapVal["a"])
		}
		if mapVal["b"] != 2 {
			t.Errorf("expected mapVal['b'] to be 2, got %d", mapVal["b"])
		}
	})

	t.Run("ErrorToAny", func(t *testing.T) {
		testErr := errors.New("test error")
		result := toany(testErr)

		errVal, ok := result.(error)
		if !ok {
			t.Error("expected error type")
		}
		if errVal == nil {
			t.Error("expected non-nil error")
		}
	})

	t.Run("BoolToAny", func(t *testing.T) {
		result := toany(true)

		boolVal, ok := result.(bool)
		if !ok {
			t.Error("expected bool type")
		}
		if !boolVal {
			t.Error("expected true, got false")
		}
	})

	t.Run("InterfaceToAny", func(t *testing.T) {
		var input any = "test string"
		result := toany(input)

		strVal, ok := result.(string)
		if !ok {
			t.Error("expected string type")
		}
		if strVal != "test string" {
			t.Errorf("expected 'test string', got %q", strVal)
		}
	})
}

func TestToAny2(t *testing.T) {
	t.Run("IntStringPair", func(t *testing.T) {
		first, second := toany2(42, "hello")

		if first != 42 {
			t.Errorf("expected first to be 42, got %d", first)
		}

		strVal, ok := second.(string)
		if !ok {
			t.Error("expected second to be string type")
		}
		if strVal != "hello" {
			t.Errorf("expected 'hello', got %q", strVal)
		}
	})

	t.Run("StringIntPair", func(t *testing.T) {
		first, second := toany2("key", 100)

		if first != "key" {
			t.Errorf("expected first to be 'key', got %q", first)
		}

		intVal, ok := second.(int)
		if !ok {
			t.Error("expected second to be int type")
		}
		if intVal != 100 {
			t.Errorf("expected 100, got %d", intVal)
		}
	})

	t.Run("StructPairs", func(t *testing.T) {
		type Key struct {
			ID int
		}
		type Value struct {
			Name string
		}

		k := Key{ID: 1}
		v := Value{Name: "Alice"}

		first, second := toany2(k, v)

		if first.ID != 1 {
			t.Errorf("expected first.ID to be 1, got %d", first.ID)
		}

		valStruct, ok := second.(Value)
		if !ok {
			t.Error("expected second to be Value type")
		}
		if valStruct.Name != "Alice" {
			t.Errorf("expected 'Alice', got %q", valStruct.Name)
		}
	})

	t.Run("ErrorPair", func(t *testing.T) {
		testErr := errors.New("test error")
		first, second := toany2("operation", testErr)

		if first != "operation" {
			t.Errorf("expected first to be 'operation', got %q", first)
		}

		err, ok := second.(error)
		if !ok {
			t.Error("expected second to be error type")
		}
		if err == nil {
			t.Error("expected non-nil error")
		}
	})

	t.Run("NilSecondValue", func(t *testing.T) {
		var nilPtr *string
		first, second := toany2("key", nilPtr)

		if first != "key" {
			t.Errorf("expected first to be 'key', got %q", first)
		}

		// Second should be nil pointer wrapped as any
		ptr, ok := second.(*string)
		if !ok {
			t.Error("expected second to be *string type")
		}
		if ptr != nil {
			t.Errorf("expected nil pointer, got %v", ptr)
		}
	})

	t.Run("BothStructs", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		p1 := Person{Name: "Alice", Age: 30}
		p2 := Person{Name: "Bob", Age: 25}

		first, second := toany2(p1, p2)

		if first.Name != "Alice" {
			t.Errorf("expected first.Name to be 'Alice', got %q", first.Name)
		}
		if first.Age != 30 {
			t.Errorf("expected first.Age to be 30, got %d", first.Age)
		}

		person2, ok := second.(Person)
		if !ok {
			t.Error("expected second to be Person type")
		}
		if person2.Name != "Bob" {
			t.Errorf("expected second.Name to be 'Bob', got %q", person2.Name)
		}
		if person2.Age != 25 {
			t.Errorf("expected second.Age to be 25, got %d", person2.Age)
		}
	})

	t.Run("IntBoolPair", func(t *testing.T) {
		first, second := toany2(42, true)

		if first != 42 {
			t.Errorf("expected first to be 42, got %d", first)
		}

		boolVal, ok := second.(bool)
		if !ok {
			t.Error("expected second to be bool type")
		}
		if !boolVal {
			t.Error("expected true, got false")
		}
	})

	t.Run("PointerPairs", func(t *testing.T) {
		val1 := 100
		val2 := "test"

		first, second := toany2(&val1, &val2)

		if *first != 100 {
			t.Errorf("expected *first to be 100, got %d", *first)
		}

		strPtr, ok := second.(*string)
		if !ok {
			t.Error("expected second to be *string type")
		}
		if *strPtr != "test" {
			t.Errorf("expected 'test', got %q", *strPtr)
		}
	})

	t.Run("SliceMapPair", func(t *testing.T) {
		slice := []int{1, 2, 3}
		mp := map[string]int{"a": 1}

		first, second := toany2(slice, mp)

		if len(first) != 3 {
			t.Errorf("expected first length to be 3, got %d", len(first))
		}

		mapVal, ok := second.(map[string]int)
		if !ok {
			t.Error("expected second to be map[string]int type")
		}
		if mapVal["a"] != 1 {
			t.Errorf("expected mapVal['a'] to be 1, got %d", mapVal["a"])
		}
	})

	t.Run("PreservesFirstType", func(t *testing.T) {
		// Verify that the first value type is completely preserved
		type CustomKey struct {
			Value string
		}

		key := CustomKey{Value: "test"}
		first, _ := toany2(key, "anything")

		// Should be able to access fields directly without type assertion
		if first.Value != "test" {
			t.Errorf("expected first.Value to be 'test', got %q", first.Value)
		}
	})
}
