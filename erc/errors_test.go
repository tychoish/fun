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
)

func (e *Stack) len() int {
	if e == nil || e.err == nil {
		return 0
	}

	var out int = 1
	for item := e.next; item != nil; item = item.next {
		if item.err != nil {
			out++
		}
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
}

func catcherHasErrors(t *testing.T, expectedNum int, catcher *Collector) {
	t.Helper()

	if catcher == nil || expectedNum <= 0 {
		t.Fatal("test issue", catcher, expectedNum)
	}

	if actual := catcher.stack.len(); actual != expectedNum {
		t.Error("should have expected number of errors", expectedNum, actual)
		t.Log(catcher.Resolve())
	}
	if !catcher.HasErrors() {
		t.Error("should have errors")
	}
	if catcher.Resolve() == nil {
		t.Error("should produce an error")
	}
}

func collectIter[T any](ctx context.Context, t testing.TB, iter fun.Iterator[T]) []T {
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	const errval = "ERRO=42"
	t.Run("Collector", func(t *testing.T) {
		t.Parallel()
		t.Run("InitialState", func(t *testing.T) {
			catcher := &Collector{}
			catcherIsEmpty(t, catcher)
		})
		t.Run("AddNilErrors", func(t *testing.T) {
			catcher := &Collector{}
			catcher.Add(nil)
			catcherIsEmpty(t, catcher)
			var err error
			catcher.Add(err)
			catcherIsEmpty(t, catcher)
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
		t.Run("CheckCtx", func(t *testing.T) {
			catcher := &Collector{}
			serr := errors.New(errval)
			CheckCtx(ctx, catcher, func(_ context.Context) error { return serr })
			catcherHasErrors(t, 1, catcher)
			err := catcher.Resolve()
			if !errors.Is(err, serr) {
				t.Error("errors is behaves unexpectedly")
			}

			if err.Error() != serr.Error() {
				t.Error("unexpected error from resolved catcher", err)
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
			if e := es.stack.Error(); e != "panic: boop" {
				t.Error(e)
			}
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
			if e := es.stack.Error(); e != "panic: kip" {
				t.Error(e)
			}
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
			if e := es.stack.Error(); e != "panic: boop" {
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
			if err := es.Resolve(); err == nil {
				t.Error("no panic recovered")
			}
			if e := es.stack.Error(); e != "panic: kip" {
				t.Error(e)
			}
			if counter != 1 {
				t.Error("callback not called")
			}
			if !errors.Is(es.Resolve(), err) {
				t.Error(es.Resolve(), "error not propogated")
			}
		})
		t.Run("Iterator", func(t *testing.T) {
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
			es := &Collector{}
			errs := collectIter(ctx, t, es.Iterator())
			assert.Zero(t, len(errs))
		})
		t.Run("WhenBasicString", func(t *testing.T) {
			ec := &Collector{}
			When(ec, false, "no error")
			if ec.stack != nil {
				t.Error("should not error")
			}
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
			if ec.stack != nil {
				t.Error("should not error")
			}
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
			if ec.stack != nil {
				t.Error("should not error")
			}
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
			if ec.stack != nil {
				t.Error("should not error")
			}
			if err := ec.Resolve(); err != nil {
				t.Fatal(err)
			}

			Whenf(ec, true, "no error: %w", serr)
			if ec.stack == nil {
				t.Error("should not error")
			}
			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if !errors.Is(err, serr) {
				t.Fatal(err)
			}
		})
		t.Run("ContextHelper", func(t *testing.T) {
			ec := &Collector{}
			ec.Add(errors.New("foo"))
			if ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Add(context.Canceled)
			ec.Add(errors.New("foo"))
			if !ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
			ec = &Collector{}
			ec.Add(errors.New("foo"))
			if ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Add(context.DeadlineExceeded)
			ec.Add(errors.New("foo"))
			if !ContextExpired(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
		})
		t.Run("Safe", func(t *testing.T) {
			ec := &Collector{}

			out := Safe(ec, func() string { panic("foo") })
			if !ec.HasErrors() {
				t.Fatal("empty collector")
			}
			if ec.Resolve().Error() != "panic: foo" {
				t.Fatal(ec.Resolve())
			}

			if out != "" {
				t.Fatal("output:", out)
			}
		})
	})
	t.Run("Stack", func(t *testing.T) {
		t.Parallel()
		t.Run("Nil", func(t *testing.T) {
			var es *Stack
			if es.len() != 0 {
				t.Fatal("defensive nil for length")
			}
			if es.append(nil) != nil {
				t.Fatal("append nil errors should always be safe")
			}
			if err := es.append(&Stack{}); err == nil {
				t.Error("nil should append to something")
			}

		})
		t.Run("UnwrapNil", func(t *testing.T) {
			es := &Stack{}
			if err := es.Unwrap(); err != nil {
				t.Fatal("unexpected unwrap empty", err)
			}
		})
		t.Run("ErrorsReportEmpty", func(t *testing.T) {
			es := &Stack{}
			if l := collectIter(ctx, t, es.Iterator()); len(l) != 0 || l != nil {
				t.Fatal("unexpected errors report", l)
			}
			if es.len() != 0 {
				t.Fatal("unexpected empty length", es.len())
			}
		})
		t.Run("ErrorsReportSingle", func(t *testing.T) {
			es := &Stack{}
			es = es.append(errors.New(errval))
			if l := collectIter(ctx, t, es.Iterator()); len(l) != 1 || l == nil {
				t.Fatal("unexpected errors report", l)
			}
		})
		t.Run("StackErrorStack", func(t *testing.T) {
			es := &Stack{err: errors.New("outer")}
			es = es.append(&Stack{err: errors.New("inner")})
			if l := collectIter(ctx, t, es.Iterator()); len(l) != 2 || l == nil {
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
			es = es.append(errors.New(errval))
			er1 := es.Error()
			es = es.append(errors.New(errval))
			er2 := es.Error()
			if er1 == er2 {
				t.Error("errors should be different", er1, er2)
			}
		})
		t.Run("Merge", func(t *testing.T) {
			es1 := &Stack{}
			es1 = es1.append(errors.New(errval))
			es1 = es1.append(errors.New(errval))
			if l := es1.len(); l != 2 {
				t.Fatal("es1 unexpected length", l)
			}

			es2 := &Stack{}
			es2 = es2.append(errors.New(errval))
			es2 = es2.append(errors.New(errval))

			if l := es2.len(); l != 2 {
				t.Fatal("es2 unexpected length", l)
			}

			es1 = es1.append(es2)
			if l := es1.len(); l != 4 {
				t.Fatal("merged unexpected length", l)
			}
		})
		t.Run("ConventionalWrap", func(t *testing.T) {
			err := fmt.Errorf("foo: %w", errors.New("bar"))
			es := &Stack{}
			es = es.append(err)
			if l := es.len(); l != 1 {
				t.Fatalf("%d, %+v", l, es)
			}
		})
		t.Run("Is", func(t *testing.T) {
			err1 := errors.New("foo")
			err2 := errors.New("bar")

			es := &Stack{}
			es = es.append(err1)
			es = es.append(err2)
			if !errors.Is(es, err1) {
				t.Fatal("expected is to find wrapped err")
			}
		})
		t.Run("OutputOrderedLogically", func(t *testing.T) {
			es := &Stack{}
			es = es.append(errors.New("one"))
			es = es.append(errors.New("two"))
			es = es.append(errors.New("three"))

			output := es.Error()
			const expected = "one; two; three"
			if output != expected {
				t.Error(output, "!=", expected)
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
						} else if _, ok := err.(*Stack); !ok {
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
			errs := Unwind(nil)
			if errs != nil {
				t.Fail()
			}
			if len(errs) != 0 {
				t.Fail()
			}
		})
		t.Run("OneError", func(t *testing.T) {
			err := errors.New("42")
			errs := Unwind(err)
			if len(errs) != 1 {
				t.Fatal(len(errs))
			}
		})
		t.Run("Stack", func(t *testing.T) {
			ec := &Collector{}
			for i := 0; i < 100; i++ {
				ec.Add(fmt.Errorf("%d", i))
			}
			errs := Unwind(ec.Resolve())
			if len(errs) != 100 {
				t.Fatal(len(errs))
			}
		})
		t.Run("Wrapped", func(t *testing.T) {
			err := errors.New("base")
			for i := 0; i < 100; i++ {
				err = fmt.Errorf("wrap %d: %w", i, err)
			}

			errs := Unwind(err)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const count int = 8

	b.Run("Reporting", func(b *testing.B) {
		b.Run("StackError", func(b *testing.B) {
			es := &Stack{}
			for i := 0; i < count; i++ {
				es = es.append(errors.New("foo"))
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
			es := &Stack{}
			for i := 0; i < count; i++ {
				es = es.append(errors.New("foo"))
			}
			errs := collectIter(ctx, b, es.Iterator())
			if len(errs) != count {
				b.Fatal(len(errs))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				errs = collectIter(ctx, b, es.Iterator())
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
