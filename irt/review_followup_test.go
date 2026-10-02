package irt

import (
	"runtime"
	"testing"
)

func TestShardOuterBreakReleases(t *testing.T) {
	for _, issued := range []int{0, 1, 2} {
		base := runtime.NumGoroutine()
		for range 10 {
			n := 0
			for sh := range Shard(t.Context(), 3, Monotonic()) {
				if n++; n > issued {
					break // handed out but never consumed
				}
				for range sh {
					break
				}
			}
		}
		goroutinesAtMost(t, base)
	}
}

func TestWithBufferSourcePanicReachesConsumer(t *testing.T) {
	src := func(yield func(int) bool) {
		yield(1)
		panic("boom")
	}
	var got []int
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Fatalf("recovered %v, want boom", r)
			}
		}()
		for v := range WithBuffer(t.Context(), src, 1) {
			got = append(got, v)
		}
		t.Fatal("expected panic")
	}()
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}
