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
func TestStack(t *testing.T) {
	const errval = "ERRO=42"

	t.Parallel()
	t.Run("Nil", func(t *testing.T) {
		var es *Stack
		if es.Len() != 0 {
			t.Fatal("defensive nil for length")
		}
		// if es.append(nil) != nil {
		// 	t.Fatal("append nil errors should always be safe")
		// }
		// if err := es.append(&Stack{}); err == nil {
		// 	t.Error("nil should append to something")
		// }

	})
	t.Run("UnwrapNil", func(t *testing.T) {
		es := &Stack{}
		if err := es.Unwrap(); err != nil {
			t.Fatal("unexpected unwrap empty", err)
		}
	})
	t.Run("ErrorsReportEmpty", func(t *testing.T) {
		es := &Stack{}
		if es.Len() != 0 {
			t.Fatal("unexpected empty length", es.Len())
		}

		if l := collect(t, es.CheckProducer()); len(l) != 0 || l != nil {
			t.Fatal("unexpected errors report", l)
		}

	})
	t.Run("ErrorsReportSingle", func(t *testing.T) {
		es := &Stack{}
		es.append(errors.New(errval))
		if l := collect(t, es.CheckProducer()); len(l) != 1 || l == nil {
			t.Fatal("unexpected errors report", l)
		}
	})
	t.Run("StackErrorStack", func(t *testing.T) {
		es := &Stack{err: errors.New("outer")}
		es.append(&Stack{err: errors.New("inner")})
		if l := collect(t, es.CheckProducer()); len(l) != 2 || l == nil {
			t.Log(es.count, es)
			t.Log(es.Error())
			t.Fatal("unexpected errors report", l)
		}
	})
	t.Run("NilErrorStillErrors", func(t *testing.T) {
		es := &Stack{}
		if e := es.Error(); e == "" {
			t.Error("every non-nil error stack should have an error")
		}
	})
	t.Run("CacheCorrectness", func(t *testing.T) {
		es := &Stack{}
		es.append(errors.New(errval))
		er1 := es.Error()
		es.append(errors.New(errval))
		er2 := es.Error()
		if er1 == er2 {
			t.Error("errors should be different", er1, er2)
		}
	})
	t.Run("Merge", func(t *testing.T) {
		es1 := &Stack{}
		es1.append(errors.New(errval))
		es1.append(errors.New(errval))
		if l := es1.Len(); l != 2 {
			t.Fatal("es1 unexpected length", l)
		}

		es2 := &Stack{}
		es2.append(errors.New(errval))
		es2.append(errors.New(errval))

		if l := es2.Len(); l != 2 {
			t.Fatal("es2 unexpected length", l)
		}

		es1.append(es2)
		if l := es1.Len(); l != 4 {
			t.Fatal("merged unexpected length", l)
		}
	})
	t.Run("ConventionalWrap", func(t *testing.T) {
		err := fmt.Errorf("foo: %w", errors.New("bar"))
		es := &Stack{}
		es.append(err)
		if l := es.Len(); l != 1 {
			t.Fatalf("%d, %+v", l, es)
		}
	})
	t.Run("Is", func(t *testing.T) {
		err1 := errors.New("foo")
		err2 := errors.New("bar")

		es := &Stack{}
		es.append(err1)
		es.append(err2)
		if !errors.Is(es, err1) {
			t.Fatal("expected is to find wrapped err")
		}
	})
	t.Run("OutputOrderedLogically", func(t *testing.T) {
		es := &Stack{}
		es.append(errors.New("one"))
		es.append(errors.New("two"))
		es.append(errors.New("three"))

		output := es.Error()
		const expected = "three: two: one"
		if output != expected {
			t.Error(output, "!=", expected)
		}
	})
}

func TestMergeLegacy(t *testing.T) {
	t.Run("Underlying", func(t *testing.T) {
		e1 := &errorTest{val: 100}
		e2 := &errorTest{val: 200}

		err := Join(e1, e2)

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
		if cp.val != e2.val {
			t.Error(cp.val)
		}
		if !strings.Contains(err.Error(), "100") {
			t.Error(err)
		}
		if !strings.Contains(err.Error(), "error: 200") {
			t.Error(err)
		}
	})
	t.Run("MergeErrors", func(t *testing.T) {
		t.Run("Both", func(t *testing.T) {
			e1 := &errorTest{val: 100}
			e2 := &errorTest{val: 200}

			err := Join(e1, e2)

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
			if cp.val != e2.val {
				t.Error(cp.val)
			}
		})
		t.Run("FirstOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Join(e1, nil)
			if err != e1 {
				t.Error(err, e1)
			}
		})

		t.Run("SecondOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Join(nil, e1)
			if err != e1 {
				t.Error(err, e1)
			}
		})
		t.Run("Neither", func(t *testing.T) {
			err := Join(nil, nil)
			if err != nil {
				t.Error(err)
			}
		})
	})
	t.Run("Splice", func(t *testing.T) {
		errs := []error{io.EOF, ErrRecoveredPanic, fmt.Errorf("hello world")}
		err := Join(errs...)

		assert.Error(t, err)
		assert.True(t, Is(err, errs...))
		assert.Equal(t, len(errs), len(Unwind(err)))
	})
	t.Run("SpliceOne", func(t *testing.T) {
		root := Error("root-error")
		err := Join(root)
		assert.Equal(t, err.Error(), root.Error())
	})
	t.Run("Many", func(t *testing.T) {
		t.Run("Formatting", func(t *testing.T) {
			jerr := Join(Error("one"), Error("two"), Error("three"), Error("four"), Error("five"), Error("six"), Error("seven"), Error("eight"))
			errs := Unwind(jerr)
			t.Log(errs)
			check.Equal(t, len(errs), 8)
			check.Equal(t, jerr.Error(), "eight: seven: six: five: four: three: two: one")
		})
		t.Run("ChainUnwrapping", func(t *testing.T) {
			jerr := fmt.Errorf("next: %w", fmt.Errorf("next: %w", fmt.Errorf("next: %w", fmt.Errorf("next: %w",
				fmt.Errorf("next: %w", fmt.Errorf("next: %w", fmt.Errorf("next: %w", fmt.Errorf("next: %w", errors.New("error")))))))))
			errs := unwind(jerr)
			check.Equal(t, len(errs), 9)
		})
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

func collect[T any](t *testing.T, prod func() (T, bool)) []T {
	t.Helper()

	assert.True(t, prod != nil)

	var out []T

	for v, ok := prod(); ok; v, ok = prod() {
		out = append(out, v)
	}
	return out
}
