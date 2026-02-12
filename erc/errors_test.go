package erc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestError(t *testing.T) {
	t.Parallel()
	const errval = "ERRO=42"
	t.Run("Collector", func(t *testing.T) {
		t.Parallel()
		t.Run("InitialState", func(t *testing.T) {
			catcher := &Collector{}
			catcherIsEmpty(t, catcher)
			check.Equal(t, 0, catcher.Len())
		})
		t.Run("AddNilErrors", func(t *testing.T) {
			catcher := &Collector{}
			catcher.Push(nil)
			catcherIsEmpty(t, catcher)
			var err error
			catcher.Push(err)
			catcherIsEmpty(t, catcher)
			check.Equal(t, 0, catcher.Len())
		})
		t.Run("SingleError", func(t *testing.T) {
			catcher := &Collector{}

			catcher.Push(errors.New(errval))
			catcherHasErrors(t, 1, catcher)

			err := catcher.Resolve()
			if err.Error() != errval {
				t.Error("unexpected error value:", err)
			}
		})
		t.Run("AsCollectorList", func(t *testing.T) {
			lst := &list{}
			lst.Push(errors.New("foo"))
			es := AsCollector(lst)
			assert.Error(t, es.Err())
			assert.Equal(t, 1, es.Len())
			assert.Equal(t, "foo", es.Error())
		})
		t.Run("Check", func(t *testing.T) {
			catcher := &Collector{}
			serr := errors.New(errval)
			catcher.Check(func() error { return serr })
			catcherHasErrors(t, 1, catcher)
			err := catcher.Resolve()
			if !errors.Is(err, serr) {
				t.Error("errors is behaves unexpectedly")
			}

			if err.Error() != serr.Error() {
				t.Error("unexpected error from resolved catcher", err)
			}
		})
		t.Run("Nested", func(t *testing.T) {
			ec1 := &Collector{}
			ec2 := &Collector{}
			ec1.Join(io.EOF, context.Canceled)
			ec2.Push(ec1)
			assert.Equal(t, 1, ec2.Len())
			assert.Equal(t, 2, ec1.Len())
			check.ErrorIs(t, ec1, io.EOF)
			check.ErrorIs(t, ec2, io.EOF)
			check.ErrorIs(t, ec1, context.Canceled)
			check.ErrorIs(t, ec2, context.Canceled)
		})
		t.Run("Future", func(t *testing.T) {
			ec := &Collector{}
			for i := 0; i < 100; i++ {
				ec.Errorf("%d", i)
			}
			count := 0
			for err := range ec.Iterator() {
				assert.Error(t, err)
				count++
			}

			if count != 100 {
				t.Log(ec.Len(), ec.Resolve())
			}
		})
		t.Run("PanicRecovery", func(t *testing.T) {
			es := &Collector{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				defer es.Recover()
				panic("boop")
			}()
			<-sig
			if err := es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}
			err := &es.list
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.Substring(t, err.Error(), "boop")
		})
		t.Run("PanicRecoveryWithError", func(t *testing.T) {
			es := &Collector{}
			sig := make(chan struct{})
			err := errors.New("kip")
			go func() {
				defer close(sig)
				defer es.Recover()
				panic(err)
			}()
			<-sig
			if errr := es.Resolve(); errr == nil {
				t.Error("no panic recovered")
			}

			check.ErrorIs(t, es.Resolve(), ers.ErrRecoveredPanic)

			t.Log(es.Resolve())
			t.Log(ers.Unwind(es.Resolve()))
			if !errors.Is(es.Resolve(), err) {
				t.Error(es.Resolve(), "error not propogated")
			}
		})
		t.Run("PanicRecoveryCallback", func(t *testing.T) {
			es := &Collector{}
			sig := make(chan struct{})
			counter := 0
			go func() {
				defer close(sig)
				defer es.WithRecoverHook(func() { counter++ })
				panic("boop")
			}()
			<-sig
			if err := es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}

			assert.ErrorIs(t, es.Resolve(), ers.ErrRecoveredPanic)
			assert.Substring(t, es.Resolve().Error(), "boop")

			if counter != 1 {
				t.Error("callback not called")
			}
		})
		t.Run("PanicRecoveryCallback", func(t *testing.T) {
			es := &Collector{}
			sig := make(chan struct{})
			counter := 0
			err := errors.New("kip")
			go func() {
				defer close(sig)
				defer es.WithRecoverHook(func() { counter++ })
				panic(err)
			}()
			<-sig
			if err = es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}

			check.ErrorIs(t, es.Resolve(), ers.ErrRecoveredPanic)

			if counter != 1 {
				t.Error("callback not called")
			}
			if !errors.Is(es.Resolve(), err) {
				t.Error(es.Resolve(), "error not propogated")
			}
		})
		t.Run("IfBasicString", func(t *testing.T) {
			ec := &Collector{}
			ec.If(false, ers.Error("no error"))
			assert.True(t, ec.Ok())
			assert.NotError(t, ec.Resolve())
			ec.If(true, ers.Error(errval))
			check.NotZero(t, ec.list.elm) // nil is zero
			check.NotZero(t, ec.list.num) // nil is zero
			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if err.Error() != errval {
				t.Fatal(err)
			}
		})
		t.Run("WhenBasicString", func(t *testing.T) {
			ec := &Collector{}
			ec.When(false, "no error")
			assert.True(t, ec.Ok())
			assert.NotError(t, ec.Resolve())
			ec.When(true, errval)
			check.NotZero(t, ec.list.elm) // nil is zero
			check.NotZero(t, ec.list.num) // nil is zero
			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if err.Error() != errval {
				t.Fatal(err)
			}
		})
		t.Run("Filter", func(t *testing.T) {
			ec := &Collector{}
			count := 0
			ec.SetFilter(func(err error) error { count++; return err })
			ec.New("error")
			assert.Equal(t, count, 1)
		})
		t.Run("WhenWrapping", func(t *testing.T) {
			serr := errors.New(errval)
			ec := &Collector{}
			ec.Whenf(false, "no error %w", serr)
			assert.True(t, ec.Ok())
			if err := ec.Resolve(); err != nil {
				t.Fatal(err)
			}

			ec.Whenf(true, "no error: %w", serr)
			assert.True(t, !ec.Ok())

			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if !errors.Is(err, serr) {
				t.Fatal(err)
			}
		})
		t.Run("ContextHelper", func(t *testing.T) {
			ec := &Collector{}
			ec.New("foo")
			if ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.New("foo")
			ec.Push(context.Canceled)
			if !ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
			ec = &Collector{}
			ec.New("foo")
			if ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Push(context.DeadlineExceeded)
			ec.New("foo")
			if !ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
		})
		t.Run("RecoverCall", func(t *testing.T) {
			ec := &Collector{}

			ec.WithRecover(func() { panic("foo") })
			if ec.Ok() {
				t.Fatal("unexpectedly empty collector")
			}
			err := ec.Resolve()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.Substring(t, err.Error(), "foo")
		})
		t.Run("Wrap", func(t *testing.T) {
			ec := &Collector{}

			ec.Wrap(nil, "bar")
			assert.Equal(t, ec.Len(), 0)
			assert.True(t, ec.Ok())
			exp := errors.New("foo")
			ec.Wrap(exp, "bar")
			assert.True(t, !ec.Ok())
			assert.Equal(t, ec.Len(), 1)
			assert.Equal(t, "bar: foo", ec.Error())
			assert.NotEqual(t, ec.Err().Error(), exp.Error())
			t.Log("exp", exp.Error())
			t.Log("ec", ec.Error())
			assert.True(t, strings.HasSuffix(ec.Error(), exp.Error()))
			assert.ErrorIs(t, ec.Err(), exp)
		})
		t.Run("Wrapf", func(t *testing.T) {
			ec := &Collector{}

			ec.Wrapf(nil, "bar %d", 1)
			assert.Equal(t, ec.Len(), 0)
			assert.True(t, ec.Ok())
			exp := errors.New("foo")
			ec.Wrapf(exp, "bar %d", 2)
			assert.True(t, !ec.Ok())
			assert.Equal(t, ec.Len(), 1)
			assert.Equal(t, "bar 2: foo", ec.Error())
			assert.NotEqual(t, ec.Err().Error(), exp.Error())
			assert.True(t, strings.HasSuffix(ec.Error(), exp.Error()))
			assert.ErrorIs(t, ec.Err(), exp)
		})
		t.Run("Annotate", func(t *testing.T) {
			ec := &Collector{}

			ec.Annotate(nil, "bar")
			assert.Equal(t, ec.Len(), 0)
			assert.True(t, ec.Ok())
			exp := errors.New("foo")
			ec.Annotate(exp, "bar")
			assert.True(t, !ec.Ok())
			assert.Equal(t, ec.Len(), 1)
			assert.Equal(t, "foo [bar]", ec.Error())
			assert.NotEqual(t, ec.Err().Error(), exp.Error())
			t.Log("exp", exp.Error())
			t.Log("ec", ec.Error())
			assert.True(t, strings.HasPrefix(ec.Error(), exp.Error()))
			assert.ErrorIs(t, ec.Err(), exp)
		})
		t.Run("Annotatef", func(t *testing.T) {
			ec := &Collector{}

			ec.Annotatef(nil, "bar %d", 1)
			assert.Equal(t, ec.Len(), 0)
			assert.True(t, ec.Ok())
			exp := errors.New("foo")
			ec.Annotatef(exp, "bar %d", 2)
			assert.True(t, !ec.Ok())
			assert.Equal(t, ec.Len(), 1)
			assert.Equal(t, "foo [bar 2]", ec.Error())
			assert.NotEqual(t, ec.Err().Error(), exp.Error())
			assert.True(t, strings.HasPrefix(ec.Error(), exp.Error()))
			assert.ErrorIs(t, ec.Err(), exp)
		})
	})
	t.Run("ChaosEndToEnd", func(t *testing.T) {
		t.Parallel()
		startAt := time.Now()
		defer func() { t.Log(time.Since(startAt)) }()
		fixtureTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}
		catcher := &Collector{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-fixtureTimeout.Done():
						return
					case <-ticker.C:
						catcher.Push(errors.New(errval))
					}
				}
			}()
		}

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()
				ticker := time.NewTicker(5 * time.Millisecond)
				defer ticker.Stop()

				var count int

				for {
					count++

					select {
					case <-fixtureTimeout.Done():
						return
					case <-ticker.C:
						if err := catcher.Resolve(); err == nil {
							if count > 10 {
								t.Error("should have one by now")
							}
						} else if _, ok := err.(*Collector); !ok {
							t.Errorf("should be an error stack: %T", err)
						}
					}
				}
			}(i)
		}
		wg.Wait()
	})
	t.Run("Unwind", func(t *testing.T) {
		t.Run("NoErrors", func(t *testing.T) {
			errs := ers.Unwind(nil)
			if errs != nil {
				t.Fail()
			}
			if len(errs) != 0 {
				t.Fail()
			}
		})
		t.Run("OneError", func(t *testing.T) {
			err := errors.New("42")
			errs := ers.Unwind(err)
			if len(errs) != 1 {
				t.Fatal(len(errs))
			}
		})
		t.Run("Stack", func(t *testing.T) {
			ec := &Collector{}
			for i := 0; i < 100; i++ {
				ec.Push(fmt.Errorf("%d", i))
			}
			errs := ers.Unwind(ec.Resolve())
			if len(errs) != 100 {
				t.Fatal(len(errs))
			}
		})
		t.Run("Wrapped", func(t *testing.T) {
			err := errors.New("base")
			for i := 0; i < 100; i++ {
				err = fmt.Errorf("wrap %d: %w", i, err)
			}

			errs := ers.Unwind(err)
			if len(errs) != 101 {
				t.Error(len(errs))
			}
			if errs[100].Error() != "base" {
				t.Error(errs[100])
			}
		})
	})
	t.Run("LockingFuture", func(t *testing.T) {
		ec := &Collector{}
		ec.Push(errors.New("ok,"))
		ec.Push(errors.New("this"))
		ec.Push(errors.New("is"))
		ec.Push(errors.New("fine"))
		ec.Push(io.EOF)

		iterfunc := ec.Iterator()
		next, closer := iter.Pull(iterfunc)
		defer closer()

		err, ok := next()
		check.True(t, ok)
		check.ErrorIs(t, ec.Resolve(), io.EOF)
		check.Equal(t, "ok,", err.Error())

		ec.Push(io.EOF)

		check.Equal(t, 6, ec.Len())
	})
	t.Run("Empty", func(t *testing.T) {
		var ec Collector
		check.Equal(t, ec.Error(), "<nil>")
	})
	t.Run("Grouping", func(t *testing.T) {
		t.Run("JoinMethod", func(t *testing.T) {
			var ec Collector
			ec.Join(ers.New("one"), ers.New("two"))
			check.Equal(t, ec.Error(), "one\ntwo")
		})
		t.Run("JoinFunction", func(t *testing.T) {
			err := Join(ers.New("one"), ers.New("two"))
			check.Equal(t, err.Error(), "one\ntwo")
		})
		t.Run("Collect", func(t *testing.T) {
			var ec Collector
			ec.Push(Join(ers.New("one"), ers.New("two")))
			ec.Push(ers.New("three"))
			check.Equal(t, ec.Error(), "one; two\nthree")
		})
	})
}

