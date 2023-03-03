// Package testt (for test tools), provides a couple of useful helpers
// for common test patterns. To be used as a optional companion of the
// assert/check library.
package testt

import (
	"context"
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
	if t.Failed() {
		t.Log(args...)
	}
}

// Logf calls t.Log with the given arguments *if* the test has failed.
func Logf(t testing.TB, format string, args ...any) {
	t.Helper()
	if t.Failed() {
		t.Logf(format, args...)
	}
}
