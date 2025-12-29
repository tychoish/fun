package irt

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// newGeneratorFactory creates a function that, when called, returns a new generator
// closure over the provided data slice. This is used to ensure that each
// benchmark iteration gets a fresh, non-exhausted generator.
func newGeneratorFactory[T any](data []T) func() func() (T, bool) {
	return func() func() (T, bool) {
		idx := 0
		return func() (T, bool) {
			if idx >= len(data) {
				var zero T
				return zero, false
			}
			val := data[idx]
			idx++
			return val, true
		}
	}
}

// BenchmarkGenerateOk compares the performance of GenerateOk with a version
// implemented using GenerateWhile2 and isOk.
func BenchmarkGenerateOk(b *testing.B) {
	for _, size := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("Ints-Size-%d", size), func(b *testing.B) {
			data := make([]int, size)
			for i := 0; i < size; i++ {
				data[i] = rand.Int()
			}
			getGen := newGeneratorFactory(data)

			b.Run("Current", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					for range GenerateOk(getGen()) {
					}
				}
			})

			b.Run("Alternative", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					for range First(GenerateWhile2(getGen(), isOk)) {
					}
				}
			})
		})

		b.Run(fmt.Sprintf("Strings-Size-%d", size), func(b *testing.B) {
			data := make([]string, size)
			for i := 0; i < size; i++ {
				data[i] = fmt.Sprintf("string-%d", rand.Int())
			}
			getGen := newGeneratorFactory(data)

			b.Run("Current", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					for range GenerateOk(getGen()) {
					}
				}
			})

			b.Run("Alternative", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					for range First(GenerateWhile2(getGen(), isOk)) {
					}
				}
			})
		})
	}
}

// BenchmarkApply2 compares the performance of Apply2 with a version
// implemented using Elems and elemApply.
func BenchmarkApply2(b *testing.B) {
	for _, size := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("Ints-Size-%d", size), func(b *testing.B) {
			data := make(map[int]int, size)
			for i := 0; i < size; i++ {
				data[i] = rand.Int()
			}

			b.Run("Current", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					Apply2(Map(data), func(k, v int) {})
				}
			})

			b.Run("Alternative", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					Apply(Elems(Map(data)), elemApply(func(k, v int) {}))
				}
			})
		})

		b.Run(fmt.Sprintf("Strings-Size-%d", size), func(b *testing.B) {
			data := make(map[string]string, size)
			for i := 0; i < size; i++ {
				key := fmt.Sprintf("key-%d", i)
				data[key] = fmt.Sprintf("string-%d", rand.Int())
			}

			b.Run("Current", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					Apply2(Map(data), func(k, v string) {})
				}
			})

			b.Run("Alternative", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					Apply(Elems(Map(data)), elemApply(func(k, v string) {}))
				}
			})
		})
	}
}

// BenchmarkIdxorz compares the performance of the current idxorz implementation
// with the commented-out alternative.
func BenchmarkIdxorz(b *testing.B) {
	for _, size := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			data := make([]int, size)
			for i := 0; i < size; i++ {
				data[i] = rand.Int()
			}

			b.Run("Current", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = idxorz(data, i%size)
				}
			})

			b.Run("Alternative", func(b *testing.B) {
				idxorzAlternative := func(sl []int, idx int) int {
					return ifdoelsedo(idx >= len(sl) || idx < 0, zero[int], idxlazy(sl, idx))
				}
				for i := 0; i < b.N; i++ {
					_ = idxorzAlternative(data, i%size)
				}
			})
		})
	}
}
