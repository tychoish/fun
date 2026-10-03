package irt

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// ucID is a struct-kinded key with symmetric text encoding.
type ucID struct{ Org, Num int }

func (i ucID) MarshalText() ([]byte, error) { return fmt.Appendf(nil, "%d/%d", i.Org, i.Num), nil }
func (i *ucID) UnmarshalText(b []byte) error {
	org, num, ok := strings.Cut(string(b), "/")
	if !ok {
		return errors.New("bad id")
	}
	var err error
	if i.Org, err = strconv.Atoi(org); err != nil {
		return err
	}
	i.Num, err = strconv.Atoi(num)
	return err
}

// ucCode is a string-kinded key with its own text codec; the string
// kind wins when encoding but UnmarshalText is used when decoding.
type ucCode string

func (c *ucCode) UnmarshalText(b []byte) error { *c = ucCode("<" + string(b) + ">"); return nil }

type ucRec struct {
	Name  string            `json:"name"`
	Tags  []string          `json:"tags,omitempty"`
	Attrs map[string]int    `json:"attrs,omitempty"`
	Next  *ucRec            `json:"next,omitempty"`
	Extra map[string]string `json:"extra"`
}

func ucPairs[K comparable, V any](keys []K, vals []V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := range keys {
			if !yield(keys[i], vals[i]) {
				return
			}
		}
	}
}

func ucRoundTrip2[K comparable, V any](t *testing.T, keys []K, vals []V) {
	t.Helper()
	data, err := MarshalJSON2(ucPairs(keys, vals))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var gotK []K
	var gotV []V
	for kv, err := range UnmarshalJSON2[K, V](bytes.NewReader(data)) {
		if err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		gotK = append(gotK, kv.Key)
		gotV = append(gotV, kv.Value)
	}
	if !slices.Equal(gotK, keys) {
		t.Fatalf("keys: got %v, want %v (%s)", gotK, keys, data)
	}
	if fmt.Sprint(gotV) != fmt.Sprint(vals) {
		t.Fatalf("values: got %v, want %v", gotV, vals)
	}
}

func TestUseCaseJSONCustomKeyRoundTrips(t *testing.T) {
	t.Run("StructKeyWithTextCodec", func(t *testing.T) {
		ucRoundTrip2(t, []ucID{{1, 2}, {0, 0}, {-3, 40}}, []string{"a", "b", "c"})
	})
	t.Run("SmallIntegerKinds", func(t *testing.T) {
		ucRoundTrip2(t, []int8{-128, 0, 127}, []int{1, 2, 3})
		ucRoundTrip2(t, []uint16{0, 65535}, []int{1, 2})
		ucRoundTrip2(t, []int64{-1 << 63, 1<<63 - 1}, []int{1, 2})
		ucRoundTrip2(t, []uint64{0, 1<<64 - 1}, []int{1, 2})
	})
	t.Run("AwkwardStringKeys", func(t *testing.T) {
		keys := []string{"", " ", "a\"b", "back\\slash", "new\nline", "tab\t", "\u0000nul", "emoji-\U0001F600", "<&>", "日本語", " "}
		vals := make([]int, len(keys))
		ucRoundTrip2(t, keys, vals)
	})
	t.Run("NamedStringKind", func(t *testing.T) {
		type name string
		ucRoundTrip2(t, []name{"x", "y z"}, []bool{true, false})
	})
	t.Run("StringKindDecodesThroughUnmarshalText", func(t *testing.T) {
		data, err := MarshalJSON2(ucPairs([]ucCode{"raw"}, []int{1}))
		if err != nil || string(data) != `{"raw":1}` {
			t.Fatalf("got %s, %v", data, err)
		}
		for kv, err := range UnmarshalJSON2[ucCode, int](bytes.NewReader(data)) {
			if err != nil || kv.Key != "<raw>" {
				t.Fatalf("got %v, %v", kv.Key, err)
			}
		}
	})
	t.Run("ComplexValues", func(t *testing.T) {
		recs := []ucRec{
			{Name: "empty"},
			{Name: "full", Tags: []string{"a", "b"}, Attrs: map[string]int{"x": 1}, Extra: map[string]string{"k": "v"}, Next: &ucRec{Name: "inner"}},
		}
		data, err := MarshalJSON2(ucPairs([]string{"one", "two"}, recs))
		if err != nil {
			t.Fatal(err)
		}
		var got []ucRec
		for kv, err := range UnmarshalJSON2[string, ucRec](bytes.NewReader(data)) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, kv.Value)
		}
		if len(got) != 2 || got[1].Next == nil || got[1].Next.Name != "inner" || got[1].Attrs["x"] != 1 || got[0].Name != "empty" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("NullAndNestedValues", func(t *testing.T) {
		data := `{"a":null,"b":[1,2],"c":{"d":null}}`
		var keys []string
		for kv, err := range UnmarshalJSON2[string, any](strings.NewReader(data)) {
			if err != nil {
				t.Fatal(err)
			}
			keys = append(keys, kv.Key)
		}
		if !slices.Equal(keys, []string{"a", "b", "c"}) {
			t.Fatalf("keys %v", keys)
		}
	})
	t.Run("DuplicateKeysArePreservedInOrder", func(t *testing.T) {
		data, err := MarshalJSON2(ucPairs([]string{"k", "k"}, []int{1, 2}))
		if err != nil || string(data) != `{"k":1,"k":2}` {
			t.Fatalf("got %s, %v", data, err)
		}
		var vals []int
		for kv, err := range UnmarshalJSON2[string, int](bytes.NewReader(data)) {
			if err != nil {
				t.Fatal(err)
			}
			vals = append(vals, kv.Value)
		}
		if !slices.Equal(vals, []int{1, 2}) {
			t.Fatalf("got %v", vals)
		}
	})
}

