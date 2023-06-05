package itertool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert/check"
)

func TestJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("RoundTrip", func(t *testing.T) {
		iter := fun.SliceIterator([]int{400, 300, 42})
		out, err := MarshalJSON[int](ctx, iter)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "[400,300,42]" {
			t.Error(string(out))
		}
		nl := []int{}
		if err := json.Unmarshal(out, &nl); err != nil {
			t.Error(err)
		}

	})
	t.Run("TypeMismatch", func(t *testing.T) {
		list := fun.SliceIterator([]int{400, 300, 42})
		out, err := MarshalJSON[int](ctx, list)
		if err != nil {
			t.Fatal(err)
		}

		iter := UnmarshalJSON[string](out)
		if iter.Next(ctx) {
			t.Error("shouldn't iterate")
		}
		if err := iter.Close(); err == nil {
			t.Error("expected error")
		}
		if iter.Value() != "" {
			t.Error(iter.Value())
		}
	})
	t.Run("Unmarshalable", func(t *testing.T) {
		list := fun.SliceIterator([]func(){func() {}, nil})
		out, err := MarshalJSON[func()](ctx, list)
		if err == nil {
			t.Fatal(string(out))
		}
	})
	t.Run("ErrorIter", func(t *testing.T) {
		iter := &errIter[int]{err: context.Canceled}
		out, err := MarshalJSON[int](ctx, iter)
		if err == nil {
			t.Fatal(string(out))
		}
		check.Zero(t, iter.Value())
	})
	t.Run("UnmarshalNil", func(t *testing.T) {
		iter := UnmarshalJSON[string](nil)
		vals, err := iter.Slice(ctx)
		if err == nil {
			t.Error("expected error")
		}
		if len(vals) != 0 {
			t.Error(len(vals), vals)
		}
	})
	t.Run("UnmarshalableType", func(t *testing.T) {
		iter := UnmarshalJSON[func()]([]byte(`["foo", "arg"]`))
		vals, err := iter.Slice(ctx)
		if err == nil {
			t.Error("expected error")
		}
		if len(vals) != 0 {
			t.Error(len(vals), vals)
		}
	})
	t.Run("Unmarshal", func(t *testing.T) {
		iter := UnmarshalJSON[string]([]byte(`["foo", "arg"]`))
		vals, err := iter.Slice(ctx)
		if err != nil {
			t.Error(err)
		}
		if len(vals) != 2 {
			t.Fatal(len(vals), vals)
		}
		if vals[0] != "foo" {
			t.Error(vals[0])
		}
		if vals[1] != "arg" {
			t.Error(vals[1])
		}
	})
}
