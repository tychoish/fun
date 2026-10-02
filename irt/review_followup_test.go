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