func TestJoinSeq(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(error) bool) {}
		err := JoinSeq(seq)
		assert.NotError(t, err)
	})

	t.Run("AllNilErrors", func(t *testing.T) {
		seq := func(yield func(error) bool) {
			yield(nil)
			yield(nil)
			yield(nil)
		}
		err := JoinSeq(seq)
		assert.NotError(t, err)
	})

	t.Run("SingleError", func(t *testing.T) {
		expectedErr := errors.New("single error")
		seq := func(yield func(error) bool) {
			yield(expectedErr)
		}
		err := JoinSeq(seq)
		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "single error", err.Error())
	})

	t.Run("SingleErrorWithNils", func(t *testing.T) {
		expectedErr := errors.New("only error")
		seq := func(yield func(error) bool) {
			yield(nil)
			yield(expectedErr)
			yield(nil)
		}
		err := JoinSeq(seq)
		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "only error", err.Error())
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		err1 := errors.New("first")
		err2 := errors.New("second")
		err3 := errors.New("third")

		seq := func(yield func(error) bool) {
			yield(err1)
			yield(err2)
			yield(err3)
		}

		err := JoinSeq(seq)
		assert.Error(t, err)

		// Verify it's a Collector
		collector, ok := err.(*Collector)
		assert.True(t, ok)
		assert.Equal(t, 3, collector.Len())

		// Verify all errors are present
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
		assert.ErrorIs(t, err, err3)
	})

	t.Run("MultipleErrorsWithNils", func(t *testing.T) {
		err1 := io.EOF
		err2 := context.Canceled
		err3 := context.DeadlineExceeded

		seq := func(yield func(error) bool) {
			yield(nil)
			yield(err1)
			yield(nil)
			yield(err2)
			yield(nil)
			yield(err3)
			yield(nil)
		}

		err := JoinSeq(seq)
		assert.Error(t, err)

		collector, ok := err.(*Collector)
		assert.True(t, ok)
		assert.Equal(t, 3, collector.Len())

		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
		assert.ErrorIs(t, err, err3)
	})

	t.Run("ErrorsIsCompatibility", func(t *testing.T) {
		targetErr := io.EOF
		seq := func(yield func(error) bool) {
			yield(errors.New("first"))
			yield(targetErr)
			yield(errors.New("third"))
		}

		err := JoinSeq(seq)
		assert.True(t, errors.Is(err, targetErr))
	})

	t.Run("ErrorsAsCompatibility", func(t *testing.T) {
		customErr := &customError{msg: "custom"}
		seq := func(yield func(error) bool) {
			yield(errors.New("first"))
			yield(customErr)
			yield(errors.New("third"))
		}

		err := JoinSeq(seq)
		var target *customError
		assert.True(t, errors.As(err, &target))
		assert.Equal(t, "custom", target.msg)
	})
}

