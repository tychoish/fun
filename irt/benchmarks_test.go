package irt

import (
	"fmt"
	"iter"
	"math/rand/v2"
	"testing"
)

// BenchmarkIdxorz compares the performance of the current idxorz implementation
// with the commented-out alternative.
func BenchmarkIdxorz(b *testing.B) {
	for _, size := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			data := make([]int, size)
			for i := 0; i < size; i++ {
				data[i] = rand.Int()
			}

			b.Run("Manual", func(b *testing.B) {
				impl := func(sl []int, idx int) int {
					if idx >= 0 && idx < len(sl) {
						return sl[idx]
					}
					return 0
				}
				for i := 0; i < b.N; i++ {
					_ = impl(data, i%size)
				}
			})

			b.Run("WithPredicate", func(b *testing.B) {
				impl := func(sl []int, idx int) int {
					if isWithin(idx, len(sl)) {
						return sl[idx]
					}
					return 0
				}
				for i := 0; i < b.N; i++ {
					_ = impl(data, i%size)
				}
			})

			b.Run("IdxOrZ", func(b *testing.B) {
				impl := func(sl []int, idx int) int {
					return idxorz(sl, idx)
				}
				for i := 0; i < b.N; i++ {
					_ = impl(data, i%size)
				}
			})

			b.Run("WhenDo", func(b *testing.B) {
				impl := func(sl []int, idx int) int {
					return whendowith(isWithin(idx, len(sl)), idxorzfn(sl), idx)
				}
				for i := 0; i < b.N; i++ {
					_ = impl(data, i%size)
				}
			})

			b.Run("IfDo", func(b *testing.B) {
				impl := func(sl []int, idx int) int {
					return ifdoelsedo(isWithin(idx, len(sl)), zero[int], idxfn(sl, idx))
				}
				for i := 0; i < b.N; i++ {
					_ = impl(data, i%size)
				}
			})
		})
	}
}

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
			mdata := make(map[int]int, size)
			for i := 0; i < size; i++ {
				mdata[i] = rand.Int()
			}
			for _, op := range []func() (string, iter.Seq2[int, int]){
				func() (string, iter.Seq2[int, int]) { return "Map", Map(mdata) },
				func() (string, iter.Seq2[int, int]) {
					return "Elems", KVsplit(Slice(Collect(KVjoin(Map(mdata)))))
				},
			} {
				name, _ := op()
				b.Run(name, func(b *testing.B) {
					_, data := op()
					b.Run("Current", func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							Apply2(data, func(k, v int) {})
						}
					})
					_, data = op()

					b.Run("Alternative", func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							Apply(KVjoin(data), elemApply(func(k, v int) {}))
						}
					})
				})
			}
		})

		b.Run(fmt.Sprintf("Strings-Size-%d", size), func(b *testing.B) {
			mdata := make(map[string]string, size)
			for i := 0; i < size; i++ {
				key := fmt.Sprintf("key-%d", i)
				mdata[key] = fmt.Sprintf("string-%d", rand.Int())
			}

			for _, op := range []func() (string, iter.Seq2[string, string]){
				func() (string, iter.Seq2[string, string]) { return "Map", Map(mdata) },
				func() (string, iter.Seq2[string, string]) {
					return "Elems", KVsplit(Slice(Collect(KVjoin(Map(mdata)))))
				},
			} {
				name, _ := op()
				b.Run(name, func(b *testing.B) {
					_, data := op()
					b.Run("Current", func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							Apply2(data, func(k, v string) {})
						}
					})
					_, data = op()

					b.Run("Alternative", func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							Apply(KVjoin(data), elemApply(func(k, v string) {}))
						}
					})
				})
			}
		})
	}
}
