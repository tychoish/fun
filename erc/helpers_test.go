package erc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
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
				t.Error(cp.val, e1.val)
				t.Log(cp)
			}
		})
		t.Run("FirstOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Join(e1, nil)
			testt.Log(t, err)
			testt.Logf(t, "%T", err)

			assert.ErrorIs(t, err, e1)
		})

		t.Run("SecondOnly", func(t *testing.T) {
			e1 := error(&errorTest{val: 100})
			err := Join(nil, e1)
			assert.ErrorIs(t, err, e1)
		})
		t.Run("Neither", func(t *testing.T) {
			err := Join(nil, nil)
			assert.NotError(t, err)
		})
	})
	t.Run("Wrap", func(t *testing.T) {
		check.NotError(t, Wrap(nil, "hello"))
		check.NotError(t, Wrapf(nil, "hello %s %s", "args", "argsd"))
		const expected ers.Error = "hello"
		err := Wrap(expected, "hello")
		assert.Equal(t, err.Error(), "hello: hello")
		assert.ErrorIs(t, err, expected)

		err = Wrapf(expected, "hello %s", "world")
		assert.Equal(t, err.Error(), "hello world: hello")
		assert.ErrorIs(t, err, expected)
	})
	t.Run("Collapse", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			if err := Join(); err != nil {
				t.Error("should be nil", err)
			}
		})
		t.Run("One", func(t *testing.T) {
			const e ers.Error = "forty-two"
			err := Join(e)
			if !errors.Is(err, e) {
				t.Error(err, e)
			}
		})
		t.Run("Many", func(t *testing.T) {
			const e0 ers.Error = "forty-two"
			const e1 ers.Error = "forty-three"
			err := Join(e0, e1)
			if !errors.Is(err, e1) {
				t.Error(err, e1)
			}
			if !errors.Is(err, e0) {
				t.Error(err, e0)
			}
			t.Log(err)
			errs := ers.Unwind(err)
			if len(errs) != 2 {
				t.Error(errs)
			}
		})
	})
	t.Run("IteratorHook", func(t *testing.T) {
		ec := &Collector{}
		count := 0
		op := fun.Producer[int](func(context.Context) (int, error) {
			ec.Add(ers.Error("hi"))
			if count >= 32 {
				return 0, io.EOF
			}
			count++
			return count, nil
		})

		iter := op.IteratorWithHook(IteratorHook[int](ec))
		assert.Equal(t, iter.Count(testt.Context(t)), 32)
		testt.Log(t, ers.Unwind(ec.Resolve()))
		assert.Equal(t, len(ers.Unwind(ec.Resolve())), 33)
		testt.Logf(t, "%T", iter.Close())
		testt.Log(t, ers.Unwind(iter.Close()))
		assert.Equal(t, len(ers.Unwind(iter.Close())), 33)
		assert.Equal(t, len(ers.Unwind(iter.Close())), 33)
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
				errs := ers.Unwind(err)
				if len(errs) != 1 {
					t.Error(errs, len(errs))
				}
			})
			t.Run("Many", func(t *testing.T) {
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant.IsTrue(len(ch) == 10)
				err := Stream(ctx, ch)
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := ers.Unwind(err)
				if len(errs) != 10 {
					t.Error(errs)
				}
			})
		})
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

func TestWithTime(t *testing.T) {
	const err ers.Error = ers.Error("ERRNO=42")
	t.Run("Nil", func(t *testing.T) {
		ec := &Collector{}
		WithTime(ec, nil)
		if ec.HasErrors() {
			t.Fatal(ec.Resolve())
		}
		if err := ec.Resolve(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("HasTimestamp", func(t *testing.T) {
		ec := &Collector{}
		now := time.Now()
		WithTime(ec, err)
		second := time.Now()
		if !ec.HasErrors() {
			t.Fatal("should have error", ec.Resolve())
		}
		if err := ec.Resolve(); err == nil {
			t.Fatal("should resolve error")
		}
		if errTime := ers.GetTime(ec.Resolve()); !now.Before(errTime) {
			t.Error(errTime, now)
		}
		if errTime := ers.GetTime(ec.Resolve()); !second.After(errTime) {
			t.Error(errTime, second)
		}
	})
}

func getPopulatedErrChan(size int) chan error {
	out := make(chan error, size)

	for i := 0; i < size; i++ {
		out <- fmt.Errorf("mock err %d", i)
	}
	return out
}
