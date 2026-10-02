package testt

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"
	"time"
)

type mockTB struct {
	*testing.T

	cleanup     []func()
	shouldFail  bool
	fatalCalled bool
	logs        []string
}

func newMock() *mockTB                    { return &mockTB{T: &testing.T{}} }
func (m *mockTB) Cleanup(fn func())       { m.cleanup = append(m.cleanup, fn) }
func (m *mockTB) Failed() bool            { return m.shouldFail }
func (m *mockTB) Log(args ...any)         { m.logs = append(m.logs, fmt.Sprint(args...)) }
func (m *mockTB) Logf(s string, a ...any) { m.logs = append(m.logs, fmt.Sprintf(s, a...)) }
func (m *mockTB) Fatal(_ ...any)          { m.fatalCalled = true }
func TestTools(t *testing.T) {
	t.Run("Mock", func(t *testing.T) {
		t.Run("Context", func(t *testing.T) {
			mock := newMock()
			ctx := Context(mock)
			if ctx.Err() != nil {
				t.Fatal("context should not be canceled")
			}

			if len(mock.cleanup) != 1 {
				t.Error("should have one cleanup function registered")
			}
		})
		t.Run("ContextWithTimeout", func(t *testing.T) {
			mock := newMock()

			ctx := ContextWithTimeout(mock, 2*time.Millisecond)
			if ctx.Err() != nil {
				t.Fatal("context should not be canceled")
			}
			time.Sleep(10 * time.Millisecond)
			runtime.Gosched()
			if ctx.Err() == nil {
				t.Fatal("context should be canceled")
			}
			if len(mock.cleanup) != 1 {
				t.Error("should have one cleanup function registered")
			}
		})
		t.Run("Timer", func(t *testing.T) {
			t.Parallel()
			mock := newMock()
			start := time.Now()
			timer := Timer(mock, 5*time.Millisecond)
			runtime.Gosched()
			<-timer.C
			dur := time.Since(start)
			if dur < 5*time.Millisecond || dur > 10*time.Millisecond {
				t.Error(dur)
			}
			if len(mock.cleanup) != 1 {
				t.Error("should have one cleanup function registered")
			}
		})
		t.Run("Ticker", func(t *testing.T) {
			t.Parallel()
			mock := newMock()
			start := time.Now()
			ticker := Ticker(mock, 20*time.Millisecond)
			runtime.Gosched()
			<-ticker.C
			dur := time.Since(start)
			if dur < 20*time.Millisecond || dur > 40*time.Millisecond {
				t.Error(dur)
			}
			if len(mock.cleanup) != 1 {
				t.Error("should have one cleanup function registered")
			}
		})
		t.Run("Log", func(t *testing.T) {
			mock := newMock()
			Log(mock, "hello world")
			if len(mock.logs) != 0 {
				t.Fatal("should not have logged")
			}
			mock.shouldFail = true
			Log(mock, "hello world")
			mock.cleanup[0]()
			if len(mock.logs) != 1 {
				t.Fatal("should have logged", len(mock.logs))
			}
		})
		t.Run("Logf", func(t *testing.T) {
			mock := newMock()
			Logf(mock, "hello world: %d", 42)
			if len(mock.logs) != 0 {
				t.Fatal("should not have logged")
			}
			mock.shouldFail = true
			Logf(mock, "hello world: %d", 42)
			mock.cleanup[0]()
			if len(mock.logs) != 1 {
				t.Fatal("should have logged", len(mock.logs))
			}
		})
	})
	t.Run("Concrete", func(t *testing.T) {
		t.Run("Timer", func(t *testing.T) {
			var timer *time.Timer
			t.Run("Example", func(t *testing.T) {
				timer = Timer(t, 10*time.Millisecond)
			})
			if timer.Stop() {
				t.Error("should already be stopped")
			}
		})
		t.Run("Ticker", func(t *testing.T) {
			var ticker *time.Ticker
			start := time.Now()
			t.Run("Example", func(t *testing.T) {
				ticker = Ticker(t, 10*time.Millisecond)
			})
			// shoud
			ticker.Stop()
			if time.Since(start) >= 10*time.Millisecond {
				t.Fatal("should not have waited this long")
			}
		})
	})
	t.Run("Must", func(t *testing.T) {
		if val := Must(strconv.Atoi("100"))(t); val != 100 {
			t.Error(val, "is not", 100)
		}
		mock := newMock()
		mock.shouldFail = true

		if val := Must(strconv.Atoi("zzzzz"))(mock); val != 0 {
			t.Error(val, "is not", 0)
		}
	})
}

func TestWithJustifiedParallelism(t *testing.T) {
	t.Run("Reason", func(t *testing.T) {
		ran := false
		t.Run("sub", func(t *testing.T) {
			WithJustifiedParallelism(t, "sleeps to observe expiration")
			ran = true
		})
		// a parallel subtest only resumes after its parent function
		// returns, so ran is checked from a cleanup.
		t.Cleanup(func() {
			if !ran {
				t.Error("parallel subtest never resumed")
			}
		})
	})
	t.Run("EmptyReason", func(t *testing.T) {
		inner := &testing.T{}
		done := make(chan struct{})
		go func() {
			defer close(done)
			WithJustifiedParallelism(inner, "")
			t.Error("unreachable: Fatal should have stopped the goroutine")
		}()
		<-done
		if !inner.Failed() {
			t.Error("empty reason should fail the test")
		}
	})
}

func TestGoroutines(t *testing.T) {
	t.Run("AlreadyAtBaseline", func(t *testing.T) {
		mock := newMock()
		GoroutinesAtMost(mock, runtime.NumGoroutine(), time.Second)
		if mock.fatalCalled {
			t.Error("should not fail")
		}
	})
	t.Run("ReturnsToBaseline", func(t *testing.T) {
		mock := newMock()
		check := NoGoroutineLeak(mock, 10*time.Second)
		stop := make(chan struct{})
		go func() { <-stop }()
		time.AfterFunc(20*time.Millisecond, func() { close(stop) })
		check()
		if mock.fatalCalled {
			t.Error("goroutine exited, should not fail")
		}
	})
	t.Run("Leak", func(t *testing.T) {
		mock := newMock()
		check := NoGoroutineLeak(mock, 20*time.Millisecond)
		stop := make(chan struct{})
		defer close(stop)
		go func() { <-stop }()
		check()
		if !mock.fatalCalled {
			t.Error("leaked goroutine should fail")
		}
	})
}