func TestCollectorFrom(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		ec := &Collector{}
		seq := func(yield func(error) bool) {}
		ec.From(seq)
		assert.True(t, ec.Ok())
		assert.Equal(t, 0, ec.Len())
		assert.NotError(t, ec.Resolve())
	})

	t.Run("AllNilErrors", func(t *testing.T) {
		ec := &Collector{}
		seq := func(yield func(error) bool) {
			yield(nil)
			yield(nil)
			yield(nil)
		}
		ec.From(seq)
		assert.True(t, ec.Ok())
		assert.Equal(t, 0, ec.Len())
		assert.NotError(t, ec.Resolve())
	})

	t.Run("SingleError", func(t *testing.T) {
		ec := &Collector{}
		expectedErr := errors.New("single error")
		seq := func(yield func(error) bool) {
			yield(expectedErr)
		}
		ec.From(seq)
		assert.True(t, !ec.Ok())
		assert.Equal(t, 1, ec.Len())
		assert.ErrorIs(t, ec.Resolve(), expectedErr)
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		ec := &Collector{}
		err1 := errors.New("first")
		err2 := errors.New("second")
		err3 := errors.New("third")

		seq := func(yield func(error) bool) {
			yield(err1)
			yield(err2)
			yield(err3)
		}

		ec.From(seq)
		assert.Equal(t, 3, ec.Len())
		assert.ErrorIs(t, ec.Resolve(), err1)
		assert.ErrorIs(t, ec.Resolve(), err2)
		assert.ErrorIs(t, ec.Resolve(), err3)
	})

	t.Run("FiltersNilErrors", func(t *testing.T) {
		ec := &Collector{}
		err1 := io.EOF
		err2 := context.Canceled

		seq := func(yield func(error) bool) {
			yield(nil)
			yield(err1)
			yield(nil)
			yield(nil)
			yield(err2)
			yield(nil)
		}

		ec.From(seq)
		assert.Equal(t, 2, ec.Len())
		assert.ErrorIs(t, ec.Resolve(), err1)
		assert.ErrorIs(t, ec.Resolve(), err2)
	})

	t.Run("AddsToExistingErrors", func(t *testing.T) {
		ec := &Collector{}
		ec.Push(errors.New("existing"))

		seq := func(yield func(error) bool) {
			yield(errors.New("new1"))
			yield(errors.New("new2"))
		}

		ec.From(seq)
		assert.Equal(t, 3, ec.Len())
	})

	t.Run("ThreadSafety", func(t *testing.T) {
		ec := &Collector{}
		wg := &sync.WaitGroup{}

		// Spawn multiple goroutines adding errors concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				seq := func(yield func(error) bool) {
					for j := 0; j < 10; j++ {
						yield(fmt.Errorf("error %d-%d", id, j))
					}
				}
				ec.From(seq)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 100, ec.Len())
	})

	t.Run("WithFilter", func(t *testing.T) {
		ec := &Collector{}
		filterCount := 0
		ec.SetFilter(func(err error) error {
			filterCount++
			// Filter out EOF errors
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		})

		seq := func(yield func(error) bool) {
			yield(io.EOF)
			yield(errors.New("keep1"))
			yield(io.EOF)
			yield(errors.New("keep2"))
			yield(nil)
			yield(nil)
			yield(nil)
			yield(nil)
		}

		ec.From(seq)
		// Should have 2 errors (the non-EOF ones)
		assert.Equal(t, 2, ec.Len())
		// Filter should have been called for all non-nil errors, in addition to the removed
		// ones
		assert.Equal(t, 4, filterCount)
	})
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func TestFromIterator(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {}
		values, err := FromIterator(seq)
		assert.NotError(t, err)
		assert.Equal(t, 0, len(values))
	})

	t.Run("AllSuccessfulValues", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
			yield(3, nil)
		}
		values, err := FromIterator(seq)
		assert.NotError(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 2, values[1])
		assert.Equal(t, 3, values[2])
	})

	t.Run("SomeErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(0, err1) // value with error is skipped
			yield(2, nil)
			yield(0, err2) // value with error is skipped
			yield(3, nil)
		}
		values, err := FromIterator(seq)
		// Should return non-nil slice with non-nil error
		assert.Error(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 2, values[1])
		assert.Equal(t, 3, values[2])

		// Verify both errors are aggregated
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
	})

	t.Run("AllErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		err3 := errors.New("error3")
		seq := func(yield func(int, error) bool) {
			yield(0, err1)
			yield(0, err2)
			yield(0, err3)
		}
		values, err := FromIterator(seq)
		// Should return non-nil error with empty slice
		assert.Error(t, err)
		assert.Equal(t, 0, len(values))

		// Verify all errors are aggregated
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
		assert.ErrorIs(t, err, err3)

		// Verify it's a Collector with 3 errors
		collector, ok := err.(*Collector)
		assert.True(t, ok)
		assert.Equal(t, 3, collector.Len())
	})

	t.Run("SingleError", func(t *testing.T) {
		expectedErr := io.EOF
		seq := func(yield func(string, error) bool) {
			yield("first", nil)
			yield("", expectedErr)
			yield("second", nil)
		}
		values, err := FromIterator(seq)
		assert.Error(t, err)
		assert.Equal(t, 2, len(values))
		assert.Equal(t, "first", values[0])
		assert.Equal(t, "second", values[1])
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("NonNilSliceWithError", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			yield(42, nil)
			yield(0, errors.New("error"))
		}
		values, err := FromIterator(seq)
		// Both slice and error should be non-nil
		assert.Error(t, err)
		assert.True(t, values != nil)
		assert.Equal(t, 1, len(values))
		assert.Equal(t, 42, values[0])
	})
}

