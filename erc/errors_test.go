package erc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
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
			ec1.Add(io.EOF, context.Canceled)
			ec2.Push(ec1)
			assert.Equal(t, 2, ec2.Len())
		})

		t.Run("Generator", func(t *testing.T) {
			ec := &Collector{}
			for i := 0; i < 100; i++ {
				ec.Push(fmt.Errorf("%d", i))
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
		t.Run("WhenBasicString", func(t *testing.T) {
			ec := &Collector{}
			ec.When(false, ers.Error("no error"))
			assert.True(t, ec.Ok())
			assert.NotError(t, ec.Resolve())
			ec.When(true, ers.Error(errval))
			check.NotZero(t, ec.list) // nil is zero
			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if err.Error() != errval {
				t.Fatal(err)
			}
		})
		t.Run("WhenWrapping", func(t *testing.T) {
			serr := errors.New(errval)
			ec := &Collector{}
			ec.Whenf(false, "no error %w", serr)
			assert.True(t, !ec.HasErrors())
			if err := ec.Resolve(); err != nil {
				t.Fatal(err)
			}

			ec.Whenf(true, "no error: %w", serr)
			assert.True(t, ec.HasErrors())

			if err := ec.Resolve(); err == nil {
				t.Fatal(err)
			} else if !errors.Is(err, serr) {
				t.Fatal(err)
			}
		})
		t.Run("ContextHelper", func(t *testing.T) {
			ec := &Collector{}
			ec.Push(errors.New("foo"))
			if ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Push(errors.New("foo"))
			ec.Push(context.Canceled)
			if !ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
			ec = &Collector{}
			ec.Push(errors.New("foo"))
			if ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}

			ec.Push(context.DeadlineExceeded)
			ec.Push(errors.New("foo"))
			if !ers.IsExpiredContext(ec.Resolve()) {
				t.Fatal(ec.Resolve())
			}
		})
		t.Run("RecoverCall", func(t *testing.T) {
			ec := &Collector{}

			ec.WithRecover(func() { panic("foo") })
			if !ec.HasErrors() {
				t.Fatal("empty collector")
			}
			err := ec.Resolve()
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.Substring(t, err.Error(), "foo")
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
						} else if _, ok := err.(*list); !ok {
							t.Error("should be an error stack")
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
	t.Run("LockingGenerator", func(t *testing.T) {
		ec := &Collector{}
		ec.Push(errors.New("ok,"))
		ec.Push(errors.New("this"))
		ec.Push(errors.New("is"))
		ec.Push(errors.New("fine."))
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
				ec.Push(errors.New("foo"))
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
