package erc

import (
	"fmt"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

type slwind struct{ out []error }

func (s slwind) Unwind() []error { return s.out }
func (s slwind) Error() string   { return fmt.Sprint("wind error:", len(s.out), s.out) }

type slwrap struct{ out []error }

func (s slwrap) Unwrap() []error { return s.out }
func (s slwrap) Error() string   { return fmt.Sprint("wrap error:", len(s.out), s.out) }

func unwind[T any](in T) (out []T) {
	if us, ok := any(in).(interface{ Unwrap() []T }); ok {
		return us.Unwrap()
	}

	for {
		out = append(out, in)
		u, ok := any(in).(interface{ Unwrap() T })
		if ok {
			in = u.Unwrap()
			continue
		}
		return
	}
}

func collect[T any](t testing.TB, prod func() (T, bool)) []T {
	t.Helper()

	assert.True(t, prod != nil)

	var out []T

	for v, ok := prod(); ok; v, ok = prod() {
		out = append(out, v)
	}
	return out
}

func catcherIsEmpty(t *testing.T, catcher *Collector) {
	t.Helper()

	if catcher == nil {
		t.Fatal("test issue")
	}

	if catcher.HasErrors() {
		t.Error("should not have errors")
	}
	if err := catcher.Resolve(); err != nil {
		t.Error("should produce nil error", err)
	}
	check.Zero(t, catcher.Len())
}

func catcherHasErrors(t *testing.T, expectedNum int, catcher *Collector) {
	t.Helper()

	if catcher == nil || expectedNum <= 0 {
		t.Fatal("test issue", catcher, expectedNum)
	}

	if actual := catcher.list.Len(); actual != expectedNum {
		t.Error("should have expected number of errors", expectedNum, actual)
		t.Log(catcher.Resolve())
	}
	if catcher.Ok() {
		t.Error("should have errors")
	}
	if catcher.Resolve() == nil {
		t.Error("should produce an error")
	}
	check.Equal(t, expectedNum, catcher.Len())
}