func TestFromIteratorAll(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {}
		values, err := FromIteratorAll(seq)
		assert.NotError(t, err)
		assert.Equal(t, 0, len(values))
	})

	t.Run("AllSuccessfulValues", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
			yield(3, nil)
		}
		values, err := FromIteratorAll(seq)
		assert.NotError(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 2, values[1])
		assert.Equal(t, 3, values[2])
	})

	t.Run("SomeErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(10, err1) // value included even with error
			yield(2, nil)
			yield(20, err2) // value included even with error
			yield(3, nil)
		}
		values, err := FromIteratorAll(seq)
		// Should return non-nil slice with ALL values and non-nil error
		assert.Error(t, err)
		assert.Equal(t, 5, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 10, values[1])
		assert.Equal(t, 2, values[2])
		assert.Equal(t, 20, values[3])
		assert.Equal(t, 3, values[4])

		// Verify both errors are aggregated
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
	})

	t.Run("AllErrors", func(t *testing.T) {
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		err3 := errors.New("error3")
		seq := func(yield func(int, error) bool) {
			yield(100, err1)
			yield(200, err2)
			yield(300, err3)
		}
		values, err := FromIteratorAll(seq)
		// Should return non-nil slice with ALL values and aggregated errors
		assert.Error(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 100, values[0])
		assert.Equal(t, 200, values[1])
		assert.Equal(t, 300, values[2])

		// Verify all errors are aggregated
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
		assert.ErrorIs(t, err, err3)

		// Verify it's a Collector with 3 errors
		collector, ok := err.(*Collector)
		assert.True(t, ok)
		assert.Equal(t, 3, collector.Len())
	})

	t.Run("SingleError", func(t *testing.T) {
		expectedErr := context.Canceled
		seq := func(yield func(string, error) bool) {
			yield("first", nil)
			yield("with-error", expectedErr)
			yield("second", nil)
		}
		values, err := FromIteratorAll(seq)
		assert.Error(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, "first", values[0])
		assert.Equal(t, "with-error", values[1])
		assert.Equal(t, "second", values[2])
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("NonNilSliceWithError", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			yield(42, nil)
			yield(99, errors.New("error"))
		}
		values, err := FromIteratorAll(seq)
		// Both slice and error should be non-nil
		assert.Error(t, err)
		assert.True(t, values != nil)
		assert.Equal(t, 2, len(values))
		assert.Equal(t, 42, values[0])
		assert.Equal(t, 99, values[1])
	})

	t.Run("DifferenceFromFromIterator", func(t *testing.T) {
		// Demonstrate the key difference: FromIteratorAll includes ALL values
		err1 := errors.New("skip-me")

		seq1 := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(999, err1) // This value is skipped in FromIterator
			yield(2, nil)
		}
		values1, _ := FromIterator(seq1)
		assert.Equal(t, 2, len(values1))
		assert.Equal(t, 1, values1[0])
		assert.Equal(t, 2, values1[1])

		seq2 := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(999, err1) // This value is included in FromIteratorAll
			yield(2, nil)
		}
		values2, _ := FromIteratorAll(seq2)
		assert.Equal(t, 3, len(values2))
		assert.Equal(t, 1, values2[0])
		assert.Equal(t, 999, values2[1])
		assert.Equal(t, 2, values2[2])
	})
}

