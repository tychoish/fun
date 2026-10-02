package irt

import (
	"errors"
	"iter"
	"runtime"
	"sync"
	"testing"
)

func panicAfter[T any](n int, v T, r any) iter.Seq[T] {
	return func(yield func(T) bool) {
		for range n {
			if !yield(v) {
				return
			}
		}
		panic(r)
	}
}

func TestAsChannelPanic(t *testing.T) {
	t.Run("ErrorValue", func(t *testing.T) {
		base := runtime.NumGoroutine()
		boom := errors.New("boom")
		ch, stop := AsChannel(t.Context(), panicAfter(2, 1, boom))
		n := 0
		for range ch {
			n++
		}
		if n != 2 {
			t.Fatalf("got %d items", n)
		}
		err := stop()
		var pe *PanicError
		if !errors.Is(err, boom) || !errors.As(err, &pe) || pe.Value != boom {
			t.Fatalf("unexpected error %v", err)
		}
		if err2 := stop(); err2 != err {
			t.Fatalf("second stop: %v", err2)
		}
		goroutinesAtMost(t, base)
	})
	t.Run("PlainValue", func(t *testing.T) {
		ch, stop := AsChannel(t.Context(), panicAfter(0, 1, "oops"))
		for range ch {
		}
		err := stop()
		var pe *PanicError
		if !errors.As(err, &pe) || pe.Value != "oops" || pe.Unwrap() != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if err.Error() != "recovered panic: oops" {
			t.Fatal(err.Error())
		}
	})
	t.Run("NoPanic", func(t *testing.T) {
		ch, stop := AsChannel(t.Context(), Slice([]int{1}))
		for range ch {
		}
		if err := stop(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("StopBeforePanic", func(t *testing.T) {
		release := make(chan struct{})
		seq := func(yield func(int) bool) { <-release; panic("late") }
		ch, stop := AsChannel(t.Context(), seq)
		if err := stop(); err != nil {
			t.Fatal(err)
		}
		close(release)
		for range ch {
		}
		if err := stop(); err == nil {
			t.Fatal("panic after stop not reported")
		}
	})
}

func TestAsGeneratorPanic(t *testing.T) {
	next := AsGenerator(panicAfter(1, 7, "oops"))
	if v, ok := next(t.Context()); !ok || v != 7 {
		t.Fatalf("got %d, %v", v, ok)
	}
	func() {
		defer func() {
			if r := recover(); r != "oops" {
				t.Fatalf("recovered %v", r)
			}
		}()
		next(t.Context())
		t.Fatal("no panic")
	}()
	if _, ok := next(t.Context()); ok {
		t.Fatal("yielded after panic")
	}
}

func TestAsPanicExtras(t *testing.T) {
	t.Run("GeneratorErrorValueIdentity", func(t *testing.T) {
		boom := errors.New("boom")
		next := AsGenerator(panicAfter(0, 1, boom))
		defer func() {
			if r := recover(); r != boom {
				t.Fatalf("recovered %v", r)
			}
		}()
		next(t.Context())
		t.Fatal("no panic")
	})
	t.Run("ConcurrentStopRace", func(t *testing.T) {
		base := runtime.NumGoroutine()
		ch, stop := AsChannel(t.Context(), panicAfter(0, 1, "x"))
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() { _ = stop() })
		}
		for range ch {
		}
		wg.Wait()
		if stop() == nil {
			t.Fatal("panic not reported")
		}
		goroutinesAtMost(t, base)
	})
}
