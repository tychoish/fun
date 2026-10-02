package irt

import (
	"bytes"
	"encoding/json"
	"errors"
	"iter"
	"strings"
	"testing"
)

type jsonLevel int

func (jsonLevel) MarshalJSON() ([]byte, error) { return []byte(`"lvl"`), nil }

type textName string

func (n textName) MarshalText() ([]byte, error) { return []byte(strings.ToLower(string(n))), nil }
func (n *textName) UnmarshalText(b []byte) error {
	*n = textName(strings.ToUpper(string(b)))
	return nil
}

type badText struct{}

func (badText) MarshalText() ([]byte, error) { return nil, errors.New("bad") }

func keyed[K comparable](keys ...K) iter.Seq2[K, int] {
	return func(yield func(K, int) bool) {
		for i, k := range keys {
			if !yield(k, i) {
				return
			}
		}
	}
}

// roundTripKeys pins the encoding and checks the keys survive a round trip.
func roundTripKeys[K comparable](t *testing.T, want string, keys ...K) {
	t.Helper()
	data, err := MarshalJSON2(keyed(keys...))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("got %s, want %s", data, want)
	}
	i := 0
	for kv, err := range UnmarshalJSON2[K, int](bytes.NewReader(data)) {
		if err != nil || kv.Key != keys[i] {
			t.Fatalf("key %d: got %v (%v), want %v", i, kv.Key, err, keys[i])
		}
		i++
	}
	if i != len(keys) {
		t.Fatalf("decoded %d keys, want %d", i, len(keys))
	}
}

func TestJSONKeys(t *testing.T) {
	t.Run("IntegerKindIgnoresMarshalJSON", func(t *testing.T) {
		roundTripKeys(t, `{"1":0,"2":1}`, jsonLevel(1), jsonLevel(2))
	})
	t.Run("StringKindWithTextMarshaler", func(t *testing.T) {
		// encoded raw, decoded through UnmarshalText.
		data, err := MarshalJSON2(keyed(textName("Ab")))
		if err != nil || string(data) != `{"Ab":0}` {
			t.Fatal(string(data), err)
		}
		for kv, err := range UnmarshalJSON2[textName, int](strings.NewReader(`{"abc":1}`)) {
			if err != nil || kv.Key != "ABC" {
				t.Fatal(kv, err)
			}
		}
	})
	t.Run("PlainString", func(t *testing.T) { roundTripKeys(t, `{"a\"b":0,"c":1}`, "a\"b", "c") })
	t.Run("Ints", func(t *testing.T) { roundTripKeys(t, `{"-1":0,"7":1}`, -1, 7) })
	t.Run("Uints", func(t *testing.T) { roundTripKeys(t, `{"7":0}`, uint8(7)) })
	t.Run("MarshalTextError", func(t *testing.T) {
		if _, err := MarshalJSON2(keyed(badText{})); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("Unsupported", func(t *testing.T) {
		if _, err := MarshalJSON2(keyed(1.5)); err == nil {
			t.Fatal("expected error")
		}
	})
}

type textInt int

func (n textInt) MarshalText() ([]byte, error) { return []byte("t" + strings.Repeat("x", int(n))), nil }

// TestJSONKeysGo125 pins Go 1.25's encoding/json key rules with
// hard-coded results, since Go 1.27 changed std's encoding of
// TextMarshaler string-kind keys. Cases where every version agrees are
// also compared with the live std library.
func TestJSONKeysGo125(t *testing.T) {
	t.Run("Pinned", func(t *testing.T) {
		roundTripKeys(t, `{"a":0}`, "a")
		roundTripKeys(t, `{"-3":0}`, -3)
		roundTripKeys(t, `{"7":0}`, uint8(7))
		// string kind wins over MarshalText: raw string, not lowercased
		data, err := MarshalJSON2(keyed(textName("ABC")))
		if err != nil || string(data) != `{"ABC":0}` {
			t.Fatalf("got %s, %v", data, err)
		}
		// MarshalText on a non-string kind wins over integer encoding
		data, err = MarshalJSON2(keyed(textInt(2)))
		if err != nil || string(data) != `{"txx":0}` {
			t.Fatalf("got %s, %v", data, err)
		}
		// decode honours UnmarshalText first
		for kv, err := range UnmarshalJSON2[textName, int](strings.NewReader(`{"abc":1}`)) {
			if err != nil || kv.Key != "ABC" {
				t.Fatalf("got %v, %v", kv.Key, err)
			}
		}
	})
	t.Run("AgreesWithStd", func(t *testing.T) {
		check := func(got []byte, err error, want any) {
			t.Helper()
			std, serr := json.Marshal(want)
			if err != nil || serr != nil || !bytes.Equal(got, std) {
				t.Fatalf("irt %s (%v), std %s (%v)", got, err, std, serr)
			}
		}
		got, err := MarshalJSON2(keyed("b"))
		check(got, err, map[string]int{"b": 0})
		got, err = MarshalJSON2(keyed(-1))
		check(got, err, map[int]int{-1: 0})
		got, err = MarshalJSON2(keyed(textInt(1)))
		check(got, err, map[textInt]int{1: 0})
	})
}
