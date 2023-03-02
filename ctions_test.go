package fun

import "testing"

func makeSliceOfAllN(n int, size int) []int {
	out := make([]int, size)
	for i := 0; i < size; i++ {
		out[i] = n
	}
	return out
}

func TestProcess(t *testing.T) {
	t.Run("One", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			sum := 0
			ProcessAll(func(in int) { sum += in }, makeSliceOfAllN(1, 100))
			if sum != 100 {
				t.Error(sum)
			}
		})
		t.Run("Variadic", func(t *testing.T) {
			sum := 0
			Process(func(in int) { sum += in }, makeSliceOfAllN(1, 100)...)
			if sum != 100 {
				t.Error(sum)
			}
		})
	})
	t.Run("Two", func(t *testing.T) {
		t.Run("Slice", func(t *testing.T) {
			sum := 0
			ProcessAll(func(in int) { sum += in }, makeSliceOfAllN(2, 128))
			if sum != 256 {
				t.Error(sum)
			}
		})
		t.Run("Variadic", func(t *testing.T) {
			sum := 0
			Process(func(in int) { sum += in }, makeSliceOfAllN(2, 128)...)
			if sum != 256 {
				t.Error(sum)
			}
		})
	})
}