func TestUseCaseJSONKeyDecodeFailures(t *testing.T) {
	failures := map[string]func() error{
		"Int8Overflow": func() error {
			return ucFirstError(UnmarshalJSON2[int8, int](strings.NewReader(`{"300":1}`)))
		},
		"UintNegative": func() error {
			return ucFirstError(UnmarshalJSON2[uint, int](strings.NewReader(`{"-1":1}`)))
		},
		"IntNotANumber": func() error {
			return ucFirstError(UnmarshalJSON2[int, int](strings.NewReader(`{"abc":1}`)))
		},
		"TextCodecRejects": func() error {
			return ucFirstError(UnmarshalJSON2[ucID, int](strings.NewReader(`{"nope":1}`)))
		},
		"UnsupportedKeyKind": func() error {
			return ucFirstError(UnmarshalJSON2[float64, int](strings.NewReader(`{"1.5":1}`)))
		},
		"BoolKey": func() error {
			return ucFirstError(UnmarshalJSON2[bool, int](strings.NewReader(`{"true":1}`)))
		},
		"ValueTypeMismatch": func() error {
			return ucFirstError(UnmarshalJSON2[string, int](strings.NewReader(`{"a":"str"}`)))
		},
		"NotAnObject":   func() error { return ucFirstError(UnmarshalJSON2[string, int](strings.NewReader(`[1]`))) },
		"Truncated":     func() error { return ucDrainError(UnmarshalJSON2[string, int](strings.NewReader(`{"a":1`))) },
		"Empty":         func() error { return ucFirstError(UnmarshalJSON2[string, int](strings.NewReader(``))) },
		"TrailingValue": func() error { return ucDrainError(UnmarshalJSON2[string, int](strings.NewReader(`{"a":1} {}`))) },
	}
	for name, run := range failures {
		t.Run(name, func(t *testing.T) {
			if err := run(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
	t.Run("ErrorEndsTheSequence", func(t *testing.T) {
		n := 0
		for kv, err := range UnmarshalJSON2[int, int](strings.NewReader(`{"1":1,"x":2,"3":3}`)) {
			n++
			if err != nil {
				break
			}
			if kv.Key != 1 {
				t.Fatalf("unexpected key %d", kv.Key)
			}
		}
		if n != 2 {
			t.Fatalf("saw %d items, want the first value then the error", n)
		}
	})
	t.Run("UnsupportedKeyKindMarshal", func(t *testing.T) {
		for name, seq := range map[string]iter.Seq2[any, int]{
			"Bool":   ucPairs([]any{true}, []int{1}),
			"Float":  ucPairs([]any{1.5}, []int{1}),
			"Struct": ucPairs([]any{struct{}{}}, []int{1}),
			"Nil":    ucPairs([]any{nil}, []int{1}),
		} {
			if _, err := MarshalJSON2(seq); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}
	})
}

func ucFirstError[T any](seq iter.Seq2[T, error]) error {
	for _, err := range seq {
		return err
	}
	return nil
}

func ucDrainError[T any](seq iter.Seq2[T, error]) error {
	var last error
	for _, err := range seq {
		if err != nil {
			last = err
		}
	}
	return last
}

func TestUseCaseJSONEmptyAndSingleRoundTrips(t *testing.T) {
	t.Run("EmptyArray", func(t *testing.T) {
		data, err := MarshalJSON(Zero[int]())
		if err != nil || string(data) != "[]" {
			t.Fatalf("got %s, %v", data, err)
		}
		if got := Collect(First(UnmarshalJSON[int](bytes.NewReader(data)))); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("EmptyObject", func(t *testing.T) {
		data, err := MarshalJSON2(Zero2[string, int]())
		if err != nil || string(data) != "{}" {
			t.Fatalf("got %s, %v", data, err)
		}
		if n := Count2(UnmarshalJSON2[string, int](bytes.NewReader(data))); n != 0 {
			t.Fatalf("decoded %d pairs", n)
		}
	})
	t.Run("One", func(t *testing.T) {
		data, err := MarshalJSON(One("only"))
		if err != nil || string(data) != `["only"]` {
			t.Fatalf("got %s, %v", data, err)
		}
		data, err = MarshalJSON2(Two("k", 1))
		if err != nil || string(data) != `{"k":1}` {
			t.Fatalf("got %s, %v", data, err)
		}
	})
	t.Run("WhitespaceAndNull", func(t *testing.T) {
		got := Collect(First(UnmarshalJSON[*int](strings.NewReader(" [ null , 1 ] "))))
		if len(got) != 2 || got[0] != nil || *got[1] != 1 {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Large", func(t *testing.T) {
		const n = 20000
		data, err := MarshalJSON(Range(1, n))
		if err != nil {
			t.Fatal(err)
		}
		got := Collect(First(UnmarshalJSON[int](bytes.NewReader(data))))
		if len(got) != n || got[0] != 1 || got[n-1] != n {
			t.Fatalf("round trip lost items: %d", len(got))
		}
		pairs, err := MarshalJSON2(With(Range(1, n), strconv.Itoa))
		if err != nil {
			t.Fatal(err)
		}
		if c := Count2(UnmarshalJSON2[int, string](bytes.NewReader(pairs))); c != n {
			t.Fatalf("pair round trip lost items: %d", c)
		}
	})
}

func TestUseCaseJSONEarlyBreakAtEveryPosition(t *testing.T) {
	data := []byte(`[1,2,3,4]`)
	obj := []byte(`{"1":1,"2":2,"3":3,"4":4}`)
	for pos := 1; pos <= 4; pos++ {
		t.Run(strconv.Itoa(pos), func(t *testing.T) {
			n := 0
			for _, err := range UnmarshalJSON[int](bytes.NewReader(data)) {
				if err != nil {
					t.Fatal(err)
				}
				if n++; n == pos {
					break
				}
			}
			if n != pos {
				t.Fatalf("array: saw %d", n)
			}
			n = 0
			for _, err := range UnmarshalJSON2[int, int](bytes.NewReader(obj)) {
				if err != nil {
					t.Fatal(err)
				}
				if n++; n == pos {
					break
				}
			}
			if n != pos {
				t.Fatalf("object: saw %d", n)
			}
		})
	}
	t.Run("ReiterationReReadsTheReader", func(t *testing.T) {
		// the reader is consumed by the first pass.
		seq := UnmarshalJSON[int](bytes.NewReader(data))
		if first := Collect(First(seq)); len(first) != 4 {
			t.Fatalf("first pass: %v", first)
		}
		if err := ucFirstError(seq); err == nil {
			t.Fatal("second pass over a drained reader should report an error")
		}
	})
}

func TestUseCaseJSONLRoundTrip(t *testing.T) {
	data, err := MarshalJSONL(Args("a", "b\nc", "d"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("JSONL output is not newline terminated: %q", data)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 3 || lines[1] != `"b\nc"` {
		t.Fatalf("got %q", lines)
	}
	if data, err := MarshalJSONL(Zero[int]()); err != nil || len(data) != 0 {
		t.Fatalf("empty: %q, %v", data, err)
	}
}
