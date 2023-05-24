package erc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

func TestCollections(t *testing.T) {
	t.Run("Merge", func(t *testing.T) {
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
			assert.True(t, err == e1)
		})

		t.Run("SecondOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Merge(nil, e1)
			assert.True(t, err == e1)
		})
		t.Run("Neither", func(t *testing.T) {
			err := Merge(nil, nil)
			assert.NotError(t, err)
		})
	})
	t.Run("Wrap", func(t *testing.T) {
		check.NotError(t, Wrap(nil, "hello"))
		check.NotError(t, Wrapf(nil, "hello %s %s", "args", "argsd"))
		const expected ConstErr = "hello"
		err := Wrap(expected, "hello")
		assert.Equal(t, err.Error(), "hello: hello")
		assert.ErrorIs(t, err, expected)

		err = Wrapf(expected, "hello %s", "world")
		assert.Equal(t, err.Error(), "hello world: hello")
		assert.ErrorIs(t, err, expected)
	})
	t.Run("Collapse", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			if err := Collapse(); err != nil {
				t.Error("should be nil", err)
			}
		})
		t.Run("One", func(t *testing.T) {
			const e ConstErr = "fourty-two"
			err := Collapse(e)
			if !errors.Is(err, e) {
				t.Error(err, e)
			}
		})
		t.Run("Many", func(t *testing.T) {
			const e0 ConstErr = "fourty-two"
			const e1 ConstErr = "fourty-three"
			err := Collapse(e0, e1)
			if !errors.Is(err, e1) {
				t.Error(err, e1)
			}
			if !errors.Is(err, e0) {
				t.Error(err, e0)
			}
			errs := Unwind(err)
			if len(errs) != 2 {
				t.Error(errs)
			}
		})
	})
	t.Run("Stream", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Run("Base", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				ch := make(chan error)
				close(ch)
				err := Stream(ctx, ch)
				if err != nil {
					t.Error("nil expected", err)
				}
			})

			t.Run("One", func(t *testing.T) {
				ch := getPopulatedErrChan(1)
				close(ch)
				err := Stream(ctx, ch)
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 1 {
					t.Error(errs)
				}
			})
			t.Run("Many", func(t *testing.T) {
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				err := Stream(ctx, ch)
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 10 {
					t.Error(errs)
				}
			})
		})
	})
	t.Run("CheckWhen", func(t *testing.T) {
		t.Run("NotCalled", func(t *testing.T) {
			ec := &Collector{}
			called := false
			CheckWhen(ec, false, func() error { called = true; return errors.New("kip") })
			assert.NotError(t, ec.Resolve())
			assert.True(t, !called)
		})
		t.Run("Called", func(t *testing.T) {
			ec := &Collector{}
			called := false
			CheckWhen(ec, true, func() error { called = true; return errors.New("kip") })
			assert.Error(t, ec.Resolve())
			assert.True(t, called)
		})
	})

	t.Run("Checkf", func(t *testing.T) {
		ec := &Collector{}
		count := 0
		Checkf(ec, func() error { count++; return nil }, "foo %s", "bar")
		check.Equal(t, count, 1)
		assert.NotError(t, ec.Resolve())
		expected := errors.New("kip")
		Checkf(ec, func() error { count++; return expected }, "foo %s", "bar")
		assert.Error(t, ec.Resolve())
		assert.ErrorIs(t, ec.Resolve(), expected)
		check.Equal(t, count, 2)
		assert.Equal(t, "foo bar: kip", ec.Resolve().Error())
	})
	t.Run("Recovery", func(t *testing.T) {
		ob := func(err error) {
			check.Error(t, err)
			check.ErrorIs(t, err, fun.ErrRecoveredPanic)
		}
		assert.NotPanic(t, func() {
			defer Recovery(ob)
			panic("hi")
		})
	})

	t.Run("Collect", func(t *testing.T) {
		ec := &Collector{}
		collect := Collect[int](ec)
		operation := func() (int, error) { return 42, errors.New("kip") }
		out := collect(operation())
		assert.Equal(t, out, 42)
		assert.Error(t, ec.Resolve())
		assert.Equal(t, ec.Resolve().Error(), "kip")
	})
}

func getPopulatedErrChan(size int) chan error {
	out := make(chan error, size)

	for i := 0; i < size; i++ {
		out <- fmt.Errorf("mock err %d", i)
	}
	return out
}