func TestFromIteratorUntil(t *testing.T) {
	t.Run("EmptySequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {}
		values, err := FromIteratorUntil(seq)
		assert.NotError(t, err)
		assert.Equal(t, 0, len(values))
	})

	t.Run("AllSuccessfulValues", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
			yield(3, nil)
		}
		values, err := FromIteratorUntil(seq)
		assert.NotError(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 2, values[1])
		assert.Equal(t, 3, values[2])
	})

	t.Run("ErrorInMiddle", func(t *testing.T) {
		err1 := errors.New("first-error")
		err2 := errors.New("second-error")
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(2, nil) {
				return
			}
			if !yield(0, err1) { // stops here
				return
			}
			if !yield(3, nil) { // never reached
				return
			}
			yield(0, err2) // never reached
		}
		values, err := FromIteratorUntil(seq)
		// Should return first error directly (not aggregated)
		assert.Error(t, err)
		assert.Equal(t, 2, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 2, values[1])

		// Verify it returns the FIRST error directly (not a Collector)
		assert.ErrorIs(t, err, err1)
		_, isCollector := err.(*Collector)
		assert.True(t, !isCollector) // NOT a collector

		// Second error should NOT be present
		check.True(t, !errors.Is(err, err2))
	})

	t.Run("ErrorAtStart", func(t *testing.T) {
		expectedErr := io.EOF
		seq := func(yield func(string, error) bool) {
			if !yield("", expectedErr) { // immediate error
				return
			}
			yield("never", nil) // never reached
		}
		values, err := FromIteratorUntil(seq)
		assert.Error(t, err)
		assert.Equal(t, 0, len(values))
		assert.ErrorIs(t, err, expectedErr)

		// Verify it's the original error, not wrapped
		assert.Equal(t, expectedErr, err)
	})

	t.Run("ErrorAtEnd", func(t *testing.T) {
		expectedErr := context.DeadlineExceeded
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
			yield(3, nil)
			yield(0, expectedErr)
		}
		values, err := FromIteratorUntil(seq)
		assert.Error(t, err)
		assert.Equal(t, 3, len(values))
		assert.Equal(t, 1, values[0])
		assert.Equal(t, 2, values[1])
		assert.Equal(t, 3, values[2])
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("NoAggregation", func(t *testing.T) {
		// Verify that errors are NOT aggregated (fail-fast behavior)
		err1 := errors.New("error1")
		err2 := errors.New("error2")
		seq := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(0, err1) {
				return
			}
			yield(0, err2)
		}
		values, err := FromIteratorUntil(seq)
		assert.Error(t, err)
		assert.Equal(t, 1, len(values))

		// Only first error should be present
		assert.ErrorIs(t, err, err1)
		check.True(t, !errors.Is(err, err2))

		// Should be the raw error, not a Collector
		assert.Equal(t, err1, err)
	})

	t.Run("DifferenceFromOtherFunctions", func(t *testing.T) {
		// Demonstrate fail-fast vs continue-on-error behavior
		err1 := errors.New("stop")

		// FromIteratorUntil stops immediately
		seq1 := func(yield func(int, error) bool) {
			if !yield(1, nil) {
				return
			}
			if !yield(0, err1) {
				return
			}
			if !yield(2, nil) {
				return
			}
			yield(3, nil)
		}
		valuesUntil, errUntil := FromIteratorUntil(seq1)
		assert.Equal(t, 1, len(valuesUntil))
		assert.Equal(t, err1, errUntil)

		// FromIterator continues and skips error values
		seq2 := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(0, err1)
			yield(2, nil)
			yield(3, nil)
		}
		valuesSkip, errSkip := FromIterator(seq2)
		assert.Equal(t, 3, len(valuesSkip))
		assert.ErrorIs(t, errSkip, err1)

		// FromIteratorAll continues and includes all values
		seq3 := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(0, err1)
			yield(2, nil)
			yield(3, nil)
		}
		valuesAll, errAll := FromIteratorAll(seq3)
		assert.Equal(t, 4, len(valuesAll))
		assert.ErrorIs(t, errAll, err1)
	})
}

