package strut

import (
	"bytes"
	"slices"
	"testing"
)

// Benchmark ASCII case conversion (optimized path).
func BenchmarkToLowerASCII(b *testing.B) {
	data := []byte("HELLO WORLD THIS IS A TEST STRING WITH ONLY ASCII CHARACTERS")
	b.Run("Mutable", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(slices.Clone(data))
			mut.ToLower()
		}
	})
	b.Run("bytes.ToLower", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.ToLower(data)
		}
	})
}

func BenchmarkToUpperASCII(b *testing.B) {
	data := []byte("hello world this is a test string with only ascii characters")
	b.Run("Mutable", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(slices.Clone(data))
			mut.ToUpper()
		}
	})
	b.Run("bytes.ToUpper", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.ToUpper(data)
		}
	})
}

// Benchmark Unicode case conversion (fallback path).
func BenchmarkToLowerUnicode(b *testing.B) {
	data := []byte("HELLO 世界 ТЕСТ МИРА")
	b.Run("Mutable", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(slices.Clone(data))
			mut.ToLower()
		}
	})
	b.Run("bytes.ToLower", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.ToLower(data)
		}
	})
}

// Benchmark capacity-aware Repeat.
func BenchmarkRepeatWithCapacity(b *testing.B) {
	data := []byte("test")
	b.Run("Mutable-with-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := make(Mutable, len(data), len(data)*10)
			copy(mut, data)
			mut.Repeat(10)
		}
	})
	b.Run("Mutable-without-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(slices.Clone(data))
			mut.Repeat(10)
		}
	})
	b.Run("bytes.Repeat", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.Repeat(data, 10)
		}
	})
}

// Benchmark capacity-aware Replace.
func BenchmarkReplaceSameLength(b *testing.B) {
	data := []byte("foo bar foo baz foo qux")
	old := []byte("foo")
	new := []byte("bar") //nolint:predeclared
	b.Run("Mutable-with-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := make(Mutable, len(data), len(data)+100)
			copy(mut, data)
			mut.ReplaceAll(old, new)
		}
	})
	b.Run("Mutable-without-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(slices.Clone(data))
			mut.ReplaceAll(old, new)
		}
	})
	b.Run("bytes.ReplaceAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.ReplaceAll(data, old, new)
		}
	})
}

func BenchmarkReplaceShrinking(b *testing.B) {
	data := []byte("foobar foobar foobar foobar") //nolint:dupword
	old := []byte("foobar")
	new := []byte("baz") //nolint:predeclared
	b.Run("Mutable-with-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := make(Mutable, len(data), len(data)+100)
			copy(mut, data)
			mut.ReplaceAll(old, new)
		}
	})
	b.Run("bytes.ReplaceAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.ReplaceAll(data, old, new)
		}
	})
}

func BenchmarkReplaceGrowing(b *testing.B) {
	data := []byte("foo foo foo foo") //nolint:dupword
	old := []byte("foo")
	new := []byte("foobar") //nolint:predeclared
	b.Run("Mutable-with-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			newLen := len(data) + 3*len("bar") // 3 replacements
			mut := make(Mutable, len(data), newLen)
			copy(mut, data)
			mut.ReplaceAll(old, new)
		}
	})
	b.Run("Mutable-without-capacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(slices.Clone(data))
			mut.ReplaceAll(old, new)
		}
	})
	b.Run("bytes.ReplaceAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.ReplaceAll(data, old, new)
		}
	})
}

// Benchmark iterator-based Split vs bytes.Split.
func BenchmarkSplit(b *testing.B) {
	data := []byte("a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p")
	sep := []byte(",")
	b.Run("Mutable-iterator", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(data)
			for range mut.Split(sep) {
			}
		}
	})
	b.Run("Mutable-collect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(data)
			_ = slices.Collect(mut.Split(sep))
		}
	})
	b.Run("bytes.Split", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.Split(data, sep)
		}
	})
}

func BenchmarkFields(b *testing.B) {
	data := []byte("hello world this is a test string with many fields")
	b.Run("Mutable-iterator", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(data)
			for range mut.Fields() {
			}
		}
	})
	b.Run("Mutable-collect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := Mutable(data)
			_ = slices.Collect(mut.Fields())
		}
	})
	b.Run("bytes.Fields", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = bytes.Fields(data)
		}
	})
}

// Benchmark pooling vs allocation.
func BenchmarkPooling(b *testing.B) {
	b.Run("NewMutable-with-pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := NewMutable()
			mut.WriteString("test data")
			mut.Release()
		}
	})
	b.Run("make-without-pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := make(Mutable, 0, 32)
			mut = append(mut, "test data"...)
			_ = mut
		}
	})
}

// Benchmark Grow operation.
func BenchmarkGrow(b *testing.B) {
	b.Run("Grow-preallocate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := NewMutable()
			mut.Grow(1000)
			for range 100 {
				mut.WriteString("test")
			}
			mut.Release()
		}
	})
	b.Run("no-Grow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mut := NewMutable()
			for range 100 {
				mut.WriteString("test")
			}
			mut.Release()
		}
	})
}
