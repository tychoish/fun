package erc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

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

	if actual := catcher.stack.Len(); actual != expectedNum {
		t.Error("should have expected number of errors", expectedNum, actual)
		t.Log(catcher.Resolve())
	}
	if catcher.OK() {
		t.Error("should have errors")
	}
	if catcher.Resolve() == nil {
		t.Error("should produce an error")
	}
	check.Equal(t, expectedNum, catcher.Len())
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

func collectIter[T any](ctx context.Context, t testing.TB, iter *fun.Iterator[T]) []T {
	t.Helper()

	out := []T{}
	for iter.Next(ctx) {
		out = append(out, iter.Value())
	}

	if err := iter.Close(); err != nil {
		t.Error(err)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

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
			catcher.Add(nil)
			catcherIsEmpty(t, catcher)
			var err error
			catcher.Add(err)
			catcherIsEmpty(t, catcher)
			check.Equal(t, 0, catcher.Len())
		})
		t.Run("SingleError", func(t *testing.T) {
			catcher := &Collector{}

			catcher.Add(errors.New(errval))
			catcherHasErrors(t, 1, catcher)

			err := catcher.Resolve()
			if err.Error() != errval {
				t.Error("unexpected error value:", err)
			}
		})
		t.Run("Check", func(t *testing.T) {
			catcher := &Collector{}
			serr := errors.New(errval)
			Check(catcher, func() error { return serr })
			catcherHasErrors(t, 1, catcher)
			err := catcher.Resolve()
			if !errors.Is(err, serr) {
				t.Error("errors is behaves unexpectedly")
			}

			if err.Error() != serr.Error() {
				t.Error("unexpected error from resolved catcher", err)
			}
		})
		t.Run("Producer", func(t *testing.T) {
			ec := &Collector{}
			for i := 0; i < 100; i++ {
				ec.Add(fmt.Errorf("%d", i))
			}
			errs := ec.Future().Resolve()
			if len(errs) != 100 {
				t.Log(errs, len(errs), ec.Len())
			}
		})
		t.Run("PanicRecovery", func(t *testing.T) {
			es := &Collector{}
			sig := make(chan struct{})
			go func() {
				defer close(sig)
				defer Recover(es)
				panic("boop")
			}()
			<-sig
			if err := es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}
			err := &es.stack
			assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
			assert.Substring(t, err.Error(), "boop")
		})

		t.Run("PanicRecoveryWithError", func(t *testing.T) {
			es := &Collector{}
			sig := make(chan struct{})
			err := errors.New("kip")
			go func() {
				defer close(sig)
				defer Recover(es)
				panic(err)
			}()
			<-sig
			if errr := es.Resolve(); errr == nil {
				t.Error("no panic recovered")
			}

			check.ErrorIs(t, es.Resolve(), fun.ErrRecoveredPanic)

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
				defer RecoverHook(es, func() { counter++ })
				panic("boop")
			}()
			<-sig
			if err := es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}
			if e := es.stack.Error(); e != "recovered panic: boop" {
				t.Error(e)
			}
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
				defer RecoverHook(es, func() { counter++ })
				panic(err)
			}()
			<-sig
			if err = es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}

			check.ErrorIs(t, es.Resolve(), fun.ErrRecoveredPanic)

			if counter != 1 {
				t.Error("callback not called")
			}
			if !errors.Is(es.Resolve(), err) {
				t.Error(es.Resolve(), "error not propogated")
			}
		})
		t.Run("Iterator", func(t *testing.T) {
			ctx := testt.Context(t)
			es := &Collector{}
			for i := 0; i < 10; i++ {
				es.Add(errors.New(errval))
			}
			if err := es.Resolve(); err == nil {
				t.Error("should have seen errors")
			}

			errs := collectIter(ctx, t, es.Iterator())
			if len(errs) != 10 {
				t.Error("iterator was incomplete", len(errs))
			}
		})
		t.Run("NilIterator", func(t *testing.T) {
			ctx := testt.Context(t)
			es := &Collector{}
			errs := collectIter(ctx, t, es.Iterator())
			assert.Zero(t, len(errs))
		})
		t.Run("ProducerCanceled", func(t *testing.T) {
			es := &Collector{}
			es.Add(errors.New("hello"))
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			prod := fun.CheckProducer(es.stack.CheckProducer())

			err, err2 := prod(ctx)
			assert.Error(t, err)
			assert.NotError(t, err2)

			err, err2 = prod(ctx)
			assert.NotError(t, err)
			assert.Error(t, err2)

			assert.ErrorIs(t, err2, context.Canceled)
		})
		t.Run("WhenBasicString", func(t *testing.T) {
			ec := &Collector{}
			When(ec, false, "no error")
			assert.True(t, ec.OK())
			assert.NotError(t, ec.Resolve())
			When(ec, true, errval)
			check.NotZero(t, ec.stack) // nil is zero
			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if err.Error() != errval {
				t.Fatal(err)
			}
		})
		t.Run("WhenBasicError", func(t *testing.T) {
			ec := &Collector{}
			When(ec, false, "no error")
			assert.True(t, ec.OK())
			assert.NotError(t, ec.Resolve())
			ex := errors.New(errval)
			When(ec, true, ex)
			check.NotZero(t, ec.stack) // nil is zero
			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if err.Error() != errval {
				t.Fatal(err)
			}
			assert.ErrorIs(t, ec.Resolve(), ex)
		})
		t.Run("WhenBasicWeirdType", func(t *testing.T) {
			ec := &Collector{}
			When(ec, false, 54)
			assert.True(t, !ec.HasErrors())
			assert.NotError(t, ec.Resolve())
			When(ec, true, 50000)
			check.NotZero(t, ec.stack) // nil is zero
			err := ec.Resolve()
			if err == nil {
				t.Fatal(err)
			}
			check.Substring(t, err.Error(), "50000")
			check.Substring(t, err.Error(), "int")
		})

		t.Run("WhenWrapping", func(t *testing.T) {
			serr := errors.New(errval)
			ec := &Collector{}
			Whenf(ec, false, "no error %w", serr)
			assert.True(t, !ec.HasErrors())
			if err := ec.Resolve(); err != nil {
				t.Fatal(err)
			}

			Whenf(ec, true, "no error: %w", serr)
			assert.True(t, ec.HasErrors())

			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if !errors.Is(err, serr) {
				t.Fatal(err)
			}
		})
		t.Run("ContextHelper", func(t *testing.T) {
			ec := &Collector{}
			ec.Add(errors.New("foo"))
			if ers.ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Add(errors.New("foo"))
			ec.Add(context.Canceled)
			if !ers.ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
			ec = &Collector{}
			ec.Add(errors.New("foo"))
			if ers.ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Add(context.DeadlineExceeded)
			ec.Add(errors.New("foo"))
			if !ers.ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
		})

		t.Run("Safe", func(t *testing.T) {
			ec := &Collector{}

			out := Safe(ec, func() string { panic("foo") })
			if !ec.HasErrors() {
				t.Fatal("empty collector")
			}
			err := ec.Resolve()
			assert.Error(t, err)
			assert.ErrorIs(t, err, fun.ErrRecoveredPanic)
			assert.Substring(t, err.Error(), "foo")

			if out != "" {
				t.Fatal("output:", out)
			}
		})
	})
	t.Run("ChaosEndToEnd", func(t *testing.T) {
		t.Parallel()
		startAt := time.Now()
		defer func() { t.Log(time.Since(startAt)) }()
		fixtureTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		wg := &fun.WaitGroup{}
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
						catcher.Add(errors.New(errval))
					}
				}
			}()
		}

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
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
						} else if _, ok := err.(*ers.Stack); !ok {
							t.Error("should be an error stack")
						}
					}
				}
			}(i)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		wg.Wait(ctx)
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
				ec.Add(fmt.Errorf("%d", i))
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
}

func BenchmarkErrorStack(b *testing.B) {
	const count int = 8

	b.Run("Reporting", func(b *testing.B) {
		b.Run("StackError", func(b *testing.B) {
			es := &ers.Stack{}
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
		b.Run("StackIterator", func(b *testing.B) {
			es := &ers.Stack{}
			for i := 0; i < count; i++ {
				es.Push(errors.New("foo"))
			}
			errs := collect(b, es.CheckProducer())
			if len(errs) != count {
				b.Fatal(len(errs))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				errs = collect(b, es.CheckProducer())
				if len(errs) == 0 {
					b.Fatal()
				}

			}
		})
		b.Run("Collector", func(b *testing.B) {
			ec := &Collector{}
			for i := 0; i < count; i++ {
				ec.Add(errors.New("foo"))
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
					ec.Add(err)
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
					ec.Add(err)

					str := ec.Resolve().Error()
					if str == "" {
						b.Fatal()
					}
				}
			})
		}
	})
}
