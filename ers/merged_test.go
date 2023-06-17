package ers

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

func TestMerge(t *testing.T) {
	t.Run("Underlying", func(t *testing.T) {
		e1 := &errorTest{val: 100}
		e2 := &errorTest{val: 200}

		err := &Combined{Current: e1, Previous: e2}

		if !errors.Is(err, e1) {
			t.Error("shold be er1", err, e1)
		}

		if !errors.Is(err, e2) {
			t.Error("shold be er2", err, e2)
		}
		cp := &errorTest{}
		if !errors.As(err, &cp) {
			t.Error("should err as", err, cp)
		}
		if cp.val != e1.val {
			t.Error(cp.val)
		}
		if !strings.Contains(err.Error(), "100") {
			t.Error(err)
		}
		if !strings.Contains(err.Error(), "[error: 200]") {
			t.Error(err)
		}
	})
	t.Run("MergeErrors", func(t *testing.T) {
		t.Run("Both", func(t *testing.T) {
			e1 := &errorTest{val: 100}
			e2 := &errorTest{val: 200}

			err := Merge(e1, e2)

			if err == nil {
				t.Fatal("should be an error")
			}
			if !errors.Is(err, e1) {
				t.Error("shold be er1", err, e1)
			}

			if !errors.Is(err, e2) {
				t.Error("shold be er2", err, e2)
			}
			cp := &errorTest{}
			if !errors.As(err, &cp) {
				t.Error("should err as", err, cp)
			}
			if cp.val != e1.val {
				t.Error(cp.val)
			}
		})
		t.Run("FirstOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Merge(e1, nil)
			if err != e1 {
				t.Error(err, e1)
			}
		})

		t.Run("SecondOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Merge(nil, e1)
			if err != e1 {
				t.Error(err, e1)
			}
		})
		t.Run("Neither", func(t *testing.T) {
			err := Merge(nil, nil)
			if err != nil {
				t.Error(err)
			}
		})
	})
	t.Run("Splice", func(t *testing.T) {
		errs := []error{io.EOF, ErrRecoveredPanic, fmt.Errorf("hello world")}
		err := Splice(errs...)

		assert.Error(t, err)
		assert.True(t, Is(err, errs...))
		assert.Equal(t, len(errs), len(unwind(err)))
	})
	t.Run("FindRoot", func(t *testing.T) {
		err := &Combined{Current: Error("hi")}
		check.Equal(t, err.findRoot(), nil)
		err.Previous = &Combined{}
		check.Equal(t, err.findRoot(), error(err.Current))
		weird := &Combined{Current: nil, Previous: &Combined{}}
		check.True(t, weird.findRoot() == nil)
	})
	t.Run("ErrStringEdges", func(t *testing.T) {
		err := &Combined{}
		check.Equal(t, err.Error(), "nil-error")
		err = &Combined{Current: Error("hi")}
		check.Equal(t, err.Error(), "hi")
	})
}

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
