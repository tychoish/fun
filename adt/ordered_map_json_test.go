package adt

import (
	"testing"

	"github.com/tychoish/fun/irt"
)

func TestOrderedMapMarshalUnmarshal(t *testing.T) {
	t.Run("EmptyMap", func(t *testing.T) {
		m := &OrderedMap[string, int]{}

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, int]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Len() != 0 {
			t.Errorf("UnmarshalJSON() length = %d, want 0", m2.Len())
		}
	})

	t.Run("SimpleMap", func(t *testing.T) {
		m := &OrderedMap[string, int]{}
		m.Set("alpha", 1)
		m.Set("beta", 2)
		m.Set("gamma", 3)

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, int]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Len() != 3 {
			t.Errorf("UnmarshalJSON() length = %d, want 3", m2.Len())
		}

		if m2.Get("alpha") != 1 {
			t.Errorf("UnmarshalJSON() Get(\"alpha\") = %d, want 1", m2.Get("alpha"))
		}
		if m2.Get("beta") != 2 {
			t.Errorf("UnmarshalJSON() Get(\"beta\") = %d, want 2", m2.Get("beta"))
		}
		if m2.Get("gamma") != 3 {
			t.Errorf("UnmarshalJSON() Get(\"gamma\") = %d, want 3", m2.Get("gamma"))
		}
	})

	t.Run("OrderPreserved", func(t *testing.T) {
		m := &OrderedMap[string, string]{}
		m.Set("first", "1")
		m.Set("second", "2")
		m.Set("third", "3")

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, string]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		// Check order is preserved
		keys := irt.Collect(irt.First(m2.Iterator()))
		expected := []string{"first", "second", "third"}

		if len(keys) != len(expected) {
			t.Fatalf("Iterator() length = %d, want %d", len(keys), len(expected))
		}

		for i, key := range keys {
			if key != expected[i] {
				t.Errorf("Iterator()[%d] = %q, want %q", i, key, expected[i])
			}
		}
	})

	t.Run("ComplexValues", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		m := &OrderedMap[string, Person]{}
		m.Set("alice", Person{Name: "Alice", Age: 30})
		m.Set("bob", Person{Name: "Bob", Age: 25})

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, Person]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Len() != 2 {
			t.Errorf("UnmarshalJSON() length = %d, want 2", m2.Len())
		}

		alice := m2.Get("alice")
		if alice.Name != "Alice" || alice.Age != 30 {
			t.Errorf("UnmarshalJSON() Get(\"alice\") = %+v, want {Name:Alice Age:30}", alice)
		}

		bob := m2.Get("bob")
		if bob.Name != "Bob" || bob.Age != 25 {
			t.Errorf("UnmarshalJSON() Get(\"bob\") = %+v, want {Name:Bob Age:25}", bob)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		var m OrderedMap[string, int]
		err := m.UnmarshalJSON([]byte(`[1,2,3]`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for array instead of object, got nil")
		}
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		var m OrderedMap[string, int]
		err := m.UnmarshalJSON([]byte(`{"key":}`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for malformed JSON, got nil")
		}
	})

	t.Run("TypeMismatch", func(t *testing.T) {
		var m OrderedMap[string, int]
		err := m.UnmarshalJSON([]byte(`{"key":"not an int"}`))
		if err == nil {
			t.Error("UnmarshalJSON() expected error for type mismatch, got nil")
		}
	})

	t.Run("RoundTrip", func(t *testing.T) {
		m := &OrderedMap[string, int]{}
		m.Set("x", 10)
		m.Set("y", 20)
		m.Set("z", 30)

		// Marshal
		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		// Unmarshal
		var m2 OrderedMap[string, int]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		// Verify same content and order
		if m.Len() != m2.Len() {
			t.Errorf("Round trip length mismatch: %d vs %d", m.Len(), m2.Len())
		}

		originalKeys := irt.Collect(irt.First(m.Iterator()))
		restoredKeys := irt.Collect(irt.First(m2.Iterator()))

		if len(originalKeys) != len(restoredKeys) {
			t.Fatalf("Round trip key count: %d vs %d", len(originalKeys), len(restoredKeys))
		}

		for i := range originalKeys {
			if originalKeys[i] != restoredKeys[i] {
				t.Errorf("Round trip key[%d]: %q vs %q", i, originalKeys[i], restoredKeys[i])
			}
			if m.Get(originalKeys[i]) != m2.Get(restoredKeys[i]) {
				t.Errorf("Round trip value[%q]: %d vs %d",
					originalKeys[i], m.Get(originalKeys[i]), m2.Get(restoredKeys[i]))
			}
		}
	})

	t.Run("EmptyStringKey", func(t *testing.T) {
		m := &OrderedMap[string, int]{}
		m.Set("", 0)
		m.Set("a", 1)

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, int]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Get("") != 0 {
			t.Errorf("UnmarshalJSON() Get(\"\") = %d, want 0", m2.Get(""))
		}
		if m2.Get("a") != 1 {
			t.Errorf("UnmarshalJSON() Get(\"a\") = %d, want 1", m2.Get("a"))
		}
	})

	t.Run("UnicodeKeys", func(t *testing.T) {
		m := &OrderedMap[string, string]{}
		m.Set("hello", "world")
		m.Set("世界", "earth")
		m.Set("🌍", "globe")

		data, err := m.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v", err)
		}

		var m2 OrderedMap[string, string]
		if err := m2.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}

		if m2.Get("世界") != "earth" {
			t.Errorf("UnmarshalJSON() Get(\"世界\") = %q, want \"earth\"", m2.Get("世界"))
		}
		if m2.Get("🌍") != "globe" {
			t.Errorf("UnmarshalJSON() Get(\"🌍\") = %q, want \"globe\"", m2.Get("🌍"))
		}
	})
}