func BenchmarkErrorList(b *testing.B) {
	const count int = 8

	b.Run("Reporting", func(b *testing.B) {
		b.Run("ListError", func(b *testing.B) {
			es := &list{}
			for i := 0; i < count; i++ {
				es.Push(errors.New("foo"))
			}
			err := es.Error()
			if err == "" {
				b.Fatal()
			}

			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				if err = es.Error(); err == "" {
					b.Fatal()
				}
			}
		})
		b.Run("Collector", func(b *testing.B) {
			ec := &Collector{}
			for i := 0; i < count; i++ {
				ec.New("foo")
			}
			err := ec.Resolve().Error()
			if err == "" {
				b.Fatal()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				if err = ec.Resolve().Error(); err == "" {
					b.Fatal()
				}
			}
		})
	})
	b.Run("Collection", func(b *testing.B) {
		for i := 2; i < 512; i *= 2 {
			b.Run(fmt.Sprint(i), func(b *testing.B) {
				err := errors.New("foo")
				for i := 0; i < b.N; i++ {
					ec := &Collector{}
					ec.Push(err)
				}
			})
		}
	})
	b.Run("RoundTrip", func(b *testing.B) {
		for i := 2; i < 512; i *= 2 {
			b.Run(fmt.Sprint(i), func(b *testing.B) {
				err := errors.New("foo")
				for i := 0; i < b.N; i++ {
					ec := &Collector{}
					ec.Push(err)

					str := ec.Resolve().Error()
					if str == "" {
						b.Fatal()
					}
				}
			})
		}
	})
}
