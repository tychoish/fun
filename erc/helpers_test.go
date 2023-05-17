package erc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
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
		t.Run("From", func(t *testing.T) {
			t.Run("Empty", func(t *testing.T) {
				ec := &Collector{}
				CollapseFrom(ec, nil)
				if err := ec.Resolve(); err != nil {
					t.Error("should be nil", err)
				}
				ec = &Collector{}
				CollapseFrom(ec, []error{})
				if err := ec.Resolve(); err != nil {
					t.Error("should be nil", err)
				}
			})
			t.Run("One", func(t *testing.T) {
				ec := &Collector{}
				const e ConstErr = "fourty-two"
				CollapseFrom(ec, []error{e})
				err := ec.Resolve()
				if !errors.Is(err, e) {
					t.Error(err, e)
				}
			})
			t.Run("Many", func(t *testing.T) {
				ec := &Collector{}
				const e0 ConstErr = "fourty-two"
				const e1 ConstErr = "fourty-three"
				CollapseFrom(ec, []error{e0, e1})
				err := ec.Resolve()
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
		t.Run("Into", func(t *testing.T) {
			t.Run("From", func(t *testing.T) {
				t.Run("Empty", func(t *testing.T) {
					ec := &Collector{}
					CollapseInto(ec, nil)
					if err := ec.Resolve(); err != nil {
						t.Error("should be nil", err)
					}
					ec = &Collector{}
					CollapseInto(ec)
					if err := ec.Resolve(); err != nil {
						t.Error("should be nil", err)
					}
				})
				t.Run("One", func(t *testing.T) {
					ec := &Collector{}
					const e ConstErr = "fourty-two"
					CollapseInto(ec, e)
					err := ec.Resolve()
					if !errors.Is(err, e) {
						t.Error(err, e)
					}
				})
				t.Run("Many", func(t *testing.T) {
					ec := &Collector{}
					const e0 ConstErr = "fourty-two"
					const e1 ConstErr = "fourty-three"
					CollapseInto(ec, e0, e1)
					err := ec.Resolve()
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
		})
		t.Run("Base", func(t *testing.T) {
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
		t.Run("WaitOne", func(t *testing.T) {
			ch := getPopulatedErrChan(1)
			close(ch)
			ec := &Collector{}
			StreamOne(ec, ch)(ctx)
			err := ec.Resolve()
			if err == nil {
				t.Logf("%T", err)
				t.Error("nil expected", err)
			}
			errs := Unwind(err)
			if len(errs) != 1 {
				t.Error(errs)
			}
		})
		t.Run("Into", func(t *testing.T) {
			t.Run("Many", func(t *testing.T) {
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				ec := &Collector{}
				StreamAll(ec, ch)(ctx)
				err := ec.Resolve()
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 10 {
					t.Error(errs)
				}
			})
			t.Run("CanceledWithContent", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				ec := &Collector{}
				StreamAll(ec, ch)(ctx)
				err := ec.Resolve()
				if err != nil {
					t.Error(err)
				}
				errs := Unwind(err)
				if len(errs) != 0 {
					t.Error("no errors expected with empty context")
				}
			})
			t.Run("Canceled", func(t *testing.T) {
				// the real test here is that it
				// doesn't deadlock
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ch := getPopulatedErrChan(0)
				ec := &Collector{}
				StreamAll(ec, ch)(ctx)
				err := ec.Resolve()
				if err != nil {
					t.Error(err)
				}
				errs := Unwind(err)
				if len(errs) != 0 {
					t.Error("no errors expected with empty context")
				}
			})
		})
		t.Run("Process", func(t *testing.T) {
			t.Run("Many", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				fun.Invariant(ctx.Err() == nil, ctx.Err())
				ec := &Collector{}
				wait := StreamProcess(ctx, ec, ch)
				wait(ctx)
				err := ec.Resolve()
				if err == nil {
					t.Logf("%T", err)
					t.Error("nil expected", err)
				}
				errs := Unwind(err)
				if len(errs) != 10 {
					t.Error(len(errs), errs, err)
				}
			})
			t.Run("Canceled", func(t *testing.T) {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				ch := getPopulatedErrChan(10)
				close(ch)
				fun.Invariant(len(ch) == 10)
				ec := &Collector{}
				wait := StreamProcess(ctx, ec, ch)
				wait(internal.BackgroundContext)
				err := ec.Resolve()
				if err != nil {
					t.Error(err)
				}
				errs := Unwind(err)
				if len(errs) != 0 {
					t.Error("no errors expected with empty context")
				}
			})

		})
	})
	t.Run("CheckWait", func(t *testing.T) {
		ec := &Collector{}
		wait := CheckWait(ec, func(ctx context.Context) error { return errors.New("hello") })
		assert.True(t, !ec.HasErrors())
		wait(testt.Context(t))
		assert.True(t, ec.HasErrors())
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
	t.Run("WithHelpers", func(t *testing.T) {
		t.Run("Safe", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := WithSafeCollector(func(_ context.Context, ec *Collector) error {
				panic(io.EOF)
			})(ctx)
			assert.Error(t, err)

			errs := Unwind(err)
			testt.Log(t, errs)

			assert.Equal(t, 2, len(errs))
			assert.ErrorIs(t, err, io.EOF)
			assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
		})
		t.Run("SafeExtra", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := WithSafeCollector(func(_ context.Context, ec *Collector) error {
				ec.Add(fun.ErrSkippedNonBlockingSend)
				panic(io.EOF)
			})(ctx)
			assert.Error(t, err)

			errs := Unwind(err)
			testt.Log(t, errs)

			assert.Equal(t, 3, len(errs))
			assert.ErrorIs(t, err, io.EOF)
			assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
			assert.ErrorIs(t, err, fun.ErrSkippedNonBlockingSend)
		})
		t.Run("Empty", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := WithCollector(func(_ context.Context, ec *Collector) error {
				return nil
			})(ctx)
			assert.NotError(t, err)
		})
		t.Run("NotCollected", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := WithCollector(func(_ context.Context, ec *Collector) error {
				return io.EOF
			})(ctx)
			assert.Error(t, err)

			errs := Unwind(err)
			testt.Log(t, errs)

			assert.Equal(t, 1, len(errs))
			assert.ErrorIs(t, err, io.EOF)
		})
		t.Run("NotCollected", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := WithCollector(func(_ context.Context, ec *Collector) error {
				ec.Add(fun.ErrSkippedNonBlockingSend)
				return io.EOF
			})(ctx)
			assert.Error(t, err)

			errs := Unwind(err)
			testt.Log(t, errs)

			assert.Equal(t, 2, len(errs))
			assert.ErrorIs(t, err, io.EOF)
			assert.ErrorIs(t, err, fun.ErrSkippedNonBlockingSend)
		})

	})
}

func getPopulatedErrChan(size int) chan error {
	out := make(chan error, size)

	for i := 0; i < size; i++ {
		out <- fmt.Errorf("mock err %d", i)
	}
	return out
}
