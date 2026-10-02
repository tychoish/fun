// Package testt (for test tools), provides a couple of useful helpers
// for common test patterns. To be used as a optional companion of the
// assert/check library.
package testt

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// Context creates a context and attaches its cancellation function to
// the test execution's Cleanup. Given the execution of tests, this
// means that the context is canceled *after* the test functions
// defers have run.
func Context(t testing.TB) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

// ContextWithTimeout creates a context with the specified timeout,
// and attaches the cancellation to the test execution's cleanup.
func ContextWithTimeout(t testing.TB, dur time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	t.Cleanup(cancel)
	return ctx
}

// Timer creates a new time.Timer with the specified starting
// duration, and manages the cleanup of the timer.
func Timer(t testing.TB, dur time.Duration) *time.Timer {
	timer := time.NewTimer(dur)
	t.Cleanup(func() { timer.Stop() })
	return timer
}

// Ticker creates a new time.Ticker with the specified interval, and
// manages the cleanup of the ticker.
func Ticker(t testing.TB, dur time.Duration) *time.Ticker {
	ticker := time.NewTicker(dur)
	t.Cleanup(func() { ticker.Stop() })
	return ticker
}

// Log calls t.Log with the given arguments *if* the test has failed.
func Log(t testing.TB, args ...any) {
	t.Helper()
	t.Cleanup(func() {
		t.Helper()
		if t.Failed() {
			t.Log(args...)
		}
	})
}

// Logf calls t.Log with the given arguments *if* the test has failed.
func Logf(t testing.TB, format string, args ...any) {
	t.Helper()
	t.Cleanup(func() {
		t.Helper()
		if t.Failed() {
			t.Logf(format, args...)
		}
	})
}

// Must is used to capture the output of a function that returns an
// error and an arbitry value and simplify call sites in test
// code. The function that returns makes a fatal assertion if the
// error is non-nil, and returns the object.
func Must[T any](out T, err error) func(t testing.TB) T {
	var zero T
	return func(t testing.TB) T {
		t.Helper()
		if err != nil {
			out = zero // for testing
			t.Fatal("unexpected error", err)
		}
		return out
	}
}

// WithJustifiedParallelism marks the test as parallel by calling
// t.Parallel(), but only if the caller supplies a non-empty reason
// explaining why concurrency is worthwhile for this test.
//
// This checks reason only checked for presence: an empty string,
// fails the test immediately via t.Fatal and does not call
// t.Parallel.
func WithJustifiedParallelism(t *testing.T, reason string) {
	t.Helper()
	if reason == "" {
		t.Fatal("WithJustifiedParallelism requires a non-empty justification")
	}
	t.Parallel()
}

// GoroutinesAtMost fails the test (with Fatal) unless the number of
// running goroutines drops to limit or below within wait. Goroutines
// wind down asynchronously after a cancellation, so this polls until
// the deadline rather than checking once.
func GoroutinesAtMost(t testing.TB, limit int, wait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(wait)
	n := runtime.NumGoroutine()
	for n > limit && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		n = runtime.NumGoroutine()
	}
	if n > limit {
		t.Fatal("goroutine leak: have", n, "want at most", limit)
	}
}

// NoGoroutineLeak records the current number of goroutines and
// returns a function that checks, waiting up to wait, that the count
// has returned to that baseline. Use it as:
//
//	defer testt.NoGoroutineLeak(t, 5*time.Second)()
func NoGoroutineLeak(t testing.TB, wait time.Duration) func() {
	base := runtime.NumGoroutine()
	return func() {
		t.Helper()
		GoroutinesAtMost(t, base, wait)
	}
}
