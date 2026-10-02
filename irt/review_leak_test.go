package irt

import (
	"runtime"
	"testing"
	"time"
)

// goroutinesAtMost waits for the goroutine count to drop to at most base.
func goroutinesAtMost(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutine leak: have %d, want <= %d", runtime.NumGoroutine(), base)
}

func TestWithBufferLeakReview(t *testing.T) {
	base := runtime.NumGoroutine()
	for range 10 {
		for range WithBuffer(t.Context(), Monotonic(), 1) {
			break
		}
	}
	goroutinesAtMost(t, base)
}
