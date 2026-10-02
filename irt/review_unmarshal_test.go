package irt

import (
	"fmt"
	"strings"
	"testing"
)

func TestUnmarshalKeysReview(t *testing.T) {
	t.Run("IntKeysParsed", func(t *testing.T) {
		got := map[int]int{}
		for kv, err := range UnmarshalJSON2[int, int](strings.NewReader(`{"1":10,"22":20}`)) {
			if err != nil {
				t.Fatal(err)
			}
			got[kv.Key] = kv.Value
		}
		if len(got) != 2 || got[1] != 10 || got[22] != 20 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("UnparseableKeyIsError", func(t *testing.T) {
		var sawErr bool
		for kv, err := range UnmarshalJSON2[int, int](strings.NewReader(`{"a":1}`)) {
			if err != nil {
				sawErr = true
				continue
			}
			t.Errorf("unexpected pair %v", kv)
		}
		if !sawErr {
			t.Error("expected an error for a non-integer key")
		}
	})
	t.Run("TextUnmarshalerKey", func(t *testing.T) {
		var got []reviewTextKey
		for kv, err := range UnmarshalJSON2[reviewTextKey, int](strings.NewReader(`{"kk-v":1}`)) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, kv.Key)
		}
		if len(got) != 1 || got[0] != (reviewTextKey{2, 1}) {
			t.Errorf("got %v", got)
		}
	})
	t.Run("UnsupportedKeyTypeIsError", func(t *testing.T) {
		var sawErr bool
		for _, err := range UnmarshalJSON2[struct{ A int }, int](strings.NewReader(`{"a":1}`)) {
			sawErr = sawErr || err != nil
		}
		if !sawErr {
			t.Error("expected an error")
		}
	})
}

func TestUnmarshalTrailingGarbageReview(t *testing.T) {
	for name, tc := range map[string]struct {
		input string
		run   func(string) (n int, err error)
	}{
		"Array": {`[1,2] garbage`, func(in string) (n int, err error) {
			for _, e := range UnmarshalJSON[int](strings.NewReader(in)) {
				if e != nil {
					err = e
					continue
				}
				n++
			}
			return
		}},
		"ArrayTwoValues": {`[1,2][3]`, func(in string) (n int, err error) {
			for _, e := range UnmarshalJSON[int](strings.NewReader(in)) {
				if e != nil {
					err = e
					continue
				}
				n++
			}
			return
		}},
		"Object": {`{"a":1} garbage`, func(in string) (n int, err error) {
			for _, e := range UnmarshalJSON2[string, int](strings.NewReader(in)) {
				if e != nil {
					err = e
					continue
				}
				n++
			}
			return
		}},
		"ObjectTruncated": {`{"a":1`, func(in string) (n int, err error) {
			for _, e := range UnmarshalJSON2[string, int](strings.NewReader(in)) {
				if e != nil {
					err = e
					continue
				}
				n++
			}
			return
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tc.run(tc.input); err == nil {
				t.Errorf("expected an error for %q", tc.input)
			}
		})
	}

	t.Run("TrailingWhitespaceOK", func(t *testing.T) {
		for _, e := range UnmarshalJSON[int](strings.NewReader("[1,2]  \n\t")) {
			if e != nil {
				t.Error(e)
			}
		}
		for _, e := range UnmarshalJSON2[string, int](strings.NewReader("{\"a\":1}\n")) {
			if e != nil {
				t.Error(e)
			}
		}
	})
	t.Run("LargeStream", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("[")
		const n = 50000
		for i := range n {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprint(&sb, i)
		}
		sb.WriteString("]")
		count := 0
		for _, err := range UnmarshalJSON[int](strings.NewReader(sb.String())) {
			if err != nil {
				t.Fatal(err)
			}
			count++
		}
		if count != n {
			t.Errorf("count %d", count)
		}
	})
}
