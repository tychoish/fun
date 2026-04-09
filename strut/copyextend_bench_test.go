package strut

import (
	"slices"
	"testing"
)

// payloads used across all sub-benchmarks.
var (
	smallPayload = []byte("hello")
	largePayload = []byte("the quick brown fox jumps over the lazy dog and keeps on going past the end")
)

// BenchmarkCopyExtendVsAppend compares a single extend operation
// with and without sufficient pre-allocated capacity.
func BenchmarkCopyExtendVsAppend(b *testing.B) {
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"small", smallPayload},
		{"large", largePayload},
	} {
		b.Run(tc.name+"/preallocated/copyExtend", func(b *testing.B) {
			for range b.N {
				mut := make(Mutable, 0, len(tc.payload))
				mut.copyExtend(tc.payload)
			}
		})
		b.Run(tc.name+"/preallocated/append", func(b *testing.B) {
			for range b.N {
				mut := make(Mutable, 0, len(tc.payload))
				mut = append(mut, tc.payload...)
				_ = mut
			}
		})
		b.Run(tc.name+"/no-capacity/copyExtend", func(b *testing.B) {
			for range b.N {
				var mut Mutable
				mut.copyExtend(tc.payload)
			}
		})
		b.Run(tc.name+"/no-capacity/append", func(b *testing.B) {
			for range b.N {
				var mut Mutable
				mut = append(mut, tc.payload...)
			}
		})
	}
}

// BenchmarkCopyExtendRepeatedVsAppend simulates the Extend* use case:
// appending many chunks sequentially into a single buffer.
func BenchmarkCopyExtendRepeatedVsAppend(b *testing.B) {
	chunks := [][]byte{
		[]byte("alpha"), []byte("bravo"), []byte("charlie"),
		[]byte("delta"), []byte("echo"), []byte("foxtrot"),
		[]byte("golf"), []byte("hotel"), []byte("india"), []byte("juliet"),
	}
	totalLen := 0
	for _, c := range chunks {
		totalLen += len(c)
	}

	b.Run("preallocated/copyExtend", func(b *testing.B) {
		for range b.N {
			mut := make(Mutable, 0, totalLen)
			for _, c := range chunks {
				mut.copyExtend(c)
			}
		}
	})
	b.Run("preallocated/append", func(b *testing.B) {
		for range b.N {
			mut := make(Mutable, 0, totalLen)
			for _, c := range chunks {
				mut = append(mut, c...)
			}
		}
	})
	b.Run("no-capacity/copyExtend", func(b *testing.B) {
		for range b.N {
			var mut Mutable
			for _, c := range chunks {
				mut.copyExtend(c)
			}
		}
	})
	b.Run("no-capacity/append", func(b *testing.B) {
		for range b.N {
			var mut Mutable
			for _, c := range chunks {
				mut = append(mut, c...)
			}
		}
	})
	b.Run("pooled-preallocated/copyExtend", func(b *testing.B) {
		for range b.N {
			mut := MakeMutable(totalLen)
			for _, c := range chunks {
				mut.copyExtend(c)
			}
			mut.Release()
		}
	})
	b.Run("pooled-preallocated/append", func(b *testing.B) {
		for range b.N {
			mut := MakeMutable(totalLen)
			for _, c := range chunks {
				*mut = append(*mut, c...)
			}
			mut.Release()
		}
	})
}

// BenchmarkCopyExtendStringVsAppend compares string-source appends.
func BenchmarkCopyExtendStringVsAppend(b *testing.B) {
	strs := slices.Collect(func(yield func(string) bool) {
		for _, s := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
			if !yield(s) {
				return
			}
		}
	})
	totalLen := 0
	for _, s := range strs {
		totalLen += len(s)
	}

	b.Run("preallocated/copyExtendString", func(b *testing.B) {
		for range b.N {
			mut := make(Mutable, 0, totalLen)
			for _, s := range strs {
				mut.copyExtendString(s)
			}
		}
	})
	b.Run("preallocated/append", func(b *testing.B) {
		for range b.N {
			mut := make(Mutable, 0, totalLen)
			for _, s := range strs {
				mut = append(mut, s...)
			}
		}
	})
	b.Run("no-capacity/copyExtendString", func(b *testing.B) {
		for range b.N {
			var mut Mutable
			for _, s := range strs {
				mut.copyExtendString(s)
			}
		}
	})
	b.Run("no-capacity/append", func(b *testing.B) {
		for range b.N {
			var mut Mutable
			for _, s := range strs {
				mut = append(mut, s...)
			}
		}
	})
}
