package irt

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

type reviewTextKey struct{ a, b int }

func (k reviewTextKey) MarshalText() ([]byte, error) {
	return []byte(strings.Repeat("k", k.a) + "-" + strings.Repeat("v", k.b)), nil
}

func (k *reviewTextKey) UnmarshalText(in []byte) error {
	left, right, ok := strings.Cut(string(in), "-")
	if !ok {
		return errors.New("bad text key")
	}
	k.a, k.b = len(left), len(right)
	return nil
}

func TestMarshalKeysReview(t *testing.T) {
	marshalers := map[string]func(seq func(yield func(any, int) bool)) ([]byte, error){
		"MarshalJSON2": func(seq func(yield func(any, int) bool)) ([]byte, error) { return MarshalJSON2(seq) },
		"MarshalToJSON2": func(seq func(yield func(any, int) bool)) ([]byte, error) {
			var buf bytes.Buffer
			err := MarshalToJSON2(seq, &buf)
			return buf.Bytes(), err
		},
	}

	for name, marshal := range marshalers {
		t.Run(name, func(t *testing.T) {
			t.Run("NonStringKeysProduceValidJSON", func(t *testing.T) {
				for key, want := range map[string]struct {
					k    any
					json string
				}{
					"int":    {1, `{"1":2}`},
					"uint":   {uint8(7), `{"7":2}`},
					"string": {"a", `{"a":2}`},
					"text":   {reviewTextKey{2, 1}, `{"kk-v":2}`},
				} {
					got, err := marshal(func(yield func(any, int) bool) { yield(want.k, 2) })
					if err != nil {
						t.Errorf("%s: %v", key, err)
						continue
					}
					if string(got) != want.json {
						t.Errorf("%s: got %s want %s", key, got, want.json)
					}
					if !json.Valid(got) {
						t.Errorf("%s: invalid JSON %s", key, got)
					}
				}
			})
			t.Run("UnmarshalableKeysError", func(t *testing.T) {
				for key, k := range map[string]any{
					"chan":   make(chan int),
					"nan":    math.NaN(),
					"struct": struct{ A int }{1},
				} {
					within(t, 2*time.Second, func() {
						var err error
						func() {
							defer func() {
								if r := recover(); r != nil {
									t.Errorf("%s: panicked: %v", key, r)
								}
							}()
							_, err = marshal(func(yield func(any, int) bool) { yield(k, 1) })
						}()
						if err == nil {
							t.Errorf("%s: expected error", key)
						}
					})
				}
			})
		})
	}
}

func TestMarshalEmptyBytesReview(t *testing.T) {
	text, err := MarshalText(Slice([]string{}))
	if err != nil || text == nil || len(text) != 0 {
		t.Errorf("MarshalText empty: %#v %v", text, err)
	}
	bin, err := MarshalBinary(Slice([][]byte{}))
	if err != nil || bin == nil || len(bin) != 0 {
		t.Errorf("MarshalBinary empty: %#v %v", bin, err)
	}
}
