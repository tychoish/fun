package erc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestList(t *testing.T) {
	const errval = "ERRO=42"

	t.Parallel()
	t.Run("Nil", func(t *testing.T) {
		var es *list
		if es.Len() != 0 {
			t.Fatal("defensive nil for length")
		}
		check.True(t, es.Ok())
	})
	t.Run("UnwrapEmpty", func(t *testing.T) {
		var es *list
		if err := es.Unwrap(); err != nil {
			t.Fatal("unexpected unwrap to be nil", err)
		}
	})
	t.Run("UnwrapList", func(t *testing.T) {
		es := &list{}
		if err := es.Unwrap(); err != nil {
			t.Fatal("unexpected unwrap empty", err)
		}
	})
	t.Run("PushElements", func(t *testing.T) {
		t.Run("Detached", func(t *testing.T) {
			exp := errors.New("test")
			es := &list{}
			elem := &element{err: exp}
			assert.Equal(t, es.Len(), 0)
			assert.True(t, !es.In(elem))
			assert.True(t, !elem.In(es))
			es.Push(elem)
			assert.Equal(t, es.Len(), 1)
			assert.NotEqual(t, es.Back(), elem)
		})
		t.Run("Reflexive", func(t *testing.T) {
			exp := errors.New("test")
			es := &list{}
			es.Push(exp)
			assert.Equal(t, es.Len(), 1)
			assert.True(t, es.Back().Ok())
			es.Push(es.Back())
			assert.Equal(t, es.Len(), 1)
		})
		t.Run("MergeListsSingleElement", func(t *testing.T) {
			exp := errors.New("test")
			es := &list{}
			ee := &list{}
			es.Push(exp)
			ee.Push(exp)
			es.Push(ee.Back())
			assert.Equal(t, 1, ee.Len())
			assert.Equal(t, 2, es.Len())
		})
		t.Run("MergeListsManyElements", func(t *testing.T) {
			exp := errors.New("test")
			es := &list{}
			ee := &list{}
			es.Push(exp)
			ee.Push(exp)
			ee.Push(exp)
			ee.Push(exp)
			assert.Equal(t, 3, ee.Len())
			assert.Equal(t, 1, es.Len())
			es.Push(ee.Front())
			assert.Equal(t, 3, ee.Len())
			assert.Equal(t, 4, es.Len())
		})
	})
	t.Run("Membership", func(t *testing.T) {
		exp := errors.New("test")
		es := &list{}
		es.Push(exp)
		elm := es.Back()
		assert.True(t, es.In(elm))
		assert.True(t, elm.In(es))
	})
	t.Run("PushFront", func(t *testing.T) {
		ferr := errors.New("front-err")
		berr := errors.New("back-err")
		es := &list{}
		es.PushFront(berr)
		assert.Equal(t, es.Len(), 1)
		es.PushFront(ferr)
		assert.Equal(t, es.Len(), 2)
		assert.Equal(t, es.Back().err, berr)
		assert.Equal(t, es.Front().err, ferr)
	})
	t.Run("Handler", func(t *testing.T) {
		es := &list{}
		check.Equal(t, es.Len(), 0)
		hf := es.Handler()
		check.Equal(t, es.Len(), 0)
		hf(nil)
		check.Equal(t, es.Len(), 0)
		hf(ers.ErrInvalidInput)
		check.Equal(t, es.Len(), 1)
		check.True(t, !es.Ok())
		check.ErrorIs(t, es, ers.ErrInvalidInput)
	})
	t.Run("NilErrorStillErrors", func(t *testing.T) {
		es := &list{}
		if e := es.Error(); e == "" {
			t.Error("every non-nil error list should have an error")
		}
	})
	t.Run("Future", func(t *testing.T) {
		es := &Collector{}
		future := es.Resolve
		check.NotError(t, future())
		es.Push(ers.ErrInvalidInput)
		es.Push(ers.ErrImmutabilityViolation)
		check.Error(t, future())
		check.ErrorIs(t, future(), ers.ErrInvalidInput)

		check.ErrorIs(t, future(), ers.ErrImmutabilityViolation)
		st := AsCollector(future())
		check.Equal(t, st.Error(), es.Error())
	})
	t.Run("CacheCorrectness", func(t *testing.T) {
		es := &list{}
		es.Add(errors.New(errval))
		er1 := es.Error()
		es.Add(errors.New(errval))
		er2 := es.Error()
		if er1 == er2 {
			t.Error("errors should be different", er1, er2)
		}
	})
	t.Run("Merge", func(t *testing.T) {
		es1 := &list{}
		es1.Add(errors.New(errval))
		es1.Add(errors.New(errval))
		if l := es1.Len(); l != 2 {
			t.Fatal("es1 unexpected length", l)
		}

		es2 := &list{}
		es2.Add(errors.New(errval))
		es2.Add(errors.New(errval))

		if l := es2.Len(); l != 2 {
			t.Fatal("es2 unexpected length", l)
		}

		es1.Add(es2)
		if l := es1.Len(); l != 4 {
			t.Fatal("merged unexpected length", l)
		}
	})
	t.Run("ConventionalWrap", func(t *testing.T) {
		err := fmt.Errorf("foo: %w", errors.New("bar"))
		es := &list{}
		es.Push(err)
		if l := es.Len(); l != 1 {
			t.Fatalf("%d, %+v", l, es)
		}
	})
	t.Run("Is", func(t *testing.T) {
		err1 := errors.New("foo")
		err2 := errors.New("bar")

		es := &list{}
		es.Push(err1)
		es.Push(err2)
		if !errors.Is(es, err1) {
			t.Fatal("expected is to find wrapped err")
		}
	})
	t.Run("OutputOrderedLogically", func(t *testing.T) {
		es := &list{}
		es.Push(errors.New("one"))
		es.Push(errors.New("two"))
		es.Push(errors.New("three"))

		output := es.Error()
		const expected = "three: two: one"
		if output != expected {
			t.Error(output, "!=", expected)
		}
	})
	t.Run("AsList", func(t *testing.T) {
		t.Run("Nil", func(t *testing.T) {
			es := AsCollector(nil)
			check.NilPtr(t, es)
		})
		t.Run("ZeroValues", func(t *testing.T) {
			var err error
			es := AsCollector(err)
			check.NilPtr(t, es)
			es = AsCollector(es)
			check.NilPtr(t, es)
		})
		t.Run("Error", func(t *testing.T) {
			es := AsCollector(ers.ErrInvalidInput)
			assert.NotNilPtr(t, es)
			check.Equal(t, es.Len(), 1)
			check.ErrorIs(t, es.Resolve(), ers.ErrInvalidInput)
		})
		t.Run("List", func(t *testing.T) {
			err := Join(ers.ErrInvalidInput, ers.ErrImmutabilityViolation, ers.ErrInvariantViolation)
			es := AsCollector(err)
			assert.NotNilPtr(t, es)
			check.Equal(t, es.Len(), 3)
			check.ErrorIs(t, es.Resolve(), ers.ErrInvalidInput)
			check.ErrorIs(t, es.Resolve(), ers.ErrInvariantViolation)
		})
		t.Run("Unwinder", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				es := AsCollector(&slwind{})
				check.NilPtr(t, es)

			})
			t.Run("Populated", func(t *testing.T) {
				es := AsCollector(&slwind{out: []error{ers.ErrInvalidInput}})
				assert.NotNilPtr(t, es)
				check.Equal(t, es.Len(), 1)
			})
		})
		t.Run("Unwrapper", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				es := AsCollector(&slwrap{})
				check.NilPtr(t, es)

			})
			t.Run("Populated", func(t *testing.T) {
				es := AsCollector(&slwrap{out: []error{ers.ErrInvalidInput}})
				assert.NotNilPtr(t, es)
				check.Equal(t, es.Len(), 1)
			})
		})
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
				t.Logf("err %T", err)
				t.Logf("e1 %T", e1)
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
		errs := []error{io.EOF, ers.ErrRecoveredPanic, fmt.Errorf("hello world")}
		err := Join(errs...)

		assert.Error(t, err)
		assert.True(t, ers.Is(err, errs...))
		assert.Equal(t, len(errs), len(ers.Unwind(err)))
	})
	t.Run("SpliceOne", func(t *testing.T) {
		root := ers.Error("root-error")
		err := Join(root)
		assert.Equal(t, err.Error(), root.Error())
	})
	t.Run("Many", func(t *testing.T) {
		t.Run("Formatting", func(t *testing.T) {
			jerr := Join(ers.Error("one"), ers.Error("two"), ers.Error("three"), ers.Error("four"), ers.Error("five"), ers.Error("six"), ers.Error("seven"), ers.Error("eight"))
			errs := ers.Unwind(jerr)
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
	t.Run("UnwindingPush", func(t *testing.T) {
		err := slwind{out: []error{io.EOF, context.Canceled, ers.ErrLimitExceeded}}
		s := &list{}
		s.Push(err)
		check.Equal(t, s.Len(), 3)
	})
	t.Run("UnwrappingPush", func(t *testing.T) {
		err := slwrap{out: []error{io.EOF, context.Canceled, ers.ErrLimitExceeded}}
		s := &list{}
		s.Push(err)
		check.Equal(t, s.Len(), 3)
	})
}
