package fun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func catcherIsEmpty(t *testing.T, catcher *ErrorCollector) {
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

func catcherHasErrors(t *testing.T, expectedNum int, catcher *ErrorCollector) {
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

func TestError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const errval = "ERRO=42"
	t.Run("Collector", func(t *testing.T) {
		t.Parallel()
		t.Run("InitialState", func(t *testing.T) {
			catcher := &ErrorCollector{}
			catcherIsEmpty(t, catcher)
		})
		t.Run("AddNilErrors", func(t *testing.T) {
			catcher := &ErrorCollector{}
			catcher.Add(nil)
			catcherIsEmpty(t, catcher)
			var err error
			catcher.Add(err)
			catcherIsEmpty(t, catcher)
		})
		t.Run("SingleError", func(t *testing.T) {
			catcher := &ErrorCollector{}

			catcher.Add(errors.New(errval))
			catcherHasErrors(t, 1, catcher)

			err := catcher.Resolve()
			if err.Error() != errval {
				t.Error("unexpected error value:", err)
			}
		})
		t.Run("ErrorsAreCached", func(t *testing.T) {
			catcher := &ErrorCollector{}

			catcher.Add(errors.New(errval))
			catcherHasErrors(t, 1, catcher)

			err := catcher.Resolve()
			err2 := catcher.Resolve()
			if err.Error() != err2.Error() {
				t.Error("unexpected error value:", err)
			}
			if err.Error() != err2.Error() {
				t.Error("should be different objects")
			}
		})
		t.Run("CacheRefreshesAsNeeded", func(t *testing.T) {
			catcher := &ErrorCollector{}
			catcher.Add(errors.New(errval))
			catcherHasErrors(t, 1, catcher)
			err := catcher.Resolve()
			catcher.Add(errors.New(errval))
			catcherHasErrors(t, 2, catcher)

			err2 := catcher.Resolve()
			if !strings.Contains(err2.Error(), err.Error()) {
				t.Error("errors should remain")
			}
		})
		t.Run("Check", func(t *testing.T) {
			catcher := &ErrorCollector{}
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
		t.Run("CheckCtx", func(t *testing.T) {
			catcher := &ErrorCollector{}
			serr := errors.New(errval)
			catcher.CheckCtx(ctx, func(_ context.Context) error { return serr })
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
			es := &ErrorCollector{}
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
			if e := es.stack.Error(); e != "panic: boop" {
				t.Error(e)
			}
		})
	})
	t.Run("Stack", func(t *testing.T) {
		t.Parallel()
		t.Run("Nil", func(t *testing.T) {
			var es *ErrorStack
			if es.len() != 0 {
				t.Fatal("defensive nil for length")
			}
			if es.append(nil) != nil {
				t.Fatal("append nil errors should always be safe")
			}
			if err := es.append(&ErrorStack{}); err == nil {
				t.Error("nil should append to something")
			}

		})
		t.Run("UnwrapNil", func(t *testing.T) {
			es := &ErrorStack{}
			if err := es.Unwrap(); err != nil {
				t.Fatal("unexpected unwrap empty", err)
			}
		})
		t.Run("ErrorsReportEmpty", func(t *testing.T) {
			es := &ErrorStack{}
			if l := es.Errors(); len(l) != 0 || l != nil {
				t.Fatal("unexpected errors report", l)
			}
			if es.len() != 0 {
				t.Fatal("unexpected empty length", es.len())
			}
		})
		t.Run("ErrorsReportSingle", func(t *testing.T) {
			es := &ErrorStack{}
			es = es.append(errors.New(errval))
			if l := es.Errors(); len(l) != 1 || l == nil {
				t.Fatal("unexpected errors report", l)
			}
		})
		t.Run("StackErrorStack", func(t *testing.T) {
			es := &ErrorStack{err: errors.New("outer")}
			es = es.append(&ErrorStack{err: errors.New("inner")})
			if l := es.Errors(); len(l) != 2 || l == nil {
				t.Fatal("unexpected errors report", l)
			}
		})
		t.Run("NilErrorStillErrors", func(t *testing.T) {
			es := &ErrorStack{}
			if e := es.Error(); e == "" {
				t.Error("every non-nil error stack should have an error")
			}
		})
		t.Run("CacheCorrectness", func(t *testing.T) {
			es := &ErrorStack{}
			es = es.append(errors.New(errval))
			er1 := es.Error()
			es = es.append(errors.New(errval))
			er2 := es.Error()
			if er1 == er2 {
				t.Error("errors should be different", er1, er2)
			}
		})
		t.Run("Merge", func(t *testing.T) {
			es1 := &ErrorStack{}
			es1 = es1.append(errors.New(errval))
			es1 = es1.append(errors.New(errval))
			if l := es1.len(); l != 2 {
				t.Fatal("es1 unexpected length", l)
			}

			es2 := &ErrorStack{}
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
			es := &ErrorStack{}
			es = es.append(err)
			if l := es.len(); l != 2 {
				t.Fatalf("%d, %+v", l, es)
			}
		})
		t.Run("Is", func(t *testing.T) {
			err1 := errors.New("foo")
			err2 := errors.New("bar")

			es := &ErrorStack{}
			es = es.append(err1)
			es = es.append(err2)
			if !errors.Is(es, err1) {
				t.Fatal("expected is to find wrapped err")
			}
		})
		t.Run("OutputOrderedLogically", func(t *testing.T) {
			es := &ErrorStack{}
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
		fixtureTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		wg := &sync.WaitGroup{}
		catcher := &ErrorCollector{}
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
				ticker := time.NewTicker(2 * time.Millisecond)
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
						} else if _, ok := err.(*ErrorStack); !ok {
							t.Error("should be an error stack")
						}
					}
				}
			}(i)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		Wait(ctx, wg)
	})
}

func BenchmarkErrorStack(b *testing.B) {
	const count int = 4

	b.Run("BuildString", func(b *testing.B) {
		es := &ErrorStack{}
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
			err = es.Error()
		}
	})

	b.Run("Slice", func(b *testing.B) {
		es := &ErrorStack{}
		for i := 0; i < count; i++ {
			es = es.append(errors.New("foo"))
		}
		errs := es.Errors()
		if len(errs) != count {
			b.Fatal(len(errs))
		}
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			errs = es.Errors()
		}
	})
}
