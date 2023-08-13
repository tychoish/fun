// Package testt (for test tools), provides a couple of useful helpers
// for common test patterns. To be used as a optional companion of the
// assert/check library.
package testt

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/dt"
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

// WithEnv sets an environment variable according to the key-value
// pair, and then and runs a function with that environment variable
// set. When WithEnv returns the state of the current process's
// environment.
//
// WithEnv will assert if there are any problems setting
// environment variables. Callers are responsible for managing
// concurrency between multiple
func WithEnv(t *testing.T, ev dt.Pair[string, string], op func()) {
	t.Helper()
	Logf(t, "key=%q value %q", ev.Key, ev.Value)

	prev, wasSet := os.LookupEnv(ev.Key)

	defer func() {

		if wasSet {
			assert.NotError(t, os.Setenv(ev.Key, prev))
			return
		}

		assert.NotError(t, os.Unsetenv(ev.Key))
	}()

	assert.NotError(t, os.Setenv(ev.Key, ev.Value))

	op()
}
