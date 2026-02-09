package irt

import "testing"

func TestElemHelper(t *testing.T) {
	// Test NewElem
	elem := MakeKV(42, "hello")
	if elem.Key != 42 || elem.Value != "hello" {
		t.Errorf("NewElem(42, 'hello') = %v, want {42, 'hello'}", elem)
	}

	// Test WithElem
	elem2 := WithKV(5, func(i int) string { return string(rune('a' + i)) })
	if elem2.Key != 5 || elem2.Value != "f" {
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
		_ = yield(1, "a") && yield(2, "b")
	}

	elemSeq := KVjoin(seq)
	result := Collect(elemSeq)

	expected := Collect(KVjoin(KVargs(MakeKV(1, "a"), MakeKV(2, "b"))))

	if len(result) != len(expected) {
		t.Errorf("Elems() length = %v, want %v", len(result), len(expected))
	}
	for i, elem := range result {
		if i < len(expected) && (elem.Key != expected[i].Key || elem.Value != expected[i].Value) {
			t.Errorf("Elems()[%d] = %v, want %v", i, elem, expected[i])
		}
	}
}

func TestSplitsHelper(t *testing.T) {
	elems := []KV[int, string]{
		{Key: 1, Value: "a"},
		{Key: 2, Value: "b"},
	}

	seq := KVsplit(Slice(elems))

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

func TestElemOrderingSemantics(t *testing.T) {
	t.Run("ElemCmp", func(t *testing.T) {
		elem1 := KV[int, string]{Key: 1, Value: "a"}
		elem2 := KV[int, string]{Key: 2, Value: "b"}
		elem3 := KV[int, string]{Key: 1, Value: "c"}

		t.Run("Compare by First", func(t *testing.T) {
			if KVcmpFirst(elem1, elem2) >= 0 {
				t.Errorf("Expected elem1 < elem2, got %v", KVcmpFirst(elem1, elem2))
			}
			if KVcmpFirst(elem2, elem1) <= 0 {
				t.Errorf("Expected elem2 > elem1, got %v", KVcmpFirst(elem2, elem1))
			}
			if KVcmpFirst(elem1, elem3) != 0 {
				t.Errorf("Expected elem1 == elem3, got %v", KVcmpFirst(elem1, elem3))
			}
		})

		t.Run("Compare by Second", func(t *testing.T) {
			if KVcmpSecond(elem1, elem3) >= 0 {
				t.Errorf("Expected elem1 < elem3, got %v", KVcmpSecond(elem1, elem3))
			}
			if KVcmpSecond(elem3, elem1) <= 0 {
				t.Errorf("Expected elem3 > elem1, got %v", KVcmpSecond(elem3, elem1))
			}
			if KVcmpSecond(elem1, elem1) != 0 {
				t.Errorf("Expected elem1 == elem1, got %v", KVcmpSecond(elem1, elem1))
			}
		})
	})

	t.Run("Compare", func(t *testing.T) {
		elem1 := KV[int, string]{Key: 1, Value: "a"}
		elem2 := KV[int, string]{Key: 2, Value: "b"}
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
		elem1 := KV[int, string]{Key: 1, Value: "a"}
		elem2 := KV[int, string]{Key: 2, Value: "b"}
		if KVcmp(elem1, elem2) >= 0 {
			t.Errorf("Expected elem1 < elem2, got %v", KVcmp(elem1, elem2))
		}
		if KVcmp(elem2, elem1) <= 0 {
			t.Errorf("Expected elem2 > elem1, got %v", KVcmp(elem2, elem1))
		}
		if KVcmp(elem1, elem1) != 0 {
			t.Errorf("Expected elem1 == elem1, got %v", KVcmp(elem1, elem1))
		}
	})

	t.Run("WithPanicsWithoutComparators", func(t *testing.T) {
		elem := kvcmp[int, int]{lh: MakeKV(100, 200)}
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		if c := elem.With(MakeKV(1, 2)); c != -1 {
			t.Error("should never run, but this is even weirder if it does", c)
		}
	})

	t.Run("CompareFirst", func(t *testing.T) {
		elem1 := KV[int, string]{Key: 1, Value: "a"}
		elem2 := KV[int, string]{Key: 2, Value: "b"}
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
		elem1 := KV[int, string]{Key: 1, Value: "a"}
		elem2 := KV[int, string]{Key: 1, Value: "b"}
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
