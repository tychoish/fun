package internal

import (
	"testing"
	"time"
)

func TestLimitingResolvers(t *testing.T) {
	t.Run("TTL", func(t *testing.T) {
		t.Run("Checking", func(t *testing.T) {
			t.Run("Negative", func(t *testing.T) {
				_, err := TTLExec[bool](-11)
				if err == nil {
					t.Error("exepcted error for nil values")
				}

			})
			t.Run("Zero is Noop", func(t *testing.T) {
				resolver, err := TTLExec[bool](0)
				if err != nil {
					t.Error("exepcted no error for empty values")
				}
				count := 0
				for range 100 {
					if out := resolver(func() bool { count++; return true }); !out {
						t.Error("should have returned reasonable value")
					}
				}
				if count != 100 {
					t.Error("expected passthrough")
				}
			})
		})
		t.Run("Under", func(t *testing.T) {
			count := 0
			resolver, err := TTLExec[int](time.Minute)
			if err != nil {
				panic(err)
			}
			for range 500 {
				if out := resolver(func() int { count++; return 42 }); out != 42 {
					t.Fatal("expected 42, got", out)
				}
			}
			if count != 1 {
				t.Error("expected function to run only once:", count)
			}
		})
		t.Run("Par", func(t *testing.T) {
			count := 0
			resolver, err := TTLExec[int](time.Nanosecond)
			if err != nil {
				panic(err)
			}
			for range 500 {
				if out := resolver(func() int { count++; return 42 }); out != 42 {
					t.Fatal("expected 42, got", out)
				}
			}
			if count != 500 {
				t.Error("expected function to run everytime:", count)
			}
		})
		t.Run("Over", func(t *testing.T) {
			count := 0
			resolver, err := TTLExec[int](50 * time.Millisecond)
			if err != nil {
				panic(err)
			}
			for range 500 {
				time.Sleep(30 * time.Millisecond)
				if out := resolver(func() int { count++; return 42 }); out != 42 {
					t.Fatal("expected 42, got", out)
				}
			}
			if count < 249 && count > 251 {
				t.Error("expected function to run half the time:", count)
			}
		})
	})
	t.Run("RunCount", func(t *testing.T) {
		t.Run("ErrorCheck", func(t *testing.T) {
			_, err := LimitExec[int](0)
			if err == nil {
				t.Error("expected error")
			}
			_, err = LimitExec[int](-1)
			if err == nil {
				t.Error("expected error")
			}
		})
		t.Run("Basic", func(t *testing.T) {
			resolver, err := LimitExec[int](1)
			if err != nil {
				t.Error(err)
			}
			count := 0
			for range 300 {
				if out := resolver(func() int { count++; return 42 }); out != 42 {
					t.Fatal("expected 42, got", out)
				}
			}
			if count != 1 {
				t.Error("expected only to run once, got:", count)
			}
		})
		t.Run("Basic", func(t *testing.T) {
			resolver, err := LimitExec[int](128)
			if err != nil {
				t.Error(err)
			}
			count := 0
			for range 300 {
				if out := resolver(func() int { count++; return 42 }); out != 42 {
					t.Fatal("expected 42, got", out)
				}
			}
			if count != 128 {
				t.Error("expected to run 128 times, got:", count)
			}
		})
	})
}
